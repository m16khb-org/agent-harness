package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestResolveProjectLifecycleNamespaceIsProjectScoped(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repoA := t.TempDir()
	repoB := t.TempDir()
	mustWrite(t, filepath.Join(repoA, ".git", "config"), "[remote \"origin\"]\n\turl = git@example.com:a/repo.git\n")
	mustWrite(t, filepath.Join(repoB, ".git", "config"), "[remote \"origin\"]\n\turl = git@example.com:b/repo.git\n")

	a, err := ResolveProjectLifecycleState(repoA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolveProjectLifecycleState(repoB)
	if err != nil {
		t.Fatal(err)
	}
	if a.RepoID == "" || b.RepoID == "" || a.RepoID == b.RepoID {
		t.Fatalf("repo ids should be non-empty and distinct: a=%q b=%q", a.RepoID, b.RepoID)
	}
	if !strings.HasPrefix(a.ProjectStateDir, filepath.Join(stateRoot, "projects")) {
		t.Fatalf("state dir not under project namespace: %s", a.ProjectStateDir)
	}
	if a.ProjectStateDir == b.ProjectStateDir {
		t.Fatalf("project state dirs should differ: %s", a.ProjectStateDir)
	}
}

func TestInitProjectLifecycleStateWritesProjectJSONOnlyWhenConfirmed(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	dry, err := InitProjectLifecycleState(repo, false)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.OK || dry.Exists || dry.NamespaceValid || dry.ProjectJSONPath == "" {
		t.Fatalf("unexpected dry lifecycle state: %+v", dry)
	}
	if _, err := os.Stat(dry.ProjectJSONPath); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote project.json or unexpected stat error: %v", err)
	}

	written, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	if !written.OK || !written.Exists || !written.NamespaceValid {
		t.Fatalf("unexpected written lifecycle state: %+v", written)
	}
	if _, err := os.Stat(written.ProjectJSONPath); err != nil {
		t.Fatalf("project.json missing: %v", err)
	}
}

func TestValidateProjectLifecycleStateDetectsNamespaceMismatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	written, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	var profile ProjectLifecycleProfile
	b, err := os.ReadFile(written.ProjectJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Fingerprint.RepoRoot = filepath.Join(t.TempDir(), "other")
	b, err = json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written.ProjectJSONPath, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	validated, err := ValidateProjectLifecycleState(repo)
	if err != nil {
		t.Fatal(err)
	}
	if validated.NamespaceValid || !containsString(validated.Warnings, "namespace_mismatch") {
		t.Fatalf("expected namespace mismatch warning: %+v", validated)
	}
}

func TestInitProjectLifecycleStateConcurrentNoDuplicates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()

	const n = 5
	var wg sync.WaitGroup
	results := make([]ProjectLifecycleStatePlan, n)
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = InitProjectLifecycleState(repo, true)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("InitProjectLifecycleState[%d] error: %v", i, err)
		}
		if !results[i].OK || !results[i].Exists {
			t.Fatalf("InitProjectLifecycleState[%d] not OK or not Exists: %+v", i, results[i])
		}
	}

	ref := results[0]
	for i := 1; i < n; i++ {
		if results[i].Profile.RepoID != ref.Profile.RepoID {
			t.Fatalf("InitProjectLifecycleState[%d] repo ID mismatch: %q vs %q", i, results[i].Profile.RepoID, ref.Profile.RepoID)
		}
		if results[i].Profile.CreatedAt != ref.Profile.CreatedAt {
			t.Fatalf("InitProjectLifecycleState[%d] CreatedAt mismatch: %q vs %q", i, results[i].Profile.CreatedAt, ref.Profile.CreatedAt)
		}
	}

	// Verify the on-disk file is valid JSON and exactly one profile exists.
	b, err := os.ReadFile(ref.ProjectJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk ProjectLifecycleProfile
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatalf("on-disk profile is not valid JSON: %v", err)
	}
	if onDisk.RepoID != ref.Profile.RepoID || onDisk.CreatedAt != ref.Profile.CreatedAt {
		t.Fatalf("on-disk profile mismatch: %+v vs %+v", onDisk, *ref.Profile)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
