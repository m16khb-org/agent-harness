package docs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type projectDocsManifest struct {
	Families []projectDocsFamily `json:"families"`
}

type projectDocsFamily struct {
	ModuleDir string `json:"module_dir"`
}

type authoringScope struct {
	moduleDirs []string
}

func loadAuthoringScope(root string) authoringScope {
	data, err := os.ReadFile(filepath.Join(
		root,
		".agent-harness",
		"documentation",
		"manifest.json",
	))
	if err != nil {
		return authoringScope{}
	}
	var manifest projectDocsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return authoringScope{}
	}
	scope := authoringScope{}
	for _, family := range manifest.Families {
		moduleDir := filepath.ToSlash(filepath.Clean(family.ModuleDir))
		if !isAgentHarnessSubdirectory(moduleDir) {
			continue
		}
		scope.moduleDirs = append(scope.moduleDirs, moduleDir+"/")
	}
	return scope
}

func (scope authoringScope) includes(relativePath string) bool {
	relativePath = filepath.ToSlash(filepath.Clean(relativePath))
	if filepath.ToSlash(filepath.Dir(relativePath)) == ".agent-harness" &&
		filepath.Ext(relativePath) == ".md" {
		return true
	}
	if strings.HasPrefix(relativePath, ".agent-harness/documentation/") {
		return true
	}
	for _, moduleDir := range scope.moduleDirs {
		if strings.HasPrefix(relativePath, moduleDir) {
			return true
		}
	}
	return false
}

func isAgentHarnessSubdirectory(path string) bool {
	return strings.HasPrefix(path, ".agent-harness/") &&
		!strings.Contains(path, "/../") &&
		path != ".agent-harness/.."
}
