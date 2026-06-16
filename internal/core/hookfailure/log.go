package hookfailure

import (
	"agent-harness/internal/core/policy"
	corestate "agent-harness/internal/core/state"
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hookFailureLogFile = corestate.HookFailureLogFile
const hookFailureSnippetLimit = 500

type HookFailureEvent struct {
	Timestamp      string   `json:"timestamp,omitempty"`
	Hook           string   `json:"hook"`
	Host           string   `json:"host,omitempty"`
	Repo           string   `json:"repo,omitempty"`
	CWD            string   `json:"cwd,omitempty"`
	Tool           string   `json:"tool,omitempty"`
	Argv           []string `json:"argv,omitempty"`
	CommandSnippet string   `json:"command_snippet,omitempty"`
	Error          string   `json:"error"`
}

type HookFailureRecordResult struct {
	OK    bool             `json:"ok"`
	Path  string           `json:"path"`
	Event HookFailureEvent `json:"event"`
}

type HookFailureListResult struct {
	OK     bool               `json:"ok"`
	Path   string             `json:"path"`
	Events []HookFailureEvent `json:"events"`
}

func RecordHookFailureEvent(event HookFailureEvent) (HookFailureRecordResult, error) {
	path := HookFailureLogPath()
	if strings.TrimSpace(event.Timestamp) == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	event.Hook = policy.RedactFreeform(strings.TrimSpace(event.Hook))
	event.Host = policy.RedactFreeform(strings.TrimSpace(event.Host))
	event.Repo = policy.RedactFreeform(strings.TrimSpace(event.Repo))
	event.CWD = policy.RedactFreeform(strings.TrimSpace(event.CWD))
	event.Tool = policy.RedactFreeform(strings.TrimSpace(event.Tool))
	event.Argv = policy.RedactArgv(event.Argv)
	event.CommandSnippet = trimHookFailureSnippet(policy.RedactFreeform(strings.TrimSpace(event.CommandSnippet)))
	event.Error = trimHookFailureSnippet(policy.RedactFreeform(strings.TrimSpace(event.Error)))
	if event.Hook == "" {
		event.Hook = "unknown"
	}
	if event.Error == "" {
		event.Error = "hook failed"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return HookFailureRecordResult{OK: false, Path: path, Event: event}, err
	}
	line, err := json.Marshal(event)
	if err != nil {
		return HookFailureRecordResult{OK: false, Path: path, Event: event}, err
	}
	// H2: serialize concurrent appends across processes. A marshalled failure line
	// can exceed PIPE_BUF (uncapped Argv/CWD plus two 500B snippet fields), so
	// O_APPEND alone does not guarantee atomic, non-interleaved writes; the flock
	// (via WithKeyLock) does. O_APPEND is kept so the OS still positions at EOF.
	writeErr := corestate.WithKeyLock(filepath.Dir(path), "hook-failures", func() error {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(append(line, '\n'))
		return err
	})
	if writeErr != nil {
		return HookFailureRecordResult{OK: false, Path: path, Event: event}, writeErr
	}
	return HookFailureRecordResult{OK: true, Path: path, Event: event}, nil
}

func ListHookFailureEvents(limit int) (HookFailureListResult, error) {
	path := HookFailureLogPath()
	if limit <= 0 {
		limit = 20
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return HookFailureListResult{OK: true, Path: path, Events: []HookFailureEvent{}}, nil
	}
	if err != nil {
		return HookFailureListResult{OK: false, Path: path}, err
	}
	defer f.Close()
	events := []HookFailureEvent{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event HookFailureEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return HookFailureListResult{OK: false, Path: path, Events: events}, err
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return HookFailureListResult{OK: true, Path: path, Events: events}, nil
}

func HookFailureLogPath() string {
	return filepath.Join(corestate.StateDir(), hookFailureLogFile)
}

func trimHookFailureSnippet(value string) string {
	if len([]byte(value)) <= hookFailureSnippetLimit {
		return value
	}
	b := []byte(value)
	return string(b[:hookFailureSnippetLimit]) + "...<truncated>"
}

type HookFailurePruneResult struct {
	OK     bool   `json:"ok"`
	Path   string `json:"path"`
	Pruned int    `json:"pruned"`
	Kept   int    `json:"kept"`
}

func PruneHookFailureLog(maxAge time.Duration) (HookFailurePruneResult, error) {
	path := HookFailureLogPath()
	result := HookFailurePruneResult{OK: false, Path: path}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		result.OK = true
		return result, nil
	}
	if err != nil {
		return result, err
	}

	cutoff := time.Now().UTC().Add(-maxAge)
	var kept []HookFailureEvent
	pruned := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event HookFailureEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			pruned++
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil {
			ts, err = time.Parse(time.RFC3339, event.Timestamp)
		}
		if err != nil || ts.Before(cutoff) {
			pruned++
			continue
		}
		kept = append(kept, event)
	}
	_ = f.Close()
	if err := scanner.Err(); err != nil {
		return result, err
	}

	tmpPath := path + ".tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return result, err
	}
	writeErr := false
	for _, event := range kept {
		line, err := json.Marshal(event)
		if err != nil {
			writeErr = true
			break
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			writeErr = true
			break
		}
	}
	// Fold Close into writeErr: a flush failure surfacing only at Close (e.g.
	// ENOSPC) must abort the rename so a truncated log never replaces the original.
	if cerr := tmp.Close(); cerr != nil {
		writeErr = true
	}
	if writeErr {
		_ = os.Remove(tmpPath)
		return result, nil
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return result, err
	}

	result.OK = true
	result.Pruned = pruned
	result.Kept = len(kept)
	return result, nil
}
