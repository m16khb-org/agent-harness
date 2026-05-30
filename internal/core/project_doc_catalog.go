package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ProjectDocCatalogEntry describes one project doc in the working repo's
// .agent-harness directory: its repo-relative path and a fixed description of
// what category of information it contains.
//
// The harness presents this catalog so the MAIN agent can decide which docs to
// read for the current prompt. The harness deliberately does not decide
// relevance itself: choosing "which docs matter for this prompt" is the
// judgment static analysis cannot do reliably, so it is left to the agent while
// the harness only provides an accurate, deterministic menu. The description is
// canonical metadata (from the doc's frontmatter or the name-keyed table), not a
// summary of the doc's current contents.
type ProjectDocCatalogEntry struct {
	RelPath     string `json:"rel_path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// DiscoverProjectDocs returns the catalog of <repoRoot>/.agent-harness/*.md for
// the repo currently being worked in. This is the TARGET repo's project docs,
// not the harness's own documentation, and not nested directories. Each doc's
// description comes from its meta frontmatter, falling back to the canonical
// name-keyed metadata so known docs are described even before bootstrap writes
// the frontmatter. The result is deterministic (sorted by path) so it can live
// in an immutable context prefix. Returns nil when repoRoot is empty or has no
// .agent-harness docs.
func DiscoverProjectDocs(repoRoot string) []ProjectDocCatalogEntry {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil
	}
	dir := filepath.Join(repoRoot, ".agent-harness")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	catalog := []ProjectDocCatalogEntry{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		_, description, _, _ := parseDocFrontmatter(string(content))
		if description == "" {
			if canonical, ok := DocMetaDescription(entry.Name()); ok {
				description = canonical
			}
		}
		catalog = append(catalog, ProjectDocCatalogEntry{
			RelPath:     filepath.ToSlash(filepath.Join(".agent-harness", entry.Name())),
			Title:       firstMarkdownTitle(string(content)),
			Description: description,
		})
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].RelPath < catalog[j].RelPath })
	return catalog
}

// FormatProjectDocCatalog renders a compact one-line menu of the project docs,
// describing each by its canonical metadata so the main agent can judge which to
// read. Returns "" when there is nothing to present.
func FormatProjectDocCatalog(entries []ProjectDocCatalogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	items := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimPrefix(entry.RelPath, ".agent-harness/")
		meta := entry.Description
		if meta == "" {
			meta = entry.Title
		}
		if meta == "" {
			items = append(items, name)
			continue
		}
		items = append(items, name+"="+meta)
	}
	return "project docs (read what's relevant): " + strings.Join(items, "; ")
}

// firstMarkdownTitle returns the first level-1 heading in content, skipping any
// leading frontmatter block. Returns "" when there is no H1.
func firstMarkdownTitle(content string) string {
	_, _, body, ok := parseDocFrontmatter(content)
	if !ok {
		body = content
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return ""
}
