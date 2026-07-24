package harnessapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"agent-harness/cmd/harness/issueopscli"
)

func stubIssueOpsChildIssueVerifier(t *testing.T, verifier func(string) error) {
	t.Helper()
	previous := issueopscli.SetChildIssueVerifier(verifier)
	t.Cleanup(func() {
		issueopscli.SetChildIssueVerifier(previous)
	})
}

func makeGitRepoForContract(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitForContract(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# contract fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/contract\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, dir, "add", "README.md", "go.mod")
	runGitForContract(t, dir,
		"-c", "user.name=Contract Test",
		"-c", "user.email=contract@example.invalid",
		"commit", "-q",
		"-m", "docs(contract): add fixture",
		"-m", "Lore:\n- Intent: Normalize preflight contract.\n- Why: Response golden should cover git DTOs.\n- Changes:\n  - Add fixture README.\n- Verify: go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden\n- Risk: Low",
	)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("TOKEN=fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makeRecordlessOrphanGitRepoForContract(t *testing.T) (string, string, string) {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	remote := filepath.Join(t.TempDir(), "remote.git")
	worktree := filepath.Join(t.TempDir(), "merged-orphan")
	branch := "merged-orphan"
	runGitForContract(t, "", "init", "-q", "--bare", remote)
	runGitForContract(t, "", "init", "-q", "-b", "main", repo)
	runGitForContract(t, repo, "config", "user.name", "Contract Test")
	runGitForContract(t, repo, "config", "user.email", "contract@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# recordless orphan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, repo, "add", "README.md")
	runGitForContract(t, repo, "commit", "-q", "-m", "docs(contract): add recordless orphan fixture")
	runGitForContract(t, repo, "remote", "add", "origin", remote)
	runGitForContract(t, repo, "push", "-q", "-u", "origin", "main")
	runGitForContract(t, repo, "branch", branch)
	runGitForContract(t, repo, "worktree", "add", "-q", worktree, branch)
	runGitForContract(t, worktree, "push", "-q", "-u", "origin", branch)
	return repo, worktree, branch
}

func runGitForContract(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func addEvalSymlinkReplacement(t *testing.T, replacements map[string]string, path, token string) {
	t.Helper()
	eval, err := filepath.EvalSymlinks(path)
	if err == nil && eval != "" {
		replacements[eval] = token
	}
}

func writeContractFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSelfAugmentCompareFixturesForContract(t *testing.T, stateDir string) {
	t.Helper()
	summary := SelfAugmentSummary{
		TotalRuns:   10,
		TotalSteps:  20,
		PassedSteps: 20,
		StepLabels:  []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{
			{Iteration: 1, Seed: 600, Label: "go test", DurationMS: 1000},
		},
	}
	fixtures := []struct {
		key         string
		elapsed     int64
		generatedAt string
	}{
		{key: "self-verify-baseline", elapsed: 1000, generatedAt: "2000-01-01T00:00:00Z"},
		{key: "self-verify-candidate", elapsed: 1100, generatedAt: "2000-01-01T00:01:00Z"},
	}
	for _, fixture := range fixtures {
		if err := writeSelfAugmentSnapshotRecord(stateDir, fixture.key, SelfAugmentStateSnapshot{
			SchemaVersion: 1,
			Kind:          "self_verification_summary",
			OK:            true,
			Iterations:    10,
			BaseSeed:      600,
			ElapsedMS:     fixture.elapsed,
			HarnessRoot:   harnessRoot(),
			GeneratedAt:   fixture.generatedAt,
			Summary:       summary,
		}); err != nil {
			t.Fatalf("write compare fixture %s: %v", fixture.key, err)
		}
	}
}
