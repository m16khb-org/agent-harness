package harnessapp

import "testing"

func TestDocsIndexContractProjectionKeepsShapeAndRequiredDocsOnly(t *testing.T) {
	input := map[string]any{
		"ok":           true,
		"version":      "0.1.0",
		"harness_root": "$HARNESS_ROOT",
		"generated_at": "$TIMESTAMP",
		"docs": []any{
			map[string]any{"rel_path": "AGENTS.md", "title": "old", "bytes": float64(1), "headings": []any{"A"}},
			map[string]any{"rel_path": "CLAUDE.md", "title": "old", "bytes": float64(2), "headings": []any{"B"}},
			map[string]any{"rel_path": ".agent-harness/CONSTITUTION.md", "title": "old", "bytes": float64(3)},
			map[string]any{"rel_path": ".agent-harness/ARCHITECTURE.md", "title": "old", "bytes": float64(4)},
			map[string]any{"rel_path": ".agent-harness/CONVENTIONS.md", "title": "old", "bytes": float64(5)},
			map[string]any{"rel_path": ".agent-harness/TESTING.md", "title": "old", "bytes": float64(6)},
			map[string]any{"rel_path": ".agent-harness/CAUTIONS.md", "title": "old", "bytes": float64(7)},
			map[string]any{"rel_path": ".agent-harness/ADR.md", "title": "old", "bytes": float64(8)},
			map[string]any{"rel_path": ".agent-harness/OPERATIONS.md", "title": "old", "bytes": float64(9)},
			map[string]any{"rel_path": ".agent-harness/AGENT_WORKFLOW.md", "title": "old", "bytes": float64(10)},
			map[string]any{"rel_path": "skills/self-verify/SKILL.md", "title": "old", "bytes": float64(11)},
			map[string]any{"rel_path": "skills/self-augment/SELF_AUGMENTATION.md", "title": "old", "bytes": float64(12)},
		},
	}

	got := docsIndexContractProjection(input)

	if got["ok"] != true || got["version"] != "0.1.0" || got["harness_root"] != "$HARNESS_ROOT" {
		t.Fatalf("stable shape fields not preserved: %#v", got)
	}
	if _, present := got["generated_at"]; present {
		t.Fatalf("generated_at must not be part of docs index contract projection: %#v", got)
	}
	if got["docs_count"] != 12 {
		t.Fatalf("docs_count mismatch: %#v", got)
	}
	required := got["required_docs"].(map[string]bool)
	if !required["AGENTS.md"] || !required[".agent-harness/TESTING.md"] || !required["skills/self-augment/SELF_AUGMENTATION.md"] {
		t.Fatalf("required docs were not detected: %#v", required)
	}
	if _, present := got["docs"]; present {
		t.Fatalf("full docs array must not be snapshotted: %#v", got)
	}
}
