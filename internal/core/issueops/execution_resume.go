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

	"agent-harness/internal/contract/issueops"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/sqlstore"
)

type ExecutionResumeRequest struct {
	ID                 string               `json:"id"`
	ExpectedGeneration uint64               `json:"expected_generation"`
	Actor              issueops.NativeActor `json:"actor"`
	CWD                string               `json:"cwd"`
	Confirm            bool                 `json:"confirm"`
}

type ExecutionResumeResult struct {
	OK                  bool               `json:"ok"`
	ID                  string             `json:"id"`
	Execution           issueops.Execution `json:"execution"`
	ClaimTokenPath      string             `json:"claim_token_path"`
	IssueBodySHA256     string             `json:"issue_body_sha256"`
	ContextPacketPath   string             `json:"context_packet_path"`
	ContextPacketSHA256 string             `json:"context_packet_sha256"`
	OwnerPromptPath     string             `json:"owner_prompt_path"`
	OwnerPromptSHA256   string             `json:"owner_prompt_sha256"`
	NextCommand         string             `json:"next_command"`
}

type executionResumeArtifacts struct {
	claimTokenPath  string
	issueBodySHA256 string
	packetPath      string
	packetSHA256    string
	promptPath      string
	promptSHA256    string
}

func readExecutionResumeArtifacts(record issueops.IssueOpsRecord) (executionResumeArtifacts, error) {
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
func readExecutionResumeClaimToken(record issueops.IssueOpsRecord, path string) (string, error) {
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
func validateExecutionResumePacket(record issueops.IssueOpsRecord, issueDigest, packetDigest string) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Lease.Generation == 0 {
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
	if packet.SchemaVersion != issueops.IssueOpsSchemaVersion || packet.LifecycleID != record.ID || packet.Mode != record.Execution.Mode ||
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

func beginOrcaExecutionResumeIntent(stateRoot string, record issueops.IssueOpsRecord, artifacts executionResumeArtifacts, runtimeID, terminalPTYID string, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	operationID, err := newExecutionOperationID()
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	return beginOrcaExecutionResumeIntentWithID(stateRoot, record, artifacts, runtimeID, terminalPTYID, operationID, now)
}

func beginOrcaExecutionResumeIntentWithID(stateRoot string, record issueops.IssueOpsRecord, artifacts executionResumeArtifacts, runtimeID, terminalPTYID, operationID string, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	return beginOrcaExecutionResumeIntentWithExpectedRaw(stateRoot, record, nil, artifacts, runtimeID, terminalPTYID, operationID, now)
}

func beginOrcaExecutionResumeIntentWithExpectedRaw(stateRoot string, record issueops.IssueOpsRecord, expectedRecordRaw []byte, artifacts executionResumeArtifacts, runtimeID, terminalPTYID, operationID string, now func() time.Time) (issueops.IssueOpsRecord, externalOrcaIntentPayload, error) {
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	if !samePath(workspace.Root, record.Execution.Workspace.Root) {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, fmt.Errorf("execution resume workspace is not canonical")
	}
	startedAt := executionNow(now)
	binding := *record.Execution.Orca
	lease := record.Execution.Lease
	prepared := &preparationcontract.OrcaWorkspaceReceipt{
		Workspace: preparationcontract.WorkspaceReceipt{
			SourceRoot: record.Execution.Workspace.SourceRoot, Root: record.Execution.Workspace.Root,
			Branch: record.Execution.Workspace.Branch, BaseHead: record.Execution.Workspace.BaseHead,
			ParentWorktree: record.Execution.Workspace.ParentWorktree, Driver: "orca", Exists: true,
		},
		RuntimeID: runtimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
		WorktreeInstanceID: binding.WorktreeInstanceID,
	}
	probe := preparationcontract.ProbeRequest{
		Repo: record.Repo, Host: binding.OwnerHost, Model: binding.OwnerModel,
		Effort: binding.OwnerEffort,
	}
	stage := preparationcontract.IntentStageTerminal
	if terminalPTYID != "" {
		stage = preparationcontract.IntentStageRun
	}
	priorBinding := intentContractBinding(binding)
	resumeLease := intentContractLease(lease)
	payload := externalOrcaIntentPayload{
		SchemaVersion: issueops.IssueOpsSchemaVersion, Purpose: orcaIntentPurposeResume,
		OperationID: operationID, LifecycleID: record.ID, Generation: lease.Generation,
		Stage: stage, StartedAt: startedAt,
		InvocationState: orcaIntentNotInvoked, Workspace: intentContractWorkspaceRequest(workspace), Probe: probe, Prepared: prepared,
		Launch: &externalOrcaLaunchIdentity{
			PromptPath: artifacts.promptPath, PromptSHA256: artifacts.promptSHA256,
			ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		},
		IssueBodySHA256: artifacts.issueBodySHA256, ClaimTokenSHA256: lease.ClaimTokenSHA256,
		TerminalPTYID: strings.TrimSpace(terminalPTYID),
		PriorBinding:  &priorBinding, ResumeLease: &resumeLease,
	}
	payload, err = sealExternalOrcaIntentPayload(record, payload)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	data, err := preparationIntentCodec.Encode(payload)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: record.ID}, externalOrcaIntentPayload{}, err
	}
	var persisted issueops.IssueOpsRecord
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
		current.Execution.Pending = &issueops.ExternalIntent{
			OperationID: operationID, Kind: pendingKindForOrcaStage(payload.Stage),
			Marker: payload.Marker, StartedAt: startedAt,
		}
		current.Execution.Failure = nil
		mutations := []sqlstore.Mutation{{Bucket: externalIntentBucket, ID: operationID, Data: data, RequireAbsent: true}}
		if expectedRecordRaw != nil {
			persisted, err = persistExecutionTransitionWithRawCAS(stateRoot, current, []sqlstore.ExpectedRecord{{Bucket: issueOpsBucket, ID: record.ID, Data: expectedRecordRaw}}, mutations)
			return err
		}
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, mutations)
		return err
	})
	return persisted, payload, err
}

func executionResumeResult(record issueops.IssueOpsRecord, artifacts executionResumeArtifacts) ExecutionResumeResult {
	generation := record.Execution.Lease.Generation
	next := ExecutionResumeNextCommand(record.ID, generation, artifacts.claimTokenPath, artifacts.issueBodySHA256, artifacts.packetSHA256)
	return ExecutionResumeResult{
		OK: true, ID: record.ID, Execution: *record.Execution,
		ClaimTokenPath: artifacts.claimTokenPath, IssueBodySHA256: artifacts.issueBodySHA256,
		ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256,
		OwnerPromptPath: artifacts.promptPath, OwnerPromptSHA256: artifacts.promptSHA256,
		NextCommand: next,
	}
}

func ExecutionResumeNextCommand(id string, generation uint64, claimTokenPath, issueBodySHA256, contextPacketSHA256 string) string {
	return "agent-harness issueops execution claim --id " + quoteExecutionOwnerArg(id) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-token-file " + quoteExecutionOwnerArg(claimTokenPath) +
		" --issue-body-sha256 " + issueBodySHA256 +
		" --context-packet-sha256 " + contextPacketSHA256
}

func ExecutionResumeRecoveryCommand(id string, generation uint64) string {
	return "agent-harness issueops execution resume --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --confirm"
}
