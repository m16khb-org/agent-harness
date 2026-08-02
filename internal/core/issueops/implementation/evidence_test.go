package implementation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestPorcelainPath(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"untracked", "?? file.go", "file.go"},
		{"modified", " M file.go", "file.go"},
		{"staged", "M  file.go", "file.go"},
		{"renamed", "R  old.go -> new.go", "new.go"},
		{"with quotes", `?? "file name.go"`, "file name.go"},
		{"too short", "M", ""},
		{"empty", "", ""},
		{"only spaces", "    ", ""},
		{"with CR", "?? file.go\r", "file.go"},
		{"renamed with spaces", "R  old name.go -> new name.go", "new name.go"},
		{"deleted", " D file.go", "file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PorcelainPath(tt.line)
			if got != tt.want {
				t.Errorf("PorcelainPath(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestCleanRelativePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"src/main.go", "src/main.go"},
		{"src/main.go", "src/main.go"},
		{"", ""},
		{".", ""},
		{"/absolute/path", ""},
		{"../escape", ""},
		{"..", ""},
		{"./relative", "relative"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := cleanRelativePath(tt.path)
			if got != tt.want {
				t.Errorf("cleanRelativePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathMatchesPlan(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0o755)

	planPath := filepath.Join(dir, "plan.md")
	os.WriteFile(planPath, []byte("content"), 0o644)

	record := model.IssueOpsRecord{PlanPath: planPath}

	tests := []struct {
		name     string
		worktree string
		path     string
		expected bool
	}{
		{"exact match", dir, planPath, true},
		{"different file", dir, filepath.Join(dir, "other.md"), false},
		{"empty plan", dir, "", false},
		{"relative plan match", dir, "plan.md", false}, // PathMatchesPlan uses record.PlanPath which is absolute
		{"empty path", dir, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "relative plan match" {
				rec := model.IssueOpsRecord{PlanPath: "plan.md"}
				got := PathMatchesPlan(rec, tt.worktree, planPath)
				if !got {
					t.Error("expected match for relative plan path")
				}
				return
			}
			got := PathMatchesPlan(record, tt.worktree, tt.path)
			if got != tt.expected {
				t.Errorf("PathMatchesPlan(rec, %q, %q) = %v, want %v", tt.worktree, tt.path, got, tt.expected)
			}
		})
	}
}

func TestDiffBaseRef(t *testing.T) {
	t.Run("nil branch prepare", func(t *testing.T) {
		rec := model.IssueOpsRecord{}
		got := diffBaseRef(rec, "/tmp")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("empty base branch", func(t *testing.T) {
		rec := model.IssueOpsRecord{
			BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: ""},
		}
		got := diffBaseRef(rec, "/tmp")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestHasEvidenceForGitAndFileTreeChanges(t *testing.T) {
	t.Run("invalid worktree", func(t *testing.T) {
		if HasEvidence(model.IssueOpsRecord{}) {
			t.Fatal("empty worktree should not have implementation evidence")
		}
	})
	t.Run("non git worktree falls back to file tree", func(t *testing.T) {
		worktree := t.TempDir()
		record := model.IssueOpsRecord{WorktreePath: worktree, PlanPath: "plan.md"}
		if HasEvidence(record) {
			t.Fatal("empty non-git worktree should not have implementation evidence")
		}
		if err := os.WriteFile(filepath.Join(worktree, "plan.md"), []byte("plan"), 0o600); err != nil {
			t.Fatal(err)
		}
		if HasEvidence(record) {
			t.Fatal("plan-only file should not count as implementation evidence")
		}
		if err := os.WriteFile(filepath.Join(worktree, "impl.go"), []byte("package impl\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if !HasEvidence(record) {
			t.Fatal("non-plan file should count as implementation evidence")
		}
	})
	t.Run("git status and fingerprint", func(t *testing.T) {
		repo := newImplementationGitRepo(t)
		record := model.IssueOpsRecord{
			WorktreePath: repo,
			PlanPath:     filepath.Join(repo, "plan.md"),
			BranchPrepare: &model.IssueOpsBranchPrepare{
				BaseBranch: "main",
			},
		}
		if HasEvidence(record) {
			t.Fatal("clean git worktree at base should not have implementation evidence")
		}
		if got := ChangeFingerprint(record); got != "" {
			t.Fatalf("clean worktree fingerprint = %q", got)
		}
		if err := os.WriteFile(filepath.Join(repo, "impl.go"), []byte("package impl\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if !HasEvidence(record) {
			t.Fatal("non-plan git status should count as implementation evidence")
		}
		if got := ChangeFingerprint(record); got == "" {
			t.Fatal("dirty implementation change should produce fingerprint")
		}
		runGit(t, repo, "add", "impl.go")
		runGit(t, repo, "commit", "-m", "add impl")
		if !gitHeadDiffersFromBase(record, repo) {
			t.Fatal("feature commit should differ from base")
		}
	})
}

func TestImplementationEvidenceUsesImmutableBranchPrepareBaseSHA(t *testing.T) {
	repo := newImplementationGitRepo(t)
	baseSHA := strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repo, "impl.go"), []byte("package impl\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "impl.go")
	runGit(t, repo, "commit", "-m", "add impl")
	featureSHA := strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "HEAD"))
	runGit(t, repo, "update-ref", "refs/remotes/origin/main", featureSHA)

	record := model.IssueOpsRecord{
		WorktreePath: repo,
		PlanPath:     filepath.Join(repo, "plan.md"),
		BranchPrepare: &model.IssueOpsBranchPrepare{
			BaseBranch: "main",
			BaseSHA:    baseSHA,
		},
	}
	if !HasEvidence(record) {
		t.Fatal("feature change disappeared after origin/main moved to feature HEAD")
	}
	if got := ChangeFingerprint(record); got == "" {
		t.Fatal("immutable base change did not produce a fingerprint")
	}

	record.BranchPrepare.BaseSHA = featureSHA
	if HasEvidence(record) {
		t.Fatal("HEAD equal to immutable base SHA reported implementation evidence")
	}
	record.BranchPrepare.BaseSHA = "not-a-full-sha"
	runGit(t, repo, "update-ref", "refs/remotes/origin/main", baseSHA)
	if !HasEvidence(record) {
		t.Fatal("malformed base SHA did not preserve moving-ref compatibility fallback")
	}
	record.BranchPrepare.BaseSHA = ""
	if !HasEvidence(record) {
		t.Fatal("missing base SHA did not preserve moving-ref compatibility fallback")
	}
}

func TestChangeFingerprintPreservesLeadingPorcelainStatusSpaceAcrossCommit(t *testing.T) {
	repo := newImplementationGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "impl.go"), []byte("package impl\nconst Value = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "impl.go")
	runGit(t, repo, "commit", "-m", "add impl")
	baseSHA := strings.TrimSpace(runGitOutput(t, repo, "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(repo, "impl.go"), []byte("package impl\nconst Value = 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		WorktreePath: repo,
		BranchPrepare: &model.IssueOpsBranchPrepare{
			BaseBranch: "main",
			BaseSHA:    baseSHA,
		},
	}
	dirtyFingerprint := ChangeFingerprint(record)
	if dirtyFingerprint == "" {
		t.Fatal("tracked dirty change should produce a fingerprint")
	}

	runGit(t, repo, "add", "impl.go")
	runGit(t, repo, "commit", "-m", "update impl")
	if committedFingerprint := ChangeFingerprint(record); committedFingerprint != dirtyFingerprint {
		t.Fatalf("same content changed fingerprint across commit: dirty=%q committed=%q", dirtyFingerprint, committedFingerprint)
	}
}

func TestHasEvidenceIgnoresTrackedPlanAsFirstPorcelainEntry(t *testing.T) {
	repo := newImplementationGitRepo(t)
	record := model.IssueOpsRecord{
		WorktreePath: repo,
		PlanPath:     filepath.Join(repo, "plan.md"),
		BranchPrepare: &model.IssueOpsBranchPrepare{
			BaseBranch: "main",
		},
	}
	if err := os.WriteFile(filepath.Join(repo, "plan.md"), []byte("updated plan"), 0o600); err != nil {
		t.Fatal(err)
	}
	if HasEvidence(record) {
		t.Fatal("tracked plan-only change should not count as implementation evidence")
	}
}

func newImplementationGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "plan.md"), []byte("plan"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "plan.md")
	runGit(t, repo, "commit", "-m", "base")
	runGit(t, repo, "checkout", "-b", "1234-impl")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
