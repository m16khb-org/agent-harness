package worktreetools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageManagerDetectsLockfiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		want string
	}{
		{name: "pnpm", file: "pnpm-lock.yaml", want: "pnpm"},
		{name: "yarn", file: "yarn.lock", want: "yarn"},
		{name: "npm", file: "package-lock.json", want: "npm"},
		{name: "none", file: "", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.file), []byte("lock"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got := PackageManager(dir); got != tc.want {
				t.Fatalf("PackageManager = %q, want %q", got, tc.want)
			}
		})
	}
	if got := PackageManager(t.TempDir()); got != "" {
		t.Fatalf("without package.json PackageManager = %q", got)
	}
}

func TestPrepareDependenciesHandlesNoManagerReadyAndManual(t *testing.T) {
	noManager := t.TempDir()
	result := PrepareResult{}
	if err := PrepareDependencies(noManager, &result); err != nil {
		t.Fatalf("no manager PrepareDependencies returned error: %v", err)
	}
	if result.DependenciesChecked || result.DependenciesReady {
		t.Fatalf("unexpected result for no manager: %#v", result)
	}

	ready := t.TempDir()
	if err := os.WriteFile(filepath.Join(ready, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ready, "pnpm-lock.yaml"), []byte("lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ready, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	result = PrepareResult{}
	if err := PrepareDependencies(ready, &result); err != nil {
		t.Fatalf("ready PrepareDependencies returned error: %v", err)
	}
	if !result.DependenciesChecked || !result.DependenciesReady || result.DependenciesAction != "already_present" {
		t.Fatalf("unexpected ready result: %#v", result)
	}

	manual := t.TempDir()
	if err := os.WriteFile(filepath.Join(manual, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manual, "yarn.lock"), []byte("lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	result = PrepareResult{}
	if err := PrepareDependencies(manual, &result); err != nil {
		t.Fatalf("manual PrepareDependencies returned error: %v", err)
	}
	if !result.DependenciesChecked || result.DependenciesReady || result.DependenciesAction != "manual" {
		t.Fatalf("unexpected manual result: %#v", result)
	}
}
