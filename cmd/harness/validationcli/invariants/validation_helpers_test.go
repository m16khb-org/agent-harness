package invariants

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateHarnessInvariantsCoversHealthyMissingAndLegacyHits(t *testing.T) {
	root := makeValidationHarnessRoot(t)
	healthy := validateHarnessInvariants(root)
	if !healthy.OK || healthy.Label != "harness invariants" || healthy.Error != "" {
		t.Fatalf("expected healthy invariants, got %+v", healthy)
	}

	missingRoot := makeValidationHarnessRoot(t)
	if err := os.Remove(filepath.Join(missingRoot, ".agent-harness", "COMMIT_POLICY.md")); err != nil {
		t.Fatal(err)
	}
	missing := validateHarnessInvariants(missingRoot)
	if missing.OK || !strings.Contains(missing.Error, "missing .agent-harness/COMMIT_POLICY.md") {
		t.Fatalf("expected missing doc invariant, got %+v", missing)
	}

	legacyRoot := makeValidationHarnessRoot(t)
	if err := os.WriteFile(filepath.Join(legacyRoot, "AGENTS.md"), []byte("legacy m"+"16kh owner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := validateHarnessInvariants(legacyRoot)
	if legacy.OK || !strings.Contains(legacy.Error, "forbidden legacy name hits") {
		t.Fatalf("expected forbidden legacy invariant, got %+v", legacy)
	}
}

func TestValidateHarnessInvariantsUsesOwnedStatePaths(t *testing.T) {
	root := makeValidationHarnessRoot(t)
	for _, rel := range []string{
		filepath.Join("internal", "core", "state", "state_io.go"),
		filepath.Join("internal", "core", "state", "state_types.go"),
	} {
		if err := os.Remove(filepath.Join(root, rel)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	owned := []string{
		filepath.Join("internal", "adapter", "outbound", "state", "state_io.go"),
		filepath.Join("internal", "contract", "state", "record.go"),
		filepath.Join("internal", "contract", "state", "results.go"),
	}
	for _, rel := range owned {
		writeValidationFile(t, filepath.Join(root, rel), "ok\n")
	}

	result := validateHarnessInvariants(root)
	if !result.OK {
		t.Fatalf("expected owned state files to satisfy invariants, got %+v", result)
	}
	for _, rel := range owned {
		missingRoot := makeValidationHarnessRoot(t)
		if err := os.Remove(filepath.Join(missingRoot, rel)); err != nil {
			t.Fatal(err)
		}
		missing := validateHarnessInvariants(missingRoot)
		if missing.OK || !strings.Contains(missing.Error, "missing "+rel) {
			t.Fatalf("expected missing owned state file %s, got %+v", rel, missing)
		}
	}
}

func TestValidateSkillShapeCoversFrontmatterAndAgentFile(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", skillName)
	writeValidationFile(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: "+skillName+"\ndescription: Git workflow\n---\nbody\n")
	writeValidationFile(t, filepath.Join(skillDir, "agents", "openai.yaml"), "model: gpt\n")
	if err := validateSkillShape(skillDir); err != nil {
		t.Fatalf("expected valid skill shape: %v", err)
	}

	badFrontmatter := filepath.Join(root, "bad-frontmatter")
	writeValidationFile(t, filepath.Join(badFrontmatter, "SKILL.md"), "name: "+skillName+"\n")
	writeValidationFile(t, filepath.Join(badFrontmatter, "agents", "openai.yaml"), "model: gpt\n")
	if err := validateSkillShape(badFrontmatter); err == nil || !strings.Contains(err.Error(), "missing YAML frontmatter") {
		t.Fatalf("expected missing frontmatter error, got %v", err)
	}

	missingAgent := filepath.Join(root, "missing-agent")
	writeValidationFile(t, filepath.Join(missingAgent, "SKILL.md"), "---\nname: "+skillName+"\ndescription: Git workflow\n---\nbody\n")
	if err := validateSkillShape(missingAgent); err == nil || !strings.Contains(err.Error(), "agents/openai.yaml missing") {
		t.Fatalf("expected missing agent error, got %v", err)
	}
}

func TestContainsForbiddenLegacyOutsideRuntimePathsMasksRuntimeAndOwner(t *testing.T) {
	root := filepath.Join(t.TempDir(), "m"+"16kh-runtime-root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runtimeText := filepath.Join(root, "worktrees", "example") + "\n"
	if containsForbiddenLegacyOutsideRuntimePaths(runtimeText, root) {
		t.Fatal("runtime root path should be masked before legacy-name scan")
	}
	ownerText := "git@github.com:m" + "16khb/agent-harness.git"
	if containsForbiddenLegacyOutsideRuntimePaths(ownerText, root) {
		t.Fatal("current owner handle should be masked before legacy-name scan")
	}
	if !containsForbiddenLegacyOutsideRuntimePaths("bad legacy m"+"16kh value", root) {
		t.Fatal("non-runtime legacy needle should still be detected")
	}
}

func TestForbiddenNameHitsAllowsCurrentOwnerAndLicense(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "LICENSE"), []byte("Copyright (c) 2026 m"+"16khb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(root, "plan.md")
	if err := os.WriteFile(planPath, []byte("git clone git@github.com:m"+"16khb/agent-harness.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("legacy m"+"16kh leak\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits := forbiddenNameHits(root)
	if len(hits) != 1 || hits[0] != "AGENTS.md contains m"+"16kh" {
		t.Fatalf("expected only the genuine legacy hit, got %+v", hits)
	}
}

func TestForbiddenNameHitsSkipsWorktreeGitPointer(t *testing.T) {
	root := t.TempDir()
	legacyPath := "gitdir: /Users/" + "m" + "16" + "kh" + "b/Workspace/agent-harness/.git/worktrees/example\n"
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte(legacyPath), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("safe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hits := forbiddenNameHits(root); len(hits) != 0 {
		t.Fatalf("worktree .git pointer should be skipped, got %+v", hits)
	}
}

func TestForbiddenNameHitsSkipsOnlyOmoTaskRuntime(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, ".omo", "senpi-task", "children", "task", "sessions")
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "transcript.jsonl"), []byte("legacy m"+"16kh runtime text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".omo", "mcp.json"), []byte("legacy m"+"16kh project config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hits := forbiddenNameHits(root)
	if len(hits) != 1 || hits[0] != filepath.Join(".omo", "mcp.json")+" contains m"+"16kh" {
		t.Fatalf("expected project config hit and no Omo task-runtime hit, got %+v", hits)
	}
}

func makeValidationHarnessRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		"AGENTS.md",
		"CLAUDE.md",
		filepath.Join(".agent-harness", "OPERATIONS.md"),
		filepath.Join(".agent-harness", "COMMIT_POLICY.md"),
		filepath.Join("skills", skillName, "scripts", "git_preflight.py"),
		filepath.Join("skills", "self-verify", "SKILL.md"),
		filepath.Join("skills", "self-verify", "CANDIDATES.md"),
		filepath.Join("skills", "project-bootstrap", "SKILL.md"),
		filepath.Join("internal", "adapter", "docs", "docs.go"),
		filepath.Join("internal", "adapter", "projectbootstrap", "project_docs_bootstrap.go"),
		filepath.Join("internal", "adapter", "projectdocs", "project_docs_render.go"),
		filepath.Join("internal", "adapter", "inspect", "inspect.go"),
		filepath.Join("internal", "adapter", "policy", "policy_evaluate.go"),
		filepath.Join("internal", "adapter", "policy", "policy_paths.go"),
		filepath.Join("internal", "adapter", "preflight", "preflight.go"),
		filepath.Join("internal", "adapter", "preflight", "package_helpers.go"),
		filepath.Join("internal", "adapter", "outbound", "state", "state_io.go"),
		filepath.Join("internal", "contract", "state", "record.go"),
		filepath.Join("internal", "contract", "state", "results.go"),
		filepath.Join("cmd", "harness", "contractgolden", "contract_golden_test.go"),
		filepath.Join("cmd", "harness", "harnessapp", "response_contract_golden_test.go"),
		filepath.Join("cmd", "harness", "selfworkflow", "summary", "self_augment_summary_test.go"),
		filepath.Join("cmd", "harness", "testdata", "usage.golden.txt"),
		filepath.Join("cmd", "harness", "testdata", "mcp_tools.golden.json"),
		filepath.Join("cmd", "harness", "testdata", "mcp_resources.golden.json"),
		filepath.Join("cmd", "harness", "testdata", "response_contracts.golden.json"),
		".mcp.json",
	} {
		writeValidationFile(t, filepath.Join(root, rel), "ok\n")
	}
	writeValidationFile(t, filepath.Join(root, "skills", skillName, "SKILL.md"), "---\nname: "+skillName+"\ndescription: Git workflow\n---\nbody\n")
	writeValidationFile(t, filepath.Join(root, "skills", skillName, "agents", "openai.yaml"), "model: gpt\n")
	return root
}

func writeValidationFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
