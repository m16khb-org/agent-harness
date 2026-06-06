package draftwiki

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func AppendQueueEvent(path string, event DraftWikiQueueEvent) error {
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

func CapQueueEvents(path string, keep int) error {
	if keep <= 0 {
		return nil
	}
	count, err := CountQueueLines(path, keep*2+1)
	if err != nil {
		return err
	}
	if count <= keep*2 {
		return nil
	}
	_, err = PruneQueuePath(path, keep)
	return err
}

func PruneQueuePath(path string, keep int) (DraftWikiQueuePruneResult, error) {
	result := DraftWikiQueuePruneResult{OK: true, Kind: "draft_wiki_queue_prune", Path: path, Keep: keep}
	events, _, err := ReadQueueEvents(path)
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
	if err := RewriteQueueEventsFunc(path, events); err != nil {
		result.OK = false
		return result, err
	}
	return result, nil
}

var RewriteQueueEventsFunc = RewriteQueueEvents

func RewriteQueueEvents(path string, events []DraftWikiQueueEvent) error {
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
