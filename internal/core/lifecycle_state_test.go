package core

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestAppendDocUpkeepEventWritesJSONL(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	result, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{
		Kind:       "operation_change",
		TargetDocs: []string{"OPERATIONS.md"},
		Summary:    "Hook behavior changed.",
		Evidence:   []string{"internal/core/hook_prompt.go"},
		Source:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Path == "" || result.Event.ID == "" {
		t.Fatalf("unexpected append result: %+v", result)
	}
	file, err := os.Open(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one jsonl record")
	}
	var got DocUpkeepEvent
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "operation_change" || got.TargetDocs[0] != "OPERATIONS.md" || got.Status != "pending" {
		t.Fatalf("unexpected event: %+v", got)
	}
	if scanner.Scan() {
		t.Fatalf("expected one event, got extra line: %s", scanner.Text())
	}
}

func TestRecordLifecycleToolUseQueuesRelevantDocUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	result, err := RecordLifecycleToolUse(HookToolUseLifecycleRequest{
		Repo:   repo,
		Tool:   "apply_patch",
		Paths:  []string{"internal/core/hook_prompt.go", "internal/core/hook_prompt_test.go"},
		Source: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Recorded || result.Event.ID == "" {
		t.Fatalf("expected recorded upkeep event: %+v", result)
	}
	if !containsString(result.Event.TargetDocs, "OPERATIONS.md") || !containsString(result.Event.TargetDocs, "TESTING.md") {
		t.Fatalf("expected operations/testing targets: %+v", result.Event.TargetDocs)
	}
}

func TestBuildLifecycleStopReminderIncludesPendingUpkeep(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	if _, err := InitProjectLifecycleState(repo, true); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{Kind: "code_change", TargetDocs: []string{"OPERATIONS.md"}, Summary: "Hook behavior changed.", Source: "test"}); err != nil {
		t.Fatal(err)
	}
	reminder := BuildLifecycleStopReminder(repo)
	if !reminder.ShouldInject || !strings.Contains(reminder.AdditionalContext, "Pending .agent-harness doc upkeep") || !strings.Contains(reminder.AdditionalContext, "OPERATIONS.md") {
		t.Fatalf("unexpected reminder: %+v", reminder)
	}
}
