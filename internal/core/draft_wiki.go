package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DraftWikiDir = ProjectDocsDir + "/draft-wiki"

var draftWikiStatusDirs = []string{"draft", "approved", "rejected"}

type DraftWikiInitRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
}

type DraftWikiInitResult struct {
	OK          bool                     `json:"ok"`
	Kind        string                   `json:"kind"`
	RepoRoot    string                   `json:"repo_root"`
	DraftDir    string                   `json:"draft_dir"`
	Write       bool                     `json:"write"`
	DryRun      bool                     `json:"dry_run"`
	GeneratedAt string                   `json:"generated_at"`
	Files       []ProjectDocsPlannedFile `json:"files"`
}

type DraftWikiListRequest struct {
	RepoRoot string `json:"repo_root"`
}

type DraftWikiListResult struct {
	OK       bool             `json:"ok"`
	Kind     string           `json:"kind"`
	RepoRoot string           `json:"repo_root"`
	DraftDir string           `json:"draft_dir"`
	Drafts   []DraftWikiDraft `json:"drafts"`
}

type DraftWikiDraft struct {
	RelPath    string `json:"rel_path"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	Source     string `json:"source,omitempty"`
	TargetWiki string `json:"target_wiki,omitempty"`
	TargetType string `json:"target_type,omitempty"`
	Summary    string `json:"summary,omitempty"`
	Bytes      int64  `json:"bytes"`
}

type DraftWikiMoveRequest struct {
	RepoRoot string `json:"repo_root"`
	Path     string `json:"path"`
}

type DraftWikiMoveResult struct {
	OK       bool           `json:"ok"`
	Kind     string         `json:"kind"`
	RepoRoot string         `json:"repo_root"`
	DraftDir string         `json:"draft_dir"`
	From     DraftWikiDraft `json:"from"`
	To       DraftWikiDraft `json:"to"`
}

type DraftWikiPromoteRequest struct {
	RepoRoot          string `json:"repo_root"`
	Path              string `json:"path"`
	TargetWiki        string `json:"target_wiki"`
	TargetType        string `json:"target_type"`
	Confirm           bool   `json:"confirm"`
	LLMWikiConfigPath string `json:"-"`
}

type DraftWikiPromoteResult struct {
	OK             bool           `json:"ok"`
	Kind           string         `json:"kind"`
	RepoRoot       string         `json:"repo_root"`
	DraftDir       string         `json:"draft_dir"`
	DryRun         bool           `json:"dry_run"`
	Confirm        bool           `json:"confirm"`
	Executed       bool           `json:"executed"`
	UpstreamTool   string         `json:"upstream_tool"`
	HandoffCommand string         `json:"handoff_command"`
	HandoffArgs    []string       `json:"handoff_args"`
	LLMWikiRoot    string         `json:"llm_wiki_root,omitempty"`
	LLMWikiRawPath string         `json:"llm_wiki_raw_path,omitempty"`
	LLMWikiRawRel  string         `json:"llm_wiki_raw_rel,omitempty"`
	LLMWikiLogPath string         `json:"llm_wiki_log_path,omitempty"`
	From           DraftWikiDraft `json:"from"`
}

func InitDraftWiki(req DraftWikiInitRequest) (DraftWikiInitResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiInitResult{}, err
	}
	files := []ProjectDocsPlannedFile{}
	for rel, content := range draftWikiSeedFiles() {
		path := filepath.Join(root, filepath.FromSlash(rel))
		action := plannedFileAction(path, content)
		if req.Write && action == "create" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return DraftWikiInitResult{}, err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return DraftWikiInitResult{}, err
			}
		}
		if req.Write && action == "update" {
			action = "preserve"
		}
		files = append(files, ProjectDocsPlannedFile{
			RelPath: rel,
			Path:    path,
			Action:  action,
			Bytes:   len([]byte(content)),
			SHA256:  sha256Hex(content),
			Reason:  "repo-local draft wiki review staging",
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return DraftWikiInitResult{
		OK:          true,
		Kind:        "draft_wiki_init",
		RepoRoot:    root,
		DraftDir:    filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		Write:       req.Write,
		DryRun:      !req.Write,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Files:       files,
	}, nil
}

func ListDraftWiki(req DraftWikiListRequest) (DraftWikiListResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiListResult{}, err
	}
	drafts := []DraftWikiDraft{}
	for _, status := range draftWikiStatusDirs {
		dir := filepath.Join(root, filepath.FromSlash(DraftWikiDir), status)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return DraftWikiListResult{}, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			draft, err := readDraftWikiDraft(root, filepath.Join(dir, entry.Name()), status)
			if err != nil {
				return DraftWikiListResult{}, err
			}
			drafts = append(drafts, draft)
		}
	}
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].RelPath < drafts[j].RelPath })
	return DraftWikiListResult{
		OK:       true,
		Kind:     "draft_wiki_list",
		RepoRoot: root,
		DraftDir: filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		Drafts:   drafts,
	}, nil
}

func ApproveDraftWiki(req DraftWikiMoveRequest) (DraftWikiMoveResult, error) {
	return moveDraftWiki(req, "draft", "approved", "draft_wiki_approve")
}

func RejectDraftWiki(req DraftWikiMoveRequest) (DraftWikiMoveResult, error) {
	return moveDraftWiki(req, "", "rejected", "draft_wiki_reject")
}

func PromoteDraftWiki(req DraftWikiPromoteRequest) (DraftWikiPromoteResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiPromoteResult{}, err
	}
	from, err := resolveDraftWikiDraft(root, req.Path)
	if err != nil {
		return DraftWikiPromoteResult{}, err
	}
	if from.Status != "approved" {
		return DraftWikiPromoteResult{}, fmt.Errorf("draft %s has status %q; promote requires approved", from.RelPath, from.Status)
	}
	targetWiki := strings.TrimSpace(req.TargetWiki)
	if targetWiki == "" {
		targetWiki = from.TargetWiki
	}
	if targetWiki == "" {
		return DraftWikiPromoteResult{}, fmt.Errorf("target wiki is required via --target-wiki or draft frontmatter target_wiki")
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = from.TargetType
	}
	if targetType == "" {
		targetType = "notes"
	}
	args := []string{"@wiki", "ingest", from.RelPath, "--wiki", targetWiki, "--type", targetType}
	result := DraftWikiPromoteResult{
		OK:             true,
		Kind:           "draft_wiki_promote",
		RepoRoot:       root,
		DraftDir:       filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		DryRun:         !req.Confirm,
		Confirm:        req.Confirm,
		Executed:       false,
		UpstreamTool:   "nvk/llm-wiki",
		HandoffCommand: joinHandoffArgs(args),
		HandoffArgs:    args,
		From:           from,
	}
	if !req.Confirm {
		return result, nil
	}
	promoted, err := promoteDraftWikiToLLMWiki(promoteDraftWikiToLLMWikiRequest{
		RepoRoot:          root,
		Draft:             from,
		TargetWiki:        targetWiki,
		TargetType:        targetType,
		LLMWikiConfigPath: req.LLMWikiConfigPath,
	})
	if err != nil {
		return DraftWikiPromoteResult{}, err
	}
	result.Executed = true
	result.LLMWikiRoot = promoted.WikiRoot
	result.LLMWikiRawPath = promoted.RawPath
	result.LLMWikiRawRel = promoted.RawRel
	result.LLMWikiLogPath = promoted.LogPath
	return result, nil
}

func moveDraftWiki(req DraftWikiMoveRequest, requiredStatus, targetStatus, kind string) (DraftWikiMoveResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiMoveResult{}, err
	}
	from, err := resolveDraftWikiDraft(root, req.Path)
	if err != nil {
		return DraftWikiMoveResult{}, err
	}
	if requiredStatus != "" && from.Status != requiredStatus {
		return DraftWikiMoveResult{}, fmt.Errorf("draft %s has status %q; %s requires %s", from.RelPath, from.Status, kind, requiredStatus)
	}
	to, err := moveDraftWikiFile(root, from, targetStatus)
	if err != nil {
		return DraftWikiMoveResult{}, err
	}
	return DraftWikiMoveResult{
		OK:       true,
		Kind:     kind,
		RepoRoot: root,
		DraftDir: filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		From:     from,
		To:       to,
	}, nil
}

func moveDraftWikiFile(root string, from DraftWikiDraft, targetStatus string) (DraftWikiDraft, error) {
	if !isDraftWikiStatus(targetStatus) {
		return DraftWikiDraft{}, fmt.Errorf("unsupported draft wiki status %q", targetStatus)
	}
	targetPath := filepath.Join(root, filepath.FromSlash(DraftWikiDir), targetStatus, filepath.Base(from.Path))
	if _, err := os.Stat(targetPath); err == nil {
		return DraftWikiDraft{}, fmt.Errorf("target draft already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return DraftWikiDraft{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return DraftWikiDraft{}, err
	}
	if err := os.Rename(from.Path, targetPath); err != nil {
		return DraftWikiDraft{}, err
	}
	return readDraftWikiDraft(root, targetPath, targetStatus)
}

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
	meta := map[string]string{}
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta
	}
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "---" {
			return meta
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch key {
		case "title", "source", "target_wiki", "target_type", "summary":
			meta[key] = value
		}
	}
	return meta
}

func draftWikiSeedFiles() map[string]string {
	files := map[string]string{
		filepath.ToSlash(filepath.Join(DraftWikiDir, "README.md")): draftWikiREADME(),
	}
	for _, status := range draftWikiStatusDirs {
		files[filepath.ToSlash(filepath.Join(DraftWikiDir, status, ".gitkeep"))] = ""
	}
	return files
}

func draftWikiREADME() string {
	return `# Draft Wiki

이 디렉토리는 agent-harness가 제안한 wiki 후보를 사용자가 검토하는 repo-local staging area다.

- ` + "`draft/`" + `: claude-mem 등에서 선별된 후보. 아직 승인되지 않았다.
- ` + "`approved/`" + `: 사용자가 llm-wiki 승격을 승인한 후보.
- ` + "`rejected/`" + `: 사용자가 거절한 후보.

주의: 이 디렉토리는 진짜 llm-wiki vault가 아니다. 실제 compile/query는 upstream ` + "`nvk/llm-wiki`" + ` 워크플로우를 사용한다.
`
}

func joinHandoffArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'") {
			quoted = append(quoted, strconvQuote(arg))
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}

func strconvQuote(arg string) string {
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}
