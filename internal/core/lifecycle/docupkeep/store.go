package docupkeep

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	"agent-harness/internal/core/lifecycle/model"
	"agent-harness/internal/core/projectdoc"
	corestate "agent-harness/internal/adapter/outbound/state"
)

type Store struct {
	Validate func(repoRoot string) (model.ProjectLifecycleStatePlan, error)
	Init     func(repoRoot string, confirm bool) (model.ProjectLifecycleStatePlan, error)
}

func Append(store Store, repoRoot string, event lifecyclecontract.DocUpkeepEvent) (model.DocUpkeepAppendResult, error) {
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
	b, err := json.Marshal(event)
	if err != nil {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	if err := corestate.WithKeyLock(context.Background(), plan.ProjectStateDir, "doc-upkeep", func(context.Context) error {
		f, err := os.OpenFile(plan.QueuePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(append(b, '\n'))
		return err
	}); err != nil {
		return model.DocUpkeepAppendResult{OK: false, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath}, err
	}
	return model.DocUpkeepAppendResult{OK: true, RepoRoot: plan.RepoRoot, RepoID: plan.RepoID, ProjectStateDir: plan.ProjectStateDir, Path: plan.QueuePath, Event: event}, nil
}

func ReadPending(store Store, repoRoot string, limit int) ([]lifecyclecontract.DocUpkeepEvent, model.ProjectLifecycleStatePlan, error) {
	plan, err := store.Validate(repoRoot)
	if err != nil || !plan.Exists || !plan.NamespaceValid {
		return []lifecyclecontract.DocUpkeepEvent{}, plan, err
	}
	events := []lifecyclecontract.DocUpkeepEvent{}
	if err := corestate.WithKeyLock(context.Background(), plan.ProjectStateDir, "doc-upkeep", func(context.Context) error {
		f, err := os.Open(plan.QueuePath)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		defer f.Close()
		validCount := 0
		malformedCount := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var event lifecyclecontract.DocUpkeepEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				malformedCount++
				continue
			}
			validCount++
			if event.Status == "" || event.Status == "pending" {
				events = append(events, event)
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if malformedCount > 0 || validCount != len(events) {
			if err := rewriteDocUpkeepQueue(plan.QueuePath, events); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
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

func docUpkeepEventID(repoID string, event lifecyclecontract.DocUpkeepEvent, at string) string {
	sum := sha256.Sum256([]byte(repoID + "\x00" + event.Kind + "\x00" + event.Summary + "\x00" + strings.Join(event.TargetDocs, ",") + "\x00" + at))
	return hex.EncodeToString(sum[:])[:24]
}

func rewriteDocUpkeepQueue(path string, events []lifecyclecontract.DocUpkeepEvent) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			_ = tmp.Close()
			return err
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
