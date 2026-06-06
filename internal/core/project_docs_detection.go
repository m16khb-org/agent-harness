package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func detectFrameworks(root string, files []string, addEvidence func(string)) []string {
	frameworks := []string{}
	addFramework := func(name, evidence string) {
		frameworks = appendUnique(frameworks, name)
		if evidence != "" {
			addEvidence(evidence)
		}
	}
	for _, rel := range files {
		base := filepath.Base(rel)
		switch base {
		case "next.config.js", "next.config.mjs", "next.config.ts":
			addFramework("Next.js", rel)
		case "vite.config.js", "vite.config.mjs", "vite.config.ts":
			addFramework("Vite", rel)
		case "nuxt.config.js", "nuxt.config.ts":
			addFramework("Nuxt", rel)
		case "astro.config.js", "astro.config.mjs", "astro.config.ts":
			addFramework("Astro", rel)
		case "nest-cli.json":
			addFramework("NestJS", rel)
		}
		if rel == "go.mod" {
			for _, mod := range readGoModules(root) {
				switch {
				case strings.Contains(mod, "github.com/spf13/cobra"):
					addFramework("Cobra", "go.mod:github.com/spf13/cobra")
				case strings.Contains(mod, "github.com/gin-gonic/gin"):
					addFramework("Gin", "go.mod:github.com/gin-gonic/gin")
				case strings.Contains(mod, "github.com/go-chi/chi"):
					addFramework("chi", "go.mod:github.com/go-chi/chi")
				case strings.Contains(mod, "github.com/labstack/echo"):
					addFramework("Echo", "go.mod:github.com/labstack/echo")
				}
			}
		}
		if rel == "package.json" {
			for dep := range readPackageDependencies(filepath.Join(root, rel)) {
				switch dep {
				case "react":
					addFramework("React", "package.json:react")
				case "next":
					addFramework("Next.js", "package.json:next")
				case "vite":
					addFramework("Vite", "package.json:vite")
				case "vue":
					addFramework("Vue", "package.json:vue")
				case "svelte":
					addFramework("Svelte", "package.json:svelte")
				case "@angular/core":
					addFramework("Angular", "package.json:@angular/core")
				case "express":
					addFramework("Express", "package.json:express")
				case "@nestjs/core":
					addFramework("NestJS", "package.json:@nestjs/core")
				case "fastify":
					addFramework("Fastify", "package.json:fastify")
				case "prisma", "@prisma/client":
					addFramework("Prisma", "package.json:"+dep)
				}
			}
		}
	}
	return frameworks
}

func detectMonorepo(root string, files []string, addEvidence func(string)) bool {
	for _, rel := range files {
		switch rel {
		case "pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json":
			addEvidence(rel)
			return true
		}
		if strings.Contains(rel, "/") && (strings.HasSuffix(rel, "/package.json") || strings.HasSuffix(rel, "/go.mod") || strings.HasSuffix(rel, "/pyproject.toml") || strings.HasSuffix(rel, "/Cargo.toml")) {
			addEvidence(rel)
			return true
		}
	}
	if workspaces := readPackageWorkspaces(filepath.Join(root, "package.json")); len(workspaces) > 0 {
		addEvidence("package.json:workspaces")
		return true
	}
	return false
}

func inferProjectTypes(root string, signals ProjectSignals, frameworks []string, monorepo bool, addEvidence func(string)) []string {
	types := []string{}
	addType := func(v, evidence string) {
		types = appendUnique(types, v)
		if evidence != "" {
			addEvidence(evidence)
		}
	}
	if monorepo {
		addType("monorepo", "")
	}
	frontend := containsAnyString(frameworks, "React", "Next.js", "Vite", "Vue", "Svelte", "Angular", "Nuxt", "Astro")
	backend := containsAnyString(frameworks, "Express", "NestJS", "Fastify", "Gin", "chi", "Echo") || containsAnyString(signals.Languages, "Go")
	cli := containsAnyString(frameworks, "Cobra")
	if frontend {
		addType("frontend", "")
	}
	if backend {
		addType("backend", "")
	}
	if frontend && backend {
		addType("fullstack", "")
	}
	if cli || dirExists(filepath.Join(root, "cmd")) {
		addType("cli", "cmd/")
	}
	if len(types) == 0 && len(signals.Languages) > 0 {
		addType("library", "")
	}
	return types
}

func readPackageDependencies(path string) map[string]bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	var pkg struct {
		Dependencies         map[string]string `json:"dependencies"`
		DevDependencies      map[string]string `json:"devDependencies"`
		PeerDependencies     map[string]string `json:"peerDependencies"`
		OptionalDependencies map[string]string `json:"optionalDependencies"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return map[string]bool{}
	}
	out := map[string]bool{}
	for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies, pkg.PeerDependencies, pkg.OptionalDependencies} {
		for dep := range deps {
			out[dep] = true
		}
	}
	return out
}

func readPackageWorkspaces(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil
	}
	var direct []string
	if err := json.Unmarshal(raw["workspaces"], &direct); err == nil && len(direct) > 0 {
		return direct
	}
	var object struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(raw["workspaces"], &object); err == nil {
		return object.Packages
	}
	return nil
}

func readGoModules(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}

func containsAnyString(items []string, wants ...string) bool {
	set := map[string]bool{}
	for _, item := range items {
		set[item] = true
	}
	for _, want := range wants {
		if set[want] {
			return true
		}
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

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
