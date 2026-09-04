package issueopsapp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestNormalizeDocsCountsForGoldenReplacesNumericDocsCounters(t *testing.T) {
	got := normalizeDocsCountsForGolden(map[string]any{
		"docs_count":   98,
		"docs_indexed": float64(98),
		"feasibility":  0.75,
		"nested": map[string]any{
			"docs_count": int64(12),
			"items": []any{
				map[string]any{"docs_indexed": json.Number("7")},
				map[string]any{"docs_count": float64(3), "note": "keep"},
			},
		},
	})

	want := map[string]any{
		"docs_count":   docsCountGoldenPlaceholder,
		"docs_indexed": docsCountGoldenPlaceholder,
		"feasibility":  0.75,
		"nested": map[string]any{
			"docs_count": docsCountGoldenPlaceholder,
			"items": []any{
				map[string]any{"docs_indexed": docsCountGoldenPlaceholder},
				map[string]any{"docs_count": docsCountGoldenPlaceholder, "note": "keep"},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeDocsCountsForGolden = %#v, want %#v", got, want)
	}
}

func TestNormalizeDocsCountsForGoldenKeepsNonNumericDocsCounters(t *testing.T) {
	got := normalizeDocsCountsForGolden(map[string]any{
		"docs_count":   "98",
		"docs_indexed": nil,
		"docs":         []any{"AGENTS.md"},
	})

	want := map[string]any{
		"docs_count":   "98",
		"docs_indexed": nil,
		"docs":         []any{"AGENTS.md"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeDocsCountsForGolden = %#v, want %#v", got, want)
	}
}

func TestNormalizeIssueOpsStateKeysForGoldenKeepsPathShape(t *testing.T) {
	got := normalizeIssueOpsStateKeysForGolden(map[string]any{
		"next_command": "issueops execution claim --claim-token-file /repo/.issueops/state/issueops-v1/b6d8a06c9683e6d9/lease-1.token",
		"unrelated":    "/repo/0123456789abcdef/file",
	})
	want := map[string]any{
		"next_command": "issueops execution claim --claim-token-file /repo/.issueops/state/issueops-v1/$ISSUEOPS_STATE_KEY/lease-1.token",
		"unrelated":    "/repo/0123456789abcdef/file",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeIssueOpsStateKeysForGolden = %#v, want %#v", got, want)
	}
}

func TestResponseContractsGoldenUsesDocsCountPlaceholder(t *testing.T) {
	path := filepath.Join("..", "testdata", "response_contracts.golden.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("unmarshal golden %s: %v", path, err)
	}

	entries := collectDocsCountGoldenEntries(value, "$")
	if len(entries) == 0 {
		t.Fatalf("golden %s has no docs counter keys; the placeholder gate would be vacuous", path)
	}
	for _, entry := range entries {
		if entry.value != docsCountGoldenPlaceholder {
			t.Fatalf("golden %s: %s = %#v, want %q (run go test ./cmd/issueops/issueopsapp -run Golden -update)",
				path, entry.path, entry.value, docsCountGoldenPlaceholder)
		}
	}
}

type docsCountGoldenEntry struct {
	path  string
	value any
}

func collectDocsCountGoldenEntries(value any, path string) []docsCountGoldenEntry {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		entries := make([]docsCountGoldenEntry, 0, len(keys))
		for _, key := range keys {
			childPath := path + "." + key
			if isDocsCountGoldenKey(key) {
				entries = append(entries, docsCountGoldenEntry{path: childPath, value: typed[key]})
				continue
			}
			entries = append(entries, collectDocsCountGoldenEntries(typed[key], childPath)...)
		}
		return entries
	case []any:
		entries := make([]docsCountGoldenEntry, 0, len(typed))
		for index, child := range typed {
			entries = append(entries, collectDocsCountGoldenEntries(child, fmt.Sprintf("%s[%d]", path, index))...)
		}
		return entries
	default:
		return nil
	}
}
