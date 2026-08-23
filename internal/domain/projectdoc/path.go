package projectdoc

import (
	"fmt"
	"path/filepath"
	"strings"
)

func PrefixedProjectDocNames() []string {
	names := ProjectDocNames()
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.ToSlash(filepath.Join(ProjectDocsDir, name)))
	}
	return out
}

// PrefixedKnownProjectDocNames returns every required and optional project doc
// path. Optional docs render only when their condition holds (for example
// DESIGN.md only for repositories with a client surface), so iteration over
// this list must skip empty render results.
func PrefixedKnownProjectDocNames() []string {
	names := append(ProjectDocNames(), optionalProjectDocNames...)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.ToSlash(filepath.Join(ProjectDocsDir, name)))
	}
	return out
}

func NormalizeRelPath(relPath string) (string, error) {
	rel := filepath.ToSlash(strings.TrimSpace(relPath))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" {
		return "", fmt.Errorf("rel_path is required")
	}
	if strings.HasPrefix(rel, ProjectDocsDir+"/") {
		rel = strings.TrimPrefix(rel, ProjectDocsDir+"/")
	}
	names := AllowedProjectDocNames()
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	if !allowed[rel] {
		// Family module documents (adr/<record>.md, testing/overview.md, ...)
		// are valid revision targets in folder-first repositories.
		if _, ok := familyModulePath(rel); ok {
			return filepath.ToSlash(filepath.Join(ProjectDocsDir, rel)), nil
		}
		return "", fmt.Errorf("unsupported project doc %q: use one of %s", relPath, strings.Join(names, ", "))
	}
	return filepath.ToSlash(filepath.Join(ProjectDocsDir, rel)), nil
}

func NonEmptyStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func AppendUnique(items []string, v string) []string {
	for _, item := range items {
		if item == v {
			return items
		}
	}
	return append(items, v)
}
