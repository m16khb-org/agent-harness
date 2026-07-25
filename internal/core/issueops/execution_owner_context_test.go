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
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestExecutionOwnerLaunchSealsIssueContextAndFullPromptBeforeDispatch(t *testing.T) {
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
		assertSealedOwnerLaunch(t, record, issueBody, prepared, launch)
		return executionOrcaReceipt(prepared), nil
	}
	reader := func(_ context.Context, provider string, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		readIssue = true
		if provider != "github" || req.Repo != record.Repo || req.URL != record.IssueURL {
			t.Fatalf("issue snapshot authority drifted: provider=%q req=%#v", provider, req)
		}
		return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: issueBody}, nil
	}

	got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model", OwnerEffort: "high",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: reader})
	if err != nil {
		t.Fatal(err)
	}
	if fake.prepareCalls != 1 || fake.launchCalls != 1 || got.ContextPacketPath == "" || got.ContextPacketSHA256 == "" || got.OwnerPromptPath == "" || got.OwnerPromptSHA256 == "" {
		t.Fatalf("sealed owner launch receipt is incomplete: prepare=%d launch=%d result=%#v", fake.prepareCalls, fake.launchCalls, got)
	}
	if got.Execution == nil || got.Execution.Lease.Status != model.LeaseStatusClaimable {
		t.Fatalf("dispatch receipt did not become claimable: %#v", got.Execution)
	}
	claimed, err := ClaimExecutionWithDependencies(context.Background(), stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: executionActor("claude", "owner"), CWD: got.Workspace.Root,
		TokenFile: got.ClaimTokenPath, IssueBodySHA256: got.IssueBodySHA256, ContextPacketSHA256: got.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: reader})
	if err != nil || claimed.Execution.Lease.Status != model.LeaseStatusActive {
		t.Fatalf("verified issue and packet digests must permit one claim: result=%#v err=%v", claimed, err)
	}
	if _, err := os.Stat(got.ClaimTokenPath); !os.IsNotExist(err) {
		t.Fatalf("successful claim did not consume token file: %v", err)
	}
}

func TestExecutionInitialOrcaClaimRejectsIssueOrPacketDigestDrift(t *testing.T) {
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
			prepared, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
				ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
				Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
			}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: reader})
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
			_, err = ClaimExecutionWithDependencies(context.Background(), stateRoot, ExecutionClaimRequest{
				ID: record.ID, Generation: 1, Actor: executionActor("claude", "owner"), CWD: prepared.Workspace.Root,
				TokenFile: prepared.ClaimTokenPath, IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketSHA256: prepared.ContextPacketSHA256,
			}, ExecutionClaimDependencies{ReadIssue: reader})
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

// sealedOrcaCycle는 봉인이 끝난 claimable orca 사이클을 만든다. reseed와 세대
// 검증 테스트가 같은 출발점을 공유해야 재봉인 전후를 비교할 수 있다.
func sealedOrcaCycle(t *testing.T, issueBody string) (string, IssueOpsRecord, ExecutionPrepareResult, ExecutionIssueSnapshotReadFunc) {
	t.Helper()
	stateRoot, record := executionPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	// reseed는 워크스페이스 스냅샷으로 Git top-level을 확인하므로 fake가 실제
	// 워크트리를 만들어야 한다(디렉토리만 만들면 재봉인 경로에 닿지 못한다).
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		if code, _, stderr := preflight.GitCmd(workspace.SourceRoot, "worktree", "add", "-q", "-b", workspace.Branch, workspace.Root, workspace.BaseHead); code != 0 {
			return port.ExecutionOrcaWorkspaceReceipt{}, fmt.Errorf("git worktree add: %s", stderr)
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}
	reader := func(_ context.Context, _ string, _ port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: issueBody}, nil
	}
	prepared, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: reader})
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, prepared, reader
}

// quiescentOrcaReplaceDeps는 owner가 이미 사라진 상태를 흉내내 reseed가 인벤토리
// 게이트를 통과하게 한다. 재봉인 동작만 관찰하기 위한 최소 구성이다.
func quiescentOrcaReplaceDeps(reader ExecutionIssueSnapshotReadFunc) ExecutionReplaceDependencies {
	return ExecutionReplaceDependencies{
		OrcaOwner: &executionOrcaOwnerInspectorFake{},
		ReadIssue: reader,
	}
}

func decodeOwnerPacketFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var packet map[string]any
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	return packet
}

// reseed는 generation을 올리고 claim token을 재발급하지만 packet을 재봉인하지
// 않았다. 그 결과 새 세대 owner가 lease_generation 1과 구 token 경로가 박힌
// packet을 읽는다.
func TestExecutionReseedResealsOwnerPacketForTheNewGeneration(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	stateRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
	requester := executionActor("codex", "reseed-requester")
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: record.Repo,
	}, quiescentOrcaReplaceDeps(reader))
	if err != nil {
		t.Fatal(err)
	}
	reseeded, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner terminal lost before claim",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, quiescentOrcaReplaceDeps(reader))
	if err != nil {
		t.Fatal(err)
	}
	if reseeded.Execution.Lease.Generation != 2 {
		t.Fatalf("reseed did not rotate the generation: %#v", reseeded.Execution.Lease)
	}
	if strings.TrimSpace(reseeded.ContextPacketSHA256) == "" || reseeded.ContextPacketSHA256 == prepared.ContextPacketSHA256 {
		t.Fatalf("reseed must publish a resealed packet digest: got %q, prior %q", reseeded.ContextPacketSHA256, prepared.ContextPacketSHA256)
	}
	// packet 경로는 세대별로 격리되므로(generation-N 디렉토리) 재봉인은 새 세대
	// 경로에 써야 한다. 재봉인이 없으면 그 경로에 파일이 아예 없어 claim이 읽기
	// 실패로 거부된다 — 검증 확대와 재봉인을 함께 넣어야 하는 이유다.
	if reseeded.ContextPacketPath == prepared.ContextPacketPath {
		t.Fatalf("resealed packet must live under the new generation path: %q", reseeded.ContextPacketPath)
	}
	packet := decodeOwnerPacketFile(t, reseeded.ContextPacketPath)
	if generation, _ := packet["lease_generation"].(float64); uint64(generation) != 2 {
		t.Fatalf("resealed packet must carry the new generation: %#v", packet["lease_generation"])
	}
	if tokenFile, _ := packet["claim_token_file"].(string); tokenFile != reseeded.ClaimTokenPath {
		t.Fatalf("resealed packet must point at the new claim token: %q want %q", tokenFile, reseeded.ClaimTokenPath)
	}
}

// 봉인 이후 이슈 본문이 정당하게 개정되면 claim이 영구 거부됐다. reseed가
// 현재 본문으로 재봉인하면 회복 경로가 생긴다.
func TestExecutionReseedRecoversFromLegitimateIssueRevision(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	stateRoot, record, prepared, _ := sealedOrcaCycle(t, issueBody)
	revised := issueBody + "\n- [ ] AC-02: 사용자 요청으로 추가된 수용 기준\n"
	revisedReader := func(_ context.Context, _ string, _ port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: revised}, nil
	}
	if _, err := ClaimExecutionWithDependencies(context.Background(), stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: executionActor("claude", "owner"), CWD: prepared.Workspace.Root,
		TokenFile: prepared.ClaimTokenPath, IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketSHA256: prepared.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: revisedReader}); err == nil {
		t.Fatal("claim against a revised issue must be rejected before reseed")
	}
	requester := executionActor("codex", "reseed-requester")
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: record.Repo,
	}, quiescentOrcaReplaceDeps(revisedReader))
	if err != nil {
		t.Fatal(err)
	}
	reseeded, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "issue scope revised by the user",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, quiescentOrcaReplaceDeps(revisedReader))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimExecutionWithDependencies(context.Background(), stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 2, Actor: executionActor("claude", "owner"), CWD: prepared.Workspace.Root,
		TokenFile: reseeded.ClaimTokenPath, IssueBodySHA256: reseeded.IssueBodySHA256, ContextPacketSHA256: reseeded.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: revisedReader}); err != nil {
		t.Fatalf("reseed must restore a claimable sealed context: %v", err)
	}
}

// generation 1이 아닌 claim은 digest 검증 자체를 건너뛰어 낡은 packet이 무음으로
// 통과했다. 봉인의 목적이 generation 2부터 사라지는 구멍이다.
func TestExecutionClaimVerifiesSealedPacketBeyondTheFirstGeneration(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	stateRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
	requester := executionActor("codex", "reseed-requester")
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: record.Repo,
	}, quiescentOrcaReplaceDeps(reader))
	if err != nil {
		t.Fatal(err)
	}
	reseeded, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "owner terminal lost before claim",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, quiescentOrcaReplaceDeps(reader))
	if err != nil {
		t.Fatal(err)
	}
	// 재봉인 이후 새 세대 packet을 외부에서 바꿔치기한다: generation 2 claim도 이를 잡아야 한다.
	if err := os.WriteFile(reseeded.ContextPacketPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ClaimExecutionWithDependencies(context.Background(), stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 2, Actor: executionActor("claude", "owner"), CWD: prepared.Workspace.Root,
		TokenFile: reseeded.ClaimTokenPath, IssueBodySHA256: reseeded.IssueBodySHA256, ContextPacketSHA256: reseeded.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: reader})
	if err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("tampered packet must reject a later-generation claim: %v", err)
	}
}

// drift 오류가 사실만 알리면 owner는 원인을 특정할 수 없다. digest는 secret이
// 아니므로 expected와 observed를 병기한다.
func TestExecutionClaimDigestDriftErrorsCarryExpectedAndObserved(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	for _, drift := range []string{"issue", "packet"} {
		t.Run(drift, func(t *testing.T) {
			stateRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
			observed := ""
			if drift == "issue" {
				revised := issueBody + "\nchanged"
				observed = digestOwnerFixture([]byte(revised))
				reader = func(_ context.Context, _ string, _ port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
					return port.ExecutionIssueSnapshot{URL: record.IssueURL, Body: revised}, nil
				}
			} else {
				tampered := []byte("{}\n")
				observed = digestOwnerFixture(tampered)
				if err := os.WriteFile(prepared.ContextPacketPath, tampered, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := ClaimExecutionWithDependencies(context.Background(), stateRoot, ExecutionClaimRequest{
				ID: record.ID, Generation: 1, Actor: executionActor("claude", "owner"), CWD: prepared.Workspace.Root,
				TokenFile: prepared.ClaimTokenPath, IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketSHA256: prepared.ContextPacketSHA256,
			}, ExecutionClaimDependencies{ReadIssue: reader})
			if err == nil {
				t.Fatalf("%s drift must reject the claim", drift)
			}
			expected := prepared.IssueBodySHA256
			if drift == "packet" {
				expected = prepared.ContextPacketSHA256
			}
			if !strings.Contains(err.Error(), expected) || !strings.Contains(err.Error(), observed) {
				t.Fatalf("%s drift error must carry expected and observed digests: %v", drift, err)
			}
		})
	}
}

func assertSealedOwnerLaunch(t *testing.T, record IssueOpsRecord, issueBody string, prepared port.ExecutionOrcaWorkspaceReceipt, launch port.ExecutionOrcaLaunchRequest) {
	t.Helper()
	if !pathWithinRoot(prepared.Workspace.Root, launch.ContextPacketPath) || !pathWithinRoot(prepared.Workspace.Root, launch.PromptPath) {
		t.Fatalf("owner artifacts escaped the canonical worktree: %#v", launch)
	}
	packetBytes := readPrivateOwnerFile(t, launch.ContextPacketPath)
	promptBytes := readPrivateOwnerFile(t, launch.PromptPath)
	if digestOwnerFixture(packetBytes) != launch.ContextPacketSHA256 || digestOwnerFixture(promptBytes) != launch.PromptSHA256 || string(promptBytes) != launch.Prompt {
		t.Fatalf("owner artifact digests do not match launch request: %#v", launch)
	}
	var packet map[string]any
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	issue, ok := packet["issue"].(map[string]any)
	if !ok || issue["url"] != record.IssueURL || issue["body"] != issueBody || issue["body_sha256"] != digestOwnerFixture([]byte(issueBody)) {
		t.Fatalf("packet did not seal the exact issue snapshot: %#v", packet["issue"])
	}
	if !reflect.DeepEqual(stringSliceFromPacket(packet["acceptance_ids"]), []string{"AC-01", "AC-23"}) {
		t.Fatalf("packet acceptance IDs drifted: %#v", packet["acceptance_ids"])
	}
	if !reflect.DeepEqual(stringSliceFromPacket(packet["verification_commands"]), []string{"go test ./... -count=1", "go vet ./..."}) {
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
	if executionPromptPlaceholder.MatchString(launch.Prompt) || strings.Contains(launch.Prompt, "issueops handoff") || strings.Contains(launch.Prompt, "issueops worktree prepare") {
		t.Fatalf("owner prompt is unresolved or selected legacy commands:\n%s", launch.Prompt)
	}
}

func readPrivateOwnerFile(t *testing.T, path string) []byte {
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

func digestOwnerFixture(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}

func pathWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func stringSliceFromPacket(value any) []string {
	rows, _ := value.([]any)
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		text, _ := row.(string)
		out = append(out, text)
	}
	return out
}
