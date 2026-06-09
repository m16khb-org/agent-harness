package implementation

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestPorcelainPath(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"untracked", "?? file.go", "file.go"},
		{"modified", " M file.go", "file.go"},
		{"staged", "M  file.go", "file.go"},
		{"renamed", "R  old.go -> new.go", "new.go"},
		{"with quotes", `?? "file name.go"`, "file name.go"},
		{"too short", "M", ""},
		{"empty", "", ""},
		{"only spaces", "    ", ""},
		{"with CR", "?? file.go\r", "file.go"},
		{"renamed with spaces", "R  old name.go -> new name.go", "new name.go"},
		{"deleted", " D file.go", "file.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PorcelainPath(tt.line)
			if got != tt.want {
				t.Errorf("PorcelainPath(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestCleanRelativePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"src/main.go", "src/main.go"},
		{"src/main.go", "src/main.go"},
		{"", ""},
		{".", ""},
		{"/absolute/path", ""},
		{"../escape", ""},
		{"..", ""},
		{"./relative", "relative"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := cleanRelativePath(tt.path)
			if got != tt.want {
				t.Errorf("cleanRelativePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathMatchesPlan(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0o755)

	planPath := filepath.Join(dir, "plan.md")
	os.WriteFile(planPath, []byte("content"), 0o644)

	record := model.IssueOpsRecord{PlanPath: planPath}

	tests := []struct {
		name     string
		worktree string
		path     string
		expected bool
	}{
		{"exact match", dir, planPath, true},
		{"different file", dir, filepath.Join(dir, "other.md"), false},
		{"empty plan", dir, "", false},
		{"relative plan match", dir, "plan.md", false}, // PathMatchesPlan uses record.PlanPath which is absolute
		{"empty path", dir, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "relative plan match" {
				rec := model.IssueOpsRecord{PlanPath: "plan.md"}
				got := PathMatchesPlan(rec, tt.worktree, planPath)
				if !got {
					t.Error("expected match for relative plan path")
				}
				return
			}
			got := PathMatchesPlan(record, tt.worktree, tt.path)
			if got != tt.expected {
				t.Errorf("PathMatchesPlan(rec, %q, %q) = %v, want %v", tt.worktree, tt.path, got, tt.expected)
			}
		})
	}
}

func TestDiffBaseRef(t *testing.T) {
	t.Run("nil branch prepare", func(t *testing.T) {
		rec := model.IssueOpsRecord{}
		got := diffBaseRef(rec, "/tmp")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
	t.Run("empty base branch", func(t *testing.T) {
		rec := model.IssueOpsRecord{
			BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: ""},
		}
		got := diffBaseRef(rec, "/tmp")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}
