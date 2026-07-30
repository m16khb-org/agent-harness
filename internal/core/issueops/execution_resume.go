package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

type ExecutionResumeRequest struct {
	ID                 string            `json:"id"`
	ExpectedGeneration uint64            `json:"expected_generation"`
	Actor              model.NativeActor `json:"actor"`
	CWD                string            `json:"cwd"`
	Confirm            bool              `json:"confirm"`
}

type ExecutionResumeDependencies struct {
	Orca      port.ExecutionOrcaProvisioner
	OrcaOwner port.ExecutionOrcaOwnerInspector
	Now       func() time.Time
}

type ExecutionResumeResult struct {
	OK                  bool            `json:"ok"`
	ID                  string          `json:"id"`
	Execution           model.Execution `json:"execution"`
	ClaimTokenPath      string          `json:"claim_token_path"`
	IssueBodySHA256     string          `json:"issue_body_sha256"`
	ContextPacketPath   string          `json:"context_packet_path"`
	ContextPacketSHA256 string          `json:"context_packet_sha256"`
	OwnerPromptPath     string          `json:"owner_prompt_path"`
	OwnerPromptSHA256   string          `json:"owner_prompt_sha256"`
	NextCommand         string          `json:"next_command"`
}

type executionResumeArtifacts struct {
	claimTokenPath  string
	issueBodySHA256 string
	packetPath      string
	packetSHA256    string
	promptPath      string
	promptSHA256    string
}

func ResumeExecutionWithDependencies(ctx context.Context, stateRoot string, req ExecutionResumeRequest, deps ExecutionResumeDependencies) (ExecutionResumeResult, error) {
	if !req.Confirm {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires confirm")
	}
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	if _, err := normalizeNativeActor(req.Actor); err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	record, err := executionRecordAtGeneration(stateRoot, req.ID, req.ExpectedGeneration)
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	if record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Orca == nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires an existing Orca binding")
	}
	if record.Execution.Pending != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume is blocked by a pending external intent; run execution reconcile")
	}
	if record.Execution.Lease.Status != model.LeaseStatusClaimable ||
		record.Execution.Lease.Holder != nil ||
		!executionSHA256.MatchString(record.Execution.Lease.ClaimTokenSHA256) {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires a holderless claimable lease")
	}
	if !samePath(req.CWD, record.Execution.Workspace.Root) {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume cwd must be the canonical worktree")
	}
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	if deps.Orca == nil || deps.OrcaOwner == nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume requires Orca mutation and owner inventory adapters")
	}
	binding := record.Execution.Orca
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID, AllowRuntimeRollover: true,
	})
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("inspect previous Orca owner: %w", err)
	}
	if err := validateExecutionRuntimeRollover(record, inventory); err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	sameGeneration := binding.LeaseGeneration == record.Execution.Lease.Generation
	if inventory.TaskLive && !inventory.TerminalLive {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("Orca owner inventory has a live task without a live terminal")
	}
	if inventory.TaskLive {
		if !sameGeneration {
			return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("previous Orca owner task is still live")
		}
		return executionResumeResult(record, artifacts), nil
	}
	reusedTerminalPTYID := ""
	if inventory.TerminalLive {
		reusedTerminalPTYID = strings.TrimSpace(inventory.TerminalID)
		if reusedTerminalPTYID == "" || reusedTerminalPTYID != strings.TrimSpace(binding.TerminalPTYID) {
			return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("Orca owner terminal identity changed")
		}
	}

	runtimeID := strings.TrimSpace(inventory.RuntimeID)
	if runtimeID == "" {
		runtimeID = binding.RuntimeID
	}
	persisted, payload, err := beginOrcaExecutionResumeIntent(stateRoot, record, artifacts, runtimeID, reusedTerminalPTYID, deps.Now)
	if err != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, err
	}
	for attempt := 0; attempt < 3 && persisted.Execution.Pending != nil; attempt++ {
		persisted, payload, err = executeOrcaIntentStage(ctx, stateRoot, persisted, payload, deps.Orca, nil, deps.Now)
		if err != nil {
			return ExecutionResumeResult{OK: false, ID: req.ID}, err
		}
	}
	if persisted.Execution.Pending != nil {
		return ExecutionResumeResult{OK: false, ID: req.ID}, fmt.Errorf("execution resume did not complete the owner launch stages")
	}
	return executionResumeResult(persisted, artifacts), nil
}

func readExecutionResumeArtifacts(record IssueOpsRecord) (executionResumeArtifacts, error) {
	tokenPath := claimTokenPath(record)
	token, err := readExecutionResumeClaimToken(record, tokenPath)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("read current generation claim token: %w", err)
	}
	if tokenSHA256(token) != record.Execution.Lease.ClaimTokenSHA256 {
		return executionResumeArtifacts{}, fmt.Errorf("current generation claim token identity changed")
	}
	packetPath, promptPath := executionOwnerArtifactPaths(record)
	packetData, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("read sealed context packet: %w", err)
	}
	packetSHA256 := digestExecutionOwnerBytes(packetData)
	var packet executionOwnerContextPacket
	if err := json.Unmarshal(packetData, &packet); err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("parse sealed context packet: %w", err)
	}
	issueBodySHA256 := strings.TrimSpace(packet.Issue.BodySHA256)
	if err := validateExecutionResumePacket(record, issueBodySHA256, packetSHA256); err != nil {
		return executionResumeArtifacts{}, err
	}
	binding := record.Execution.Orca
	if binding == nil || packet.OwnerHost != binding.OwnerHost || packet.OwnerModel != binding.OwnerModel || packet.OwnerEffort != binding.OwnerEffort {
		return executionResumeArtifacts{}, fmt.Errorf("sealed owner profile does not match the Orca binding")
	}
	expectedPrompt, err := renderExecutionOwnerPrompt(packet, packetPath, packetSHA256)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("render sealed owner prompt: %w", err)
	}
	promptData, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, promptPath)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("read sealed owner prompt: %w", err)
	}
	if string(promptData) != expectedPrompt {
		return executionResumeArtifacts{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	return executionResumeArtifacts{
		claimTokenPath: tokenPath, issueBodySHA256: issueBodySHA256,
		packetPath: packetPath, packetSHA256: packetSHA256,
		promptPath: promptPath, promptSHA256: digestExecutionOwnerBytes(promptData),
	}, nil
}

// readExecutionResumeClaimToken은 owner를 다시 띄우기 전에 현재 generation의
// 비공개 token이 그대로 남아 있는지만 확인한다. Claim mutation은 application
// vertical이 소유하며, 이 함수는 파일을 소비하거나 상태를 바꾸지 않는다.
func readExecutionResumeClaimToken(record IssueOpsRecord, path string) (string, error) {
	expected := claimTokenPath(record)
	if !samePath(path, expected) {
		return "", fmt.Errorf("claim_token_file must be the deterministic current-generation path")
	}
	data, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, expected)
	if err != nil {
		return "", err
	}
	if len(data) > 256 {
		return "", fmt.Errorf("claim token file is oversized")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("claim token file is empty")
	}
	return token, nil
}

// validateExecutionResumePacket은 이미 claim vertical이 봉인한 packet을 owner
// 재기동 전에 다시 읽어 generation과 artifact digest가 변하지 않았는지 검증한다.
// mutation 규칙을 복제하지 않고 resume의 read-only 사전조건만 확인한다.
func validateExecutionResumePacket(record IssueOpsRecord, issueDigest, packetDigest string) error {
	if record.Execution == nil || record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Lease.Generation == 0 {
		return fmt.Errorf("sealed owner context no longer matches an Orca execution generation")
	}
	generation := record.Execution.Lease.Generation
	packetPath, _ := executionOwnerArtifactPaths(record)
	data, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if observed := digestExecutionOwnerBytes(data); observed != packetDigest {
		return fmt.Errorf("sealed context packet digest mismatch: expected=%s observed=%s path=%s", packetDigest, observed, packetPath)
	}
	var packet executionOwnerContextPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	if packet.SchemaVersion != model.IssueOpsSchemaVersion || packet.LifecycleID != record.ID || packet.Mode != record.Execution.Mode ||
		!samePath(packet.SourceRoot, record.Execution.Workspace.SourceRoot) || !samePath(packet.WorktreeRoot, record.Execution.Workspace.Root) ||
		packet.Branch != record.Execution.Workspace.Branch || packet.BaseHead != record.Execution.Workspace.BaseHead ||
		packet.LeaseGeneration != generation || packet.ClaimTokenFile != claimTokenPath(record) || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch: packet_generation=%d expected_generation=%d", packet.LeaseGeneration, generation)
	}
	if packet.Issue.BodySHA256 != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch: expected=%s observed=%s", issueDigest, packet.Issue.BodySHA256)
	}
	if observed := digestExecutionOwnerBytes([]byte(packet.Issue.Body)); observed != issueDigest {
		return fmt.Errorf("sealed context packet issue body does not hash to its sealed digest: expected=%s observed=%s", issueDigest, observed)
	}
	for name, digest := range packet.ArtifactManifest {
		path := filepath.Join(record.Execution.Workspace.Root, filepath.FromSlash(IssueOpsArtifactDir), name+".md")
		artifact, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, path)
		if err != nil {
			return fmt.Errorf("read sealed artifact %s: %w", name, err)
		}
		if digestExecutionOwnerBytes(artifact) != digest {
			return fmt.Errorf("sealed artifact %s digest mismatch", name)
		}
	}
	return nil
}

func beginOrcaExecutionResumeIntent(stateRoot string, record IssueOpsRecord, artifacts executionResumeArtifacts, runtimeID, terminalPTYID string, now func() time.Time) (IssueOpsRecord, externalOrcaIntentPayload, error) {
	operationID, err := newExecutionOperationID()
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	if !samePath(workspace.Root, record.Execution.Workspace.Root) {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, fmt.Errorf("execution resume workspace is not canonical")
	}
	startedAt := executionNow(now)
	binding := *record.Execution.Orca
	lease := record.Execution.Lease
	prepared := &port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: record.Execution.Workspace.SourceRoot, Root: record.Execution.Workspace.Root,
			Branch: record.Execution.Workspace.Branch, BaseHead: record.Execution.Workspace.BaseHead,
			ParentWorktree: record.Execution.Workspace.ParentWorktree, Driver: "orca", Exists: true,
		},
		RuntimeID: runtimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
		WorktreeInstanceID: binding.WorktreeInstanceID,
	}
	probe := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: binding.OwnerHost, Model: binding.OwnerModel,
		Effort: binding.OwnerEffort,
	}
	stage := port.ExecutionOrcaIntentTerminal
	if terminalPTYID != "" {
		stage = port.ExecutionOrcaIntentTask
	}
	payload := externalOrcaIntentPayload{
		SchemaVersion: model.IssueOpsSchemaVersion, Purpose: orcaIntentPurposeResume,
		OperationID: operationID, LifecycleID: record.ID, Generation: lease.Generation,
		Stage: stage, StartedAt: startedAt,
		InvocationState: orcaIntentNotInvoked, Workspace: workspace, Probe: probe, Prepared: prepared,
		Launch: &externalOrcaLaunchIdentity{
			PromptPath: artifacts.promptPath, PromptSHA256: artifacts.promptSHA256,
			ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		},
		IssueBodySHA256: artifacts.issueBodySHA256, ClaimTokenSHA256: lease.ClaimTokenSHA256,
		TerminalPTYID: strings.TrimSpace(terminalPTYID),
		PriorBinding:  &binding, ResumeLease: &lease,
	}
	payload, err = sealExternalOrcaIntentPayload(record, payload)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		current, err := executionRecordAtGeneration(stateRoot, record.ID, lease.Generation)
		if err != nil {
			return err
		}
		if current.Execution.Pending != nil ||
			!reflect.DeepEqual(current.Execution.Lease, lease) ||
			!reflect.DeepEqual(current.Execution.Orca, &binding) {
			return fmt.Errorf("execution resume authority changed before intent persistence")
		}
		if err := validateOrcaIntentRecordIdentity(current, payload); err != nil {
			return err
		}
		current.Execution.Pending = &model.ExternalIntent{
			OperationID: operationID, Kind: pendingKindForOrcaStage(payload.Stage),
			Marker: payload.Marker, StartedAt: startedAt,
		}
		current.Execution.Failure = nil
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []sqlstore.Mutation{{
			Bucket: externalIntentBucket, ID: operationID, Data: data, RequireAbsent: true,
		}})
		return err
	})
	return persisted, payload, err
}

func executionResumeResult(record IssueOpsRecord, artifacts executionResumeArtifacts) ExecutionResumeResult {
	generation := record.Execution.Lease.Generation
	next := "agent-harness issueops execution claim --id " + quoteExecutionOwnerArg(record.ID) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-token-file " + quoteExecutionOwnerArg(artifacts.claimTokenPath) +
		" --issue-body-sha256 " + artifacts.issueBodySHA256 +
		" --context-packet-sha256 " + artifacts.packetSHA256
	return ExecutionResumeResult{
		OK: true, ID: record.ID, Execution: *record.Execution,
		ClaimTokenPath: artifacts.claimTokenPath, IssueBodySHA256: artifacts.issueBodySHA256,
		ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		OwnerPromptPath: artifacts.promptPath, OwnerPromptSHA256: artifacts.promptSHA256,
		NextCommand: next,
	}
}

func executionResumeCommand(id string, generation uint64) string {
	return "agent-harness issueops execution resume --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --confirm"
}
