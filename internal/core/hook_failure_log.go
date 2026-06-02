package core

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hookFailureLogFile = "hook-failures.jsonl"
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
	event.Hook = redactFreeform(strings.TrimSpace(event.Hook))
	event.Host = redactFreeform(strings.TrimSpace(event.Host))
	event.Repo = redactFreeform(strings.TrimSpace(event.Repo))
	event.CWD = redactFreeform(strings.TrimSpace(event.CWD))
	event.Tool = redactFreeform(strings.TrimSpace(event.Tool))
	event.Argv = redactArgv(event.Argv)
	event.CommandSnippet = trimHookFailureSnippet(redactFreeform(strings.TrimSpace(event.CommandSnippet)))
	event.Error = trimHookFailureSnippet(redactFreeform(strings.TrimSpace(event.Error)))
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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return HookFailureRecordResult{OK: false, Path: path, Event: event}, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return HookFailureRecordResult{OK: false, Path: path, Event: event}, err
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
	return filepath.Join(StateDir(), hookFailureLogFile)
}

func trimHookFailureSnippet(value string) string {
	if len([]byte(value)) <= hookFailureSnippetLimit {
		return value
	}
	b := []byte(value)
	return string(b[:hookFailureSnippetLimit]) + "...<truncated>"
}
