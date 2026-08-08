package hookfailure

import (
	hookfailurecontract "agent-harness/internal/contract/hookfailure"
	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/domain/policy"
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hookFailureLogFile = statecontract.HookFailureLogFile
const hookFailureSnippetLimit = 500

func RecordHookFailureEvent(event hookfailurecontract.HookFailureEvent) (hookfailurecontract.HookFailureRecordResult, error) {
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
		return hookfailurecontract.HookFailureRecordResult{OK: false, Path: path, Event: event}, err
	}
	line, err := json.Marshal(event)
	if err != nil {
		return hookfailurecontract.HookFailureRecordResult{OK: false, Path: path, Event: event}, err
	}
	// H2: 프로세스 간 동시 append를 직렬화한다. marshal된 실패 라인은 PIPE_BUF를
	// 초과할 수 있어(제한 없는 Argv/CWD와 500B snippet 필드 2개), O_APPEND만으로는
	// 원자적이고 뒤섞이지 않는 쓰기를 보장하지 못한다. state span lock(WithKeyLock)이
	// 이를 보장하며, O_APPEND는 OS 위치를 EOF에 유지한다.
	writeErr := WithKeyLock(context.Background(), filepath.Dir(path), "hook-failures", func(context.Context) error {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(append(line, '\n'))
		return err
	})
	if writeErr != nil {
		return hookfailurecontract.HookFailureRecordResult{OK: false, Path: path, Event: event}, writeErr
	}
	return hookfailurecontract.HookFailureRecordResult{OK: true, Path: path, Event: event}, nil
}

func ListHookFailureEvents(limit int) (hookfailurecontract.HookFailureListResult, error) {
	path := HookFailureLogPath()
	if limit <= 0 {
		limit = 20
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return hookfailurecontract.HookFailureListResult{OK: true, Path: path, Events: []hookfailurecontract.HookFailureEvent{}}, nil
	}
	if err != nil {
		return hookfailurecontract.HookFailureListResult{OK: false, Path: path}, err
	}
	defer f.Close()
	events := []hookfailurecontract.HookFailureEvent{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event hookfailurecontract.HookFailureEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return hookfailurecontract.HookFailureListResult{OK: false, Path: path, Events: events}, err
	}
	if len(events) > limit {
		events = events[len(events)-limit:]
	}
	return hookfailurecontract.HookFailureListResult{OK: true, Path: path, Events: events}, nil
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

func PruneHookFailureLog(maxAge time.Duration) (hookfailurecontract.HookFailurePruneResult, error) {
	path := HookFailureLogPath()
	result := hookfailurecontract.HookFailurePruneResult{OK: false, Path: path}

	// read+rewrite+rename을 동시 append와 직렬화한다. append도 같은
	// lock을 잡는다(RecordHookFailureEvent). 이게 없으면 read와 rename 사이에 예전
	// inode로 들어온 append가 rename이 그 inode를 unlink할 때 유실된다.
	lockErr := WithKeyLock(context.Background(), filepath.Dir(path), "hook-failures", func(context.Context) error {
		f, err := os.Open(path)
		if os.IsNotExist(err) {
			result.OK = true
			return nil
		}
		if err != nil {
			return err
		}

		cutoff := time.Now().UTC().Add(-maxAge)
		var kept []hookfailurecontract.HookFailureEvent
		pruned := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var event hookfailurecontract.HookFailureEvent
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
			return err
		}

		tmpPath := path + ".tmp"
		tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
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
		// Close를 writeErr에 합친다: Close에서만 드러나는 flush 실패(예: ENOSPC)는
		// rename을 중단시켜야 잘린 로그가 원본을 대체하지 않는다.
		if cerr := tmp.Close(); cerr != nil {
			writeErr = true
		}
		if writeErr {
			_ = os.Remove(tmpPath)
			return nil
		}

		if err := os.Rename(tmpPath, path); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}

		result.OK = true
		result.Pruned = pruned
		result.Kept = len(kept)
		return nil
	})
	if lockErr != nil {
		return result, lockErr
	}
	return result, nil
}
