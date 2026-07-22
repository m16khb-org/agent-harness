package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestExecutionV1OwnerLaunchSealsIssueContextAndFullPromptBeforeDispatch(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n- [ ] AC-23: last\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\ngo vet ./...\n```\n"
	readIssue := false
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, request port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		if !readIssue {
			t.Fatal("remote issue snapshot must be read before the first Orca mutation")
		}
		pending, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		if pending.Execution == nil || pending.Execution.Pending == nil || pending.Execution.Pending.Marker != request.Marker {
			t.Fatalf("worktree mutation did not follow a durable exact intent: %#v", pending.Execution)
		}
		if err := os.MkdirAll(workspace.Root, 0o700); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}
	fake.launch = func(prepared port.ExecutionOrcaWorkspaceReceipt, _ port.ExecutionOrcaProbeRequest, launch port.ExecutionOrcaLaunchRequest) (port.ExecutionOrcaReceipt, error) {
		assertSealedOwnerLaunchV1(t, record, issueBody, prepared, launch)
		return executionOrcaReceipt(prepared), nil
	}
	reader := func(_ context.Context, provider string, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		readIssue = true
		if provider != "github" || req.Repo != record.Repo || req.URL != record.IssueURL {
			t.Fatalf("issue snapshot authority drifted: provider=%q req=%#v", provider, req)
		}
		return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: issueBody}, nil
	}

	got, err := PrepareExecutionV1(context.Background(), stateRoot, ExecutionPrepareRequestV1{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActorV1("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model", OwnerEffort: "high",
	}, ExecutionPrepareDependenciesV1{Orca: fake, ReadIssue: reader})
	if err != nil {
		t.Fatal(err)
	}
	if fake.prepareCalls != 1 || fake.launchCalls != 1 || got.ContextPacketPath == "" || got.ContextPacketSHA256 == "" || got.OwnerPromptPath == "" || got.OwnerPromptSHA256 == "" {
		t.Fatalf("sealed owner launch receipt is incomplete: prepare=%d launch=%d result=%#v", fake.prepareCalls, fake.launchCalls, got)
	}
	if got.Execution == nil || got.Execution.Lease.Status != model.LeaseStatusClaimable {
		t.Fatalf("dispatch receipt did not become claimable: %#v", got.Execution)
	}
	claimed, err := ClaimExecutionV1WithDependencies(context.Background(), stateRoot, ExecutionClaimRequestV1{
		ID: record.ID, Generation: 1, Actor: executionActorV1("claude", "owner"), CWD: got.Workspace.Root,
		TokenFile: got.ClaimTokenPath, IssueBodySHA256: got.IssueBodySHA256, ContextPacketSHA256: got.ContextPacketSHA256,
	}, ExecutionClaimDependenciesV1{ReadIssue: reader})
	if err != nil || claimed.Execution.Lease.Status != model.LeaseStatusActive {
		t.Fatalf("verified issue and packet digests must permit one claim: result=%#v err=%v", claimed, err)
	}
	if _, err := os.Stat(got.ClaimTokenPath); !os.IsNotExist(err) {
		t.Fatalf("successful claim did not consume token file: %v", err)
	}
}

func TestExecutionV1InitialOrcaClaimRejectsIssueOrPacketDigestDrift(t *testing.T) {
	for _, drift := range []string{"issue", "packet"} {
		t.Run(drift, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
			fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
			fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
				if err := os.MkdirAll(workspace.Root, 0o700); err != nil {
					return port.ExecutionOrcaWorkspaceReceipt{}, err
				}
				return executionOrcaWorkspaceReceipt(workspace), nil
			}
			reader := func(_ context.Context, _ string, _ port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
				return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: issueBody}, nil
			}
			prepared, err := PrepareExecutionV1(context.Background(), stateRoot, ExecutionPrepareRequestV1{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActorV1("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
			}, ExecutionPrepareDependenciesV1{Orca: fake, ReadIssue: reader})
			if err != nil {
				t.Fatal(err)
			}
			if drift == "issue" {
				reader = func(_ context.Context, _ string, _ port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
					return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: issueBody + "\nchanged"}, nil
				}
			} else if err := os.WriteFile(prepared.ContextPacketPath, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = ClaimExecutionV1WithDependencies(context.Background(), stateRoot, ExecutionClaimRequestV1{
				ID: record.ID, Generation: 1, Actor: executionActorV1("claude", "owner"), CWD: prepared.Workspace.Root,
				TokenFile: prepared.ClaimTokenPath, IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketSHA256: prepared.ContextPacketSHA256,
			}, ExecutionClaimDependenciesV1{ReadIssue: reader})
			if err == nil || !strings.Contains(err.Error(), "digest") {
				t.Fatalf("%s digest drift must reject claim: %v", drift, err)
			}
			current, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if current.Execution.Lease.Status != model.LeaseStatusClaimable {
				t.Fatalf("digest drift changed lease authority: %#v", current.Execution.Lease)
			}
		})
	}
}

func assertSealedOwnerLaunchV1(t *testing.T, record IssueOpsRecord, issueBody string, prepared port.ExecutionOrcaWorkspaceReceipt, launch port.ExecutionOrcaLaunchRequest) {
	t.Helper()
	if !pathWithinRootV1(prepared.Workspace.Root, launch.ContextPacketPath) || !pathWithinRootV1(prepared.Workspace.Root, launch.PromptPath) {
		t.Fatalf("owner artifacts escaped the canonical worktree: %#v", launch)
	}
	packetBytes := readPrivateOwnerFileV1(t, launch.ContextPacketPath)
	promptBytes := readPrivateOwnerFileV1(t, launch.PromptPath)
	if digestOwnerFixtureV1(packetBytes) != launch.ContextPacketSHA256 || digestOwnerFixtureV1(promptBytes) != launch.PromptSHA256 || string(promptBytes) != launch.Prompt {
		t.Fatalf("owner artifact digests do not match launch request: %#v", launch)
	}
	var packet map[string]any
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	issue, ok := packet["issue"].(map[string]any)
	if !ok || issue["url"] != record.IssueURL || issue["body"] != issueBody || issue["body_sha256"] != digestOwnerFixtureV1([]byte(issueBody)) {
		t.Fatalf("packet did not seal the exact issue snapshot: %#v", packet["issue"])
	}
	if !reflect.DeepEqual(stringSliceFromPacketV1(packet["acceptance_ids"]), []string{"AC-01", "AC-23"}) {
		t.Fatalf("packet acceptance IDs drifted: %#v", packet["acceptance_ids"])
	}
	if !reflect.DeepEqual(stringSliceFromPacketV1(packet["verification_commands"]), []string{"go test ./... -count=1", "go vet ./..."}) {
		t.Fatalf("packet verification commands drifted: %#v", packet["verification_commands"])
	}
	claimTokenPath, _ := packet["claim_token_file"].(string)
	token, err := os.ReadFile(claimTokenPath)
	if err != nil || len(token) == 0 {
		t.Fatalf("claim token file was not prepared before dispatch: path=%q err=%v", claimTokenPath, err)
	}
	if strings.Contains(string(packetBytes), string(token)) || strings.Contains(launch.Prompt, string(token)) {
		t.Fatal("claim token raw value leaked into packet or prompt")
	}
	for _, required := range []string{
		"당신은 agent-harness 저장소의 IssueOps v1 implementation owner다.",
		"schema_version=1", record.IssueURL, launch.ContextPacketPath, launch.ContextPacketSHA256,
		"issueops execution status", "issueops execution claim", "issueops remote create-pr", "issueops execution complete",
		"- Status: completed | blocked", "- Blockers:",
	} {
		if !strings.Contains(launch.Prompt, required) {
			t.Fatalf("full owner prompt is missing %q", required)
		}
	}
	if executionV1PromptPlaceholder.MatchString(launch.Prompt) || strings.Contains(launch.Prompt, "issueops handoff") || strings.Contains(launch.Prompt, "issueops worktree prepare") {
		t.Fatalf("owner prompt is unresolved or selected legacy commands:\n%s", launch.Prompt)
	}
}

func readPrivateOwnerFileV1(t *testing.T, path string) []byte {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("owner artifact must be a private regular file: %s mode=%s", path, info.Mode())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func digestOwnerFixtureV1(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func pathWithinRootV1(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func stringSliceFromPacketV1(value any) []string {
	rows, _ := value.([]any)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		text, _ := row.(string)
		out = append(out, text)
	}
	return out
}
