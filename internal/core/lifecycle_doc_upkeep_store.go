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
