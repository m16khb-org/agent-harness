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
	EnforceVCSLinking    bool     `json:"enforce_vcs_linking,omitempty"`
	EnforceGitOpsKubectl bool     `json:"enforce_gitops_kubectl,omitempty"`
	EnforceStagedChecks  bool     `json:"enforce_staged_checks,omitempty"`
	ExpectedWorktree     string   `json:"expected_worktree,omitempty"`
	SourceCheckout       string   `json:"source_checkout,omitempty"`
	ProjectPath          string   `json:"project_path,omitempty"`
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
		if reason := mcpWorktreeRootBlockReason(req); reason != "" {
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
	if result.Decision != "block" && req.EnforceVCSLinking {
		if reason := vcsIssueLinkingBlockReason(req); reason != "" {
			result.Decision = "block"
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceStagedChecks {
		if decision, reason := stagedCheckDecision(req); decision != "" {
			result.Decision = decision
			result.Reason = reason
		}
	}
	if result.Decision != "block" && req.EnforceGitOpsKubectl {
		if decision, reason := gitOpsKubectlDecision(req.Tool, req.Command); decision != "" {
			result.Decision = decision
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
		return "IssueOps remote artifact gate requires inspectable Korean title and body before issue/pr/mr create/edit; provide --title and --body-file/--body after running the Korean gate"
	}
	hangul, englishWords := scoreKoreanRemoteArtifactLanguage(artifact.title + "\n" + artifact.body)
	cli := remoteArtifactCLIName(artifact)
	if hangul < 20 {
		return fmt.Sprintf("IssueOps remote artifact gate failed: expected at least 20 Hangul chars before %s %s %s, got %d", cli, artifact.kind, artifact.action, hangul)
	}
	if hangul > 0 && float64(englishWords)/float64(hangul) > 1.2 {
		return fmt.Sprintf("IssueOps remote artifact gate failed: English prose ratio too high before %s %s %s (english_words=%d, hangul_chars=%d)", cli, artifact.kind, artifact.action, englishWords, hangul)
	}
	return ""
}

type remoteArtifactCommand struct {
	provider  string
	kind      string
	action    string
	title     string
	body      string
	labels    []string
	assignees []string
}

func remoteArtifactCLIName(artifact remoteArtifactCommand) string {
	switch artifact.provider {
	case "gitlab":
		return "glab"
	case "github":
		return "gh"
	default:
		return "remote"
	}
}

func parseGHRemoteArtifactCommand(command string, repo string) (remoteArtifactCommand, bool) {
	tokens := splitCommandTokens(command)
	for i := 0; i+2 < len(tokens); i++ {
		cli := searchTokenName(tokens[i])
		provider := ""
		switch cli {
		case "gh":
			provider = "github"
		case "glab":
			provider = "gitlab"
		default:
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(tokens[i+1]))
		action := strings.ToLower(strings.TrimSpace(tokens[i+2]))
		// gh uses issue/pr + create/edit; glab uses issue/mr + create/edit/update.
		if kind != "issue" && kind != "pr" && kind != "mr" {
			continue
		}
		if action != "create" && action != "edit" && action != "update" {
			continue
		}
		artifact := remoteArtifactCommand{provider: provider, kind: kind, action: action}
		args := tokens[i+3:]
		if remoteArtifactHelpRequested(args) {
			return remoteArtifactCommand{}, false
		}
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
			// gh: --body/-b and --body-file/-F. glab: --description/-d.
			case arg == "--body" || arg == "-b" || arg == "--description" || arg == "-d":
				if j+1 < len(args) {
					artifact.body = args[j+1]
					j++
				}
			case strings.HasPrefix(arg, "--body="):
				artifact.body = strings.TrimPrefix(arg, "--body=")
			case strings.HasPrefix(arg, "--description="):
				artifact.body = strings.TrimPrefix(arg, "--description=")
			case arg == "--body-file" || arg == "-F":
				if j+1 < len(args) {
					artifact.body = readRemoteArtifactBodyFile(repo, args[j+1])
					j++
				}
			case strings.HasPrefix(arg, "--body-file="):
				artifact.body = readRemoteArtifactBodyFile(repo, strings.TrimPrefix(arg, "--body-file="))
			case arg == "--label" || arg == "-l" || arg == "--labels" || arg == "--add-label":
				if j+1 < len(args) {
					artifact.labels = appendRemoteArtifactLabels(artifact.labels, args[j+1])
					j++
				}
			case strings.HasPrefix(arg, "--label="):
				artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--label="))
			case strings.HasPrefix(arg, "--labels="):
				artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--labels="))
			case strings.HasPrefix(arg, "--add-label="):
				artifact.labels = appendRemoteArtifactLabels(artifact.labels, strings.TrimPrefix(arg, "--add-label="))
			case arg == "--assignee" || arg == "-a" || arg == "--assignees" || arg == "--add-assignee":
				if j+1 < len(args) {
					artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, args[j+1])
					j++
				}
			case strings.HasPrefix(arg, "--assignee="):
				artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--assignee="))
			case strings.HasPrefix(arg, "--assignees="):
				artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--assignees="))
			case strings.HasPrefix(arg, "--add-assignee="):
				artifact.assignees = appendRemoteArtifactListValues(artifact.assignees, strings.TrimPrefix(arg, "--add-assignee="))
			}
		}
		return artifact, true
	}
	return remoteArtifactCommand{}, false
}

func remoteArtifactHelpRequested(args []string) bool {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "--help", "-h":
			return true
		}
	}
	return false
}

func appendRemoteArtifactLabels(labels []string, raw string) []string {
	return appendRemoteArtifactListValues(labels, raw)
}

func appendRemoteArtifactListValues(values []string, raw string) []string {
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

var (
	planLinkHeadingRe = regexp.MustCompile(`(?mi)^#{1,6}\s*(Plan Link|Plan link|계획\s*링크)\s*$`)
	relatedHeadingRe  = regexp.MustCompile(`(?mi)^#{1,6}\s*(Related Issues|Related issues|관련\s*이슈)\s*$`)
)

// vcsIssueLinkingBlockReason enforces the provider-specific IssueOps linking
// rules in skills/issueops/references/remote-issue.md: no Plan Link section on
// any provider, and on GitLab related issues belong in native linked items
// rather than a body "Related Issues" section.
func vcsIssueLinkingBlockReason(req HookToolUseLifecycleRequest) string {
	artifact, ok := parseGHRemoteArtifactCommand(req.Command, req.Repo)
	if !ok {
		return ""
	}
	body := artifact.body
	if strings.TrimSpace(body) != "" {
		if planLinkHeadingRe.MatchString(body) {
			return fmt.Sprintf("IssueOps issue body must not contain a Plan Link section before %s %s %s; plan tracking lives in issueops link-plan state and the PR/MR body, not the issue body", artifact.provider, artifact.kind, artifact.action)
		}
		if artifact.provider == "gitlab" && (artifact.kind == "issue") && relatedHeadingRe.MatchString(body) {
			return fmt.Sprintf("GitLab related issues must be attached as native linked items, not a body Related Issues section, before glab %s %s; use glab api projects/:id/issues/:iid/links with link_type=relates_to", artifact.kind, artifact.action)
		}
	}
	if artifact.action == "create" && len(artifact.labels) == 0 {
		return fmt.Sprintf("IssueOps remote %s create must include labels before %s %s create; copy the linked issue labels or pass an explicit manual label flag", artifact.kind, artifact.provider, artifact.kind)
	}
	if artifact.action == "create" && len(artifact.assignees) == 0 {
		return fmt.Sprintf("IssueOps remote %s create must include an assignee before %s %s create; assign the artifact to the currently authenticated user and verify the assignee list before reporting readiness", artifact.kind, artifact.provider, artifact.kind)
	}
	return ""
}

func stagedCheckDecision(req HookToolUseLifecycleRequest) (string, string) {
	if !isShellTool(req.Tool) {
		return "", ""
	}
	for _, command := range expandPackageScriptCommands(req.Repo, req.Command) {
		if broadBiomeCheckCommand(command) {
			return "ask", "Broad lint/format checks can fail on unrelated existing debt. Prefer staged or explicit changed-file checks such as `biome check --staged`, `biome format --staged`, lint-staged, or a file list for this diff; ask the user before running a repo-wide apps/libs check."
		}
	}
	return "", ""
}

func expandPackageScriptCommands(repo string, command string) []string {
	commands := []string{command}
	tokens := splitCommandTokens(command)
	for i := 0; i+2 < len(tokens); i++ {
		cli := searchTokenName(tokens[i])
		if cli != "npm" {
			continue
		}
		action := strings.ToLower(searchTokenName(tokens[i+1]))
		if action != "run" && action != "run-script" {
			continue
		}
		scriptName := strings.TrimSpace(tokens[i+2])
		if script := packageScript(repo, scriptName); script != "" {
			commands = append(commands, script)
		}
	}
	return commands
}

func packageScript(repo string, scriptName string) string {
	if strings.TrimSpace(repo) == "" || strings.TrimSpace(scriptName) == "" {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(repo, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil {
		return ""
	}
	return strings.TrimSpace(pkg.Scripts[scriptName])
}

func broadBiomeCheckCommand(command string) bool {
	tokens := splitCommandTokens(command)
	for i := 0; i+1 < len(tokens); i++ {
		if searchTokenName(tokens[i]) != "biome" {
			continue
		}
		subcommand := strings.ToLower(searchTokenName(tokens[i+1]))
		if subcommand != "check" && subcommand != "format" && subcommand != "ci" {
			continue
		}
		args := tokens[i+2:]
		if biomeArgsAreScoped(args) {
			continue
		}
		if biomeArgsIncludeBroadRepoDirs(args) {
			return true
		}
	}
	return false
}

func biomeArgsAreScoped(args []string) bool {
	for _, arg := range args {
		name := strings.TrimSpace(arg)
		if name == "--staged" || name == "--changed" || strings.HasPrefix(name, "--since") {
			return true
		}
	}
	return false
}

func biomeArgsIncludeBroadRepoDirs(args []string) bool {
	for _, arg := range args {
		name := strings.Trim(strings.TrimSpace(arg), `"'`)
		if name == "apps" || name == "libs" || name == "apps/" || name == "libs/" {
			return true
		}
	}
	return false
}

func gitOpsKubectlDecision(tool string, command string) (string, string) {
	if !isShellTool(tool) {
		return "", ""
	}
	tokens := splitCommandTokens(command)
	for i, token := range tokens {
		if searchTokenName(token) != "kubectl" {
			continue
		}
		verb, subverb := kubectlVerb(tokens[i+1:])
		if kubectlLiveAccessNeedsConfirmation(verb) {
			return "ask", "kubectl live cluster access requires explicit user confirmation: exec and port-forward can expose live workloads or local ports. Confirm before running this command."
		}
		if kubectlMutationBlocked(verb, subverb, tokens[i+1:]) {
			return "block", "GitOps is the source of truth for cluster changes: do not run direct mutating kubectl commands from the agent. Edit Kubernetes manifests in git and use the repo's GitOps review/apply path instead."
		}
	}
	return "", ""
}

func kubectlVerb(args []string) (string, string) {
	verbIndex := -1
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if kubectlFlagTakesValue(arg) && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		verbIndex = i
		break
	}
	if verbIndex == -1 {
		return "", ""
	}
	verb := strings.ToLower(searchTokenName(args[verbIndex]))
	subverb := ""
	for _, arg := range args[verbIndex+1:] {
		arg = strings.TrimSpace(arg)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if isShellSeparator(arg) {
			break
		}
		subverb = strings.ToLower(searchTokenName(arg))
		break
	}
	return verb, subverb
}

func kubectlFlagTakesValue(flag string) bool {
	name := strings.TrimLeft(strings.TrimSpace(flag), "-")
	if cut, _, ok := strings.Cut(name, "="); ok {
		name = cut
	}
	switch name {
	case "n", "namespace", "context", "kubeconfig", "server", "token", "as", "as-group", "user", "cluster", "request-timeout", "field-manager", "filename", "f", "output", "o", "selector", "l":
		return true
	default:
		return false
	}
}

func kubectlMutationBlocked(verb string, subverb string, args []string) bool {
	if verb == "" {
		return false
	}
	if kubectlDryRun(args) {
		return false
	}
	switch verb {
	case "apply", "annotate", "autoscale", "cordon", "create", "delete", "drain", "edit", "expose", "label", "patch", "replace", "run", "scale", "set", "taint", "uncordon":
		return true
	case "rollout":
		return subverb == "restart" || subverb == "undo" || subverb == "pause" || subverb == "resume"
	default:
		return false
	}
}

func kubectlLiveAccessNeedsConfirmation(verb string) bool {
	switch verb {
	case "exec", "port-forward":
		return true
	default:
		return false
	}
}

func kubectlDryRun(args []string) bool {
	for i, arg := range args {
		switch {
		case arg == "--dry-run=client", arg == "--dry-run=server":
			return true
		case arg == "--dry-run" && i+1 < len(args) && (args[i+1] == "client" || args[i+1] == "server"):
			return true
		}
	}
	return false
}

func isShellSeparator(token string) bool {
	switch token {
	case "&&", "||", ";", "|":
		return true
	default:
		return false
	}
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
	if !toolUseMayMutateLifecycleFiles(req.Tool, req.Command) {
		return ""
	}
	expected := cleanAbsPath(req.ExpectedWorktree)
	if expected == "" {
		// No explicit expected worktree: judge by the current work's own cycle.
		// The cycle id is deterministic per (repo, branch), so only this branch's
		// record is consulted; legacy/stale records have different ids and are
		// never read, and a done cycle releases the source checkout.
		rec, ok := ActiveIssueOpsCycleForBranch(req.Repo, gitBranchFromHead(req.Repo))
		targets := worktreeGuardEditTargets(req)
		if len(targets) == 0 {
			return ""
		}
		if !ok || !IssueOpsPhaseExpectsWorktree(rec.Phase) {
			return ""
		}
		linked := cleanAbsPath(rec.WorktreePath)
		if linked == "" {
			return ""
		}
		for _, target := range targets {
			if !pathWithin(target, linked) {
				return "mutating tool target is outside the linked IssueOps worktree for " + rec.ID + "; run issue-based work from " + linked + " or mark the stale cycle done"
			}
		}
		return ""
	}
	targets := worktreeGuardTargets(req)
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

func mcpWorktreeRootBlockReason(req HookToolUseLifecycleRequest) string {
	expected := cleanAbsPath(req.ExpectedWorktree)
	if expected == "" {
		return ""
	}
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	switch {
	case isCodeGraphTool(tool):
		projectPath := cleanAbsPath(req.ProjectPath)
		if projectPath == "" {
			return "CodeGraph in an IssueOps worktree must set projectPath to the expected IssueOps worktree: " + expected
		}
		if projectPath != expected {
			return "CodeGraph projectPath is outside the expected IssueOps worktree; set projectPath to " + expected
		}
	case strings.Contains(tool, "filesystem") || strings.Contains(tool, "serena"):
		return "source-root-bound MCP tool is not allowed during IssueOps worktree implementation; use native absolute-path file tools, rg rooted at the IssueOps worktree, git -C, or CodeGraph with projectPath " + expected
	}
	return ""
}

func worktreeGuardTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	if repo := cleanAbsPath(req.Repo); repo != "" {
		targets = append(targets, repo)
	}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

// worktreeGuardEditTargets prefers explicit edit paths and only falls back to the
// repo cwd when none are given. This lets an absolute edit into the isolated
// worktree pass even while the shell cwd is still the source checkout.
func worktreeGuardEditTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(req.Repo, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && isShellTool(req.Tool) {
		for _, path := range shellCommandWorktreeGuardPaths(req.Command) {
			if target := resolveHookTargetPath(req.Repo, path); target != "" {
				targets = append(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		if repo := cleanAbsPath(req.Repo); repo != "" {
			targets = append(targets, repo)
		}
	}
	return targets
}

func shellCommandWorktreeGuardPaths(command string) []string {
	tokens := splitCommandTokens(command)
	out := []string{}
	seen := map[string]bool{}
	for i, token := range tokens {
		switch token {
		case "cd":
			if i+1 < len(tokens) {
				addWorktreeGuardPath(&out, seen, tokens[i+1])
			}
		case "-C":
			if i > 0 && searchTokenName(tokens[i-1]) == "git" && i+1 < len(tokens) {
				addWorktreeGuardPath(&out, seen, tokens[i+1])
			}
		case ">", ">>", "1>", "1>>", "2>", "2>>":
			if i+1 < len(tokens) {
				addWorktreeGuardPath(&out, seen, tokens[i+1])
			}
		default:
			for _, prefix := range []string{">>", ">", "1>>", "1>", "2>>", "2>"} {
				if strings.HasPrefix(token, prefix) && len(token) > len(prefix) {
					addWorktreeGuardPath(&out, seen, strings.TrimPrefix(token, prefix))
					break
				}
			}
		}
	}
	return out
}

func addWorktreeGuardPath(out *[]string, seen map[string]bool, value string) {
	path := strings.TrimSpace(value)
	if path == "" || strings.HasPrefix(path, "-") || strings.Contains(path, "$(") || strings.Contains(path, "`") {
		return
	}
	if seen[path] {
		return
	}
	seen[path] = true
	*out = append(*out, path)
}

// gitBranchFromHead returns the current branch of the checkout at repo by reading
// HEAD, resolving the linked-worktree gitdir indirection when .git is a file.
// It returns "" for a detached HEAD or when HEAD cannot be read.
func gitBranchFromHead(repo string) string {
	root := cleanAbsPath(repo)
	if root == "" {
		return ""
	}
	gitPath := filepath.Join(root, ".git")
	headPath := filepath.Join(gitPath, "HEAD")
	if info, err := os.Stat(gitPath); err == nil && !info.IsDir() {
		if b, err := os.ReadFile(gitPath); err == nil {
			line := strings.TrimSpace(string(b))
			if rest, ok := strings.CutPrefix(line, "gitdir:"); ok {
				// git writes a gitdir relative to the worktree; resolve it against
				// the repo root, not the process CWD.
				resolved := strings.TrimSpace(rest)
				if !filepath.IsAbs(resolved) {
					resolved = filepath.Join(root, resolved)
				}
				headPath = filepath.Join(resolved, "HEAD")
			}
		}
	}
	b, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	head := strings.TrimSpace(string(b))
	if rest, ok := strings.CutPrefix(head, "ref: refs/heads/"); ok {
		return strings.TrimSpace(rest)
	}
	return ""
}

// isInsideWorktreesPath reports whether any path segment ends with ".worktrees",
// matching the IssueOps sibling worktree convention `<repo>.worktrees/<branch>`.
func isInsideWorktreesPath(target string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(target), "/") {
		if strings.HasSuffix(segment, ".worktrees") {
			return true
		}
	}
	return false
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
	result.Reason = missingNumberedNextActionsReason()
	return result
}

func missingNumberedNextActionsReason() string {
	return "Stop hook blocked because the final response lacks numbered next actions. Continue by briefly explaining that missing next-action choices caused the block, then present a context-specific `선택지:` section with exactly three numbered options and exactly one `(추천)` option."
}

const defaultNextActionAutoProceedThreshold = 0.80

// NextActionCandidate is a parsed numbered next-action option.
// Score and Destructive are legacy fields used only by the deprecated
// auto-proceed evaluators; hook-facing judgement triggers must leave them unset.
type NextActionCandidate struct {
	Index       int     `json:"index"`
	Text        string  `json:"text"`
	Recommended bool    `json:"recommended"`
	Destructive bool    `json:"destructive"`
	Score       float64 `json:"score"`
}

// NextActionJudgementTriggerResult reports only facts observed in the final
// assistant response. It is not a safety, reversibility, confidence, destructive,
// or execution-eligibility verdict; the main agent owns those judgements.
type NextActionJudgementTriggerResult struct {
	OK                 bool                  `json:"ok"`
	ShouldReenterAgent bool                  `json:"should_reenter_agent"`
	ChoicesFound       bool                  `json:"choices_found"`
	ChoiceCount        int                   `json:"choice_count"`
	RecommendedCount   int                   `json:"recommended_count"`
	RecommendedIndex   int                   `json:"recommended_index,omitempty"`
	RecommendedText    string                `json:"recommended_text,omitempty"`
	Reason             string                `json:"reason"`
	Evidence           []string              `json:"evidence"`
	Candidates         []NextActionCandidate `json:"candidates"`
}

// BuildNextActionJudgementTrigger detects whether the assistant reached an
// explicit next-action review point and returns inspectable facts for the main
// agent. It deliberately does not score, classify, or veto choices.
func BuildNextActionJudgementTrigger(message string) NextActionJudgementTriggerResult {
	result := NextActionJudgementTriggerResult{OK: true, Candidates: []NextActionCandidate{}}
	candidates := parseNextActionCandidateFacts(message)
	if len(candidates) == 0 {
		result.Reason = "no explicit next-action choices found"
		return result
	}
	result.ChoicesFound = true
	result.ShouldReenterAgent = true
	result.ChoiceCount = len(candidates)
	result.Candidates = candidates
	result.Evidence = append(result.Evidence, fmt.Sprintf("explicit next-action choices found: %d", len(candidates)))
	for _, candidate := range candidates {
		if !candidate.Recommended {
			continue
		}
		result.RecommendedCount++
		if result.RecommendedIndex == 0 {
			result.RecommendedIndex = candidate.Index
			result.RecommendedText = candidate.Text
		}
	}
	result.Evidence = append(result.Evidence, fmt.Sprintf("recommended marker count: %d", result.RecommendedCount))
	switch result.RecommendedCount {
	case 0:
		result.Reason = "next-action choices found without an explicit recommendation"
	case 1:
		result.Reason = "next-action choices found with exactly one explicit recommendation"
	default:
		result.Reason = "next-action choices found with multiple explicit recommendations"
	}
	return result
}

// NextActionAutoProceedResult reports whether the recommended next action is a
// guarded candidate for context-aware auto-proceed judgement by the main agent.
type NextActionAutoProceedResult struct {
	OK                     bool                  `json:"ok"`
	AutoProceed            bool                  `json:"auto_proceed"`
	AgentJudgementRequired bool                  `json:"agent_judgement_required"`
	Threshold              float64               `json:"threshold"`
	TopScore               float64               `json:"top_score"`
	SelectedIndex          int                   `json:"selected_index,omitempty"`
	SelectedText           string                `json:"selected_text,omitempty"`
	Reason                 string                `json:"reason"`
	BlockedByGuard         string                `json:"blocked_by_guard,omitempty"`
	Candidates             []NextActionCandidate `json:"candidates"`
}

// EvaluateNextActionAutoProceed scores parsed next-action choices for the legacy
// auto-proceed experiment. It is not used by the Stop hook path; hook-facing code
// must use BuildNextActionJudgementTrigger so the hook relays facts instead of
// judging, scoring, or classifying choices.
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
		result.AgentJudgementRequired = true
		result.Reason = fmt.Sprintf("recommended action scored %.2f >= threshold %.2f and is reversible; agent judgement required", recommended.Score, threshold)
		return result
	}
	result.Reason = fmt.Sprintf("recommended action scored %.2f below threshold %.2f; user decision required", recommended.Score, threshold)
	return result
}

func parseNextActionCandidates(message string) []NextActionCandidate {
	candidates := parseNextActionCandidateFacts(message)
	for i := range candidates {
		candidates[i].Destructive = nextActionIsDestructive(candidates[i].Text)
		candidates[i].Score = scoreNextActionCandidate(candidates[i])
	}
	return candidates
}

func parseNextActionCandidateFacts(message string) []NextActionCandidate {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	candidates := []NextActionCandidate{}
	inChoices := false
	for _, line := range lines {
		trimmed := normalizeNextActionLine(line)
		if nextActionSectionHeader(trimmed) {
			inChoices = true
			continue
		}
		if !inChoices {
			continue
		}
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
			}
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

// scoreNextActionCandidate assigns a 0..1 auto-proceed confidence to a parsed
// next-action option. The calibration is deliberately discerning because this
// heuristic is the fallback when the external-LLM auto-proceed gate is
// unavailable, so its verdict must be safe on its own:
//
//   - Destructive/irreversible → 0 (hard veto; never auto-proceeds).
//   - Ambiguous/hedged recommendation → dampened well below threshold so an
//     uncertain "forward" step still stops for the user.
//   - recommended + a clear forward/safe verb (reversible, non-ambiguous) ≈ 1.0,
//     comfortably clearing the 0.80 threshold.
//   - recommended + reversible but NO forward verb stays just under 0.80 so it
//     does NOT auto-proceed (unchanged from prior behavior).
//
// Weights: recommended 0.55, forward/safe verb 0.30, reversible baseline 0.15.
// Only the recommended candidate is ever auto-proceeded by the caller, but the
// score is computed uniformly so debug output reflects every option.
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
	// Reversible, non-destructive baseline. recommended (0.55) + reversible
	// baseline (0.15) = 0.70, which stays just below the 0.80 threshold so a
	// recommended option with no forward verb does not auto-proceed.
	score += 0.15
	// Ambiguity/uncertainty dampening: hedged language signals the action is not
	// a confident forward step. Subtract enough to push even a recommended +
	// forward-verb candidate (1.00) below the 0.80 threshold so it stops for the
	// user instead of auto-proceeding on a guess.
	if nextActionIsAmbiguous(candidate.Text) {
		score -= 0.45
	}
	return clampScore(score)
}

// nextActionForwardVerbs reward low-risk forward / verification / local-only
// steps in English and Korean. These are read-only or reversible-by-default
// actions that are safe to auto-execute when explicitly recommended.
var nextActionForwardVerbs = []string{
	"진행", "계속", "구현", "추가", "작성", "검증", "테스트", "빌드", "린트", "확인", "점검", "실행", "수정", "반영", "적용",
	"proceed", "continue", "implement", "add", "write", "verify", "test", "lint", "build", "check", "inspect", "dry-run", "dry run", "run", "apply", "fix", "update",
}

// nextActionAmbiguityNeedles are hedging/uncertainty markers in English and
// Korean. Their presence dampens the score below threshold: an action the agent
// is unsure about is not a confident forward step and must defer to the user.
var nextActionAmbiguityNeedles = []string{
	"아마도", "아마", "확실치", "확실하지", "추정", "미확인", "검토 필요", "검토필요",
	"maybe", "perhaps", "might", "not sure", "unsure", "tbd", "???",
}

// nextActionDestructiveWordNeedles are matched on ASCII word boundaries so that
// "force" does not match "enforce"/"reinforce" and "merge" does not match
// "merged" status text. These are outbound / irreversible operations — pushing,
// deploying, releasing, publishing, merging, rebasing, dropping/truncating data,
// deleting, sending notifications, payments, rollouts, and infra apply/delete —
// that must force score 0 and never auto-proceed.
var nextActionDestructiveWordNeedles = []string{
	"delete", "remove", "drop", "truncate", "reset", "revert", "rollback", "rollout", "overwrite", "force", "discard", "purge", "close", "merge", "rebase",
	"push", "deploy", "release", "publish", "ship", "send", "email", "notify", "payment", "charge", "refund", "terraform", "kubectl", "prod", "production",
}

// nextActionDestructiveRawNeedles are matched as substrings: Korean terms (no ASCII
// word boundary) and command fragments that carry their own delimiters.
var nextActionDestructiveRawNeedles = []string{
	"삭제", "제거", "지우", "되돌리", "덮어", "초기화", "닫기", "강제", "병합", "머지",
	"푸시", "배포", "릴리즈", "게시", "전송", "결제", "환불", "운영", "프로덕션", "롤백", "롤아웃",
	"rm ", "--force", "-f ", "reset --hard", "push --force", "force-push", "terraform apply", "kubectl apply", "kubectl delete",
}

var nextActionDestructiveWordRe = regexp.MustCompile(`(?i)\b(?:` + strings.Join(nextActionDestructiveWordNeedles, "|") + `)\b`)

func nextActionIsRecommended(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(text, "추천") || strings.Contains(lower, "(recommended)") || strings.Contains(lower, "recommended")
}

func nextActionIsDestructive(text string) bool {
	lower := strings.ToLower(text)
	for _, needle := range nextActionDestructiveRawNeedles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return nextActionDestructiveWordRe.MatchString(text)
}

func nextActionIsAmbiguous(text string) bool {
	lower := strings.ToLower(text)
	for _, needle := range nextActionAmbiguityNeedles {
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
	inChoices := false
	for _, line := range lines {
		trimmed := normalizeNextActionLine(line)
		if nextActionSectionHeader(trimmed) {
			inChoices = true
			continue
		}
		if !inChoices {
			continue
		}
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

func normalizeNextActionLine(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "- ")
	trimmed = strings.TrimPrefix(trimmed, "* ")
	trimmed = strings.TrimPrefix(trimmed, "+ ")
	return strings.TrimSpace(trimmed)
}

func nextActionSectionHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(trimmed, "선택지:") ||
		strings.HasPrefix(lower, "options:") ||
		strings.HasPrefix(lower, "next actions:")
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
