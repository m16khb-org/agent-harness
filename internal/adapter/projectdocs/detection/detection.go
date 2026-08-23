package detection

import (
	"path/filepath"
	"strings"
)

func Frameworks(root string, files []string, addEvidence func(string)) []string {
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
		case "tauri.conf.json", "tauri.conf.json5":
			addFramework("Tauri", rel)
		case "electron-builder.yml", "electron-builder.yaml", "electron.vite.config.ts":
			addFramework("Electron", rel)
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
				case "electron":
					addFramework("Electron", "package.json:electron")
				case "@tauri-apps/api", "@tauri-apps/cli":
					addFramework("Tauri", "package.json:"+dep)
				}
			}
		}
	}
	return frameworks
}

func Monorepo(root string, files []string, addEvidence func(string)) bool {
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

func ProjectTypes(root string, languages, frameworks []string, monorepo bool, addEvidence func(string)) []string {
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
	desktopClient := containsAnyString(frameworks, "Tauri", "Electron")
	backend := containsAnyString(frameworks, "Express", "NestJS", "Fastify", "Gin", "chi", "Echo") || containsAnyString(languages, "Go")
	cli := containsAnyString(frameworks, "Cobra")
	if frontend {
		addType("frontend", "")
	}
	if desktopClient {
		addType("desktop-client", "")
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
	if len(types) == 0 && len(languages) > 0 {
		addType("library", "")
	}
	return types
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
