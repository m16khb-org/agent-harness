package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	Repo    string   `json:"repo,omitempty"`
	Tool    string   `json:"tool,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Command string   `json:"command,omitempty"`
	Source  string   `json:"source,omitempty"`
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
	return HookPreToolUseDecisionResult{
		OK:       true,
		Decision: "allow",
		Tool:     strings.TrimSpace(req.Tool),
		Paths:    append([]string{}, req.Paths...),
		Command:  strings.TrimSpace(req.Command),
		Source:   source,
	}
}

func RecordLifecycleToolUse(req HookToolUseLifecycleRequest) (HookToolUseLifecycleResult, error) {
	repo := strings.TrimSpace(req.Repo)
	if repo == "" {
		return HookToolUseLifecycleResult{OK: true, Warnings: []string{"repo_missing"}}, nil
	}
	targets := []string{}
	for _, path := range req.Paths {
		targets = append(targets, docTargetsForLifecyclePath(path)...)
	}
	if strings.TrimSpace(req.Command) != "" {
		targets = append(targets, docTargetsForLifecyclePath(req.Command)...)
	}
	targets = normalizeTargetDocs(targets)
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
		b.WriteString("- Pending doc upkeep preserved across compaction:\n")
		for _, event := range capsule.PendingDocUpkeep {
			b.WriteString("  - ")
			if len(event.TargetDocs) > 0 {
				b.WriteString(strings.Join(event.TargetDocs, ", "))
				b.WriteString(": ")
			}
			b.WriteString(event.Summary)
			b.WriteString("\n")
		}
	}
	b.WriteString("Use this as routing context only; read/update project docs when the resumed task touches the listed areas.")
	return strings.TrimSpace(b.String())
}

func docTargetsForLifecyclePath(path string) []string {
	p := strings.ToLower(filepath.ToSlash(strings.TrimSpace(path)))
	if p == "" {
		return nil
	}
	out := []string{}
	if strings.Contains(p, "hook") || strings.Contains(p, "install") || strings.Contains(p, "daemon") || strings.Contains(p, "mcp") || strings.Contains(p, "doctor") || strings.Contains(p, "lifecycle_state") || strings.Contains(p, "state.go") {
		out = append(out, "OPERATIONS.md", "CONVENTIONS.md", "ARCHITECTURE.md")
	}
	if strings.Contains(p, "_test.go") || strings.Contains(p, "testdata/") || strings.Contains(p, "golden") {
		out = append(out, "TESTING.md")
	}
	if strings.Contains(p, "api_doc") || strings.Contains(p, "openapi") || strings.Contains(p, "swagger") {
		out = append(out, "OPEN_API_SPEC.md")
	}
	return out
}
