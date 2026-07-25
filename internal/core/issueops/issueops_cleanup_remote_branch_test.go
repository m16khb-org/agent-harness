package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

const (
	remoteBranchTestBranch    = "116-remote-branch-delete"
	remoteBranchTestArtifact  = "https://github.com/acme/repo/pull/117"
	remoteBranchTestOrigin    = "https://github.com/acme/repo.git"
	remoteBranchTestHeadOID   = "1111111111111111111111111111111111111111"
	remoteBranchTestTagOID    = "2222222222222222222222222222222222222222"
	remoteBranchTestPushedOID = "3333333333333333333333333333333333333333"
)

// fakeRemoteBranchGit은 원격 관측·삭제만 흉내내며, fully-qualified 질의가
// 아니면 동명 태그를 돌려주고 삭제도 거부한다 — 실서버의 ref 모호성 방어를
// 테스트 안으로 끌어온다.
type fakeRemoteBranchGit struct {
	remoteOID    string
	lsRemoteFail bool
	originURL    string
	pushFail     bool
	pushArgs     []string
	pushes       int
}

func (g *fakeRemoteBranchGit) run(_ context.Context, _ string, args ...string) (int, string) {
	switch args[0] {
	case "ls-remote":
		if g.lsRemoteFail {
			return 128, "fatal: 'origin' does not appear to be a git repository"
		}
		if !containsString(args, "--heads") || !containsString(args, "refs/heads/"+remoteBranchTestBranch) {
			return 0, remoteBranchTestTagOID + "\trefs/tags/" + remoteBranchTestBranch + "\n"
		}
		if g.remoteOID == "" {
			return 0, ""
		}
		return 0, g.remoteOID + "\trefs/heads/" + remoteBranchTestBranch + "\n"
	case "remote":
		if g.originURL == "" {
			return 128, "fatal: No such remote 'origin'"
		}
		return 0, g.originURL + "\n"
	case "push":
		g.pushes++
		g.pushArgs = append([]string{}, args...)
		if !containsString(args, "refs/heads/"+remoteBranchTestBranch) {
			return 1, "error: dst refspec " + remoteBranchTestBranch + " matches more than one"
		}
		if g.pushFail {
			return 1, "! [remote rejected] " + remoteBranchTestBranch + " (stale info)"
		}
		g.remoteOID = ""
		return 0, ""
	}
	return 0, ""
}

func remoteBranchTestRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: remoteBranchTestBranch})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = IssueOpsPhaseDone
	record.IssueURL = "https://github.com/acme/repo/issues/116"
	record.BranchPrepare = &IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL,
		Branch: remoteBranchTestBranch, BaseBranch: "main",
	}
	record.RemoteArtifact = &IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: remoteBranchTestArtifact,
	}
	record.Execution = &Execution{
		Mode: model.ExecutionModeDirect,
		Workspace: Workspace{
			SourceRoot: repo, Root: filepath.Join(repo, "wt"), Branch: remoteBranchTestBranch,
			BaseHead: "deadbeef", Driver: "git", LinkedAt: "2026-07-25T00:00:00Z",
		},
		Lease: WriteLease{Generation: 1, Status: model.LeaseStatusReleased},
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) { *rec = record })
	return stateRoot, record
}

func remoteBranchGit() *fakeRemoteBranchGit {
	return &fakeRemoteBranchGit{remoteOID: remoteBranchTestHeadOID, originURL: remoteBranchTestOrigin}
}

func remoteBranchDeps(git *fakeRemoteBranchGit) CleanupRemoteBranchDeps {
	return CleanupRemoteBranchDeps{
		Git: git.run,
		VerifyMergedArtifact: func(IssueOpsRemoteArtifactVerification) (CleanupRemoteBranchArtifactHead, error) {
			return CleanupRemoteBranchArtifactHead{
				HeadRefName: remoteBranchTestBranch, HeadRefOID: remoteBranchTestHeadOID,
			}, nil
		},
	}
}

func remoteBranchRequest(id string, apply bool, fingerprint string) CleanupRemoteBranchRequest {
	return CleanupRemoteBranchRequest{ID: id, Apply: apply, Confirm: apply, Fingerprint: fingerprint}
}

// AC-01: preview → apply로 원격 브랜치가 삭제되고, 부재 상태 재실행은 이전
// fingerprint를 그대로 들고 있어도 stale 거부 없이 멱등 성공한다.
func TestCleanupRemoteBranchPreviewApplyDeletesAndRerunIsIdempotent(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)

	preview, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.RemoteBranchPresent || preview.Fingerprint == "" || preview.RemoteOID != remoteBranchTestHeadOID {
		t.Fatalf("preview must observe the remote branch and issue a fingerprint: %+v", preview)
	}
	if preview.Deleted || preview.AlreadyAbsent {
		t.Fatalf("preview must not mutate: %+v", preview)
	}

	applied, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Deleted || applied.AlreadyAbsent || git.pushes != 1 {
		t.Fatalf("apply must delete the remote branch exactly once: %+v pushes=%d", applied, git.pushes)
	}

	// 부재 재실행: 관측이 absent이므로 fingerprint stale 검사 이전에 성공한다.
	rerun, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatalf("absent rerun must be idempotently successful: %v", err)
	}
	if rerun.Deleted || !rerun.AlreadyAbsent || rerun.RemoteBranchPresent || git.pushes != 1 {
		t.Fatalf("absent rerun must not push again: %+v pushes=%d", rerun, git.pushes)
	}
}

// AC-02: 삭제 명령은 fully-qualified ref와 관측 OID에 결속된 force-with-lease를
// 사용한다(동명 태그 보호 + preview→push 사이 TOCTOU 봉쇄).
func TestCleanupRemoteBranchDeleteUsesFullyQualifiedRefAndForceWithLease(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)

	preview, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"push", "origin", "--delete", "refs/heads/" + remoteBranchTestBranch,
		"--force-with-lease=refs/heads/" + remoteBranchTestBranch + ":" + remoteBranchTestHeadOID,
	}
	if strings.Join(git.pushArgs, " ") != strings.Join(want, " ") {
		t.Fatalf("delete command literal drifted:\n got: %v\nwant: %v", git.pushArgs, want)
	}
}

// AC-02: fail-closed 9종 전수.
func TestCleanupRemoteBranchFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*IssueOpsRecord)
		deps    func(*CleanupRemoteBranchDeps)
		git     func(*fakeRemoteBranchGit)
		missing string
	}{
		{
			name: "unmerged artifact",
			deps: func(d *CleanupRemoteBranchDeps) {
				d.VerifyMergedArtifact = func(IssueOpsRemoteArtifactVerification) (CleanupRemoteBranchArtifactHead, error) {
					return CleanupRemoteBranchArtifactHead{}, fmt.Errorf("remote artifact is not verified merged")
				}
			},
			missing: "remote_artifact_merged",
		},
		{
			name: "readback failure",
			deps: func(d *CleanupRemoteBranchDeps) {
				d.VerifyMergedArtifact = func(IssueOpsRemoteArtifactVerification) (CleanupRemoteBranchArtifactHead, error) {
					return CleanupRemoteBranchArtifactHead{}, fmt.Errorf("gh: HTTP 503")
				}
			},
			missing: "remote_artifact_merged",
		},
		{
			name: "tip pushed after merge",
			git:  func(g *fakeRemoteBranchGit) { g.remoteOID = remoteBranchTestPushedOID },
			// 관측 원격 OID가 artifact headRefOid와 다르면 머지 이후 push된
			// 커밋이 있다는 뜻이므로 삭제하지 않는다.
			missing: "remote_tip_equals_merged_head",
		},
		{
			name: "artifact head is another branch",
			deps: func(d *CleanupRemoteBranchDeps) {
				d.VerifyMergedArtifact = func(IssueOpsRemoteArtifactVerification) (CleanupRemoteBranchArtifactHead, error) {
					return CleanupRemoteBranchArtifactHead{
						HeadRefName: "999-other-branch", HeadRefOID: remoteBranchTestHeadOID,
					}, nil
				}
			},
			missing: "artifact_head_branch_match",
		},
		{
			name:    "origin unreadable",
			git:     func(g *fakeRemoteBranchGit) { g.lsRemoteFail = true; g.originURL = "" },
			missing: "remote_branch_readable",
		},
		{
			name:    "origin project mismatch",
			git:     func(g *fakeRemoteBranchGit) { g.originURL = "https://github.com/other/repo.git" },
			missing: "remote_identity_match",
		},
		{
			name: "lease still active",
			mutate: func(rec *IssueOpsRecord) {
				rec.Execution.Lease.Status = model.LeaseStatusActive
				rec.Execution.Lease.ClaimedAt = "2026-07-25T00:00:00Z"
				rec.Execution.Lease.Holder = &NativeActor{
					Host: "claude", SessionID: "s",
					SessionProcess: &NativeProcessReceipt{
						PID: 1234, StartedAt: "2026-07-25T00:00:00Z", Executable: "claude",
					},
				}
			},
			missing: "lease_released",
		},
		{
			name:    "phase not done",
			mutate:  func(rec *IssueOpsRecord) { rec.Phase = IssueOpsPhasePR },
			missing: "phase_done",
		},
		{
			name:    "branch equals base",
			mutate:  func(rec *IssueOpsRecord) { rec.BranchPrepare.BaseBranch = remoteBranchTestBranch },
			missing: "branch_not_base",
		},
		{
			name: "unclosed child",
			mutate: func(rec *IssueOpsRecord) {
				rec.IssueLinks = append(rec.IssueLinks, IssueOpsIssueLink{
					Type: "child", URL: "https://github.com/acme/repo/issues/118",
				})
			},
			missing: "child_tasks_closed",
		},
		{
			name:    "remote artifact absent",
			mutate:  func(rec *IssueOpsRecord) { rec.RemoteArtifact = nil },
			missing: "remote_artifact_present",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := remoteBranchTestRecord(t)
			if tc.mutate != nil {
				mutateFinishRecord(t, stateRoot, record.ID, tc.mutate)
			}
			git := remoteBranchGit()
			if tc.git != nil {
				tc.git(git)
			}
			deps := remoteBranchDeps(git)
			if tc.deps != nil {
				tc.deps(&deps)
			}
			result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, "any"), deps)
			if err == nil || !containsString(result.Missing, tc.missing) {
				t.Fatalf("expected missing %q: err=%v missing=%v", tc.missing, err, result.Missing)
			}
			if git.pushes != 0 {
				t.Fatalf("a blocked gate must never push: %v", git.pushArgs)
			}
		})
	}
}

// AC-02: preview 이후 원격 tip이 바뀌면 apply는 stale fingerprint로 멈춘다.
func TestCleanupRemoteBranchApplyRejectsStaleFingerprint(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	preview, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint+"0"), deps); err == nil ||
		!strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("stale fingerprint must be rejected: %v", err)
	}
	if git.pushes != 0 {
		t.Fatalf("stale fingerprint must not push: %v", git.pushArgs)
	}
	req := remoteBranchRequest(record.ID, true, preview.Fingerprint)
	req.Confirm = false
	if _, err := CleanupRemoteBranch(context.Background(), stateRoot, req, deps); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("apply without confirm must be rejected: %v", err)
	}
}

// AC-01 보강: push 실패는 레코드를 바꾸지 않고 실패 지점을 남긴 채 끝난다.
func TestCleanupRemoteBranchPushFailureSurfacesFailedStep(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	git.pushFail = true
	deps := remoteBranchDeps(git)
	preview, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != "remote_branch_delete" || result.Deleted {
		t.Fatalf("push failure must be surfaced without claiming deletion: %v %+v", err, result)
	}
}

// 감사(brooks M12): apply 성공 시 브랜치·OID·시각이 이슈 본문 감사 라인으로
// 멱등 반영되고, 반영 실패는 삭제를 되돌리지 않되 결과에 드러난다.
func TestCleanupRemoteBranchApplyReflectsAuditLine(t *testing.T) {
	stateRoot, record := remoteBranchTestRecord(t)
	git := remoteBranchGit()
	deps := remoteBranchDeps(git)
	var seen string
	deps.ReflectAudit = func(_ IssueOpsRecord, _ port.IssueProviderCompletionSection, audit string) error {
		seen = audit
		return nil
	}
	preview, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupRemoteBranch(context.Background(), stateRoot, remoteBranchRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AuditReflected || result.AuditError != "" {
		t.Fatalf("audit must be reflected: %+v", result)
	}
	if !strings.Contains(seen, remoteBranchTestBranch) || !strings.Contains(seen, remoteBranchTestHeadOID) || !strings.Contains(seen, result.DeletedAt) {
		t.Fatalf("audit line must carry branch, oid and time: %q", seen)
	}

	stateRoot2, record2 := remoteBranchTestRecord(t)
	git2 := remoteBranchGit()
	deps2 := remoteBranchDeps(git2)
	deps2.ReflectAudit = func(IssueOpsRecord, port.IssueProviderCompletionSection, string) error {
		return fmt.Errorf("provider unavailable")
	}
	preview2, err := CleanupRemoteBranch(context.Background(), stateRoot2, remoteBranchRequest(record2.ID, false, ""), deps2)
	if err != nil {
		t.Fatal(err)
	}
	result2, err := CleanupRemoteBranch(context.Background(), stateRoot2, remoteBranchRequest(record2.ID, true, preview2.Fingerprint), deps2)
	if err != nil {
		t.Fatal(err)
	}
	if !result2.Deleted || result2.AuditReflected || !strings.Contains(result2.AuditError, "provider unavailable") {
		t.Fatalf("audit failure must be surfaced without undoing the deletion: %+v", result2)
	}
}
