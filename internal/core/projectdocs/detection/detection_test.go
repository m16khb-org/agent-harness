package detection

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestAppendUnique(t *testing.T) {
	got := appendUnique(nil, "a")
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("expected [a], got %v", got)
	}
	got = appendUnique(got, "b")
	if len(got) != 2 {
		t.Fatalf("expected [a b], got %v", got)
	}
	got = appendUnique(got, "a")
	if len(got) != 2 {
		t.Fatalf("expected dedup, got %v", got)
	}
}

func TestContainsAnyString(t *testing.T) {
	if containsAnyString(nil) {
		t.Fatal("empty list should not contain anything")
	}
	if containsAnyString(nil, "x") {
		t.Fatal("empty list should not contain x")
	}
	if !containsAnyString([]string{"a", "b"}, "b", "c") {
		t.Fatal("should find b")
	}
	if containsAnyString([]string{"a"}, "x") {
		t.Fatal("should not find x")
	}
}

func TestFrameworks_FromFiles(t *testing.T) {
	dir := t.TempDir()

	// package.json with react dependency
	writeFile(t, filepath.Join(dir, "package.json"), `{"dependencies":{"react":"^18.0.0"}}`)

	// go.mod with cobra
	writeFile(t, filepath.Join(dir, "go.mod"), "module example\nrequire github.com/spf13/cobra v1.0.0\n")

	// nest-cli.json
	writeFile(t, filepath.Join(dir, "nest-cli.json"), "{}")

	files := []string{"package.json", "go.mod", "nest-cli.json"}

	var evidence []string
	frameworks := Frameworks(dir, files, func(e string) { evidence = append(evidence, e) })

	if len(frameworks) == 0 {
		t.Fatal("expected at least one framework")
	}
	sort.Strings(frameworks)
	sort.Strings(evidence)

	expected := []string{"Cobra", "NestJS", "React"}
	for _, name := range expected {
		if !containsString(frameworks, name) {
			t.Errorf("expected framework %q, got %v", name, frameworks)
		}
	}
	if len(evidence) == 0 {
		t.Error("expected evidence")
	}
}

func TestFrameworks_ConfigFiles(t *testing.T) {
	dir := t.TempDir()

	for _, f := range []string{"next.config.js", "vite.config.ts", "nuxt.config.ts", "astro.config.mjs"} {
		writeFile(t, filepath.Join(dir, f), "")
	}

	var evidence []string
	frameworks := Frameworks(dir, []string{"next.config.js", "vite.config.ts", "nuxt.config.ts", "astro.config.mjs"}, func(e string) { evidence = append(evidence, e) })

	sort.Strings(frameworks)
	expected := []string{"Astro", "Next.js", "Nuxt", "Vite"}
	for _, name := range expected {
		if !containsString(frameworks, name) {
			t.Errorf("expected framework %q, got %v", name, frameworks)
		}
	}
}

func TestFrameworks_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	var evidence []string
	frameworks := Frameworks(dir, []string{"README.md"}, func(e string) { evidence = append(evidence, e) })
	if len(frameworks) != 0 {
		t.Errorf("expected no frameworks, got %v", frameworks)
	}
}

func TestMonorepo_WorkspaceFiles(t *testing.T) {
	dir := t.TempDir()

	signals := []string{"pnpm-workspace.yaml", "turbo.json", "nx.json", "lerna.json"}
	for _, s := range signals {
		t.Run(s, func(t *testing.T) {
			var evidence []string
			got := Monorepo(dir, []string{s}, func(e string) { evidence = append(evidence, e) })
			if !got {
				t.Errorf("expected monorepo=true for %s", s)
			}
			if len(evidence) == 0 {
				t.Errorf("expected evidence for %s", s)
			}
		})
	}
}

func TestMonorepo_NestedPackageFiles(t *testing.T) {
	dir := t.TempDir()
	nested := []string{"packages/a/package.json", "services/b/go.mod", "libs/c/Cargo.toml"}
	for _, n := range nested {
		t.Run(n, func(t *testing.T) {
			var evidence []string
			got := Monorepo(dir, []string{n}, func(e string) { evidence = append(evidence, e) })
			if !got {
				t.Errorf("expected monorepo=true for %s", n)
			}
		})
	}
}

func TestMonorepo_NotMonorepo(t *testing.T) {
	dir := t.TempDir()
	var evidence []string
	got := Monorepo(dir, []string{"README.md", "src/main.go"}, func(e string) { evidence = append(evidence, e) })
	if got {
		t.Error("expected monorepo=false")
	}
}

func TestMonorepo_PackageWorkspaces(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"workspaces":["packages/*"]}`)
	var evidence []string
	got := Monorepo(dir, []string{"package.json"}, func(e string) { evidence = append(evidence, e) })
	if !got {
		t.Error("expected monorepo=true from package.json workspaces")
	}
}

func TestProjectTypes(t *testing.T) {
	tests := []struct {
		name       string
		languages  []string
		frameworks []string
		monorepo   bool
		cmdDir     bool
		wantTypes  []string
	}{
		{"frontend", []string{"TypeScript"}, []string{"React"}, false, false, []string{"frontend"}},
		{"backend go", []string{"Go"}, nil, false, false, []string{"backend"}},
		{"fullstack", []string{"TypeScript", "Go"}, []string{"React", "Express"}, false, false, []string{"frontend", "backend", "fullstack"}},
		{"cli cobra", []string{"Go"}, []string{"Cobra"}, false, false, []string{"backend", "cli"}},
		{"monorepo", []string{"TypeScript"}, []string{"React"}, true, false, []string{"monorepo", "frontend"}},
		{"library", []string{"Rust"}, nil, false, false, []string{"library"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.cmdDir {
				os.MkdirAll(filepath.Join(dir, "cmd"), 0o755)
			}
			var evidence []string
			types := ProjectTypes(dir, tt.languages, tt.frameworks, tt.monorepo, func(e string) { evidence = append(evidence, e) })
			for _, want := range tt.wantTypes {
				if !containsString(types, want) {
					t.Errorf("expected type %q in %v", want, types)
				}
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	dir := t.TempDir()
	if !dirExists(dir) {
		t.Error("temp dir should exist")
	}
	if dirExists(filepath.Join(dir, "nope")) {
		t.Error("non-existent dir should not exist")
	}
}

func TestReadGoModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example\nrequire github.com/spf13/cobra v1.0.0\nrequire github.com/gin-gonic/gin v1.9.0\n")
	lines := readGoModules(dir)
	if len(lines) < 2 {
		t.Fatalf("expected >= 2 lines, got %d: %v", len(lines), lines)
	}
}

func TestReadGoModules_Missing(t *testing.T) {
	dir := t.TempDir()
	lines := readGoModules(dir)
	if lines != nil {
		t.Errorf("expected nil for missing go.mod, got %v", lines)
	}
}

func TestReadPackageDependencies(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
		"dependencies": {"react": "^18.0.0", "express": "^4.0.0"},
		"devDependencies": {"@types/react": "^18.0.0"},
		"peerDependencies": {"react-dom": "^18.0.0"}
	}`)
	deps := readPackageDependencies(filepath.Join(dir, "package.json"))
	for _, want := range []string{"react", "express", "@types/react", "react-dom"} {
		if !deps[want] {
			t.Errorf("expected dep %q", want)
		}
	}
}

func TestReadPackageDependencies_Missing(t *testing.T) {
	dir := t.TempDir()
	deps := readPackageDependencies(filepath.Join(dir, "package.json"))
	if len(deps) != 0 {
		t.Errorf("expected empty, got %v", deps)
	}
}

func TestReadPackageWorkspaces(t *testing.T) {
	t.Run("direct array", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "package.json"), `{"workspaces":["packages/*","tools/*"]}`)
		ws := readPackageWorkspaces(filepath.Join(dir, "package.json"))
		if len(ws) != 2 {
			t.Fatalf("expected 2 workspaces, got %v", ws)
		}
	})
	t.Run("object with packages", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "package.json"), `{"workspaces":{"packages":["a","b"]}}`)
		ws := readPackageWorkspaces(filepath.Join(dir, "package.json"))
		if len(ws) != 2 {
			t.Fatalf("expected 2 workspaces, got %v", ws)
		}
	})
	t.Run("no workspaces", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)
		ws := readPackageWorkspaces(filepath.Join(dir, "package.json"))
		if ws != nil {
			t.Errorf("expected nil, got %v", ws)
		}
	})
	t.Run("missing", func(t *testing.T) {
		dir := t.TempDir()
		ws := readPackageWorkspaces(filepath.Join(dir, "package.json"))
		if ws != nil {
			t.Errorf("expected nil, got %v", ws)
		}
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
