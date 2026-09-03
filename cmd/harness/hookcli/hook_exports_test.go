package hookcli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunHookExport(t *testing.T) {
	if err := RunHook([]string{"--help"}); err == nil {
		t.Fatalf("expected ErrHelp, got nil")
	}
	if err := RunHook([]string{}); err == nil {
		t.Fatalf("expected error for empty args, got nil")
	}
	if err := RunHook([]string{"unknown-hook"}); err == nil {
		t.Fatalf("expected error for unknown hook, got nil")
	}
}

func TestResolveTarget(t *testing.T) {
	tmp := t.TempDir()
	got := ResolveTarget(tmp)
	absTmp, _ := filepath.Abs(tmp)
	if got != absTmp {
		t.Fatalf("ResolveTarget(%q) = %q, want %q", tmp, got, absTmp)
	}

	t.Run("with CLAUDE_PROJECT_DIR", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", tmp)
		t.Setenv("PWD", "")
		if res := ResolveTarget(""); res != absTmp {
			t.Fatalf("expected %q, got %q", absTmp, res)
		}
	})

	t.Run("with PWD", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", tmp)
		if res := ResolveTarget(""); res != absTmp {
			t.Fatalf("expected %q, got %q", absTmp, res)
		}
	})

	t.Run("with neither env var", func(t *testing.T) {
		t.Setenv("CLAUDE_PROJECT_DIR", "")
		t.Setenv("PWD", "")
		cwd, _ := os.Getwd()
		absCwd, _ := filepath.Abs(cwd)
		if res := ResolveTarget(""); res != absCwd {
			t.Fatalf("expected %q, got %q", absCwd, res)
		}
	})
}
