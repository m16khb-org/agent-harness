package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

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
