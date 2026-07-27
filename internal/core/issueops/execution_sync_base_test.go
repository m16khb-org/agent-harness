package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

const (
	syncBaseWorkOID   = "1111111111111111111111111111111111111111"
	syncBaseBaseOID   = "2222222222222222222222222222222222222222"
	syncBaseMergeOID  = "3333333333333333333333333333333333333333"
	syncBaseRemoteOID = "1111111111111111111111111111111111111111"
	syncBaseFinalHead = "4444444444444444444444444444444444444444"
)

type syncBaseGitCall struct {
	dir  string
	args []string
}

// fakeSyncBaseGit은 주입된 Git 표면 전체를 대체해 호출 순서와 인자를 기록한다.
// 실제 git을 쓰지 않으므로 fetch→merge-tree→merge→push 계약을 결정적으로
// 검증할 수 있다.
type fakeSyncBaseGit struct {
	gitDir string
	calls  []syncBaseGitCall

	currentBranch string
	remoteRef     string
	headOID       string
	fetchHeadOID  string
	mergeHeadOID  string
	statusOut     string

	mergeHead      bool
	cherryPickHead bool
	rebaseHead     bool
	ancestor       map[string]bool

	fetchCode     int
	mergeTreeCode int
	mergeTreeOut  string
	mergeCode     int
	mergeOut      string
	// postMergeHead는 merge 성공 시 HEAD가 전이될 OID다 — 실제 git처럼
	// merge 이후에만 HEAD가 바뀌어야 apply의 fingerprint 재계산이 preview와
	// 일치한다(정적 선설정은 TOCTOU 게이트를 오탐시킨다).
	postMergeHead string
	unmergedOut   string
	diffCheckCode int
	commitCode    int
	pushCode      int
	pushOut       string
	aborted       bool
}

func (g *fakeSyncBaseGit) run(_ context.Context, dir string, args ...string) (int, string) {
	g.calls = append(g.calls, syncBaseGitCall{dir: dir, args: append([]string(nil), args...)})
	switch args[0] {
	case "branch":
		return 0, g.currentBranch
	case "ls-remote":
		return 0, g.remoteRef
	case "rev-parse":
		return g.revParse(args[1:])
	case "status":
		return 0, g.statusOut
	case "fetch":
		return g.fetchCode, ""
	case "merge-base":
		if g.ancestor[args[len(args)-2]] {
			return 0, ""
		}
		return 1, ""
	case "merge-tree":
		return g.mergeTreeCode, g.mergeTreeOut
	case "merge":
		if len(args) > 1 && args[1] == "--abort" {
			g.aborted = true
			return 0, ""
		}
		if g.mergeCode == 0 && g.postMergeHead != "" {
			g.headOID = g.postMergeHead
		}
		return g.mergeCode, g.mergeOut
	case "ls-files":
		return 0, g.unmergedOut
	case "diff":
		return g.diffCheckCode, "f.go:1: leftover conflict marker"
	case "commit":
		return g.commitCode, ""
	case "push":
		return g.pushCode, g.pushOut
	}
	return 0, ""
}

func (g *fakeSyncBaseGit) revParse(args []string) (int, string) {
	switch target := args[len(args)-1]; target {
	case "MERGE_HEAD":
		if !g.mergeHead {
			return 1, ""
		}
		return 0, g.mergeHeadOID
	case "CHERRY_PICK_HEAD":
		if !g.cherryPickHead {
			return 1, ""
		}
		return 0, syncBaseBaseOID
	case "REBASE_HEAD":
		if !g.rebaseHead {
			return 1, ""
		}
		return 0, syncBaseBaseOID
	case "rebase-merge", "rebase-apply", "MERGE_MSG":
		return 0, filepath.Join(g.gitDir, target)
	case "HEAD":
		return 0, g.headOID
	case "FETCH_HEAD":
		return 0, g.fetchHeadOID
	default:
		return 0, ""
	}
}

func (g *fakeSyncBaseGit) verbs() []string {
	tracked := map[string]bool{"fetch": true, "merge-tree": true, "merge": true, "push": true, "commit": true}
	seen := []string{}
	for _, call := range g.calls {
		if tracked[call.args[0]] {
			seen = append(seen, call.args[0])
		}
	}
	return seen
}

func (g *fakeSyncBaseGit) callWith(verb string) []string {
	for _, call := range g.calls {
		if call.args[0] == verb {
			return call.args
		}
	}
	return nil
}

type syncBaseFixture struct {
	stateRoot string
	record    IssueOpsRecord
	worktree  string
	branch    string
	actor     model.NativeActor
	git       *fakeSyncBaseGit
}

// newSyncBaseFixture는 completion까지 끝난 뒤 같은 holder가 lease를 다시 쥔
// 상태(재claim 계약)를 만든다 — 설계 v2가 변형 3모드에 요구하는 전제다.
func newSyncBaseFixture(t *testing.T, branch string) syncBaseFixture {
	t.Helper()
	stateRoot := t.TempDir()
	claimable := newClaimableExecutionFixture(t, stateRoot, branch)
	prepareExecutionCompletionFixture(t, stateRoot, &claimable)
	actor := executionActor("codex", "sync-base-"+branch)
	if _, err := claimExecution(stateRoot, ExecutionClaimRequest{
		ID: claimable.record.ID, Generation: 1, Actor: actor,
		CWD: claimable.worktree, TokenFile: claimable.tokenPath,
	}); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(stateRoot, claimable.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Completion = &model.ExecutionCompletion{
		FinalHead:         syncBaseFinalHead,
		TuringReportPath:  filepath.Join(claimable.worktree, "turing-report.json"),
		Verification:      []string{"go test ./... -count=1"},
		RemoteArtifactURL: "https://github.com/example/agent-harness/pull/69",
		CompletedAt:       "2026-07-25T00:00:00Z",
	}
	record, err = writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	git := &fakeSyncBaseGit{
		gitDir:        t.TempDir(),
		currentBranch: branch,
		remoteRef:     syncBaseRemoteOID + "\trefs/heads/" + branch,
		headOID:       syncBaseWorkOID,
		fetchHeadOID:  syncBaseBaseOID,
		mergeHeadOID:  syncBaseBaseOID,
		ancestor:      map[string]bool{syncBaseRemoteOID: true},
	}
	return syncBaseFixture{stateRoot: stateRoot, record: record, worktree: claimable.worktree, branch: branch, actor: actor, git: git}
}

func (f syncBaseFixture) request(mode string) ExecutionSyncBaseRequest {
	return ExecutionSyncBaseRequest{ID: f.record.ID, Mode: mode, Actor: f.actor, CWD: f.worktree}
}

func (f syncBaseFixture) run(t *testing.T, req ExecutionSyncBaseRequest) (ExecutionSyncBaseResult, error) {
	t.Helper()
	return SyncExecutionBase(context.Background(), f.stateRoot, req, ExecutionSyncBaseDeps{Git: f.git.run})
}

func (f syncBaseFixture) rewrite(t *testing.T, mutate func(*IssueOpsRecord)) {
	t.Helper()
	record, err := ReadIssueOps(f.stateRoot, f.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&record)
	if _, err := writeIssueOps(f.stateRoot, record); err != nil {
		t.Fatal(err)
	}
}

// 게이트 10종 전수 거부. 설계 v2의 번호와 missing 토큰이 1:1로 대응한다.
func TestExecutionSyncBaseGatesRejectEveryMissingPrecondition(t *testing.T) {
	baseline := newSyncBaseFixture(t, "114-gates")
	cases := []struct {
		name    string
		mode    string
		mutate  func(*testing.T, *syncBaseFixture)
		missing string
	}{
		{"completion", ExecutionSyncBasePreview, func(t *testing.T, f *syncBaseFixture) {
			f.rewrite(t, func(r *IssueOpsRecord) { r.Execution.Completion = nil })
		}, "completion_present"},
		{"remote artifact", ExecutionSyncBasePreview, func(t *testing.T, f *syncBaseFixture) {
			f.rewrite(t, func(r *IssueOpsRecord) { r.RemoteArtifact = nil })
		}, "remote_artifact_present"},
		{"remote branch", ExecutionSyncBasePreview, func(_ *testing.T, f *syncBaseFixture) {
			f.git.remoteRef = ""
		}, "remote_branch_present"},
		{"pending intent", ExecutionSyncBasePreview, func(t *testing.T, f *syncBaseFixture) {
			f.rewrite(t, func(r *IssueOpsRecord) {
				r.Execution.Pending = &model.ExternalIntent{
					OperationID: "op-1", Kind: "orca", Marker: "m", StartedAt: "2026-07-25T00:00:00Z",
				}
			})
		}, "pending_intent_absent"},
		{"cwd canonical", ExecutionSyncBasePreview, nil, "cwd_canonical"},
		{"detached head", ExecutionSyncBasePreview, func(_ *testing.T, f *syncBaseFixture) {
			f.git.currentBranch = ""
		}, "head_on_recorded_branch"},
		{"base fetch", ExecutionSyncBasePreview, func(_ *testing.T, f *syncBaseFixture) {
			f.git.fetchCode = 1
		}, "base_fetch"},
		{"merge state clean", ExecutionSyncBaseApply, func(_ *testing.T, f *syncBaseFixture) {
			f.git.mergeHead = true
		}, "merge_state_clean"},
		{"worktree clean", ExecutionSyncBaseApply, func(_ *testing.T, f *syncBaseFixture) {
			f.git.statusOut = " M internal/x.go"
		}, "worktree_clean"},
		{"lease holder", ExecutionSyncBaseApply, nil, "lease_holder"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := baseline
			gitState := *baseline.git
			fixture.git = &gitState
			if _, err := writeIssueOps(fixture.stateRoot, baseline.record); err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(t, &fixture)
			}
			req := fixture.request(tc.mode)
			req.Confirm, req.Fingerprint = true, strings.Repeat("a", 64)
			switch tc.missing {
			case "cwd_canonical":
				req.CWD = t.TempDir()
			case "lease_holder":
				req.Actor = executionActor("codex", "not-the-holder")
			}
			result, err := fixture.run(t, req)
			if err == nil || !containsString(result.Missing, tc.missing) {
				t.Fatalf("expected missing %q: err=%v missing=%v", tc.missing, err, result.Missing)
			}
		})
	}

	t.Run("worktree present", func(t *testing.T) {
		fixture := baseline
		if err := os.RemoveAll(fixture.worktree); err != nil {
			t.Fatal(err)
		}
		result, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
		if err == nil || !containsString(result.Missing, "worktree_present") {
			t.Fatalf("absent canonical worktree must fail closed: %v %v", err, result.Missing)
		}
	})
}

// preview는 released·비-holder에서도 진단 채널로 열려 있어야 하고, 예상 충돌
// 파일과 fingerprint를 함께 발급해야 한다.
func TestExecutionSyncBasePreviewReportsConflictsAndIssuesFingerprint(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-preview")
	fixture.rewrite(t, func(r *IssueOpsRecord) {
		r.Execution.Lease.Status = model.LeaseStatusReleased
		r.Execution.Lease.Holder = nil
		r.Execution.Lease.ReleasedAt = "2026-07-25T00:00:00Z"
	})
	fixture.git.mergeTreeCode = 1
	fixture.git.mergeTreeOut = "treeoid\x00internal/a.go\x00internal/b.go\x00\x00CONFLICT (content)\n"

	req := fixture.request(ExecutionSyncBasePreview)
	req.Actor = model.NativeActor{}
	result, err := fixture.run(t, req)
	if err != nil {
		t.Fatalf("released preview must stay open as a diagnosis channel: %v", err)
	}
	if len(result.Fingerprint) != 64 || !result.MergeNeeded || !result.RemoteBranchPresent {
		t.Fatalf("preview did not expose the merge inventory: %#v", result)
	}
	if len(result.ConflictFiles) != 2 || result.ConflictFiles[0] != "internal/a.go" {
		t.Fatalf("preview did not list predicted conflicts: %#v", result.ConflictFiles)
	}
	if !strings.Contains(result.NextCommand, "--apply --confirm --fingerprint "+result.Fingerprint) {
		t.Fatalf("preview must hand over one finite apply command: %q", result.NextCommand)
	}
	for _, verb := range fixture.git.verbs() {
		if verb == "merge" || verb == "push" || verb == "commit" {
			t.Fatalf("preview must not touch the worktree: %v", fixture.git.verbs())
		}
	}
}

// 무충돌 fast 경로: fetch→merge-tree→merge→push 순서와 인자를 전수 검증한다.
func TestExecutionSyncBaseApplyRunsFetchMergePushInOrder(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-apply")
	preview, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.calls = nil
	fixture.git.postMergeHead = syncBaseMergeOID

	req := fixture.request(ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err != nil {
		t.Fatalf("clean fast path must complete: %v", err)
	}
	if !result.Merged || !result.Pushed || result.MergeCommit != syncBaseMergeOID {
		t.Fatalf("apply did not complete the merge and push: %#v", result)
	}
	want := []string{"fetch", "merge-tree", "merge", "push"}
	got := fixture.git.verbs()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("git sequence = %v, want %v", got, want)
	}
	if args := fixture.git.callWith("fetch"); strings.Join(args, " ") != "fetch --quiet origin main" {
		t.Fatalf("fetch must precede the merge with the recorded base branch: %v", args)
	}
	if args := fixture.git.callWith("merge-tree"); strings.Join(args, " ") !=
		"merge-tree --write-tree --name-only -z "+syncBaseWorkOID+" "+syncBaseBaseOID {
		t.Fatalf("merge-tree args = %v", args)
	}
	if args := fixture.git.callWith("merge"); strings.Join(args, " ") != "merge --no-ff --no-edit "+syncBaseBaseOID {
		t.Fatalf("merge must be a non-fast-forward merge of the fetched base: %v", args)
	}
	if args := fixture.git.callWith("push"); strings.Join(args, " ") !=
		"push origin refs/heads/"+fixture.branch+":refs/heads/"+fixture.branch {
		t.Fatalf("push must be an explicit non-forced refspec: %v", args)
	}
}

// 충돌은 merge-in-progress를 남기고 정지한다 — push도 이벤트도 없다.
func TestExecutionSyncBaseApplyStopsAtConflictWithoutPush(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-conflict")
	preview, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.mergeTreeCode, fixture.git.mergeTreeOut = 1, "treeoid\x00internal/a.go\x00\x00"
	fixture.git.mergeCode, fixture.git.mergeOut = 1, "CONFLICT (content): Merge conflict in internal/a.go"

	req := fixture.request(ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err != nil {
		t.Fatalf("conflict stop is an actionable outcome, not a gate failure: %v", err)
	}
	if result.Merged || result.Pushed || !result.MergeInProgress || len(result.ConflictFiles) != 1 {
		t.Fatalf("conflict stop did not preserve the merge-in-progress contract: %#v", result)
	}
	if !strings.Contains(result.NextCommand, "--finalize") {
		t.Fatalf("conflict stop must name the resolution command: %q", result.NextCommand)
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Execution.SyncBaseEvents) != 0 {
		t.Fatalf("conflict stop must not record a durable event: %#v", persisted.Execution.SyncBaseEvents)
	}
}

// push 실패는 로컬 merge commit을 남기고, 재실행은 merge 없이 push만 수행해
// 멱등 수렴한다(설계 v2 push 계약).
func TestExecutionSyncBaseApplyPushFailureConvergesIdempotently(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-push-retry")
	preview, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.postMergeHead = syncBaseMergeOID
	fixture.git.pushCode, fixture.git.pushOut = 1, "! [rejected] non-fast-forward"

	req := fixture.request(ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err == nil || result.FailedStep != "push" || !result.PushRetryRequired {
		t.Fatalf("push failure must surface as a typed error: err=%v result=%#v", err, result)
	}

	// 병합은 이미 반영됐다 — 재preview는 merge 불필요 + ahead를 보고해야 한다.
	fixture.git.ancestor[syncBaseBaseOID] = true
	fixture.git.pushCode, fixture.git.pushOut = 0, ""
	fixture.git.calls = nil
	retryPreview, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	if retryPreview.MergeNeeded || !retryPreview.PushRetryRequired {
		t.Fatalf("preview must report push-only convergence: %#v", retryPreview)
	}
	fixture.git.calls = nil
	retry := fixture.request(ExecutionSyncBaseApply)
	retry.Confirm, retry.Fingerprint = true, retryPreview.Fingerprint
	final, err := fixture.run(t, retry)
	if err != nil {
		t.Fatalf("apply re-run must converge: %v", err)
	}
	if !final.Pushed || final.Merged {
		t.Fatalf("apply re-run must skip the merge and only push: %#v", final)
	}
	for _, verb := range fixture.git.verbs() {
		if verb == "merge" || verb == "merge-tree" {
			t.Fatalf("re-run must not merge again: %v", fixture.git.verbs())
		}
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Execution.SyncBaseEvents) != 1 {
		t.Fatalf("exactly one durable event must survive the retry: %#v", persisted.Execution.SyncBaseEvents)
	}
}

// 성공한 apply는 durable 이벤트를 남기고 Completion.FinalHead는 불변이다.
func TestExecutionSyncBaseRecordsDurableEventAndKeepsFinalHeadImmutable(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-durable")
	preview, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	fixture.git.postMergeHead = syncBaseMergeOID
	req := fixture.request(ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	if _, err := fixture.run(t, req); err != nil {
		t.Fatal(err)
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	events := persisted.Execution.SyncBaseEvents
	if len(events) != 1 {
		t.Fatalf("apply must append exactly one durable event: %#v", events)
	}
	event := events[0]
	if event.Mode != model.ExecutionSyncBaseEventApply || event.BaseOID != syncBaseBaseOID ||
		event.MergeCommit != syncBaseMergeOID || event.BaseBranch != "main" ||
		!strings.Contains(event.Actor, "sync-base-114-durable") || strings.TrimSpace(event.At) == "" {
		t.Fatalf("durable event is not the merge receipt: %#v", event)
	}
	if persisted.Execution.Completion.FinalHead != syncBaseFinalHead {
		t.Fatalf("Completion.FinalHead must stay immutable: %q", persisted.Execution.Completion.FinalHead)
	}
}

// finalize는 미해소 인덱스와 잔존 충돌 마커를 각각 거부한다.
func TestExecutionSyncBaseFinalizeRejectsUnresolvedConflictsAndMarkers(t *testing.T) {
	t.Run("unmerged index", func(t *testing.T) {
		fixture := newSyncBaseFixture(t, "114-finalize-unmerged")
		fixture.git.mergeHead = true
		fixture.git.unmergedOut = "100644 " + syncBaseBaseOID + " 1\tinternal/a.go\x00"
		result, err := fixture.run(t, fixture.request(ExecutionSyncBaseFinalize))
		if err == nil || !containsString(result.Missing, "conflict_resolution_complete") {
			t.Fatalf("unresolved paths must block finalize: %v %v", err, result.Missing)
		}
		if fixture.git.callWith("commit") != nil || fixture.git.callWith("push") != nil {
			t.Fatal("blocked finalize must not commit or push")
		}
	})
	t.Run("conflict markers", func(t *testing.T) {
		fixture := newSyncBaseFixture(t, "114-finalize-markers")
		fixture.git.mergeHead = true
		fixture.git.diffCheckCode = 1
		result, err := fixture.run(t, fixture.request(ExecutionSyncBaseFinalize))
		if err == nil || !containsString(result.Missing, "conflict_markers_absent") {
			t.Fatalf("leftover conflict markers must block finalize: %v %v", err, result.Missing)
		}
	})
	t.Run("clean finalize", func(t *testing.T) {
		fixture := newSyncBaseFixture(t, "114-finalize-clean")
		fixture.git.mergeHead = true
		fixture.git.headOID = syncBaseMergeOID
		result, err := fixture.run(t, fixture.request(ExecutionSyncBaseFinalize))
		if err != nil {
			t.Fatalf("resolved finalize must complete: %v", err)
		}
		if !result.Merged || !result.Pushed {
			t.Fatalf("finalize did not commit and push: %#v", result)
		}
		// finalize는 재fetch하지 않는다 — base tip은 MERGE_HEAD가 확정한다.
		if fixture.git.callWith("fetch") != nil {
			t.Fatal("finalize must not re-fetch the base")
		}
		persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted.Execution.SyncBaseEvents) != 1 ||
			persisted.Execution.SyncBaseEvents[0].Mode != model.ExecutionSyncBaseEventFinalize {
			t.Fatalf("finalize must record its own durable event: %#v", persisted.Execution.SyncBaseEvents)
		}
	})
}

func TestExecutionSyncBaseAbortWithdrawsTheMergeWithoutEvent(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-abort")
	fixture.git.mergeHead = true
	result, err := fixture.run(t, fixture.request(ExecutionSyncBaseAbort))
	if err != nil {
		t.Fatalf("abort must be available to the holder: %v", err)
	}
	if !result.Aborted || result.MergeInProgress || !fixture.git.aborted {
		t.Fatalf("abort did not withdraw the merge: %#v", result)
	}
	persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Execution.SyncBaseEvents) != 0 {
		t.Fatalf("abort is a withdrawal and must not be recorded: %#v", persisted.Execution.SyncBaseEvents)
	}

	// 진행 중 머지가 없으면 abort/finalize 모두 전제 미충족이다.
	clean := newSyncBaseFixture(t, "114-abort-noop")
	result, err = clean.run(t, clean.request(ExecutionSyncBaseAbort))
	if err == nil || !containsString(result.Missing, "merge_in_progress") {
		t.Fatalf("abort without a merge in progress must fail closed: %v %v", err, result.Missing)
	}
}

// fingerprint TOCTOU: preview 발급 이후 상태가 바뀌면 apply가 멈춘다.
func TestExecutionSyncBaseApplyRejectsStaleFingerprint(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-toctou")
	preview, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	// 외부에서 base가 전진했다 — 같은 fingerprint로는 진행할 수 없다.
	fixture.git.fetchHeadOID = syncBaseMergeOID
	req := fixture.request(ExecutionSyncBaseApply)
	req.Confirm, req.Fingerprint = true, preview.Fingerprint
	result, err := fixture.run(t, req)
	if err == nil || !strings.Contains(err.Error(), "stale execution sync-base fingerprint") {
		t.Fatalf("stale fingerprint was accepted: %v %#v", err, result)
	}
	if fixture.git.callWith("merge") != nil {
		t.Fatal("stale fingerprint must stop before any merge")
	}

	missingConfirm := fixture.request(ExecutionSyncBaseApply)
	missingConfirm.Fingerprint = preview.Fingerprint
	if _, err := fixture.run(t, missingConfirm); err == nil {
		t.Fatal("apply without --confirm must be rejected")
	}
}

// git 2.38 미만 등으로 merge-tree --write-tree가 없으면 fail-closed다.
func TestExecutionSyncBaseFailsClosedWhenMergeTreeIsUnavailable(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-mergetree")
	fixture.git.mergeTreeCode, fixture.git.mergeTreeOut = 129, "unknown option `write-tree'"
	result, err := fixture.run(t, fixture.request(ExecutionSyncBasePreview))
	if err == nil || !containsString(result.Missing, "merge_tree_supported") {
		t.Fatalf("unsupported merge-tree must fail closed in preview: %v %v", err, result.Missing)
	}
}

// 변형 3모드는 활성 holder 필수 — released·claimable·비-holder 전부 거부.
func TestExecutionSyncBaseMutatingModesRequireTheActiveHolder(t *testing.T) {
	for _, mode := range []string{ExecutionSyncBaseApply, ExecutionSyncBaseFinalize, ExecutionSyncBaseAbort} {
		t.Run(mode, func(t *testing.T) {
			fixture := newSyncBaseFixture(t, "114-holder-"+mode)
			fixture.git.mergeHead = true
			fixture.rewrite(t, func(r *IssueOpsRecord) {
				r.Execution.Lease.Status = model.LeaseStatusReleased
				r.Execution.Lease.Holder = nil
				r.Execution.Lease.ReleasedAt = "2026-07-25T00:00:00Z"
			})
			req := fixture.request(mode)
			req.Confirm, req.Fingerprint = true, strings.Repeat("a", 64)
			result, err := fixture.run(t, req)
			if err == nil || !containsString(result.Missing, "lease_holder") {
				t.Fatalf("%s from a released lease must be denied: %v %v", mode, err, result.Missing)
			}
		})
	}
}

func TestExecutionSyncBaseRejectsUnknownMode(t *testing.T) {
	fixture := newSyncBaseFixture(t, "114-mode")
	if _, err := fixture.run(t, fixture.request("rebase")); err == nil {
		t.Fatal("unsupported mode must be rejected; rebase is explicitly out of scope")
	}
}
