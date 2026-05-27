package core

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DocsIndexResult struct {
	OK          bool           `json:"ok"`
	Version     string         `json:"version"`
	HarnessRoot string         `json:"harness_root"`
	Docs        []DocIndexInfo `json:"docs"`
	GeneratedAt string         `json:"generated_at"`
}

type DocIndexInfo struct {
	RelPath  string   `json:"rel_path"`
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Headings []string `json:"headings"`
	Bytes    int64    `json:"bytes"`
}

func ListDocs(root string) []string {
	var docs []string
	for _, p := range []string{"AGENTS.md", "CLAUDE.md", "GENIUS_THINK.md", "agent_docs"} {
		full := filepath.Join(root, p)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			docs = append(docs, full)
			continue
		}
		_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
				docs = append(docs, path)
			}
			return nil
		})
	}
	sort.Strings(docs)
	return docs
}

func DocsIndex(root, version string) DocsIndexResult {
	paths := ListDocs(root)
	docs := make([]DocIndexInfo, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		title, headings := readDocHeadings(path)
		docs = append(docs, DocIndexInfo{
			RelPath:  filepath.ToSlash(rel),
			Path:     path,
			Title:    title,
			Headings: headings,
			Bytes:    info.Size(),
		})
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].RelPath < docs[j].RelPath })
	return DocsIndexResult{
		OK:          true,
		Version:     version,
		HarnessRoot: root,
		Docs:        docs,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
}

func readDocHeadings(path string) (string, []string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	title := ""
	headings := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
			continue
		}
		text := strings.TrimSpace(line[level+1:])
		if text == "" {
			continue
		}
		if title == "" && level == 1 {
			title = text
		}
		headings = append(headings, text)
		if len(headings) >= 20 {
			break
		}
	}
	if title == "" && len(headings) > 0 {
		title = headings[0]
	}
	return title, headings
}
