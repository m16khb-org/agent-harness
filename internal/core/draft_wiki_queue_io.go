package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
