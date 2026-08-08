package guard

import (
	"net/url"
	"path/filepath"
	"strings"
)

func isGuardRelevantPath(rel string) bool {
	if secretPathRe.MatchString(filepath.ToSlash(rel)) {
		return true
	}
	ext := strings.ToLower(filepath.Ext(rel))
	if ext == "" {
		return strings.Contains(filepath.ToSlash(rel), "testdata/") || strings.HasSuffix(rel, "Dockerfile")
	}
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh", ".bash", ".zsh", ".fish", ".yaml", ".yml", ".json", ".toml", ".md", ".sql":
		return true
	default:
		return false
	}
}

func isTestPath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	return strings.Contains(p, "test") || strings.Contains(p, "spec") || strings.Contains(p, "fixture") || strings.Contains(p, "golden")
}

func isExecutableTestSourcePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	if strings.Contains(p, "testdata/") || strings.Contains(p, ".golden.") || strings.Contains(p, "/fixtures/") || strings.Contains(p, "/fixture/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh":
		return strings.Contains(p, "test") || strings.Contains(p, "spec")
	default:
		return false
	}
}

func guardAllowsFixtureURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	return host == "example.com" ||
		host == "example.org" ||
		host == "example.net" ||
		host == "example.invalid" ||
		host == "127.0.0.1" ||
		host == "localhost" ||
		(host == "github.com" && strings.HasPrefix(path, "/example/"))
}

func isSourcePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(rel))
	if isTestPath(p) || strings.HasPrefix(p, ".agent-harness/") || strings.HasPrefix(p, "docs/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java", ".kt", ".kts", ".cs", ".php", ".swift", ".scala", ".sh", ".sql":
		return true
	default:
		return false
	}
}

func isContractSurfacePath(rel string) bool {
	p := filepath.ToSlash(rel)
	return strings.HasPrefix(p, "cmd/harness/") || strings.HasPrefix(p, "internal/adapter/") || strings.HasPrefix(p, "internal/core/")
}
