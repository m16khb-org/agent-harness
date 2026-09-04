package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var staleCanonicalLayerPaths = []string{
	"internal/core",
	"internal/adapter/cli",
	"internal/adapter/fs",
}

var canonicalArchitectureDocs = map[string][]string{
	"README.md": {
		"cmd/issueops/",
		"internal/contract/",
		"internal/domain/",
		"internal/application/",
		"internal/port/",
		"internal/adapter/",
	},
	"AGENTS.md": {
		"cmd/issueops/",
		"internal/contract/",
		"internal/domain/",
		"internal/application/",
		"internal/port/",
		"internal/adapter/",
	},
	"README.en.md": {
		"cmd/issueops/",
		"internal/contract/",
		"internal/domain/",
		"internal/application/",
		"internal/port/",
		"internal/adapter/",
	},
	".issueops/ARCHITECTURE.md": {
		"architecture/hexagonal-core.md",
		"internal/domain",
		"internal/application",
		"internal/port",
		"internal/adapter",
	},
	".issueops/CONVENTIONS.md": {
		"conventions/go-and-packages.md",
		"internal/domain",
		"internal/application",
		"internal/port",
		"internal/adapter",
	},
	".issueops/architecture/hexagonal-core.md": {
		"cmd/issueops",
		"internal/contract",
		"internal/domain",
		"internal/application",
		"internal/port",
		"internal/adapter",
	},
	".issueops/conventions/go-and-packages.md": {
		"cmd/issueops",
		"internal/contract",
		"internal/domain",
		"internal/application",
		"internal/port",
		"internal/adapter",
	},
}

var canonicalFirstPartyHostDocs = []string{
	"README.md",
	"README.en.md",
	"AGENTS.md",
	".issueops/architecture/hexagonal-core.md",
	"skills/self-augment/SKILL.md",
	"skills/self-augment/SELF_AUGMENTATION.md",
}

func TestCanonicalArchitectureDocsUseCurrentLayerPaths(t *testing.T) {
	root := findRepoRoot(t)
	for relPath, required := range canonicalArchitectureDocs {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
		if err != nil {
			t.Fatalf("read canonical architecture doc %s: %v", relPath, err)
		}
		text := string(body)
		for _, stale := range staleCanonicalLayerPaths {
			if strings.Contains(text, stale) {
				t.Errorf("%s still references removed current-layer path %q", relPath, stale)
			}
		}
		for _, current := range required {
			if !strings.Contains(text, current) {
				t.Errorf("%s is missing current-layer path %q", relPath, current)
			}
		}
	}
}

func TestCanonicalHostDocsNameEveryFirstPartyHost(t *testing.T) {
	root := findRepoRoot(t)
	for _, relPath := range canonicalFirstPartyHostDocs {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
		if err != nil {
			t.Fatalf("read canonical host doc %s: %v", relPath, err)
		}
		text := string(body)
		for _, host := range []string{"Codex", "Claude", "Omo"} {
			if !strings.Contains(text, host) {
				t.Errorf("%s omits first-party host %q", relPath, host)
			}
		}
	}
}
