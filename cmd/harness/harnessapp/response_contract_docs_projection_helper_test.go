package harnessapp

import (
	"strings"
	"testing"
)

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

func inspectContractProjection(value any) map[string]any {
	obj, _ := value.(map[string]any)
	projection := make(map[string]any, len(obj))
	for key, item := range obj {
		projection[key] = item
	}
	docs, _ := obj["docs"].([]any)
	seen := make(map[string]bool, len(docs))
	for _, item := range docs {
		path, _ := item.(string)
		for _, rel := range docsIndexRequiredDocs {
			if path == rel || strings.HasSuffix(path, "/"+rel) {
				seen[rel] = true
			}
		}
	}
	projectedDocs := make([]any, 0, len(docsIndexRequiredDocs))
	for _, rel := range docsIndexRequiredDocs {
		if seen[rel] {
			projectedDocs = append(projectedDocs, rel)
		}
	}
	projection["docs"] = projectedDocs
	return projection
}

func inspectMCPContractProjection(t *testing.T, value any) map[string]any {
	t.Helper()
	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected MCP harness_inspect object, got %T", value)
	}
	content, ok := obj["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected MCP harness_inspect content, got %#v", value)
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected MCP harness_inspect content object, got %#v", content[0])
	}
	return map[string]any{
		"content": []any{
			map[string]any{
				"type": first["type"],
				"json": inspectContractProjection(first["json"]),
			},
		},
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
