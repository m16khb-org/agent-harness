package repopath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRoot(t *testing.T) {
	t.Run("current directory", func(t *testing.T) {
		root, err := NormalizeRoot("")
		if err != nil {
			t.Fatalf("NormalizeRoot empty: %v", err)
		}
		if root == "" {
			t.Error("expected non-empty root")
		}
	})

	t.Run("temp dir", func(t *testing.T) {
		dir := t.TempDir()
		root, err := NormalizeRoot(dir)
		if err != nil {
			t.Fatalf("NormalizeRoot: %v", err)
		}
		if root != dir {
			t.Errorf("got %q, want %q", root, dir)
		}
	})

	t.Run("non-existent", func(t *testing.T) {
		_, err := NormalizeRoot("/nonexistent/path")
		if err == nil {
			t.Error("expected error for non-existent dir")
		}
	})

	t.Run("file not dir", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "file.txt")
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := NormalizeRoot(f)
		if err == nil {
			t.Error("expected error for file path")
		}
	})
}

func TestResolveFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "test.md")
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		root    string
		path    string
		wantErr bool
	}{
		{"absolute exists", dir, file, false},
		{"relative exists", dir, "test.md", false},
		{"empty path", dir, "", true},
		{"escape root", dir, "../../../etc/passwd", true},
		{"directory", dir, dir, true},
		{"non-existent", dir, "no.md", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveFile(tt.root, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveFile(%q, %q) error=%v, wantErr=%v", tt.root, tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestExpandLeadingTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		input string
	}{
		{"/absolute/path"},
		{"relative/path"},
		{"~/Documents"},
		{"~"},
		{""},
	}
	for _, tt := range tests {
		got := ExpandLeadingTilde(tt.input)
		if tt.input == "~/" || tt.input == "~" {
			if home != "" && got[:len(home)] != home {
				t.Errorf("ExpandLeadingTilde(%q) should start with home dir, got %q", tt.input, got)
			}
		}
	}
}
