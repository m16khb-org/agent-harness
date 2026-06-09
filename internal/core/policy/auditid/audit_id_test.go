package auditid

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	id := Generate("/tmp/repo", "/tmp/repo/src", []string{"git", "status"})
	if id == "" {
		t.Error("expected non-empty audit id")
	}
	if !strings.HasPrefix(id, "audit-") {
		t.Errorf("expected 'audit-' prefix, got %q", id)
	}

	// Same inputs should produce same hash portion
	id2 := Generate("/tmp/repo", "/tmp/repo/src", []string{"git", "status"})
	hash1 := id[strings.LastIndex(id, "-")+1:]
	hash2 := id2[strings.LastIndex(id2, "-")+1:]
	if hash1 != hash2 {
		t.Errorf("same inputs should produce same hash: %q vs %q", hash1, hash2)
	}

	// Different inputs should produce different ids
	id3 := Generate("/other", "/tmp/repo/src", []string{"git", "status"})
	if id3 == id {
		t.Error("different workspace should produce different id")
	}
}
