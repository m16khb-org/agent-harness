package projectdoc

import (
	"fmt"
	"path/filepath"
	"strings"
)

func PrefixedProjectDocNames() []string {
	out := make([]string, 0, len(projectDocNames))
	for _, name := range projectDocNames {
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
	allowed := map[string]bool{}
	for _, name := range projectDocNames {
		allowed[name] = true
	}
	if !allowed[rel] {
		return "", fmt.Errorf("unsupported project doc %q: use one of %s", relPath, strings.Join(projectDocNames, ", "))
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
