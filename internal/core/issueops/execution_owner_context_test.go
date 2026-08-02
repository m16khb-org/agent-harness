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

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestExecutionOwnerLaunchSealsIssueContextAndFullPromptBeforeDispatch(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
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
	if got.Execution == nil || got.Execution.Lease.Status != issueops.LeaseStatusClaimable {
		t.Fatalf("dispatch receipt did not become claimable: %#v", got.Execution)
	}
	claimed, err := claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: executionActor("claude", "owner"), CWD: got.Workspace.Root,
		TokenFile: got.ClaimTokenPath, IssueBodySHA256: got.IssueBodySHA256, ContextPacketSHA256: got.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: reader})
	if err != nil || claimed.Execution.Lease.Status != issueops.LeaseStatusActive {
		t.Fatalf("verified issue and packet digests must permit one claim: result=%#v err=%v", claimed, err)
	}
	if _, err := os.Stat(got.ClaimTokenPath); !os.IsNotExist(err) {
		t.Fatalf("successful claim did not consume token file: %v", err)
	}
}

func TestExecutionInitialOrcaClaimRejectsIssueOrPacketDigestDrift(t *testing.T) {
	for _, drift := range []string{"issue", "packet"} {
		t.Run(drift, func(t *testing.T) {
			stateRoot, record := orcaPrepareRecord(t)
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
			_, err = claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
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
			if current.Execution.Lease.Status != issueops.LeaseStatusClaimable {
				t.Fatalf("digest drift changed lease authority: %#v", current.Execution.Lease)
			}
		})
	}
}

// sealedOrcaCycle는 봉인이 끝난 claimable orca 사이클을 만든다. reseed와 세대
// 검증 테스트가 같은 출발점을 공유해야 재봉인 전후를 비교할 수 있다.
func sealedOrcaCycle(t *testing.T, issueBody string) (string, issueops.IssueOpsRecord, ExecutionPrepareResult, ExecutionIssueSnapshotReadFunc) {
	return sealedOrcaCycleWithArtifacts(t, issueBody, nil)
}

func sealedOrcaCycleWithArtifacts(t *testing.T, issueBody string, artifacts map[string]string) (string, issueops.IssueOpsRecord, ExecutionPrepareResult, ExecutionIssueSnapshotReadFunc) {
	t.Helper()
	stateRoot, record := orcaPrepareRecord(t)
	for name, content := range artifacts {
		if _, err := StageIssueOpsArtifact(stateRoot, record.ID, name, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
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

func TestExecutionOrcaPrepareRecordsBindingLeaseGeneration(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	_, _, prepared, _ := sealedOrcaCycle(t, issueBody)
	if prepared.Execution.Orca == nil ||
		prepared.Execution.Orca.LeaseGeneration != prepared.Execution.Lease.Generation {
		t.Fatalf("prepare binding generation = %#v lease=%d", prepared.Execution.Orca, prepared.Execution.Lease.Generation)
	}
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
	reseeded, err := reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
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

type revokingSealedOrcaCycle struct {
	stateRoot string
	record    issueops.IssueOpsRecord
	prepared  ExecutionPrepareResult
	reader    ExecutionIssueSnapshotReadFunc
	requester issueops.NativeActor
	deps      ExecutionReplaceDependencies
	preview   ExecutionReplaceResult
}

func newRevokingSealedOrcaCycle(t *testing.T, issueBody string) revokingSealedOrcaCycle {
	t.Helper()
	stateRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
	current, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(prepared.ClaimTokenPath); err != nil {
		t.Fatal(err)
	}
	// 중단된 이전 owner는 실제 프로세스가 사라졌지만 durable holder 기록은
	// 남아 있는 상태다. replacement가 검증해야 하는 운영 상황을 그대로 만든다.
	current.Execution.Lease = issueops.WriteLease{
		Generation: 1, Status: issueops.LeaseStatusActive, ClaimedAt: "2026-07-28T00:00:00Z",
		Holder: &issueops.NativeActor{
			Host: "claude", SessionID: "dead-owner",
			SessionProcess: &issueops.NativeProcessReceipt{
				PID: 999999, StartedAt: "2026-07-28T00:00:00Z", Executable: "claude",
			},
		},
	}
	if _, err := writeIssueOps(stateRoot, current); err != nil {
		t.Fatal(err)
	}

	requester := executionActor("codex", "replacement-requester")
	deps := quiescentOrcaReplaceDeps(reader)
	inventory, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Repo,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceRevoke, ExpectedGeneration: 1,
		InventoryFingerprint: inventory.InventoryFingerprint, Reason: "interrupted owner session",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, deps); err != nil {
		t.Fatal(err)
	}
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: requester, CWD: prepared.Workspace.Root,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	return revokingSealedOrcaCycle{
		stateRoot: stateRoot, record: record, prepared: prepared, reader: reader,
		requester: requester, deps: deps, preview: preview,
	}
}

func TestExecutionFinalizeResealsOwnerPacketBeforeReplacementClaim(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	fixture := newRevokingSealedOrcaCycle(t, issueBody)

	finalized, err := ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: fixture.preview.QuiescenceFingerprint,
		Actor:                 fixture.requester, CWD: fixture.prepared.Workspace.Root, Confirm: true,
	}, fixture.deps)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Execution.Lease.Status != issueops.LeaseStatusClaimable ||
		finalized.Execution.Lease.Generation != 2 ||
		strings.TrimSpace(finalized.IssueBodySHA256) == "" ||
		strings.TrimSpace(finalized.ContextPacketSHA256) == "" ||
		strings.TrimSpace(finalized.OwnerPromptSHA256) == "" {
		t.Fatalf("finalize가 claim 가능한 재봉인 증거를 반환하지 않았다: %#v", finalized)
	}
	if finalized.ContextPacketPath == fixture.prepared.ContextPacketPath {
		t.Fatalf("finalize packet이 이전 세대 경로를 재사용했다: %q", finalized.ContextPacketPath)
	}
	packet := decodeOwnerPacketFile(t, finalized.ContextPacketPath)
	if generation, _ := packet["lease_generation"].(float64); uint64(generation) != 2 {
		t.Fatalf("finalize packet 세대가 교체 세대와 다르다: %#v", packet["lease_generation"])
	}
	if tokenFile, _ := packet["claim_token_file"].(string); tokenFile != finalized.ClaimTokenPath {
		t.Fatalf("finalize packet의 token 경로가 새 token과 다르다: %q want %q", tokenFile, finalized.ClaimTokenPath)
	}

	claimed, err := claimViaVerticalWithDeps(context.Background(), fixture.stateRoot, ExecutionClaimRequest{
		ID: fixture.record.ID, Generation: 2, Actor: executionActor("claude", "replacement-owner"),
		CWD: fixture.prepared.Workspace.Root, TokenFile: finalized.ClaimTokenPath,
		IssueBodySHA256: finalized.IssueBodySHA256, ContextPacketSHA256: finalized.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: fixture.reader})
	if err != nil || claimed.Execution.Lease.Status != issueops.LeaseStatusActive {
		t.Fatalf("finalize가 만든 세대별 봉인으로 claim하지 못했다: result=%#v err=%v", claimed, err)
	}
}

func TestExecutionFinalizeResealFailureKeepsRevokingStateAtomic(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	fixture := newRevokingSealedOrcaCycle(t, issueBody)
	revoking, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	tokenPath := claimTokenPath(revoking)

	_, err = ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: fixture.preview.QuiescenceFingerprint,
		Actor:                 fixture.requester, CWD: fixture.prepared.Workspace.Root, Confirm: true,
	}, ExecutionReplaceDependencies{OrcaOwner: fixture.deps.OrcaOwner})
	if err == nil || !strings.Contains(err.Error(), "remote issue reader") {
		t.Fatalf("재봉인 의존성 없는 finalize가 claimable로 진행됐다: %v", err)
	}
	persisted, readErr := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.Execution.Lease.Status != issueops.LeaseStatusRevoking ||
		persisted.Execution.Lease.Generation != 2 ||
		persisted.Execution.Lease.Holder == nil {
		t.Fatalf("재봉인 실패가 durable revoking 상태를 바꿨다: %#v", persisted.Execution.Lease)
	}
	if _, statErr := os.Lstat(tokenPath); !os.IsNotExist(statErr) {
		t.Fatalf("재봉인 실패 후 새 claim token이 남았다: %v", statErr)
	}
}

func TestExecutionReseedFailurePreservesCurrentClaimToken(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	stateRoot, record, prepared, reader := sealedOrcaCycle(t, issueBody)
	requester := executionActor("codex", "failed-reseed-requester")
	deps := quiescentOrcaReplaceDeps(reader)
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Repo,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	_, err = reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "test reseal failure",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, ExecutionReplaceDependencies{OrcaOwner: deps.OrcaOwner})
	if err == nil || !strings.Contains(err.Error(), "remote issue reader") {
		t.Fatalf("reader 없는 reseed가 실패하지 않았다: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Lease.Status != issueops.LeaseStatusClaimable || persisted.Execution.Lease.Generation != 1 {
		t.Fatalf("실패한 reseed가 durable 현재 세대를 바꿨다: %#v", persisted.Execution.Lease)
	}
	if _, err := os.Lstat(prepared.ClaimTokenPath); err != nil {
		t.Fatalf("실패한 reseed가 현재 세대 token을 잃었다: %v", err)
	}
	if _, err := claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: executionActor("claude", "original-owner"),
		CWD: prepared.Workspace.Root, TokenFile: prepared.ClaimTokenPath,
		IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketSHA256: prepared.ContextPacketSHA256,
	}, ExecutionClaimDependencies{ReadIssue: reader}); err != nil {
		t.Fatalf("실패한 reseed 뒤 원래 claim 경로가 복구되지 않았다: %v", err)
	}
}

func TestExecutionFinalizeCleansPartialOwnerArtifactsBeforeRetry(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	fixture := newRevokingSealedOrcaCycle(t, issueBody)
	revoking, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	tokenPath := claimTokenPath(revoking)
	packetPath, promptPath := executionOwnerArtifactPaths(revoking)
	reader := func(ctx context.Context, provider string, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		if err := os.MkdirAll(filepath.Dir(promptPath), 0o700); err != nil {
			return port.ExecutionIssueSnapshot{}, err
		}
		if err := os.WriteFile(promptPath, []byte("conflicting partial prompt\n"), 0o600); err != nil {
			return port.ExecutionIssueSnapshot{}, err
		}
		return fixture.reader(ctx, provider, req)
	}
	deps := fixture.deps
	deps.ReadIssue = reader

	_, err = ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: fixture.preview.QuiescenceFingerprint,
		Actor:                 fixture.requester, CWD: fixture.prepared.Workspace.Root, Confirm: true,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "immutable owner artifact") {
		t.Fatalf("부분 owner artifact 충돌이 finalize를 멈추지 않았다: %v", err)
	}
	for _, path := range []string{tokenPath, packetPath, promptPath} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("실패한 finalize의 generation residue가 남았다: path=%q err=%v", path, statErr)
		}
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Lease.Status != issueops.LeaseStatusRevoking || persisted.Execution.Lease.Generation != 2 {
		t.Fatalf("부분 파일 실패가 durable lease를 바꿨다: %#v", persisted.Execution.Lease)
	}

	if _, err := ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: fixture.preview.QuiescenceFingerprint,
		Actor:                 fixture.requester, CWD: fixture.prepared.Workspace.Root, Confirm: true,
	}, fixture.deps); err != nil {
		t.Fatalf("부분 파일 정리 뒤 같은 세대 finalize 재시도가 실패했다: %v", err)
	}
}

func TestExecutionFinalizeRecoversUncommittedGenerationResidue(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	fixture := newRevokingSealedOrcaCycle(t, issueBody)
	revoking, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	tokenPath := claimTokenPath(revoking)
	packetPath, promptPath := executionOwnerArtifactPaths(revoking)
	for _, path := range []string{tokenPath, packetPath, promptPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("uncommitted residue\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalizePreview, ExpectedGeneration: 2,
		Actor: fixture.requester, CWD: fixture.prepared.Workspace.Root,
	}, fixture.deps)
	if err != nil {
		t.Fatal(err)
	}

	finalized, err := ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, ExecutionReplaceRequest{
		ID: fixture.record.ID, Action: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: preview.QuiescenceFingerprint,
		Actor:                 fixture.requester, CWD: fixture.prepared.Workspace.Root, Confirm: true,
	}, fixture.deps)
	if err != nil {
		t.Fatalf("durable revoking 세대가 uncommitted residue를 회수하지 못했다: %v", err)
	}
	if finalized.Execution.Lease.Status != issueops.LeaseStatusClaimable ||
		finalized.ClaimTokenPath != tokenPath ||
		finalized.ContextPacketPath != packetPath ||
		finalized.OwnerPromptPath != promptPath {
		t.Fatalf("residue 회수 후 다른 세대/경로를 발급했다: %#v", finalized)
	}
}

func TestExecutionReseedAdoptsQuiescentOrcaRuntimeRollover(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	stateRoot, record, _, reader := sealedOrcaCycle(t, issueBody)
	inspector := &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
		RuntimeID: "runtime-2", TaskStatus: "completed", DispatchStatus: "failed",
	}}
	deps := ExecutionReplaceDependencies{OrcaOwner: inspector, ReadIssue: reader}
	requester := executionActor("codex", "runtime-rollover-requester")

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: record.Repo,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !inspector.last.AllowRuntimeRollover {
		t.Fatal("holderless reseed preview가 제한된 runtime rollover 관측을 요청하지 않았다")
	}
	reseeded, err := reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "Orca runtime restarted after owner exit",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if reseeded.Execution.Orca == nil || reseeded.Execution.Orca.RuntimeID != "runtime-2" {
		t.Fatalf("reseed 결과가 current runtime으로 재봉인되지 않았다: %#v", reseeded.Execution.Orca)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Orca == nil || persisted.Execution.Orca.RuntimeID != "runtime-2" {
		t.Fatalf("persisted Orca binding이 current runtime으로 교체되지 않았다: %#v", persisted.Execution.Orca)
	}
}

func TestExecutionReseedRejectsRuntimeRolloverWithLiveOwner(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	stateRoot, record, _, reader := sealedOrcaCycle(t, issueBody)
	deps := ExecutionReplaceDependencies{
		OrcaOwner: &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
			RuntimeID: "runtime-2", TaskLive: true, TaskStatus: "ready", DispatchStatus: "failed",
		}},
		ReadIssue: reader,
	}
	_, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: executionActor("codex", "runtime-rollover-requester"), CWD: record.Repo,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "runtime rollover owner is not quiescent") {
		t.Fatalf("살아 있는 owner를 동반한 runtime rollover가 preview를 통과했다: %v", err)
	}
}

func TestExecutionReseedRuntimeRolloverHonorsInventoryCAS(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\n```\n"
	stateRoot, record, _, reader := sealedOrcaCycle(t, issueBody)
	inspector := &executionOrcaOwnerInspectorFake{inventory: port.ExecutionOrcaOwnerInventory{
		RuntimeID: "runtime-2", TaskStatus: "completed", DispatchStatus: "failed",
	}}
	deps := ExecutionReplaceDependencies{OrcaOwner: inspector, ReadIssue: reader}
	requester := executionActor("codex", "runtime-rollover-requester")
	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1, Actor: requester, CWD: record.Repo,
	}, deps)
	if err != nil {
		t.Fatal(err)
	}

	inspector.inventory.RuntimeID = "runtime-3"
	_, err = reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "Orca runtime restarted twice",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, deps)
	if err == nil || !strings.Contains(err.Error(), "stale replacement inventory fingerprint") {
		t.Fatalf("runtime이 다시 바뀐 stale preview가 reseed를 통과했다: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Execution.Lease.Generation != 1 || persisted.Execution.Orca == nil || persisted.Execution.Orca.RuntimeID != "runtime-1" {
		t.Fatalf("실패한 CAS가 기존 generation/runtime 봉인을 변경했다: %#v", persisted.Execution)
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
	if _, err := claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
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
	reseeded, err := reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplaceReseed, ExpectedGeneration: 1,
		InventoryFingerprint: preview.InventoryFingerprint, Reason: "issue scope revised by the user",
		Actor: requester, CWD: record.Repo, Confirm: true,
	}, quiescentOrcaReplaceDeps(revisedReader))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
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
	reseeded, err := reseedExecutionCompatibilityOracle(context.Background(), stateRoot, ExecutionReplaceRequest{
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
	_, err = claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
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
			_, err := claimViaVerticalWithDeps(context.Background(), stateRoot, ExecutionClaimRequest{
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

func TestExtractExecutionOwnerVerificationAcceptsCanonicalIssueHeadings(t *testing.T) {
	for _, heading := range []string{"검증", "Verification"} {
		t.Run(heading, func(t *testing.T) {
			body := "## " + heading + "\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
			got := extractExecutionOwnerVerification(body)
			if !reflect.DeepEqual(got, []string{"go test ./internal/core/issueops -count=1"}) {
				t.Fatalf("canonical issue heading %q verification = %#v", heading, got)
			}
		})
	}
}

func TestExtractExecutionOwnerVerificationRejectsUnfencedProse(t *testing.T) {
	body := "## 검증\n\ngo test ./internal/core/issueops -count=1\n"
	if got := extractExecutionOwnerVerification(body); len(got) != 0 {
		t.Fatalf("unfenced verification prose must not become an executable command: %#v", got)
	}
}

func assertSealedOwnerLaunch(t *testing.T, record issueops.IssueOpsRecord, issueBody string, prepared port.ExecutionOrcaWorkspaceReceipt, launch port.ExecutionOrcaLaunchRequest) {
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
