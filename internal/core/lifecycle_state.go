package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ProjectLifecycleSchemaVersion = 1
const projectLifecycleProfileFile = "project.json"
const docUpkeepQueueFile = "doc-upkeep-queue.jsonl"
const compactCapsuleFile = "compact-capsule.json"

type ProjectFingerprint struct {
	RepoRoot      string `json:"repo_root"`
	GitDir        string `json:"git_dir,omitempty"`
	GitOriginHash string `json:"git_origin_hash,omitempty"`
}

type ProjectLifecycleProfile struct {
	SchemaVersion int                `json:"schema_version"`
	RepoID        string             `json:"repo_id"`
	Fingerprint   ProjectFingerprint `json:"fingerprint"`
	Metadata      *ProjectProfile    `json:"metadata,omitempty"`
	CreatedAt     string             `json:"created_at"`
	UpdatedAt     string             `json:"updated_at"`
}

type ProjectLifecycleStatePlan struct {
	OK              bool                     `json:"ok"`
	SchemaVersion   int                      `json:"schema_version"`
	RepoRoot        string                   `json:"repo_root"`
	RepoID          string                   `json:"repo_id"`
	StateRoot       string                   `json:"state_root"`
	ProjectStateDir string                   `json:"project_state_dir"`
	ProjectJSONPath string                   `json:"project_json_path"`
	QueuePath       string                   `json:"queue_path"`
	CompactPath     string                   `json:"compact_path"`
	Fingerprint     ProjectFingerprint       `json:"fingerprint"`
	Exists          bool                     `json:"exists"`
	NamespaceValid  bool                     `json:"namespace_valid"`
	Profile         *ProjectLifecycleProfile `json:"profile,omitempty"`
	Warnings        []string                 `json:"warnings,omitempty"`
}

type DocUpkeepEvent struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	TargetDocs []string `json:"target_docs"`
	Summary    string   `json:"summary"`
	Evidence   []string `json:"evidence,omitempty"`
	Source     string   `json:"source,omitempty"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
}

type DocUpkeepAppendResult struct {
	OK              bool           `json:"ok"`
	RepoRoot        string         `json:"repo_root"`
	RepoID          string         `json:"repo_id"`
	ProjectStateDir string         `json:"project_state_dir"`
	Path            string         `json:"path"`
	Event           DocUpkeepEvent `json:"event"`
}

func ResolveProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	root, err := normalizeRepoRoot(repoRoot)
	if err != nil {
		return ProjectLifecycleStatePlan{OK: false, StateRoot: StateDir(), SchemaVersion: ProjectLifecycleSchemaVersion}, err
	}
	fingerprint := projectFingerprint(root)
	repoID := projectRepoID(fingerprint)
	stateRoot := StateDir()
	projectDir := filepath.Join(stateRoot, "projects", repoID)
	plan := ProjectLifecycleStatePlan{
		OK:              true,
		SchemaVersion:   ProjectLifecycleSchemaVersion,
		RepoRoot:        root,
		RepoID:          repoID,
		StateRoot:       stateRoot,
		ProjectStateDir: projectDir,
		ProjectJSONPath: filepath.Join(projectDir, projectLifecycleProfileFile),
		QueuePath:       filepath.Join(projectDir, docUpkeepQueueFile),
		CompactPath:     filepath.Join(projectDir, compactCapsuleFile),
		Fingerprint:     fingerprint,
		Warnings:        []string{},
	}
	profile, err := readProjectLifecycleProfile(plan.ProjectJSONPath)
	if os.IsNotExist(err) {
		return plan, nil
	}
	if err != nil {
		plan.Warnings = append(plan.Warnings, "project_json_read_error")
		return plan, nil
	}
	plan.Exists = true
	plan.Profile = &profile
	plan.NamespaceValid = projectFingerprintEqual(profile.Fingerprint, fingerprint) && profile.RepoID == repoID && profile.SchemaVersion == ProjectLifecycleSchemaVersion
	if !plan.NamespaceValid {
		plan.Warnings = append(plan.Warnings, "namespace_mismatch")
	}
	return plan, nil
}

func InitProjectLifecycleState(repoRoot string, confirm bool, metadata ...ProjectProfile) (ProjectLifecycleStatePlan, error) {
	plan, err := ResolveProjectLifecycleState(repoRoot)
	if err != nil || !confirm {
		return plan, err
	}
	if plan.Exists && !plan.NamespaceValid {
		return plan, nil
	}
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		plan.OK = false
		return plan, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := now
	if plan.Profile != nil && plan.Profile.CreatedAt != "" {
		createdAt = plan.Profile.CreatedAt
	}
	var meta *ProjectProfile
	if len(metadata) > 0 {
		m := metadata[0]
		meta = &m
	} else if plan.Profile != nil {
		meta = plan.Profile.Metadata
	}
	profile := ProjectLifecycleProfile{
		SchemaVersion: ProjectLifecycleSchemaVersion,
		RepoID:        plan.RepoID,
		Fingerprint:   plan.Fingerprint,
		Metadata:      meta,
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}
	if err := writeJSONAtomic(plan.ProjectJSONPath, profile, 0o600); err != nil {
		plan.OK = false
		return plan, err
	}
	plan.Exists = true
	plan.NamespaceValid = true
	plan.Profile = &profile
	return plan, nil
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return ResolveProjectLifecycleState(repoRoot)
}

func AppendDocUpkeepEvent(repoRoot string, event DocUpkeepEvent) (DocUpkeepAppendResult, error) {
	plan, err := ValidateProjectLifecycleState(repoRoot)
	if err != nil {
		return DocUpkeepAppendResult{OK: false}, err
	}
	if !plan.Exists {
		plan, err = InitProjectLifecycleState(repoRoot, true)
		if err != nil {
			return DocUpkeepAppendResult{OK: false}, err
		}
	}
	if !plan.NamespaceValid {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, fmt.Errorf("project lifecycle namespace mismatch for %s", plan.RepoRoot)
	}
	if strings.TrimSpace(event.Kind) == "" {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, fmt.Errorf("doc upkeep event kind is required")
	}
	if strings.TrimSpace(event.Summary) == "" {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, fmt.Errorf("doc upkeep event summary is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if event.ID == "" {
		event.ID = docUpkeepEventID(plan.RepoID, event, now)
	}
	if event.Status == "" {
		event.Status = "pending"
	}
	if event.CreatedAt == "" {
		event.CreatedAt = now
	}
	event.TargetDocs = normalizeTargetDocs(event.TargetDocs)
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	f, err := os.OpenFile(plan.QueuePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	defer f.Close()
	b, err := json.Marshal(event)
	if err != nil {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	return DocUpkeepAppendResult{OK: true, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath, Event: event}, nil
}

func ReadPendingDocUpkeepEvents(repoRoot string, limit int) ([]DocUpkeepEvent, ProjectLifecycleStatePlan, error) {
	plan, err := ValidateProjectLifecycleState(repoRoot)
	if err != nil || !plan.Exists || !plan.NamespaceValid {
		return []DocUpkeepEvent{}, plan, err
	}
	f, err := os.Open(plan.QueuePath)
	if os.IsNotExist(err) {
		return []DocUpkeepEvent{}, plan, nil
	}
	if err != nil {
		return []DocUpkeepEvent{}, plan, err
	}
	defer f.Close()
	events := []DocUpkeepEvent{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event DocUpkeepEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.Status == "" || event.Status == "pending" {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return events, plan, err
	}
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, plan, nil
}

func projectFingerprint(root string) ProjectFingerprint {
	gitDir := ""
	if info, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if info.IsDir() {
			gitDir = filepath.Join(root, ".git")
		} else if b, err := os.ReadFile(filepath.Join(root, ".git")); err == nil {
			gitDir = strings.TrimSpace(string(b))
		}
	}
	originHash := ""
	if origin := readGitOriginURL(root); origin != "" {
		sum := sha256.Sum256([]byte(origin))
		originHash = hex.EncodeToString(sum[:])
	}
	return ProjectFingerprint{RepoRoot: root, GitDir: gitDir, GitOriginHash: originHash}
}

func projectRepoID(fp ProjectFingerprint) string {
	parts := []string{fp.RepoRoot, fp.GitDir, fp.GitOriginHash}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:24]
}

func readGitOriginURL(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	inOrigin := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = trimmed == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(trimmed, "url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func readProjectLifecycleProfile(path string) (ProjectLifecycleProfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ProjectLifecycleProfile{}, err
	}
	var profile ProjectLifecycleProfile
	if err := json.Unmarshal(b, &profile); err != nil {
		return ProjectLifecycleProfile{}, err
	}
	return profile, nil
}

func writeJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(append(b, '\n')); err != nil {
			return err
		}
		if err := tmp.Chmod(perm); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func projectFingerprintEqual(a, b ProjectFingerprint) bool {
	return a.RepoRoot == b.RepoRoot && a.GitDir == b.GitDir && a.GitOriginHash == b.GitOriginHash
}

func normalizeTargetDocs(docs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		doc = strings.TrimPrefix(filepath.ToSlash(doc), ProjectDocsDir+"/")
		if !strings.HasSuffix(doc, ".md") {
			continue
		}
		if !seen[doc] {
			seen[doc] = true
			out = append(out, doc)
		}
	}
	sort.Strings(out)
	return out
}

func docUpkeepEventID(repoID string, event DocUpkeepEvent, at string) string {
	sum := sha256.Sum256([]byte(repoID + "\x00" + event.Kind + "\x00" + event.Summary + "\x00" + strings.Join(event.TargetDocs, ",") + "\x00" + at))
	return hex.EncodeToString(sum[:])[:24]
}

type HookToolUseLifecycleRequest struct {
	Repo                 string   `json:"repo,omitempty"`
	Tool                 string   `json:"tool,omitempty"`
	Paths                []string `json:"paths,omitempty"`
	Command              string   `json:"command,omitempty"`
	Source               string   `json:"source,omitempty"`
	EnforceSearchRouting bool     `json:"enforce_search_routing,omitempty"`
	EnforceWorktree      bool     `json:"enforce_worktree,omitempty"`
	EnforceKoreanRemote  bool     `json:"enforce_korean_remote,omitempty"`
	ExpectedWorktree     string   `json:"expected_worktree,omitempty"`
	SourceCheckout       string   `json:"source_checkout,omitempty"`
}

type HookToolUseLifecycleResult struct {
	OK       bool           `json:"ok"`
	Recorded bool           `json:"recorded"`
	Event    DocUpkeepEvent `json:"event,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
}

type HookPreToolUseDecisionResult struct {
	OK       bool     `json:"ok"`
	Decision string   `json:"decision"`
	Reason   string   `json:"reason,omitempty"`
	Tool     string   `json:"tool,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Command  string   `json:"command,omitempty"`
	Source   string   `json:"source,omitempty"`
}

type LifecycleStopReminderResult struct {
	OK                bool   `json:"ok"`
	ShouldInject      bool   `json:"should_inject"`
	AdditionalContext string `json:"additional_context,omitempty"`
	PendingCount      int    `json:"pending_count"`
}

type NumberedNextActionsDecisionResult struct {
	OK       bool   `json:"ok"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	Source   string `json:"source"`
}

type LifecycleCompactCapsule struct {
	SchemaVersion     int              `json:"schema_version"`
	RepoRoot          string           `json:"repo_root"`
	RepoID            string           `json:"repo_id"`
	CreatedAt         string           `json:"created_at"`
	RequiredDocs      []string         `json:"required_docs,omitempty"`
	PendingDocUpkeep  []DocUpkeepEvent `json:"pending_doc_upkeep,omitempty"`
	AdditionalSummary string           `json:"additional_summary,omitempty"`
}

type LifecycleCompactResult struct {
	OK                bool     `json:"ok"`
	Recorded          bool     `json:"recorded,omitempty"`
	ShouldInject      bool     `json:"should_inject,omitempty"`
	AdditionalContext string   `json:"additional_context,omitempty"`
	PendingCount      int      `json:"pending_count,omitempty"`
	CompactPath       string   `json:"compact_path,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
}

func BuildLifecyclePreToolUseDecision(req HookToolUseLifecycleRequest) HookPreToolUseDecisionResult {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "pre-tool-use"
	}
	result := HookPreToolUseDecisionResult{
		OK:       true,
		Decision: "allow",
		Tool:     strings.TrimSpace(req.Tool),
		Paths:    append([]string{}, req.Paths...),
		Command:  strings.TrimSpace(req.Command),
		Source:   source,
	}
	if req.EnforceSearchRouting {
		if reason := searchRoutingBlockReason(result.Tool, result.Command, req.Repo); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceWorktree {
		if reason := worktreeGuardBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceKoreanRemote {
		if reason := koreanRemoteArtifactBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	return result
}

var (
	hangulRe       = regexp.MustCompile(`[가-힣]`)
	asciiWordRe    = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_+-]*\b`)
	codeFenceRe    = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe   = regexp.MustCompile("`[^`]*`")
	urlRe          = regexp.MustCompile(`https?://\S+`)
	pathLikeTextRe = regexp.MustCompile(`(?:^|\s)[./~]?[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+`)
)

func koreanRemoteArtifactBlockReason(req HookToolUseLifecycleRequest) string {
	artifact, ok := parseGHRemoteArtifactCommand(req.Command, req.Repo)
	if !ok {
		return ""
	}
	if strings.TrimSpace(artifact.title) == "" || strings.TrimSpace(artifact.body) == "" {
		return "IssueOps remote artifact gate requires inspectable Korean title and body before gh issue/pr create/edit; provide --title and --body-file/--body after running the Korean gate"
	}
	hangul, englishWords := scoreKoreanRemoteArtifactLanguage(artifact.title + "\n" + artifact.body)
	if hangul < 20 {
		return fmt.Sprintf("IssueOps remote artifact gate failed: expected at least 20 Hangul chars before gh %s %s, got %d", artifact.kind, artifact.action, hangul)
	}
	if hangul > 0 && float64(englishWords)/float64(hangul) > 1.2 {
		return fmt.Sprintf("IssueOps remote artifact gate failed: English prose ratio too high before gh %s %s (english_words=%d, hangul_chars=%d)", artifact.kind, artifact.action, englishWords, hangul)
	}
	return ""
}

type remoteArtifactCommand struct {
	kind   string
	action string
	title  string
	body   string
}

func parseGHRemoteArtifactCommand(command string, repo string) (remoteArtifactCommand, bool) {
	tokens := splitCommandTokens(command)
	for i := 0; i+2 < len(tokens); i++ {
		if searchTokenName(tokens[i]) != "gh" {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(tokens[i+1]))
		action := strings.ToLower(strings.TrimSpace(tokens[i+2]))
		if (kind != "issue" && kind != "pr") || (action != "create" && action != "edit") {
			continue
		}
		artifact := remoteArtifactCommand{kind: kind, action: action}
		args := tokens[i+3:]
		for j := 0; j < len(args); j++ {
			arg := args[j]
			switch {
			case arg == "--title" || arg == "-t":
				if j+1 < len(args) {
					artifact.title = args[j+1]
					j++
				}
			case strings.HasPrefix(arg, "--title="):
				artifact.title = strings.TrimPrefix(arg, "--title=")
			case arg == "--body" || arg == "-b":
				if j+1 < len(args) {
					artifact.body = args[j+1]
					j++
				}
			case strings.HasPrefix(arg, "--body="):
				artifact.body = strings.TrimPrefix(arg, "--body=")
			case arg == "--body-file" || arg == "-F":
				if j+1 < len(args) {
					artifact.body = readRemoteArtifactBodyFile(repo, args[j+1])
					j++
				}
			case strings.HasPrefix(arg, "--body-file="):
				artifact.body = readRemoteArtifactBodyFile(repo, strings.TrimPrefix(arg, "--body-file="))
			}
		}
		return artifact, true
	}
	return remoteArtifactCommand{}, false
}

func readRemoteArtifactBodyFile(repo string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" || p == "-" {
		return ""
	}
	if !filepath.IsAbs(p) {
		base := cleanAbsPath(repo)
		if base != "" {
			p = filepath.Join(base, p)
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func scoreKoreanRemoteArtifactLanguage(text string) (int, int) {
	prose := codeFenceRe.ReplaceAllString(text, " ")
	prose = inlineCodeRe.ReplaceAllString(prose, " ")
	prose = urlRe.ReplaceAllString(prose, " ")
	prose = pathLikeTextRe.ReplaceAllString(prose, " ")
	return len(hangulRe.FindAllString(prose, -1)), len(asciiWordRe.FindAllString(prose, -1))
}

func worktreeGuardBlockReason(req HookToolUseLifecycleRequest) string {
	expected := cleanAbsPath(req.ExpectedWorktree)
	if expected == "" {
		return ""
	}
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	targets := []string{}
	if repo := cleanAbsPath(req.Repo); repo != "" {
		targets = append(targets, repo)
	}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return ""
	}
	for _, target := range targets {
		if !pathWithin(target, expected) {
			return "mutating tool target is outside expected IssueOps worktree; set cwd/target path to the isolated worktree before editing"
		}
	}
	return ""
}

func resolveHookTargetPath(repo, path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return cleanAbsPath(p)
	}
	base := cleanAbsPath(repo)
	if base == "" {
		return ""
	}
	return cleanAbsPath(filepath.Join(base, p))
}

func cleanAbsPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return filepath.Clean(p)
		}
		p = abs
	}
	return filepath.Clean(p)
}

func pathWithin(path, root string) bool {
	p := cleanAbsPath(path)
	r := cleanAbsPath(root)
	if p == "" || r == "" {
		return false
	}
	if p == r {
		return true
	}
	rel, err := filepath.Rel(r, p)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func searchRoutingBlockReason(tool string, command string, repo string) string {
	switch {
	case isShellTool(tool):
		if shouldBlockRawStructuralSourceSearch(command, repo) {
			return "Use CodeGraph first for structural repo-local source search: call codegraph_context for broad areas, codegraph_search for symbols, or codegraph_trace for call paths. Keep raw grep/rg for exact strings, env keys, filenames, errors, docs, golden fixtures, or literal evidence."
		}
	case isCodeGraphTool(tool):
		if looksLikeExactSearchQuery(command) {
			return "Use rg first for exact text search such as env keys, filenames, error messages, TODO/comment/log text, or literal strings. Keep CodeGraph for symbols, call paths, module dependencies, and impact analysis."
		}
	}
	return ""
}

func shouldBlockRawStructuralSourceSearch(command string, repo string) bool {
	normalizedCommand := strings.ToLower(strings.TrimSpace(command))
	if normalizedCommand == "" {
		return false
	}
	searchArgs, ok := rawTextSearchArgs(normalizedCommand)
	if !ok {
		return false
	}
	return sourceSearchNeedsCodeGraph(searchArgs, repo)
}

func isCodeGraphTool(tool string) bool {
	normalized := strings.ToLower(strings.TrimSpace(tool))
	return normalized == "codegraph" || strings.HasPrefix(normalized, "codegraph_") || strings.Contains(normalized, "__codegraph__")
}

func isShellTool(tool string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "bash", "sh", "zsh", "shell", "exec", "run_command", "shell_command", "unified_exec", "exec_command":
		return true
	default:
		return false
	}
}

func rawTextSearchArgs(command string) ([]string, bool) {
	tokens := splitCommandTokens(command)
	for i, token := range tokens {
		name := searchTokenName(token)
		if name == "git" && i+1 < len(tokens) && searchTokenName(tokens[i+1]) == "grep" {
			return tokens[i+2:], true
		}
		switch name {
		case "rg", "grep", "ag", "ack":
			return tokens[i+1:], true
		}
	}
	return nil, false
}

func splitCommandTokens(command string) []string {
	tokens := []string{}
	var current strings.Builder
	var quote rune
	for _, r := range command {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func searchTokenName(token string) string {
	cleaned := strings.Trim(token, `"'`)
	return filepath.Base(cleaned)
}

func sourceSearchNeedsCodeGraph(args []string, repo string) bool {
	if !hasStructuralSourceSearchPattern(args) {
		return false
	}
	targets := []string{}
	for _, arg := range args {
		target := searchTargetToken(arg)
		if target == "" || strings.HasPrefix(target, "-") {
			continue
		}
		if looksLikeSearchTarget(target) {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return true
	}
	for _, target := range targets {
		if isDocsOrFixtureTarget(target) {
			continue
		}
		if !isRepoLocalSearchTarget(target, repo) {
			continue
		}
		return true
	}
	return false
}

func hasStructuralSourceSearchPattern(args []string) bool {
	for _, arg := range args {
		pattern := searchPatternToken(arg)
		if pattern == "" {
			continue
		}
		if looksLikeStructuralSourcePattern(pattern) {
			return true
		}
		if !strings.HasPrefix(pattern, "-") {
			return false
		}
	}
	return false
}

func searchPatternToken(token string) string {
	cleaned := strings.Trim(strings.TrimSpace(token), `"',;`)
	if cleaned == "" || strings.HasPrefix(cleaned, "-") || looksLikeSearchTarget(cleaned) {
		return ""
	}
	return cleaned
}

func looksLikeStructuralSourcePattern(pattern string) bool {
	lower := strings.ToLower(strings.TrimSpace(pattern))
	if lower == "" {
		return false
	}
	structuralNeedles := []string{
		"func ",
		"function ",
		"type ",
		"class ",
		"interface ",
		"struct ",
		"enum ",
		"def ",
		"impl ",
		"trait ",
		"extends ",
		"implements ",
		"@controller",
		"@injectable",
	}
	for _, needle := range structuralNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func looksLikeExactSearchQuery(query string) bool {
	cleaned := strings.Trim(strings.TrimSpace(query), `"',;`)
	if cleaned == "" {
		return false
	}
	lower := strings.ToLower(cleaned)
	if strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") || strings.Contains(lower, "log.") || strings.Contains(lower, "console.") || strings.Contains(lower, "comment") {
		return true
	}
	if strings.Contains(cleaned, ".") && !strings.Contains(cleaned, " ") {
		ext := strings.ToLower(filepath.Ext(cleaned))
		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".kt", ".md", ".json", ".yaml", ".yml", ".toml", ".env":
			return true
		}
	}
	if strings.Contains(cleaned, " ") && looksLikeErrorPhrase(lower) {
		return true
	}
	hasUnderscore := strings.Contains(cleaned, "_")
	hasUpper := false
	hasLower := false
	for _, r := range cleaned {
		if r >= 'A' && r <= 'Z' {
			hasUpper = true
		}
		if r >= 'a' && r <= 'z' {
			hasLower = true
		}
	}
	if hasUnderscore && hasUpper && !hasLower {
		return true
	}
	return strings.HasSuffix(cleaned, "Error") || strings.HasSuffix(cleaned, "Exception") || strings.HasSuffix(cleaned, "Failure")
}

func looksLikeErrorPhrase(lower string) bool {
	errorNeedles := []string{"cannot ", "can't ", "failed", "failure", "error", "undefined", "not found", "read property", "exception"}
	for _, needle := range errorNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func isRepoLocalSearchTarget(target string, repo string) bool {
	cleaned := strings.TrimSpace(target)
	if cleaned == "" {
		return true
	}
	if !filepath.IsAbs(cleaned) {
		return true
	}
	root := strings.TrimSpace(repo)
	if root == "" {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(cleaned)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

func searchTargetToken(token string) string {
	return strings.Trim(strings.TrimSpace(token), `"',;`)
}

func looksLikeSearchTarget(token string) bool {
	if token == "." || strings.HasPrefix(token, "./") || strings.HasPrefix(token, "/") {
		return true
	}
	if strings.Contains(token, "/") || strings.Contains(token, "*") {
		return true
	}
	switch token {
	case "cmd", "internal", "pkg", "src", "app", "lib", "docs", "testdata":
		return true
	default:
		return strings.HasSuffix(token, ".md")
	}
}

func isDocsOrFixtureTarget(token string) bool {
	cleaned := strings.TrimPrefix(token, "./")
	if strings.HasSuffix(cleaned, ".md") {
		return true
	}
	base := filepath.Base(cleaned)
	if base == "readme" || strings.HasPrefix(base, "readme.") {
		return true
	}
	for _, segment := range strings.Split(cleaned, "/") {
		switch segment {
		case "docs", ".agent-harness", "testdata", "golden", "goldens", "fixture", "fixtures", "snapshot", "snapshots", "__snapshots__":
			return true
		}
	}
	return false
}

func RecordLifecycleToolUse(req HookToolUseLifecycleRequest) (HookToolUseLifecycleResult, error) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return HookToolUseLifecycleResult{OK: true, Warnings: []string{"repo_missing"}}, nil
	}
	targets := lifecycleDocTargetsForToolUse(req)
	if len(targets) == 0 {
		return HookToolUseLifecycleResult{OK: true}, nil
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "post-tool-use"
	}
	summary := "Relevant harness files changed; shared project docs may need review."
	if req.Tool != "" {
		summary = fmt.Sprintf("%s touched harness lifecycle-relevant files; shared project docs may need review.", req.Tool)
	}
	appendResult, err := AppendDocUpkeepEvent(repo, DocUpkeepEvent{
		Kind:       "code_change",
		TargetDocs: targets,
		Summary:    summary,
		Evidence:   append([]string{}, req.Paths...),
		Source:     source,
	})
	if err != nil {
		return HookToolUseLifecycleResult{OK: false}, err
	}
	return HookToolUseLifecycleResult{OK: true, Recorded: true, Event: appendResult.Event}, nil
}

func BuildLifecycleStopReminder(repo string) LifecycleStopReminderResult {
	events, _, err := ReadPendingDocUpkeepEvents(repo, 5)
	if err != nil || len(events) == 0 {
		return LifecycleStopReminderResult{OK: true}
	}
	var b strings.Builder
	b.WriteString("Pending .agent-harness doc upkeep:\n")
	for _, event := range events {
		b.WriteString("- ")
		if len(event.TargetDocs) > 0 {
			b.WriteString(strings.Join(event.TargetDocs, ", "))
			b.WriteString(": ")
		}
		b.WriteString(event.Summary)
		b.WriteString("\n")
	}
	b.WriteString("Use project_docs_record for ADR/caution entries or project_docs_read/project_docs_update for evidence-preserving doc refreshes.")
	return LifecycleStopReminderResult{OK: true, ShouldInject: true, AdditionalContext: strings.TrimSpace(b.String()), PendingCount: len(events)}
}

func BuildNumberedNextActionsDecision(message string, enforce bool, source string) NumberedNextActionsDecisionResult {
	result := NumberedNextActionsDecisionResult{
		OK:       true,
		Decision: "allow",
		Source:   strings.TrimSpace(source),
	}
	if !enforce {
		return result
	}
	message = strings.TrimSpace(message)
	if message == "" {
		result.Decision = "allow"
		result.Reason = "no assistant message available to inspect"
		return result
	}
	if hasNumberedNextActions(message) {
		return result
	}
	result.Decision = "block"
	result.Reason = "IssueOps response must end with numbered next actions: 1. proceed/recommended, 2. narrower alternative, 3. pause/defer"
	return result
}

const defaultNextActionAutoProceedThreshold = 0.80

// NextActionCandidate is a parsed numbered next-action option with auto-proceed signals.
type NextActionCandidate struct {
	Index       int     `json:"index"`
	Text        string  `json:"text"`
	Recommended bool    `json:"recommended"`
	Destructive bool    `json:"destructive"`
	Score       float64 `json:"score"`
}

// NextActionAutoProceedResult reports whether the recommended next action is
// confident and safe enough to advance without stopping for user selection.
type NextActionAutoProceedResult struct {
	OK             bool                  `json:"ok"`
	AutoProceed    bool                  `json:"auto_proceed"`
	Threshold      float64               `json:"threshold"`
	TopScore       float64               `json:"top_score"`
	SelectedIndex  int                   `json:"selected_index,omitempty"`
	SelectedText   string                `json:"selected_text,omitempty"`
	Reason         string                `json:"reason"`
	BlockedByGuard string                `json:"blocked_by_guard,omitempty"`
	Candidates     []NextActionCandidate `json:"candidates"`
}

// EvaluateNextActionAutoProceed scores parsed next-action choices and decides
// whether the recommended option can be executed automatically. Auto-proceed
// requires an explicit recommendation marker, a forward/constructive verb, and
// a reversible (non-destructive) action. Destructive or ambiguous choices never
// auto-proceed, preserving user-decision safety at cleanup/interpretation gates.
func EvaluateNextActionAutoProceed(message string, threshold float64) NextActionAutoProceedResult {
	if threshold <= 0 {
		threshold = defaultNextActionAutoProceedThreshold
	}
	result := NextActionAutoProceedResult{OK: true, Threshold: threshold, Candidates: []NextActionCandidate{}}
	candidates := parseNextActionCandidates(message)
	if len(candidates) < 2 {
		result.Reason = "no numbered next-action choices to evaluate"
		return result
	}
	result.Candidates = candidates

	recommended := selectRecommendedNextAction(candidates)
	if recommended == nil {
		result.Reason = "no explicitly recommended next action; user decision required"
		return result
	}
	result.SelectedIndex = recommended.Index
	result.SelectedText = recommended.Text
	result.TopScore = recommended.Score
	if recommended.Destructive {
		result.BlockedByGuard = "destructive_action"
		result.Reason = "recommended action is destructive or irreversible; user decision required"
		return result
	}
	if recommended.Score >= threshold {
		result.AutoProceed = true
		result.Reason = fmt.Sprintf("recommended action scored %.2f >= threshold %.2f and is reversible", recommended.Score, threshold)
		return result
	}
	result.Reason = fmt.Sprintf("recommended action scored %.2f below threshold %.2f; user decision required", recommended.Score, threshold)
	return result
}

func parseNextActionCandidates(message string) []NextActionCandidate {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	candidates := []NextActionCandidate{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for i := 1; i <= 9; i++ {
			prefixDot := fmt.Sprintf("%d.", i)
			prefixParen := fmt.Sprintf("%d)", i)
			var rest string
			switch {
			case strings.HasPrefix(trimmed, prefixDot):
				rest = strings.TrimSpace(strings.TrimPrefix(trimmed, prefixDot))
			case strings.HasPrefix(trimmed, prefixParen):
				rest = strings.TrimSpace(strings.TrimPrefix(trimmed, prefixParen))
			default:
				continue
			}
			if rest == "" {
				continue
			}
			candidate := NextActionCandidate{
				Index:       i,
				Text:        rest,
				Recommended: nextActionIsRecommended(rest),
				Destructive: nextActionIsDestructive(rest),
			}
			candidate.Score = scoreNextActionCandidate(candidate)
			candidates = append(candidates, candidate)
			break
		}
	}
	return candidates
}

func selectRecommendedNextAction(candidates []NextActionCandidate) *NextActionCandidate {
	for i := range candidates {
		if candidates[i].Recommended {
			return &candidates[i]
		}
	}
	return nil
}

func scoreNextActionCandidate(candidate NextActionCandidate) float64 {
	if candidate.Destructive {
		return 0
	}
	score := 0.0
	if candidate.Recommended {
		score += 0.55
	}
	if nextActionHasForwardVerb(candidate.Text) {
		score += 0.30
	}
	// Reversible, non-destructive bonus.
	score += 0.15
	return clampScore(score)
}

var nextActionForwardVerbs = []string{
	"진행", "계속", "구현", "추가", "작성", "검증", "실행", "수정", "반영", "적용",
	"proceed", "continue", "implement", "add", "write", "verify", "run", "apply", "fix", "update",
}

var nextActionDestructiveNeedles = []string{
	"삭제", "제거", "지우", "되돌리", "덮어", "초기화", "닫기", "강제",
	"delete", "remove", "drop", "reset", "revert", "overwrite", "force", "discard", "purge", "close",
	"rm ", "--force", "-f ", "reset --hard", "push --force", "force-push",
}

func nextActionIsRecommended(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(text, "추천") || strings.Contains(lower, "(recommended)") || strings.Contains(lower, "recommended")
}

func nextActionIsDestructive(text string) bool {
	lower := strings.ToLower(text)
	for _, needle := range nextActionDestructiveNeedles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func nextActionHasForwardVerb(text string) bool {
	lower := strings.ToLower(text)
	for _, verb := range nextActionForwardVerbs {
		if strings.Contains(lower, strings.ToLower(verb)) {
			return true
		}
	}
	return false
}

func hasNumberedNextActions(message string) bool {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	seen := map[int]bool{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimPrefix(trimmed, "+ ")
		trimmed = strings.TrimSpace(trimmed)
		if len(trimmed) < 2 {
			continue
		}
		for i := 1; i <= 3; i++ {
			prefix := fmt.Sprintf("%d.", i)
			if strings.HasPrefix(trimmed, prefix) || strings.HasPrefix(trimmed, fmt.Sprintf("%d)", i)) {
				seen[i] = true
			}
		}
	}
	return seen[1] && seen[2] && seen[3]
}

func BuildLifecyclePreCompactCapsule(repo string) LifecycleCompactResult {
	events, plan, err := ReadPendingDocUpkeepEvents(repo, 8)
	if err != nil {
		return LifecycleCompactResult{OK: true, Warnings: []string{"pending_doc_upkeep_read_error"}}
	}
	if !plan.Exists || !plan.NamespaceValid || len(events) == 0 {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	capsule := LifecycleCompactCapsule{
		SchemaVersion:     ProjectLifecycleSchemaVersion,
		RepoRoot:          plan.RepoRoot,
		RepoID:            plan.RepoID,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		RequiredDocs:      docsFromDocUpkeepEvents(events),
		PendingDocUpkeep:  events,
		AdditionalSummary: "Session compaction capsule: restore these lifecycle/doc-upkeep hints after compacting to avoid rediscovering project-doc context.",
	}
	if err := writeJSONAtomic(plan.CompactPath, capsule, 0o600); err != nil {
		return LifecycleCompactResult{OK: false, PendingCount: len(events), CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_write_error"}}
	}
	return LifecycleCompactResult{OK: true, Recorded: true, PendingCount: len(events), CompactPath: plan.CompactPath}
}

func BuildLifecyclePostCompactReminder(repo string) LifecycleCompactResult {
	plan, err := ValidateProjectLifecycleState(repo)
	if err != nil {
		return LifecycleCompactResult{OK: true, Warnings: []string{"lifecycle_state_read_error"}}
	}
	if !plan.Exists || !plan.NamespaceValid {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	b, err := os.ReadFile(plan.CompactPath)
	if os.IsNotExist(err) {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	if err != nil {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_read_error"}}
	}
	var capsule LifecycleCompactCapsule
	if err := json.Unmarshal(b, &capsule); err != nil {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_decode_error"}}
	}
	if capsule.SchemaVersion != ProjectLifecycleSchemaVersion || capsule.RepoID != plan.RepoID {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath, Warnings: []string{"compact_capsule_namespace_mismatch"}}
	}
	context := renderLifecycleCompactContext(capsule)
	if strings.TrimSpace(context) == "" {
		return LifecycleCompactResult{OK: true, CompactPath: plan.CompactPath}
	}
	_ = os.Remove(plan.CompactPath)
	return LifecycleCompactResult{
		OK:                true,
		ShouldInject:      true,
		AdditionalContext: context,
		PendingCount:      len(capsule.PendingDocUpkeep),
		CompactPath:       plan.CompactPath,
	}
}

func docsFromDocUpkeepEvents(events []DocUpkeepEvent) []string {
	docs := []string{}
	for _, event := range events {
		docs = append(docs, event.TargetDocs...)
	}
	return normalizeTargetDocs(docs)
}

func renderLifecycleCompactContext(capsule LifecycleCompactCapsule) string {
	if len(capsule.PendingDocUpkeep) == 0 && len(capsule.RequiredDocs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Restored agent-harness compaction capsule:\n")
	if len(capsule.RequiredDocs) > 0 {
		b.WriteString("- Relevant project docs: ")
		b.WriteString(strings.Join(capsule.RequiredDocs, ", "))
		b.WriteString("\n")
	}
	if len(capsule.PendingDocUpkeep) > 0 {
		b.WriteString("- Pending doc upkeep preserved across compaction")
		docs := docsFromDocUpkeepEvents(capsule.PendingDocUpkeep)
		if len(docs) > 0 {
			b.WriteString(": ")
			b.WriteString(strings.Join(docs, ", "))
		}
		b.WriteString(". UserPromptSubmit will keep surfacing the current details until the queue is resolved.\n")
	}
	b.WriteString("Use this as routing context only; read/update project docs when the resumed task touches the listed areas.")
	return strings.TrimSpace(b.String())
}
