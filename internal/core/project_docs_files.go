package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func listInterestingFiles(root string) []string {
	interesting := map[string]bool{}
	maxDepth := 4
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "build" || base == ".agent-harness" {
				return filepath.SkipDir
			}
			if len(parts) > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(rel)
		if len(parts) <= maxDepth && (isProjectSignalFile(base) || strings.HasPrefix(rel, ".github/workflows/") || strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.ts")) {
			interesting[rel] = true
		}
		return nil
	})
	out := make([]string, 0, len(interesting))
	for rel := range interesting {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func isProjectSignalFile(base string) bool {
	switch base {
	case "AGENTS.md", "CLAUDE.md", "README.md", "go.mod", "go.sum", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml", "yarn.lock", "package-lock.json", "pyproject.toml", "requirements.txt", "Cargo.toml", "Cargo.lock", "Makefile", "Taskfile.yml", "Taskfile.yaml", "Dockerfile", "docker-compose.yml", "docker-compose.yaml", "next.config.js", "next.config.mjs", "next.config.ts", "vite.config.js", "vite.config.mjs", "vite.config.ts", "nuxt.config.js", "nuxt.config.ts", "astro.config.js", "astro.config.mjs", "astro.config.ts", "tailwind.config.js", "tailwind.config.ts", "tsconfig.json", "turbo.json", "nx.json", "lerna.json", "nest-cli.json":
		return true
	default:
		return false
	}
}
