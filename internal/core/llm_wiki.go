package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const DefaultLLMWikiRoot = "~/workspace/knowledge-base/llm-wiki"

const maxLLMWikiReadBytes = 128 * 1024
const maxLLMWikiSearchBytes = 512 * 1024

type LLMWikiInventory struct {
	OK          bool                `json:"ok"`
	Status      string              `json:"status"`
	Root        string              `json:"root"`
	Exists      bool                `json:"exists"`
	GeneratedAt string              `json:"generated_at"`
	Counts      LLMWikiCounts       `json:"counts"`
	EntryPoints []LLMWikiEntryPoint `json:"entry_points"`
	ProjectPath string              `json:"project_path,omitempty"`
	Notes       []string            `json:"notes,omitempty"`
}

type LLMWikiCounts struct {
	MarkdownFiles int `json:"markdown_files"`
	Meta          int `json:"meta"`
	Reports       int `json:"reports"`
	Sources       int `json:"sources"`
	Concepts      int `json:"concepts"`
	Entities      int `json:"entities"`
	Summaries     int `json:"summaries"`
	Sessions      int `json:"sessions"`
	Archive       int `json:"archive"`
}

type LLMWikiEntryPoint struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Bytes  int64  `json:"bytes,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type LLMWikiSessionContext struct {
	Inventory LLMWikiInventory `json:"inventory"`
	Text      string           `json:"text"`
}

type LLMWikiSearchResult struct {
	Path    string   `json:"path"`
	Title   string   `json:"title,omitempty"`
	Type    string   `json:"type,omitempty"`
	Status  string   `json:"status,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Score   int      `json:"score"`
	Snippet string   `json:"snippet,omitempty"`
}

type LLMWikiSearchResponse struct {
	OK      bool                  `json:"ok"`
	Root    string                `json:"root"`
	Query   string                `json:"query"`
	Limit   int                   `json:"limit"`
	Results []LLMWikiSearchResult `json:"results"`
}

type LLMWikiReadResult struct {
	OK        bool   `json:"ok"`
	Root      string `json:"root"`
	Path      string `json:"path"`
	Title     string `json:"title,omitempty"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"`
	Bytes     int    `json:"bytes"`
	Truncated bool   `json:"truncated"`
	Content   string `json:"content"`
}

type LLMWikiCaptureRequest struct {
	Root        string   `json:"root,omitempty"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Sources     []string `json:"sources,omitempty"`
	Related     []string `json:"related,omitempty"`
	ProjectPath string   `json:"project_path,omitempty"`
	Status      string   `json:"status,omitempty"`
	Now         time.Time
}

type LLMWikiCaptureResult struct {
	OK            bool   `json:"ok"`
	Root          string `json:"root"`
	Path          string `json:"path"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	Created       bool   `json:"created"`
	SchemaChecked bool   `json:"schema_checked"`
	LogUpdated    bool   `json:"log_updated"`
	Bytes         int    `json:"bytes"`
}

func ResolveLLMWikiRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		root = strings.TrimSpace(os.Getenv("LLM_WIKI_ROOT"))
	}
	if strings.TrimSpace(root) == "" {
		root = DefaultLLMWikiRoot
	}
	root = expandHome(root)
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func LLMWikiInventoryFor(root, projectPath string) (LLMWikiInventory, error) {
	resolved, err := ResolveLLMWikiRoot(root)
	if err != nil {
		return LLMWikiInventory{}, err
	}
	inv := LLMWikiInventory{
		Root:        resolved,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		ProjectPath: projectPath,
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			inv.Status = "missing"
			inv.Notes = []string{"llm-wiki root does not exist; set LLM_WIKI_ROOT or create ~/workspace/knowledge-base/llm-wiki"}
			inv.EntryPoints = llmWikiEntryPoints(resolved)
			return inv, nil
		}
		return inv, err
	}
	if !info.IsDir() {
		inv.Status = "not_directory"
		inv.Notes = []string{"llm-wiki root exists but is not a directory"}
		inv.EntryPoints = llmWikiEntryPoints(resolved)
		return inv, nil
	}
	inv.Exists = true
	inv.OK = true
	inv.Status = "ready"
	if err := filepath.WalkDir(resolved, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			inv.Notes = append(inv.Notes, fmt.Sprintf("walk skipped %s: %v", relOrBase(resolved, path), err))
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == resolved {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".obsidian" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
		rel := filepath.ToSlash(relOrBase(resolved, path))
		inv.Counts.MarkdownFiles++
		switch {
		case strings.HasPrefix(rel, "00-meta/reports/"):
			inv.Counts.Reports++
		case strings.HasPrefix(rel, "00-meta/"):
			inv.Counts.Meta++
		case strings.HasPrefix(rel, "10-sources/") && !strings.Contains(rel, "/_snapshots/"):
			inv.Counts.Sources++
		case strings.HasPrefix(rel, "20-wiki/concepts/"):
			inv.Counts.Concepts++
		case strings.HasPrefix(rel, "20-wiki/entities/"):
			inv.Counts.Entities++
		case strings.HasPrefix(rel, "20-wiki/summaries/"):
			inv.Counts.Summaries++
		case strings.HasPrefix(rel, "30-sessions/"):
			inv.Counts.Sessions++
		case strings.HasPrefix(rel, "_archive/"):
			inv.Counts.Archive++
		}
		return nil
	}); err != nil {
		return inv, err
	}
	inv.EntryPoints = llmWikiEntryPoints(resolved)
	if !entryPointExists(inv.EntryPoints, "00-meta/AGENTS.md") {
		inv.OK = false
		inv.Status = "schema_missing"
		inv.Notes = append(inv.Notes, "missing 00-meta/AGENTS.md schema")
	}
	if !entryPointExists(inv.EntryPoints, "00-meta/index.md") {
		inv.OK = false
		inv.Status = "index_missing"
		inv.Notes = append(inv.Notes, "missing 00-meta/index.md catalog")
	}
	return inv, nil
}

func LLMWikiSessionContextFor(root, projectPath string) (LLMWikiSessionContext, error) {
	inv, err := LLMWikiInventoryFor(root, projectPath)
	if err != nil {
		return LLMWikiSessionContext{}, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# LLM Wiki Session Context\n\n")
	fmt.Fprintf(&b, "Canonical vault root: `%s`\n", inv.Root)
	if inv.ProjectPath != "" {
		fmt.Fprintf(&b, "Current project: `%s`\n", inv.ProjectPath)
	}
	fmt.Fprintf(&b, "Status: `%s`; markdown=%d, sources=%d, concepts=%d, entities=%d, summaries=%d, sessions=%d.\n\n",
		inv.Status, inv.Counts.MarkdownFiles, inv.Counts.Sources, inv.Counts.Concepts, inv.Counts.Entities, inv.Counts.Summaries, inv.Counts.Sessions)
	fmt.Fprintf(&b, "## When to use\n\n")
	fmt.Fprintf(&b, "Use llm-wiki only when durable memory, previous decisions, repository reference cards, citation-backed local knowledge, or explicit capture/update work would materially improve the task. Do not query it reflexively for every prompt.\n\n")
	fmt.Fprintf(&b, "## How to use through agent-harness\n\n")
	fmt.Fprintf(&b, "- Search: call MCP tool `llm_wiki_search` with focused terms.\n")
	fmt.Fprintf(&b, "- Read: call `llm_wiki_read` on returned relative paths or known slugs.\n")
	fmt.Fprintf(&b, "- Orient: read resource `harness://llm-wiki/index` or call `llm_wiki_inventory`.\n")
	fmt.Fprintf(&b, "- Capture: call `llm_wiki_capture` only for reusable findings or user-requested persistence.\n\n")
	fmt.Fprintf(&b, "## Hard constraints\n\n")
	fmt.Fprintf(&b, "- Read `00-meta/AGENTS.md` before wiki writes; this tool checks that schema exists before capture.\n")
	fmt.Fprintf(&b, "- Treat `10-sources/` bodies as read-only evidence; write synthesized knowledge to `20-wiki/` and session notes to `30-sessions/`.\n")
	fmt.Fprintf(&b, "- Preserve source fidelity, cite with Obsidian wikilinks, and mark synthesis or unverified claims explicitly.\n")
	fmt.Fprintf(&b, "- Never edit `.obsidian/`; archive obsolete pages instead of deleting.\n\n")
	if inv.Exists {
		if excerpt := readRelativeExcerpt(inv.Root, "00-meta/index.md", 12*1024); excerpt != "" {
			fmt.Fprintf(&b, "## Current index excerpt\n\n```markdown\n%s\n```\n", strings.TrimSpace(excerpt))
		}
	}
	return LLMWikiSessionContext{Inventory: inv, Text: b.String()}, nil
}

func LLMWikiSearch(root, query string, limit int) (LLMWikiSearchResponse, error) {
	resolved, err := ResolveLLMWikiRoot(root)
	if err != nil {
		return LLMWikiSearchResponse{}, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return LLMWikiSearchResponse{}, fmt.Errorf("query is required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	tokens := queryTokens(query)
	if len(tokens) == 0 {
		return LLMWikiSearchResponse{}, fmt.Errorf("query has no searchable tokens")
	}
	results := []LLMWikiSearchResult{}
	if err := filepath.WalkDir(resolved, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if path == resolved {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".obsidian" || strings.HasPrefix(name, ".") || name == "_archive" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		rel := filepath.ToSlash(relOrBase(resolved, path))
		if !isSearchableLLMWikiRel(rel) {
			return nil
		}
		content, err := readFileBounded(path, maxLLMWikiSearchBytes)
		if err != nil {
			return nil
		}
		meta := parseMarkdownMeta(string(content))
		score := scoreLLMWikiMatch(rel, meta, string(content), tokens)
		if score <= 0 {
			return nil
		}
		results = append(results, LLMWikiSearchResult{
			Path:    rel,
			Title:   firstNonEmpty(meta["title"], titleFromRel(rel)),
			Type:    meta["type"],
			Status:  meta["status"],
			Tags:    splitFrontmatterList(meta["tags"]),
			Score:   score,
			Snippet: snippetForTokens(string(content), tokens, 280),
		})
		return nil
	}); err != nil {
		return LLMWikiSearchResponse{}, err
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].Path < results[j].Path
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return LLMWikiSearchResponse{OK: true, Root: resolved, Query: query, Limit: limit, Results: results}, nil
}

func LLMWikiRead(root, page string) (LLMWikiReadResult, error) {
	resolved, err := ResolveLLMWikiRoot(root)
	if err != nil {
		return LLMWikiReadResult{}, err
	}
	rel, abs, err := resolveLLMWikiPage(resolved, page)
	if err != nil {
		return LLMWikiReadResult{}, err
	}
	content, err := readFileBounded(abs, maxLLMWikiReadBytes)
	if err != nil {
		return LLMWikiReadResult{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return LLMWikiReadResult{}, err
	}
	meta := parseMarkdownMeta(string(content))
	return LLMWikiReadResult{
		OK:        true,
		Root:      resolved,
		Path:      rel,
		Title:     firstNonEmpty(meta["title"], titleFromRel(rel)),
		Type:      meta["type"],
		Status:    meta["status"],
		Bytes:     int(info.Size()),
		Truncated: info.Size() > int64(maxLLMWikiReadBytes),
		Content:   string(content),
	}, nil
}

func LLMWikiCapture(req LLMWikiCaptureRequest) (LLMWikiCaptureResult, error) {
	resolved, err := ResolveLLMWikiRoot(req.Root)
	if err != nil {
		return LLMWikiCaptureResult{}, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return LLMWikiCaptureResult{}, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return LLMWikiCaptureResult{}, fmt.Errorf("content is required")
	}
	schemaPath := filepath.Join(resolved, "00-meta", "AGENTS.md")
	if _, err := os.Stat(schemaPath); err != nil {
		return LLMWikiCaptureResult{}, fmt.Errorf("cannot capture before reading/validating schema: %w", err)
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	date := now.Format("2006-01-02")
	typ := strings.TrimSpace(req.Type)
	if typ == "" {
		typ = "session"
	}
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = "active"
	}
	slug := slugify(req.Title)
	if slug == "" {
		slug = "capture"
	}
	relDir, pageType, err := llmWikiCaptureDir(typ, now)
	if err != nil {
		return LLMWikiCaptureResult{}, err
	}
	fileName := slug + ".md"
	if pageType == "session" && !strings.HasPrefix(slug, date) {
		fileName = date + "-" + slug + ".md"
	}
	absDir := filepath.Join(resolved, filepath.FromSlash(relDir))
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return LLMWikiCaptureResult{}, err
	}
	abs := filepath.Join(absDir, fileName)
	abs = uniquePath(abs)
	rel := filepath.ToSlash(relOrBase(resolved, abs))
	body := renderLLMWikiCapture(req, pageType, status, date)
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		return LLMWikiCaptureResult{}, err
	}
	logUpdated := appendLLMWikiLog(resolved, now, fmt.Sprintf("- `%s | agent-harness | capture | [[%s]] | %s`\n", now.Format("2006-01-02 15:04"), strings.TrimSuffix(filepath.Base(rel), ".md"), strings.TrimSpace(req.Title))) == nil
	return LLMWikiCaptureResult{
		OK:            true,
		Root:          resolved,
		Path:          rel,
		Type:          pageType,
		Status:        status,
		Created:       true,
		SchemaChecked: true,
		LogUpdated:    logUpdated,
		Bytes:         len([]byte(body)),
	}, nil
}

func llmWikiEntryPoints(root string) []LLMWikiEntryPoint {
	paths := []string{"00-meta/AGENTS.md", "00-meta/index.md", "00-meta/log.md"}
	out := make([]LLMWikiEntryPoint, 0, len(paths))
	for _, rel := range paths {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		entry := LLMWikiEntryPoint{Path: rel}
		if b, err := os.ReadFile(abs); err == nil {
			entry.Exists = true
			entry.Bytes = int64(len(b))
			h := sha256.Sum256(b)
			entry.SHA256 = hex.EncodeToString(h[:])
		}
		out = append(out, entry)
	}
	return out
}

func entryPointExists(entries []LLMWikiEntryPoint, rel string) bool {
	for _, entry := range entries {
		if entry.Path == rel && entry.Exists {
			return true
		}
	}
	return false
}

func readRelativeExcerpt(root, rel string, max int) string {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	b, err := readFileBounded(abs, max)
	if err != nil {
		return ""
	}
	return string(b)
}

func readFileBounded(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(max+1)))
	if err != nil {
		return nil, err
	}
	if len(b) > max {
		b = b[:max]
	}
	return b, nil
}

func relOrBase(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Base(path)
	}
	return rel
}

func isSearchableLLMWikiRel(rel string) bool {
	return strings.HasPrefix(rel, "00-meta/") || strings.HasPrefix(rel, "10-sources/") || strings.HasPrefix(rel, "20-wiki/") || strings.HasPrefix(rel, "30-sessions/")
}

func resolveLLMWikiPage(root, page string) (string, string, error) {
	page = strings.TrimSpace(page)
	if page == "" {
		return "", "", fmt.Errorf("page is required")
	}
	page = strings.Trim(page, "[]")
	page = strings.Split(page, "#")[0]
	page = strings.TrimSpace(page)
	candidates := []string{}
	if strings.Contains(page, "/") || strings.Contains(page, "\\") || strings.HasSuffix(page, ".md") {
		candidates = append(candidates, page)
		if !strings.HasSuffix(page, ".md") {
			candidates = append(candidates, page+".md")
		}
	} else {
		slug := strings.TrimSuffix(page, ".md")
		for _, prefix := range []string{"00-meta", "20-wiki/concepts", "20-wiki/entities", "20-wiki/summaries", "10-sources", "30-sessions"} {
			candidates = append(candidates, prefix+"/"+slug+".md")
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == root {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".obsidian" || strings.HasPrefix(d.Name(), ".") || d.Name() == "_archive" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(strings.TrimSuffix(d.Name(), ".md"), slug) {
				candidates = append(candidates, filepath.ToSlash(relOrBase(root, path)))
			}
			return nil
		})
	}
	seen := map[string]bool{}
	for _, rel := range candidates {
		rel = filepath.ToSlash(filepath.Clean(rel))
		if seen[rel] || strings.HasPrefix(rel, "../") || rel == ".." || strings.HasPrefix(rel, ".obsidian/") || strings.Contains(rel, "/.obsidian/") {
			continue
		}
		seen[rel] = true
		abs := filepath.Join(root, filepath.FromSlash(rel))
		cleanAbs, err := filepath.Abs(abs)
		if err != nil {
			continue
		}
		if !isWithin(root, cleanAbs) {
			continue
		}
		info, err := os.Stat(cleanAbs)
		if err == nil && !info.IsDir() {
			return filepath.ToSlash(relOrBase(root, cleanAbs)), cleanAbs, nil
		}
	}
	return "", "", fmt.Errorf("page not found: %s", page)
}

func isWithin(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func queryTokens(query string) []string {
	fields := regexp.MustCompile(`[^\pL\pN._-]+`).Split(strings.ToLower(query), -1)
	out := []string{}
	seen := map[string]bool{}
	for _, field := range fields {
		field = strings.Trim(field, "._-")
		if len([]rune(field)) < 2 || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func scoreLLMWikiMatch(rel string, meta map[string]string, content string, tokens []string) int {
	lowerPath := strings.ToLower(rel)
	lowerTitle := strings.ToLower(meta["title"])
	lowerContent := strings.ToLower(content)
	score := 0
	for _, token := range tokens {
		if strings.Contains(lowerTitle, token) {
			score += 20
		}
		if strings.Contains(lowerPath, token) {
			score += 12
		}
		count := strings.Count(lowerContent, token)
		if count > 10 {
			count = 10
		}
		score += count
	}
	return score
}

func snippetForTokens(content string, tokens []string, max int) string {
	lower := strings.ToLower(content)
	idx := -1
	for _, token := range tokens {
		if i := strings.Index(lower, token); i >= 0 && (idx == -1 || i < idx) {
			idx = i
		}
	}
	if idx < 0 {
		return trimWhitespace(content, max)
	}
	start := idx - max/3
	if start < 0 {
		start = 0
	}
	end := start + max
	if end > len(content) {
		end = len(content)
	}
	return trimWhitespace(content[start:end], max)
}

func trimWhitespace(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func parseMarkdownMeta(content string) map[string]string {
	meta := map[string]string{}
	if !strings.HasPrefix(content, "---\n") {
		return meta
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return meta
	}
	front := content[4 : 4+end]
	lines := strings.Split(front, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" {
			meta[key] = value
		}
	}
	return meta
}

func splitFrontmatterList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	parts := strings.Split(value, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func titleFromRel(rel string) string {
	base := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	base = strings.ReplaceAll(base, "-", " ")
	return base
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func llmWikiCaptureDir(typ string, now time.Time) (string, string, error) {
	switch typ {
	case "session", "capture":
		return "30-sessions/" + now.Format("2006"), "session", nil
	case "concept":
		return "20-wiki/concepts", "concept", nil
	case "entity":
		return "20-wiki/entities", "entity", nil
	case "summary":
		return "20-wiki/summaries/" + now.Format("2006"), "summary", nil
	default:
		return "", "", fmt.Errorf("unsupported capture type %q; use session, concept, entity, or summary", typ)
	}
}

func renderLLMWikiCapture(req LLMWikiCaptureRequest, typ, status, date string) string {
	title := strings.TrimSpace(req.Title)
	tags := req.Tags
	if len(tags) == 0 {
		tags = []string{"agent-harness", "capture"}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "title: %s\n", yamlString(title))
	fmt.Fprintf(&b, "type: %s\n", typ)
	fmt.Fprintf(&b, "status: %s\n", status)
	fmt.Fprintf(&b, "created: %s\n", date)
	fmt.Fprintf(&b, "updated: %s\n", date)
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(cleanInlineList(tags), ", "))
	writeYAMLList(&b, "sources", req.Sources)
	writeYAMLList(&b, "related", req.Related)
	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", title)
	if strings.TrimSpace(req.ProjectPath) != "" {
		fmt.Fprintf(&b, "> Project: `%s`\n\n", strings.TrimSpace(req.ProjectPath))
	}
	fmt.Fprintf(&b, "## Notes\n\n%s\n\n", strings.TrimSpace(req.Content))
	fmt.Fprintf(&b, "## Changelog\n\n- %s: Created by agent-harness capture.\n", date)
	return b.String()
}

func writeYAMLList(b *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", key)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			fmt.Fprintf(b, "  - %s\n", yamlString(value))
		}
	}
}

func cleanInlineList(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func yamlString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func appendLLMWikiLog(root string, now time.Time, line string) error {
	path := filepath.Join(root, "00-meta", "log.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte("# Wiki Log\n\n"), 0o644); err != nil {
			return err
		}
	}
	_ = now
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^\pL\pN]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if len(value) > 80 {
		value = strings.Trim(value[:80], "-")
	}
	return value
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
