package docupkeep

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

	"agent-harness/internal/core/lifecycle/model"
	"agent-harness/internal/core/projectdoc"
)

type Store struct {
	Validate func(repoRoot string) (model.ProjectLifecycleStatePlan, error)
	Init     func(repoRoot string, confirm bool) (model.ProjectLifecycleStatePlan, error)
}

func Append(store Store, repoRoot string, event model.DocUpkeepEvent) (model.DocUpkeepAppendResult, error) {
	plan, err := store.Validate(repoRoot)
	if err != nil {
		return model.DocUpkeepAppendResult{OK: false}, err
	}
	if !plan.Exists {
		plan, err = store.Init(repoRoot, true)
		if err != nil {
			return model.DocUpkeepAppendResult{OK: false}, err
		}
	}
	if !plan.NamespaceValid {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, fmt.Errorf("project lifecycle namespace mismatch for %s", plan.RepoRoot)
	}
	if strings.TrimSpace(event.Kind) == "" {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, fmt.Errorf("doc upkeep event kind is required")
	}
	if strings.TrimSpace(event.Summary) == "" {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, fmt.Errorf("doc upkeep event summary is required")
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
	event.TargetDocs = NormalizeTargetDocs(event.TargetDocs)
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	f, err := os.OpenFile(plan.QueuePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	defer f.Close()
	b, err := json.Marshal(event)
	if err != nil {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	return model.DocUpkeepAppendResult{OK: true, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath, Event: event}, nil
}

func ReadPending(store Store, repoRoot string, limit int) ([]model.DocUpkeepEvent, model.ProjectLifecycleStatePlan, error) {
	plan, err := store.Validate(repoRoot)
	if err != nil || !plan.Exists || !plan.NamespaceValid {
		return []model.DocUpkeepEvent{}, plan, err
	}
	f, err := os.Open(plan.QueuePath)
	if os.IsNotExist(err) {
		return []model.DocUpkeepEvent{}, plan, nil
	}
	if err != nil {
		return []model.DocUpkeepEvent{}, plan, err
	}
	defer f.Close()
	events := []model.DocUpkeepEvent{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event model.DocUpkeepEvent
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

func NormalizeTargetDocs(docs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		doc = strings.TrimPrefix(filepath.ToSlash(doc), projectdoc.ProjectDocsDir+"/")
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

func docUpkeepEventID(repoID string, event model.DocUpkeepEvent, at string) string {
	sum := sha256.Sum256([]byte(repoID + "\x00" + event.Kind + "\x00" + event.Summary + "\x00" + strings.Join(event.TargetDocs, ",") + "\x00" + at))
	return hex.EncodeToString(sum[:])[:24]
}
