package projectdoc

import (
	projectdocdomain "agent-harness/internal/domain/projectdoc"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	projectDocCatalogMaxEntries    = 64
	projectDocCatalogMaxRawEntries = 128
	projectDocCatalogMaxFileBytes  = 256 * 1024
	projectDocCatalogMaxTotalBytes = 2 * 1024 * 1024
)

// projectdocdomain.ProjectDocCatalogEntry describes one project doc in the working repo's
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

// DiscoverProjectDocs returns the catalog of <repoRoot>/.agent-harness/*.md for
// the repo currently being worked in. This is the TARGET repo's project docs,
// not the harness's own documentation, and not nested directories. Each doc's
// description comes from its meta frontmatter, falling back to the canonical
// name-keyed metadata so known docs are described even before bootstrap writes
// the frontmatter. The result is deterministic (sorted by path) so it can live
// in an immutable context prefix. Returns nil when repoRoot is empty or has no
// .agent-harness docs.
func DiscoverProjectDocs(repoRoot string) []projectdocdomain.ProjectDocCatalogEntry {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil
	}
	defer root.Close()
	docsInfo, err := root.Lstat(".agent-harness")
	if err != nil || !docsInfo.IsDir() || docsInfo.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	docsDir, err := root.Open(".agent-harness")
	if err != nil {
		return nil
	}
	defer docsDir.Close()
	entries, err := readProjectDocCatalogEntries(docsDir)
	if err != nil {
		return nil
	}
	catalog := []projectdocdomain.ProjectDocCatalogEntry{}
	totalBytes := 0
	for _, entry := range entries {
		if len(catalog) == projectDocCatalogMaxEntries || totalBytes == projectDocCatalogMaxTotalBytes {
			break
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, ok := readProjectDocCatalogFile(root, filepath.Join(".agent-harness", entry.Name()), projectDocCatalogMaxTotalBytes-totalBytes)
		if !ok {
			continue
		}
		totalBytes += len(content)
		_, description, _, _ := projectdocdomain.ParseFrontmatter(string(content))
		if description == "" {
			if canonical, ok := projectdocdomain.DocMetaDescription(entry.Name()); ok {
				description = canonical
			}
		}
		catalog = append(catalog, projectdocdomain.ProjectDocCatalogEntry{
			RelPath:     filepath.ToSlash(filepath.Join(".agent-harness", entry.Name())),
			Title:       firstMarkdownTitle(string(content)),
			Description: description,
		})
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].RelPath < catalog[j].RelPath })
	return catalog
}

func readProjectDocCatalogEntries(dir *os.File) ([]os.DirEntry, error) {
	entries, err := dir.ReadDir(projectDocCatalogMaxRawEntries)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return entries, nil
}

func readProjectDocCatalogFile(root *os.Root, name string, remaining int) ([]byte, bool) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, false
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return nil, false
	}
	limit := projectDocCatalogMaxFileBytes + 1
	if remaining+1 < limit {
		limit = remaining + 1
	}
	content, err := io.ReadAll(io.LimitReader(file, int64(limit)))
	if err != nil || len(content) > projectDocCatalogMaxFileBytes || len(content) > remaining {
		return nil, false
	}
	return content, true
}

// FormatProjectDocCatalog renders a compact one-line menu of the project docs,
// describing each by its canonical metadata so the main agent can judge which to
// read. Returns "" when there is nothing to present.
func FormatProjectDocCatalog(entries []projectdocdomain.ProjectDocCatalogEntry) string {
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
	_, _, body, ok := projectdocdomain.ParseFrontmatter(content)
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
