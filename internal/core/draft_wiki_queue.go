package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const draftWikiQueueFile = "draft-wiki-queue.jsonl"
const draftWikiQueueLockFile = "draft-wiki-queue.lock"
const maxDraftWikiQueueEvents = 200

type DraftWikiQueueAppendRequest struct {
	RepoRoot       string   `json:"repo_root"`
	Tool           string   `json:"tool,omitempty"`
	Command        string   `json:"command,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	SourceMaterial string   `json:"source_material"`
	TargetWiki     string   `json:"target_wiki,omitempty"`
	TargetType     string   `json:"target_type,omitempty"`
	Source         string   `json:"source,omitempty"`
}

type DraftWikiQueueEvent struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	RepoRoot       string   `json:"repo_root"`
	Tool           string   `json:"tool,omitempty"`
	Command        string   `json:"command,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	SourceMaterial string   `json:"source_material,omitempty"`
	TargetWiki     string   `json:"target_wiki,omitempty"`
	TargetType     string   `json:"target_type,omitempty"`
	Source         string   `json:"source,omitempty"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
	DraftRelPath   string   `json:"draft_rel_path,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type DraftWikiQueueAppendResult struct {
	OK              bool                `json:"ok"`
	RepoRoot        string              `json:"repo_root"`
	RepoID          string              `json:"repo_id"`
	ProjectStateDir string              `json:"project_state_dir"`
	Path            string              `json:"path"`
	Event           DraftWikiQueueEvent `json:"event"`
}

type DraftWikiQueueProcessRequest struct {
	RepoRoot        string        `json:"repo_root"`
	AgyCommand      string        `json:"agy_command,omitempty"`
	AgyModel        string        `json:"agy_model,omitempty"`
	AgySettingsPath string        `json:"-"`
	TargetWiki      string        `json:"target_wiki,omitempty"`
	TargetType      string        `json:"target_type,omitempty"`
	Limit           int           `json:"limit,omitempty"`
	Timeout         time.Duration `json:"-"`
}

type DraftWikiQueueProcessResult struct {
	OK              bool                  `json:"ok"`
	Kind            string                `json:"kind"`
	RepoRoot        string                `json:"repo_root"`
	RepoID          string                `json:"repo_id"`
	ProjectStateDir string                `json:"project_state_dir"`
	Path            string                `json:"path"`
	Processed       int                   `json:"processed"`
	Succeeded       int                   `json:"succeeded"`
	Failed          int                   `json:"failed"`
	Warnings        []string              `json:"warnings,omitempty"`
	Events          []DraftWikiQueueEvent `json:"events"`
}

type DraftWikiQueuePruneResult struct {
	OK              bool   `json:"ok"`
	Kind            string `json:"kind"`
	RepoRoot        string `json:"repo_root,omitempty"`
	RepoID          string `json:"repo_id,omitempty"`
	ProjectStateDir string `json:"project_state_dir,omitempty"`
	Path            string `json:"path"`
	Keep            int    `json:"keep"`
	Before          int    `json:"before"`
	After           int    `json:"after"`
	Pruned          int    `json:"pruned"`
}

type DraftWikiQueuePruneAllResult struct {
	OK       bool                        `json:"ok"`
	Kind     string                      `json:"kind"`
	StateDir string                      `json:"state_dir"`
	Keep     int                         `json:"keep"`
	Before   int                         `json:"before"`
	After    int                         `json:"after"`
	Pruned   int                         `json:"pruned"`
	Queues   []DraftWikiQueuePruneResult `json:"queues"`
}

func AppendDraftWikiQueueEvent(req DraftWikiQueueAppendRequest) (DraftWikiQueueAppendResult, error) {
	plan, path, err := draftWikiQueuePath(req.RepoRoot, true)
	if err != nil {
		return DraftWikiQueueAppendResult{OK: false}, err
	}
	material := trimDraftWikiQueueMaterial(req.SourceMaterial)
	if material == "" {
		return DraftWikiQueueAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, fmt.Errorf("draft wiki queue source material is required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	event := DraftWikiQueueEvent{
		ID:             draftWikiQueueEventID(plan.RepoID, material, now),
		Kind:           "draft_wiki_suggest",
		RepoRoot:       plan.RepoRoot,
		Tool:           strings.TrimSpace(req.Tool),
		Command:        redactFreeform(strings.TrimSpace(req.Command)),
		Paths:          redactStringSlice(req.Paths),
		SourceMaterial: material,
		TargetWiki:     strings.TrimSpace(req.TargetWiki),
		TargetType:     targetType,
		Source:         strings.TrimSpace(req.Source),
		Status:         WorkerStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if event.Source == "" {
		event.Source = "post-tool-use"
	}
	if err := appendDraftWikiQueueEvent(path, event); err != nil {
		return DraftWikiQueueAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, err
	}
	if err := capDraftWikiQueueEvents(path, maxDraftWikiQueueEvents); err != nil {
		return DraftWikiQueueAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, err
	}
	return DraftWikiQueueAppendResult{OK: true, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path, Event: draftWikiQueueEventForResponse(event)}, nil
}

func PruneDraftWikiQueue(repoRoot string, keep int) (DraftWikiQueuePruneResult, error) {
	plan, path, err := draftWikiQueuePath(repoRoot, true)
	if err != nil {
		return DraftWikiQueuePruneResult{OK: false}, err
	}
	result, err := pruneDraftWikiQueuePath(path, keep)
	result.RepoRoot = plan.RepoRoot
	result.RepoID = plan.RepoID
	result.ProjectStateDir = plan.ProjectStateDir
	return result, err
}

func PruneAllDraftWikiQueues(stateDir string, keep int) (DraftWikiQueuePruneAllResult, error) {
	if strings.TrimSpace(stateDir) == "" {
		stateDir = StateDir()
	}
	result := DraftWikiQueuePruneAllResult{OK: true, Kind: "draft_wiki_queue_prune_all", StateDir: stateDir, Keep: keep, Queues: []DraftWikiQueuePruneResult{}}
	projectsDir := filepath.Join(stateDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		result.OK = false
		return result, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(projectsDir, entry.Name(), draftWikiQueueFile)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			result.OK = false
			return result, err
		}
		queue, err := pruneDraftWikiQueuePath(path, keep)
		queue.ProjectStateDir = filepath.Dir(path)
		if err != nil {
			result.OK = false
			return result, err
		}
		result.Before += queue.Before
		result.After += queue.After
		result.Pruned += queue.Pruned
		result.Queues = append(result.Queues, queue)
	}
	return result, nil
}

func ProcessDraftWikiQueue(req DraftWikiQueueProcessRequest) (DraftWikiQueueProcessResult, error) {
	plan, path, err := draftWikiQueuePath(req.RepoRoot, true)
	if err != nil {
		return DraftWikiQueueProcessResult{OK: false}, err
	}
	result := DraftWikiQueueProcessResult{
		OK:              true,
		Kind:            "draft_wiki_queue_process",
		RepoRoot:        plan.RepoRoot,
		RepoID:          plan.RepoID,
		ProjectStateDir: plan.ProjectStateDir,
		Path:            path,
		Events:          []DraftWikiQueueEvent{},
	}
	unlock, locked, err := acquireDraftWikiQueueLock(plan.ProjectStateDir)
	if err != nil {
		return result, err
	}
	if !locked {
		result.Warnings = append(result.Warnings, "draft-wiki queue is already being processed")
		return result, nil
	}
	defer unlock()
	events, warnings, err := readDraftWikiQueueEvents(path)
	if err != nil {
		return DraftWikiQueueProcessResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: path}, err
	}
	result.Warnings = append(result.Warnings, warnings...)
	limit := req.Limit
	if limit <= 0 {
		limit = 1
	}
	for i := range events {
		if result.Processed >= limit {
			break
		}
		if events[i].Status != "" && events[i].Status != WorkerStatusQueued {
			continue
		}
		events[i].Status = WorkerStatusRunning
		events[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := rewriteDraftWikiQueueEventsFunc(path, events); err != nil {
			return result, err
		}
		processed := processDraftWikiQueueEvent(req, events[i])
		events[i] = processed
		if err := rewriteDraftWikiQueueEventsFunc(path, events); err != nil {
			return result, err
		}
		result.Processed++
		if processed.Status == WorkerStatusSucceeded {
			result.Succeeded++
		} else {
			result.Failed++
		}
		result.Events = append(result.Events, draftWikiQueueEventForResponse(processed))
	}
	return result, nil
}

func processDraftWikiQueueEvent(req DraftWikiQueueProcessRequest, event DraftWikiQueueEvent) DraftWikiQueueEvent {
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = strings.TrimSpace(event.TargetType)
	}
	if targetType == "" {
		targetType = "notes"
	}
	targetWiki := strings.TrimSpace(req.TargetWiki)
	if targetWiki == "" {
		targetWiki = strings.TrimSpace(event.TargetWiki)
	}
	agyCommand := strings.TrimSpace(req.AgyCommand)
	if agyCommand == "" {
		agyCommand = "agy"
	}
	settingsPath := resolveAgySettingsPath(req.AgySettingsPath)
	configuredModel, err := readAgyConfiguredModel(settingsPath)
	if err != nil {
		return failDraftWikiQueueEvent(event, err)
	}
	agyModel := strings.TrimSpace(req.AgyModel)
	if agyModel == "" {
		agyModel = configuredModel
	}
	if configuredModel != agyModel {
		return failDraftWikiQueueEvent(event, fmt.Errorf("agy model mismatch: settings %s has %q, want %q", settingsPath, configuredModel, agyModel))
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	prompt := buildDraftWikiSuggestPrompt(DraftWikiSuggestRequest{
		Title:      "Draft wiki hook memory",
		TargetWiki: targetWiki,
		TargetType: targetType,
	}, event.SourceMaterial, agyModel, targetType)
	llm, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Command: agyCommand, WorkDir: event.RepoRoot, Prompt: prompt, Timeout: timeout})
	if err != nil {
		return failDraftWikiQueueEvent(event, fmt.Errorf("agy print failed: %w: %s", err, strings.TrimSpace(string(llm.Output))))
	}
	draftBody, err := decodeDraftWikiSuggestAgyOutput(llm.Output)
	if err != nil {
		return failDraftWikiQueueEvent(event, err)
	}
	draftPath, err := writeSuggestedDraft(event.RepoRoot, "Draft wiki hook memory", targetWiki, targetType, agyModel, draftBody)
	if err != nil {
		return failDraftWikiQueueEvent(event, err)
	}
	draft, err := readDraftWikiDraft(event.RepoRoot, draftPath, "draft")
	if err != nil {
		return failDraftWikiQueueEvent(event, err)
	}
	event.Status = WorkerStatusSucceeded
	event.DraftRelPath = draft.RelPath
	event.Error = ""
	event.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return event
}

func failDraftWikiQueueEvent(event DraftWikiQueueEvent, err error) DraftWikiQueueEvent {
	event.Status = WorkerStatusFailed
	event.Error = redactFreeform(err.Error())
	event.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return event
}

func draftWikiQueueEventForResponse(event DraftWikiQueueEvent) DraftWikiQueueEvent {
	event.SourceMaterial = ""
	return event
}

func draftWikiQueuePath(repoRoot string, ensure bool) (ProjectLifecycleStatePlan, string, error) {
	plan, err := ValidateProjectLifecycleState(repoRoot)
	if err != nil {
		return plan, "", err
	}
	if ensure && !plan.Exists {
		plan, err = InitProjectLifecycleState(repoRoot, true)
		if err != nil {
			return plan, "", err
		}
	}
	if ensure && !plan.NamespaceValid {
		return plan, "", fmt.Errorf("project lifecycle namespace mismatch for %s", plan.RepoRoot)
	}
	return plan, filepath.Join(plan.ProjectStateDir, draftWikiQueueFile), nil
}

func appendDraftWikiQueueEvent(path string, event DraftWikiQueueEvent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func capDraftWikiQueueEvents(path string, keep int) error {
	if keep <= 0 {
		return nil
	}
	count, err := countDraftWikiQueueLines(path, keep*2+1)
	if err != nil {
		return err
	}
	if count <= keep*2 {
		return nil
	}
	_, err = pruneDraftWikiQueuePath(path, keep)
	return err
}

func pruneDraftWikiQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	result := DraftWikiQueuePruneResult{OK: true, Kind: "draft_wiki_queue_prune", Path: path, Keep: keep}
	events, _, err := readDraftWikiQueueEvents(path)
	if err != nil {
		result.OK = false
		return result, err
	}
	result.Before = len(events)
	if keep < 0 {
		keep = 0
		result.Keep = 0
	}
	if keep > 0 && len(events) > keep {
		events = events[len(events)-keep:]
	} else if keep == 0 {
		events = []DraftWikiQueueEvent{}
	}
	result.After = len(events)
	result.Pruned = result.Before - result.After
	if err := rewriteDraftWikiQueueEventsFunc(path, events); err != nil {
		result.OK = false
		return result, err
	}
	return result, nil
}

func countDraftWikiQueueLines(path string, limit int) (int, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	count := 0
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		count++
		if limit > 0 && count >= limit {
			return count, nil
		}
	}
	return count, scanner.Err()
}

func readDraftWikiQueueEvents(path string) ([]DraftWikiQueueEvent, []string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []DraftWikiQueueEvent{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	events := []DraftWikiQueueEvent{}
	warnings := []string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event DraftWikiQueueEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			warnings = append(warnings, formatDraftWikiQueueMalformedWarning(lineNumber, line))
			continue
		}
		events = append(events, event)
	}
	return events, warnings, scanner.Err()
}

func acquireDraftWikiQueueLock(projectStateDir string) (func(), bool, error) {
	if err := os.MkdirAll(projectStateDir, 0o700); err != nil {
		return nil, false, err
	}
	path := filepath.Join(projectStateDir, draftWikiQueueLockFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if os.IsExist(err) {
		return func() {}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	_, writeErr := fmt.Fprintf(f, "%d %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return nil, false, writeErr
		}
		return nil, false, closeErr
	}
	return func() { _ = os.Remove(path) }, true, nil
}

func formatDraftWikiQueueMalformedWarning(lineNumber int, line string) string {
	line = redactFreeform(line)
	const maxLineBytes = 120
	if len([]byte(line)) > maxLineBytes {
		line = string([]byte(line)[:maxLineBytes]) + "...[truncated]"
	}
	return fmt.Sprintf("malformed JSONL line %d skipped: %s", lineNumber, line)
}

var rewriteDraftWikiQueueEventsFunc = rewriteDraftWikiQueueEvents

func rewriteDraftWikiQueueEvents(path string, events []DraftWikiQueueEvent) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.WriteString(b.String()); err != nil {
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
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

func trimDraftWikiQueueMaterial(material string) string {
	material = strings.TrimSpace(material)
	if material == "" {
		return ""
	}
	lines := strings.Split(material, "\n")
	for i, line := range lines {
		lines[i] = redactFreeform(line)
	}
	material = strings.TrimSpace(strings.Join(lines, "\n"))
	const maxBytes = 12000
	if len([]byte(material)) <= maxBytes {
		return material
	}
	return string([]byte(material)[:maxBytes]) + "\n[truncated]"
}

func draftWikiQueueEventID(repoID, material, at string) string {
	sum := sha256.Sum256([]byte(repoID + "\x00" + material + "\x00" + at))
	return "dwq-" + hex.EncodeToString(sum[:])[:24]
}
