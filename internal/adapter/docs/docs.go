package docs

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const draftWikiDir = ".agent-harness/draft-wiki"
const evidenceDir = ".agent-harness/evidence"

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
	var candidates []string
	for _, p := range []string{"AGENTS.md", "CLAUDE.md", "GENIUS_THINK.md", ".agent-harness", "skills/self-verify", "skills/self-augment"} {
		full := filepath.Join(root, p)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			candidates = append(candidates, full)
			continue
		}
		_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") && !isExcludedDoc(root, path) {
				candidates = append(candidates, path)
			}
			return nil
		})
	}
	docs := hermeticTrackedDocs(root, candidates)
	sort.Strings(docs)
	return docs
}

// hermeticTrackedDocs keeps only git-TRACKED candidates so the docs index — and
// the response-contract golden that snapshots it — is hermetic: untracked files
// (e.g. research artifacts written into .agent-harness/research during a
// session) must not drift the index. It compares ROOT-RELATIVE paths and never
// reconstructs absolute paths, so a symlinked root (macOS /var -> /private/var,
// where git resolves the symlink but filepath.WalkDir does not) cannot cause a
// mismatch. When git is unavailable (e.g. a non-repo temp dir) it falls back to
// ALL candidates; and if the tracked set matched NO candidate at all — a sign the
// matching is broken, not that every doc is untracked — it also falls back rather
// than silently emptying the index.
func hermeticTrackedDocs(root string, candidates []string) []string {
	tracked, ok := gitTrackedRelPaths(root)
	if !ok {
		return candidates
	}
	matched := make([]string, 0, len(candidates))
	for _, path := range candidates {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if tracked[filepath.ToSlash(rel)] {
			matched = append(matched, path)
		}
	}
	if len(matched) == 0 && len(candidates) > 0 {
		return candidates
	}
	return matched
}

// gitTrackedRelPaths returns the repo's tracked paths relative to root
// (slash-separated), ok=true when git resolved a non-empty set. It uses -z
// (NUL-delimited, never C-quoted) and core.quotepath=false so non-ASCII (e.g.
// Korean) filenames match WalkDir's UTF-8 paths byte-for-byte, and never builds
// absolute paths (which would diverge from WalkDir under a symlinked root).
func gitTrackedRelPaths(root string) (map[string]bool, bool) {
	out, err := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "ls-files", "-z").Output()
	if err != nil {
		return nil, false
	}
	set := make(map[string]bool)
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			set[p] = true
		}
	}
	if len(set) == 0 {
		return nil, false
	}
	return set, true
}

// isExcludedDoc reports whether path is under a .agent-harness subtree that must not
// appear in the docs index: draft-wiki (in-progress drafts) or evidence (gitignored,
// working-tree-dependent runtime artifacts). Including evidence would make the docs
// index — and the response-contract golden that snapshots it — non-hermetic.
func isExcludedDoc(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, dir := range []string{draftWikiDir, evidenceDir} {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
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
		title, headings := ReadHeadings(path)
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

func ReadHeadings(path string) (string, []string) {
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
