package harnessapp

import (
	"reflect"
	"testing"
)

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

func TestInspectContractProjectionKeepsOnlyRequiredDocs(t *testing.T) {
	input := map[string]any{
		"docs": []any{
			"$HARNESS_ROOT/.agent-harness/CONSTITUTION.md",
			"$HARNESS_ROOT/.agent-harness/plans/transient.md",
			"$HARNESS_ROOT/AGENTS.md",
		},
		"version": "0.1.0",
	}
	want := map[string]any{
		"docs": []any{
			"AGENTS.md",
			".agent-harness/CONSTITUTION.md",
		},
		"version": "0.1.0",
	}
	if got := inspectContractProjection(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("inspect contract projection = %#v, want %#v", got, want)
	}
}

func TestInspectMCPContractProjectionKeepsOnlyRequiredDocs(t *testing.T) {
	input := map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"json": map[string]any{
					"docs": []any{
						"$HARNESS_ROOT/.agent-harness/CONSTITUTION.md",
						"$HARNESS_ROOT/.agent-harness/plans/transient.md",
						"$HARNESS_ROOT/AGENTS.md",
					},
				},
			},
		},
	}
	want := map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"json": map[string]any{
					"docs": []any{"AGENTS.md", ".agent-harness/CONSTITUTION.md"},
				},
			},
		},
	}
	if got := inspectMCPContractProjection(t, input); !reflect.DeepEqual(got, want) {
		t.Fatalf("MCP inspect contract projection = %#v, want %#v", got, want)
	}
}

func TestFirstContractValueDifferenceSortsMapKeys(t *testing.T) {
	want := map[string]any{"z": "expected-z", "a": "expected-a"}
	got := map[string]any{"z": "actual-z", "a": "actual-a"}
	if difference := firstContractValueDifference(want, got, "$"); difference != "$.a: got \"actual-a\", want \"expected-a\"" {
		t.Fatalf("first difference = %q", difference)
	}
}
