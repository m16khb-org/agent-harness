package searchrouting

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchTokenName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"rg", "rg"},
		{"/usr/bin/rg", "rg"},
		{`"grep"`, "grep"},
		{"/usr/bin/git", "git"},
		{"", "."},
	}
	for _, tt := range tests {
		got := searchTokenName(tt.input)
		if got != tt.expected {
			t.Errorf("searchTokenName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSearchTargetToken(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"./src", "./src"},
		{`"cmd/"`, "cmd/"},
		{"/tmp/test", "/tmp/test"},
		{"", ""},
	}
	for _, tt := range tests {
		got := searchTargetToken(tt.input)
		if got != tt.expected {
			t.Errorf("searchTargetToken(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSearchPatternToken(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"func main", "func main"},
		{`"type Handler"`, "type Handler"},
		{"-i", ""},
		{"./src/", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := searchPatternToken(tt.input)
		if got != tt.expected {
			t.Errorf("searchPatternToken(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsShellTool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"bash", true},
		{"sh", true},
		{"zsh", true},
		{"shell", true},
		{"Bash", true},
		{"read_file", false},
		{"codegraph", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsShellTool(tt.input)
		if got != tt.expected {
			t.Errorf("IsShellTool(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsCodeGraphTool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"codegraph", true},
		{"codegraph_search", true},
		{"mcp__codegraph__tool", true},
		{"bash", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsCodeGraphTool(tt.input)
		if got != tt.expected {
			t.Errorf("IsCodeGraphTool(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestLooksLikeExactSearchQuery(t *testing.T) {
	tests := []struct {
		query    string
		expected bool
	}{
		{"TODO: refactor", true},
		{"FIXME: bug", true},
		{"log.Error", true},
		{"console.log", true},
		{"// comment", true},
		{"main.go", true},
		{"MyService.ts", true},
		{"cannot find module", true},
		{"undefined variable", true},
		{"CLIENT_SECRET", true},
		{"AppError", true},
		{"simple function name", false},
		{"foo", false},
		{"", false},
	}
	for _, tt := range tests {
		got := LooksLikeExactSearchQuery(tt.query)
		if got != tt.expected {
			t.Errorf("LooksLikeExactSearchQuery(%q) = %v, want %v", tt.query, got, tt.expected)
		}
	}
}

func TestLooksLikeSearchTarget(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{".", true},
		{"./cmd", true},
		{"/absolute/path", true},
		{"src/main.go", true},
		{"*.go", true},
		{"cmd", true},
		{"internal", true},
		{"docs", true},
		{"README.md", true},
		{"myVar", false},
		{"func", false},
	}
	for _, tt := range tests {
		got := looksLikeSearchTarget(tt.token)
		if got != tt.expected {
			t.Errorf("looksLikeSearchTarget(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestIsDocsOrFixtureTarget(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"./docs/README.md", true},
		{".agent-harness/CONSTITUTION.md", true},
		{"testdata/fixture.json", true},
		{"golden/snapshot.json", true},
		{"fixtures/data.json", true},
		{"README.md", true},
		{"readme.txt", true},  // "readme." prefix matches "readme.txt"
		{"src/main.go", false},
		{"cmd/harness/main.go", false},
	}
	for _, tt := range tests {
		got := isDocsOrFixtureTarget(tt.token)
		if got != tt.expected {
			t.Errorf("isDocsOrFixtureTarget(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestIsRepoLocalSearchTarget(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0o755)

	tests := []struct {
		name     string
		target   string
		repo     string
		expected bool
	}{
		{"empty target", "", dir, true},
		{"relative", "src", dir, true},
		{"absolute inside", subDir, dir, true},
		{"absolute outside", "/tmp/outside", dir, false},
		{"empty repo", "/tmp/outside", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRepoLocalSearchTarget(tt.target, tt.repo)
			if got != tt.expected {
				t.Errorf("isRepoLocalSearchTarget(%q, %q) = %v, want %v", tt.target, tt.repo, got, tt.expected)
			}
		})
	}
}

func TestLooksLikeStructuralSourcePattern(t *testing.T) {
	tests := []struct {
		pattern  string
		expected bool
	}{
		{"func main", true},
		{"function test", true},
		{"type Handler", true},
		{"class MyClass", true},
		{"interface Reader", true},
		{"struct Config", true},
		{"enum Color", true},
		{"def process", true},
		{"@Controller", true},
		{"@Injectable", true},
		{"simpletext", false},
		{"", false},
	}
	for _, tt := range tests {
		got := looksLikeStructuralSourcePattern(tt.pattern)
		if got != tt.expected {
			t.Errorf("looksLikeStructuralSourcePattern(%q) = %v, want %v", tt.pattern, got, tt.expected)
		}
	}
}

func TestHasStructuralSourceSearchPattern(t *testing.T) {
	t.Run("has structural pattern", func(t *testing.T) {
		if !HasStructuralSourceSearchPattern([]string{"func main", "./src"}) {
			t.Error("expected true for structural pattern")
		}
	})
	t.Run("no structural pattern", func(t *testing.T) {
		if HasStructuralSourceSearchPattern([]string{"simpletext", "./src"}) {
			t.Error("expected false for non-structural pattern")
		}
	})
	t.Run("all flag args", func(t *testing.T) {
		if HasStructuralSourceSearchPattern([]string{"-i", "-n", "./src"}) {
			t.Error("expected false for flag-only args")
		}
	})
	t.Run("empty", func(t *testing.T) {
		if HasStructuralSourceSearchPattern([]string{}) {
			t.Error("expected false for empty")
		}
	})
}

func TestSourceSearchNeedsCodeGraph(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)

	t.Run("structural pattern in source dir", func(t *testing.T) {
		if !SourceSearchNeedsCodeGraph([]string{"func main", "./src"}, dir) {
			t.Error("expected true for structural pattern in source dir")
		}
	})
	t.Run("structural pattern in docs dir", func(t *testing.T) {
		if SourceSearchNeedsCodeGraph([]string{"func main", "./docs"}, dir) {
			t.Error("expected false for structural pattern in docs dir")
		}
	})
	t.Run("no structural pattern", func(t *testing.T) {
		if SourceSearchNeedsCodeGraph([]string{"some error text", "./src"}, dir) {
			t.Error("expected false for no structural pattern")
		}
	})
	t.Run("no targets defaults to true", func(t *testing.T) {
		if !SourceSearchNeedsCodeGraph([]string{"func main"}, dir) {
			t.Error("expected true for no targets (default to code graph)")
		}
	})
}

func TestSearchRoutingBlockReason(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)

	t.Run("bash with structural search blocked", func(t *testing.T) {
		reason := SearchRoutingBlockReason("bash", `rg "func main" ./src`, dir)
		if reason == "" {
			t.Error("expected block reason for bash structural search")
		}
	})
	t.Run("bash with exact search allowed", func(t *testing.T) {
		reason := SearchRoutingBlockReason("bash", `rg "TODO: refactor" .`, dir)
		if reason != "" {
			t.Errorf("expected no block for exact search, got %q", reason)
		}
	})
	t.Run("codegraph with exact query blocked", func(t *testing.T) {
		reason := SearchRoutingBlockReason("codegraph", `TODO fixme`, dir)
		if reason == "" {
			t.Error("expected block for codegraph on exact query")
		}
	})
	t.Run("non-shell non-codegraph passes", func(t *testing.T) {
		reason := SearchRoutingBlockReason("read_file", `anything`, dir)
		if reason != "" {
			t.Errorf("expected no block, got %q", reason)
		}
	})
}

func TestShouldBlockRawStructuralSourceSearch(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)

	t.Run("rg func in src should block", func(t *testing.T) {
		if !ShouldBlockRawStructuralSourceSearch(`rg "func main" ./src`, dir) {
			t.Error("expected block")
		}
	})
	t.Run("rg TODO should not block", func(t *testing.T) {
		if ShouldBlockRawStructuralSourceSearch(`rg "TODO: fix" .`, dir) {
			t.Error("expected no block")
		}
	})
	t.Run("empty command", func(t *testing.T) {
		if ShouldBlockRawStructuralSourceSearch("", dir) {
			t.Error("expected no block for empty")
		}
	})
}
