package contextregion_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/domain/contextregion"
)

func contextToAny(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func contextMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStableProjectionStripsVolatileFieldsWithoutMutatingInput(t *testing.T) {
	input := map[string]any{
		"title":        "docs",
		"generated_at": "2026-05-30T00:00:00Z",
		"nested": map[string]any{
			"updated_at": "2026-05-30T00:00:01Z",
			"keep":       float64(7),
		},
		"list": []any{
			map[string]any{"started_at": "2026-05-30T00:00:02Z", "ok": true},
		},
	}

	got := contextregion.StableProjection(input)
	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map projection, got %T", got)
	}
	if _, present := gotMap["generated_at"]; present {
		t.Fatalf("generated_at must be stripped from immutable prefix: %v", gotMap)
	}
	if gotMap["title"] != "docs" {
		t.Fatalf("stable field title must survive projection: %v", gotMap)
	}
	nested := gotMap["nested"].(map[string]any)
	if _, present := nested["updated_at"]; present {
		t.Fatalf("nested volatile field must be stripped: %v", nested)
	}
	if nested["keep"] != float64(7) {
		t.Fatalf("nested stable field must survive: %v", nested)
	}
	first := gotMap["list"].([]any)[0].(map[string]any)
	if _, present := first["started_at"]; present {
		t.Fatalf("volatile field inside slice must be stripped: %v", first)
	}
	if first["ok"] != true {
		t.Fatalf("stable field inside slice must survive: %v", first)
	}

	if _, present := input["generated_at"]; !present {
		t.Fatalf("StableProjection must not mutate its input")
	}
}

func TestContextSerializationStableDistinguishesDriftFromVolatile(t *testing.T) {
	// Deterministic builder: identical stable content every call.
	stable, _, err := contextregion.ContextSerializationStable(func() any {
		return map[string]any{"title": "fixed", "generated_at": "ignored"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatalf("deterministic builder must report stable")
	}

	// Volatile-only drift: only a volatile field changes, so the stable
	// projection stays identical and the builder is still stable.
	volatileCalls := 0
	volatileStable, _, err := contextregion.ContextSerializationStable(func() any {
		volatileCalls++
		return map[string]any{"title": "fixed", "generated_at": volatileCalls}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !volatileStable {
		t.Fatalf("volatile-only drift must remain stable: a stable field never changed")
	}

	// Non-volatile drift: a stable field changes between builds, which must
	// be reported as unstable.
	driftCalls := 0
	drifted, _, err := contextregion.ContextSerializationStable(func() any {
		driftCalls++
		return map[string]any{"title": driftCalls, "generated_at": "ignored"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if drifted {
		t.Fatalf("non-volatile drift must be reported as unstable")
	}
}

// TestDocsIndexImmutablePrefixIsByteDeterministic protects the determinism
// contract the response-contract golden masks with $TIMESTAMP: docs_index is
// reused as an immutable context prefix, so everything except its volatile
// fields must serialize byte-identically across repeated builds. The volatile
// generated_at field is asserted present in the raw output and absent from the
// stable projection so the regression net cannot be satisfied by dropping the
// field instead of stabilizing the content.
func TestDocsIndexImmutablePrefixIsByteDeterministic(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Root\n\n## Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent-harness", "ARCHITECTURE.md"), []byte("# Arch\n\n## Boundaries\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	first := contextMarshal(t, contextregion.StableProjection(contextToAny(t, core.DocsIndex(root, "0.1.0"))))
	second := contextMarshal(t, contextregion.StableProjection(contextToAny(t, core.DocsIndex(root, "0.1.0"))))
	if first != second {
		t.Fatalf("docs_index immutable prefix drifted across builds:\nfirst=%s\nsecond=%s", first, second)
	}

	raw, ok := contextToAny(t, core.DocsIndex(root, "0.1.0")).(map[string]any)
	if !ok {
		t.Fatalf("expected docs_index to serialize as an object")
	}
	if _, present := raw["generated_at"]; !present {
		t.Fatalf("expected volatile generated_at field in raw docs_index")
	}
	if _, present := contextregion.StableProjection(raw).(map[string]any)["generated_at"]; present {
		t.Fatalf("stable projection must exclude generated_at")
	}
}
