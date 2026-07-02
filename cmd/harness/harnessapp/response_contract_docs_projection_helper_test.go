package harnessapp

import "testing"

var docsIndexRequiredDocs = []string{
	"AGENTS.md",
	"CLAUDE.md",
	".agent-harness/CONSTITUTION.md",
	".agent-harness/ARCHITECTURE.md",
	".agent-harness/CONVENTIONS.md",
	".agent-harness/TESTING.md",
	".agent-harness/CAUTIONS.md",
	".agent-harness/ADR.md",
	".agent-harness/OPERATIONS.md",
	".agent-harness/AGENT_WORKFLOW.md",
	"skills/self-verify/SKILL.md",
	"skills/self-augment/SELF_AUGMENTATION.md",
}

func docsIndexContractProjection(value any) map[string]any {
	obj, _ := value.(map[string]any)
	docs, _ := obj["docs"].([]any)
	seen := make(map[string]bool, len(docs))
	for _, item := range docs {
		doc, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rel, _ := doc["rel_path"].(string)
		if rel != "" {
			seen[rel] = true
		}
	}
	required := make(map[string]bool, len(docsIndexRequiredDocs))
	for _, rel := range docsIndexRequiredDocs {
		required[rel] = seen[rel]
	}
	return map[string]any{
		"ok":            obj["ok"],
		"version":       obj["version"],
		"harness_root":  obj["harness_root"],
		"docs_count":    len(docs),
		"required_docs": required,
	}
}

func docsIndexMCPContractProjection(t *testing.T, value any) map[string]any {
	t.Helper()
	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected MCP docs_index object, got %T", value)
	}
	content, ok := obj["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected MCP docs_index content, got %#v", value)
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected MCP docs_index content object, got %#v", content[0])
	}
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": first["type"],
				"json": docsIndexContractProjection(first["json"]),
			},
		},
	}
}
