package statepath

import (
	"path/filepath"
	"testing"
)

func TestDir(t *testing.T) {
	t.Run("uses HARNESS_STATE_DIR", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HARNESS_STATE_DIR", dir)
		got := Dir()
		if got != dir {
			t.Errorf("Dir() = %q, want %q", got, dir)
		}
	})

	t.Run("default path", func(t *testing.T) {
		t.Setenv("HARNESS_STATE_DIR", "")
		got := Dir()
		if got == "" {
			t.Error("expected non-empty Dir")
		}
	})
}

func TestPath(t *testing.T) {
	got := Path("/state/dir", "my-key")
	expected := filepath.Join("/state/dir", "my-key.json")
	if got != expected {
		t.Errorf("Path() = %q, want %q", got, expected)
	}
}
