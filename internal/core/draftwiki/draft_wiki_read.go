package draftwiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/draftwiki/draftmeta"
)

func resolveDraftWikiDraft(root, draftPath string) (DraftWikiDraft, error) {
	if strings.TrimSpace(draftPath) == "" {
		return DraftWikiDraft{}, fmt.Errorf("draft path is required")
	}
	path := draftPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return DraftWikiDraft{}, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return DraftWikiDraft{}, err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return DraftWikiDraft{}, fmt.Errorf("draft path escapes repo root: %s", draftPath)
	}
	relSlash := filepath.ToSlash(rel)
	status, ok := draftWikiStatusFromRel(relSlash)
	if !ok {
		return DraftWikiDraft{}, fmt.Errorf("draft path must be inside %s/{draft,approved,rejected}: %s", DraftWikiDir, relSlash)
	}
	if !strings.HasSuffix(relSlash, ".md") {
		return DraftWikiDraft{}, fmt.Errorf("draft path must be a markdown file: %s", relSlash)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return DraftWikiDraft{}, err
	}
	if info.IsDir() {
		return DraftWikiDraft{}, fmt.Errorf("draft path is a directory: %s", relSlash)
	}
	return readDraftWikiDraft(root, abs, status)
}

func readDraftWikiDraft(root, path, status string) (DraftWikiDraft, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return DraftWikiDraft{}, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	meta := parseDraftWikiFrontmatter(string(b))
	title := meta["title"]
	if title == "" {
		title, _ = readDocHeadings(path)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		return DraftWikiDraft{}, err
	}
	return DraftWikiDraft{
		RelPath:    rel,
		Path:       path,
		Status:     status,
		Title:      title,
		Source:     meta["source"],
		TargetWiki: meta["target_wiki"],
		TargetType: meta["target_type"],
		Summary:    meta["summary"],
		Bytes:      info.Size(),
	}, nil
}

func draftWikiStatusFromRel(rel string) (string, bool) {
	prefix := DraftWikiDir + "/"
	if !strings.HasPrefix(rel, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(rel, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", false
	}
	if !isDraftWikiStatus(parts[0]) {
		return "", false
	}
	return parts[0], true
}

func isDraftWikiStatus(status string) bool {
	for _, candidate := range draftWikiStatusDirs {
		if status == candidate {
			return true
		}
	}
	return false
}

func parseDraftWikiFrontmatter(content string) map[string]string {
	return draftmeta.ParseFrontmatter(content)
}
