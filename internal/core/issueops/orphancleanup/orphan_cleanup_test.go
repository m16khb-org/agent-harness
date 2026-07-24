package orphancleanup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	coreissueops "agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	corehealth "agent-harness/internal/core/operationalhealth"
)

func TestPreviewFailsClosedForUnsafeRecordlessTargets(t *testing.T) {
	fixture := newOrphanCleanupGitFixture(t)
	request := fixture.request()

	tests := []struct {
		name    string
		prepare func(corehealth.Snapshot) corehealth.Snapshot
		verify  func(model.IssueOpsRemoteArtifactVerification) error
		want    string
	}{
		{
			name: "dirty worktree",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				if err := os.WriteFile(filepath.Join(fixture.worktree, "dirty.txt"), []byte("dirty\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				return snapshot
			},
			want: "worktree_dirty",
		},
		{
			name: "canonical main worktree",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				return corehealth.Snapshot{
					RepoRoot:        fixture.repo,
					CanonicalBranch: "main",
					SourceHead:      fixture.sourceHead,
					SourceClean:     true,
					GitWorktrees: []corehealth.GitWorktree{{
						Path: fixture.repo, Branch: "main", Head: fixture.sourceHead, Clean: true, Canonical: true,
					}},
					LocalRefs: []corehealth.GitRef{{Name: "refs/heads/main", Branch: "main", OID: fixture.sourceHead, Location: "local"}},
					Messages:  corehealth.MessagePresence{CompleteAbsence: true},
				}
			},
			want: "canonical_worktree",
		},
		{
			name: "branch mismatch",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				return snapshot
			},
			want: "branch_mismatch",
		},
		{
			name: "duplicate target inventory",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				snapshot.GitWorktrees = append(snapshot.GitWorktrees, snapshot.GitWorktrees[1])
				return snapshot
			},
			want: "target_worktree_count",
		},
		{
			name: "preserved lifecycle owns target",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				snapshot.Cycles = append(snapshot.Cycles, corehealth.Cycle{
					ID:                  "io-live-owner",
					Repo:                fixture.repo,
					Branch:              fixture.branch,
					Phase:               "implement",
					ExecutionMode:       "direct",
					LeaseStatus:         "active",
					Generation:          1,
					WorktreePath:        fixture.worktree,
					HolderHost:          "codex",
					HolderSessionID:     "session",
					HolderPID:           1,
					HolderStartedAt:     "2026-07-24T00:00:00Z",
					HolderExecutable:    "codex",
					HolderProcessStatus: corehealth.ProcessStatusLive,
				})
				return snapshot
			},
			want: "target_lifecycle_owner",
		},
		{
			name: "lifecycle missing target path remains authority unknown",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				snapshot.Cycles = append(snapshot.Cycles, corehealth.Cycle{
					ID: "io-owner-without-worktree", Repo: fixture.repo, Branch: fixture.branch,
					Phase: "implement", ExecutionMode: "direct", LeaseStatus: "active", Generation: 1,
				})
				return snapshot
			},
			want: "target_lifecycle_authority_unknown",
		},
		{
			name: "lease holder index claims target lifecycle id",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				snapshot.LeaseHolderIndexes = append(snapshot.LeaseHolderIndexes, corehealth.LeaseHolderIndex{
					Key: "active-holder", LifecycleID: request.ID, Generation: 1, Host: "codex", SessionID: "session",
				})
				return snapshot
			},
			want: "target_lease_authority",
		},
		{
			name: "merged remote evidence unavailable",
			prepare: func(snapshot corehealth.Snapshot) corehealth.Snapshot {
				return snapshot
			},
			verify: func(model.IssueOpsRemoteArtifactVerification) error {
				return errors.New("provider readback unavailable")
			},
			want: "remote_artifact_merged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "dirty worktree" {
				t.Cleanup(func() {
					if err := os.Remove(filepath.Join(fixture.worktree, "dirty.txt")); err != nil && !errors.Is(err, os.ErrNotExist) {
						t.Fatal(err)
					}
				})
			}
			current := fixture.snapshot()
			deps := fixture.deps(func(context.Context, string) (corehealth.Snapshot, error) {
				return tt.prepare(current), nil
			}, tt.verify)
			candidate := request
			if tt.name == "canonical main worktree" {
				candidate.WorktreePath = fixture.repo
				candidate.Branch = "main"
			} else if tt.name == "branch mismatch" {
				candidate.Branch = "another-branch"
			}

			result, err := Preview(context.Background(), candidate, deps)
			if err != nil {
				t.Fatalf("Preview() error = %v", err)
			}
			if result.Ready || !contains(result.Missing, tt.want) {
				t.Fatalf("Preview() = %+v, want blocked by %q", result, tt.want)
			}
		})
	}
}

func TestPreviewAndApplyRemoveOnlyConfirmedRecordlessLocalArtifacts(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	fixture := newOrphanCleanupGitFixture(t)
	request := fixture.request()
	collectCalls := 0
	deps := fixture.deps(func(context.Context, string) (corehealth.Snapshot, error) {
		collectCalls++
		return fixture.snapshot(), nil
	}, nil)

	preview, err := Preview(context.Background(), request, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Ready || preview.Fingerprint == "" || preview.HeadSHA == "" || preview.RecoveryPath == "" {
		t.Fatalf("preview must carry confirmed cleanup and recovery evidence: %+v", preview)
	}
	if !preview.RecordAbsent || !preview.RemoteMerged || preview.RemoteBranchDeletion == "" {
		t.Fatalf("preview must prove the recordless remote-merged boundary: %+v", preview)
	}

	applied, err := Apply(context.Background(), request, ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint}, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || !applied.LocalWorktreeRemoved || !applied.LocalBranchRemoved {
		t.Fatalf("Apply() = %+v, want both local artifacts removed", applied)
	}
	if collectCalls < 2 {
		t.Fatalf("apply must re-read inventory, collect calls = %d", collectCalls)
	}
	if _, err := os.Stat(fixture.worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree still exists after apply: %v", err)
	}
	if got := gitOutput(t, fixture.repo, "branch", "--list", fixture.branch); strings.TrimSpace(got) != "" {
		t.Fatalf("local branch still exists after apply: %q", got)
	}
	if got := gitOutput(t, fixture.repo, "ls-remote", "--heads", "origin", fixture.branch); strings.TrimSpace(got) == "" {
		t.Fatal("remote branch was removed or unavailable; orphan cleanup must not delete it")
	}
	ids, err := coreissueops.ListIssueOpsIDs(coreissueops.IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("recordless cleanup must not create an IssueOps lifecycle: %v", ids)
	}
}

func TestApplyRejectsStalePreviewFingerprint(t *testing.T) {
	fixture := newOrphanCleanupGitFixture(t)
	request := fixture.request()
	deps := fixture.deps(func(context.Context, string) (corehealth.Snapshot, error) {
		return fixture.snapshot(), nil
	}, nil)

	preview, err := Preview(context.Background(), request, deps)
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, fixture.worktree, "commit", "--allow-empty", "-m", "advance feature after preview")

	result, err := Apply(context.Background(), request, ApplyRequest{Confirm: true, Fingerprint: preview.Fingerprint}, deps)
	if err == nil || !strings.Contains(err.Error(), "stale preview fingerprint") {
		t.Fatalf("Apply() error = %v, want stale fingerprint", err)
	}
	if result.Applied {
		t.Fatalf("stale apply must not mutate: %+v", result)
	}
	if _, err := os.Stat(fixture.worktree); err != nil {
		t.Fatalf("stale apply removed worktree: %v", err)
	}
}

type orphanCleanupGitFixture struct {
	repo       string
	worktree   string
	branch     string
	sourceHead string
}

func newOrphanCleanupGitFixture(t *testing.T) orphanCleanupGitFixture {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "feature-worktree")
	gitRun(t, "", "init", "--bare", remote)
	gitRun(t, "", "init", "-b", "main", repo)
	gitRun(t, repo, "config", "user.name", "Test User")
	gitRun(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "README.md")
	gitRun(t, repo, "commit", "-m", "base")
	gitRun(t, repo, "remote", "add", "origin", remote)
	gitRun(t, repo, "push", "-u", "origin", "main")
	branch := "merged-feature"
	gitRun(t, repo, "branch", branch)
	gitRun(t, repo, "worktree", "add", worktree, branch)
	gitRun(t, worktree, "push", "-u", "origin", branch)
	return orphanCleanupGitFixture{
		repo:       repo,
		worktree:   worktree,
		branch:     branch,
		sourceHead: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}
}

func (fixture orphanCleanupGitFixture) request() Request {
	return Request{
		ID:           "io-f4e347fe9827",
		RepoRoot:     fixture.repo,
		WorktreePath: fixture.worktree,
		Branch:       fixture.branch,
		Artifact: model.IssueOpsRemoteArtifactVerification{
			Provider: "github",
			Kind:     "pr",
			URL:      "https://github.com/example/repo/pull/42",
		},
	}
}

func (fixture orphanCleanupGitFixture) deps(collect func(context.Context, string) (corehealth.Snapshot, error), verify func(model.IssueOpsRemoteArtifactVerification) error) Dependencies {
	if verify == nil {
		verify = func(model.IssueOpsRemoteArtifactVerification) error { return nil }
	}
	return Dependencies{Collect: collect, VerifyMerged: verify}
}

func (fixture orphanCleanupGitFixture) snapshot() corehealth.Snapshot {
	featureHead := strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", fixture.branch))
	return corehealth.Snapshot{
		RepoRoot:        fixture.repo,
		CanonicalBranch: "main",
		SourceHead:      strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "main")),
		SourceClean:     true,
		GitWorktrees: []corehealth.GitWorktree{
			{Path: fixture.repo, Branch: "main", Head: strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "main")), Clean: true, Canonical: true},
			{Path: fixture.worktree, Branch: fixture.branch, Head: featureHead, Clean: true},
		},
		LocalRefs: []corehealth.GitRef{
			{Name: "refs/heads/main", Branch: "main", OID: strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "main")), Location: "local"},
			{Name: "refs/heads/" + fixture.branch, Branch: fixture.branch, OID: featureHead, Location: "local"},
		},
		RemoteRefs: []corehealth.GitRef{
			{Name: "refs/heads/main", Branch: "main", OID: strings.TrimSpace(gitOutputNoFail(fixture.repo, "rev-parse", "main")), Location: "remote"},
			{Name: "refs/heads/" + fixture.branch, Branch: fixture.branch, OID: featureHead, Location: "remote"},
		},
		Messages: corehealth.MessagePresence{CompleteAbsence: true},
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	if dir != "" {
		command.Dir = dir
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func gitOutputNoFail(dir string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = dir
	output, _ := command.Output()
	return string(output)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
