package inspect

import (
	inspectcontract "agent-harness/internal/contract/inspect"
	"path/filepath"
	"testing"
)

func TestInspectHarnessIndexesSkillsAndDocs(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	info := InspectHarness(root, root, t.TempDir(), "test-version", "atomic-commit-push")
	if !info.OK {
		t.Fatalf("InspectHarness ok=false: %+v", info)
	}
	if info.Version != "test-version" || info.HarnessRoot != root || info.TargetRepo != root {
		t.Fatalf("unexpected identity fields: %+v", info)
	}
	if !containsSkill(info.Skills, "atomic-commit-push") {
		t.Fatalf("unexpected skills: %+v", info.Skills)
	}
	if !containsDoc(info.Docs, ".agent-harness/OPERATIONS.md") {
		t.Fatalf("USAGE.md not indexed: %+v", info.Docs)
	}
	if !info.Integration.ProjectClaudeMCPConfig {
		t.Fatalf("project MCP config not detected: %+v", info.Integration)
	}
}

func containsDoc(paths []string, suffix string) bool {
	for _, path := range paths {
		if filepath.ToSlash(path) == suffix || filepath.ToSlash(path) == "../../"+suffix || filepath.Base(path) == filepath.Base(suffix) {
			return true
		}
		if rel, err := filepath.Rel(filepath.Join("..", "..", ".."), path); err == nil && filepath.ToSlash(rel) == suffix {
			return true
		}
	}
	return false
}

func containsSkill(skills []inspectcontract.SkillInfo, name string) bool {
	for _, skill := range skills {
		if skill.Name == name && skill.HasSkillMD {
			return true
		}
	}
	return false
}
