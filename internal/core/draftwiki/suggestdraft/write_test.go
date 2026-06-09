package suggestdraft

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripMarkdownFence(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"no fence", "plain text", "plain text"},
		{"fenced markdown", "```\n# Title\ncontent\n```", "# Title\ncontent"},
		{"fenced with language", "```markdown\n# Title\n```", "# Title"},
		{"fenced single line", "```\nline\n```", "line"},
		{"unclosed fence", "```\nsome text", "```\nsome text"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripMarkdownFence(tt.input)
			if got != tt.expect {
				t.Errorf("StripMarkdownFence(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestStripAgyOutputPreamble(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"plain text", "hello world", "hello world"},
		{"with preamble", "ULTRAWORK MODE ENABLED\n\nactual content", "actual content"},
		{"with blank lines", "\n\ncontent", "content"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAgyOutputPreamble(tt.input)
			if got != tt.expect {
				t.Errorf("StripAgyOutputPreamble(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"My Draft Title", "my-draft-title"},
		{"hello world", "hello-world"},
		{"special!@#chars", "special-chars"},
		{"a--b", "a-b"},
		{"", "draft"},
		{"---", "draft"},
		{"  already-slug  ", "already-slug"},
		{"Korean한글test", "korean-test"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.expect {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestFrontmatter(t *testing.T) {
	fm := Frontmatter("Test", "wiki", "notes", "gpt-4")
	if fm == "" {
		t.Error("expected non-empty frontmatter")
	}
	// Verify defaults
	fm2 := Frontmatter("", "", "", "")
	if fm2 == "" {
		t.Error("expected non-empty frontmatter with defaults")
	}
}

func TestWrite(t *testing.T) {
	t.Run("writes draft file", func(t *testing.T) {
		dir := t.TempDir()
		draftWikiDir := filepath.Join(dir, ".agent-harness", "draft-wiki")
		output := "---\ntitle: Test Draft\n---\n\n# Body\n"
		path, err := Write(dir, draftWikiDir, "Test Draft", "dev-fundamentals", "notes", "gpt-4", output)
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("draft file not found at %q: %v", path, err)
		}
		content, _ := os.ReadFile(path)
		if len(content) == 0 {
			t.Error("draft file is empty")
		}
	})

	t.Run("empty output returns error", func(t *testing.T) {
		dir := t.TempDir()
		draftWikiDir := filepath.Join(dir, ".agent-harness", "draft-wiki")
		_, err := Write(dir, draftWikiDir, "Test", "wiki", "notes", "gpt-4", "")
		if err == nil {
			t.Error("expected error for empty output")
		}
	})

	t.Run("output without frontmatter gets frontmatter added", func(t *testing.T) {
		dir := t.TempDir()
		draftWikiDir := filepath.Join(dir, ".agent-harness", "draft-wiki")
		output := "# Just a heading\ncontent"
		path, err := Write(dir, draftWikiDir, "Test", "wiki", "notes", "gpt-4", output)
		if err != nil {
			t.Fatalf("Write error: %v", err)
		}
		content, _ := os.ReadFile(path)
		if string(content[:4]) != "---\n" {
			t.Errorf("expected frontmatter at start, got %q", string(content[:50]))
		}
	})
}
