package looprun

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	defaultMaxAttempts = 5
	maxMaxAttempts     = 50
	defaultLoopStatus  = "active"
)

func Start(req StartLoopRequest) (LoopRun, error) {
	repo, err := normalizeRepo(req.Repo)
	if err != nil {
		return LoopRun{OK: false}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return LoopRun{OK: false}, fmt.Errorf("name is required")
	}
	goal := redactFreeform(req.Goal)
	if goal == "" {
		return LoopRun{OK: false}, fmt.Errorf("goal is required")
	}
	maxAttempts, err := normalizeMaxAttempts(req.MaxAttempts)
	if err != nil {
		return LoopRun{OK: false}, err
	}
	loopID := newLoopID(repo, name)
	var loop LoopRun
	err = withLoopLock(loopID, func() error {
		existing, readErr := ReadLoop(loopID)
		if readErr == nil {
			if existing.Status == "active" {
				loop = existing
				return nil
			}
			return fmt.Errorf("loop_terminal")
		}
		if !errors.Is(readErr, fs.ErrNotExist) {
			return readErr
		}
		now := timestampNow()
		loop = LoopRun{
			OK:            true,
			SchemaVersion: LoopRunCurrentSchemaVersion,
			ID:            loopID,
			Repo:          repo,
			Name:          name,
			Goal:          goal,
			VerifyArgv:    cleanStrings(req.VerifyArgv),
			MaxAttempts:   maxAttempts,
			Status:        defaultLoopStatus,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		var writeErr error
		loop, writeErr = writeLoop(loop)
		return writeErr
	})
	return loop, err
}

func RecordAttempt(loopID string, req RecordAttemptRequest) (LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return LoopRun{OK: false}, err
	}
	verdict := strings.TrimSpace(req.Verdict)
	if verdict != "pass" && verdict != "fail" {
		return LoopRun{OK: false, ID: loopID}, fmt.Errorf("verdict_invalid")
	}
	evidence := redactStrings(cleanStrings(req.Evidence))
	if len(evidence) == 0 {
		return LoopRun{OK: false, ID: loopID}, fmt.Errorf("evidence_required")
	}
	var loop LoopRun
	err = withLoopLock(loopID, func() error {
		var readErr error
		loop, readErr = ReadLoop(loopID)
		if readErr != nil {
			return readErr
		}
		if loop.Status != "active" {
			return fmt.Errorf("loop_not_active")
		}
		now := timestampNow()
		loop.Attempts = append(loop.Attempts, LoopAttempt{
			Seq:      len(loop.Attempts) + 1,
			Verdict:  verdict,
			Evidence: evidence,
			At:       now,
		})
		if verdict == "fail" && len(loop.Attempts) >= loop.MaxAttempts {
			loop.Status = "exhausted"
		}
		loop.UpdatedAt = now
		var writeErr error
		loop, writeErr = writeLoop(loop)
		return writeErr
	})
	return loop, err
}

func Stop(loopID string, success bool, reason string) (LoopRun, error) {
	loopID, err := normalizeLoopID(loopID)
	if err != nil {
		return LoopRun{OK: false}, err
	}
	var loop LoopRun
	err = withLoopLock(loopID, func() error {
		var readErr error
		loop, readErr = ReadLoop(loopID)
		if readErr != nil {
			return readErr
		}
		if loop.Status == "succeeded" || loop.Status == "stopped" {
			return fmt.Errorf("loop_terminal")
		}
		now := timestampNow()
		if success {
			if len(loop.Attempts) == 0 || loop.Attempts[len(loop.Attempts)-1].Verdict != "pass" {
				return fmt.Errorf("loop_success_requires_pass")
			}
			loop.Status = "succeeded"
		} else {
			reason = redactFreeform(reason)
			if len(reason) < 10 {
				return fmt.Errorf("stop_reason_too_short")
			}
			loop.Status = "stopped"
			loop.StopReason = reason
		}
		loop.UpdatedAt = now
		var writeErr error
		loop, writeErr = writeLoop(loop)
		return writeErr
	})
	return loop, err
}

func Status(loopID string) (StatusResult, error) {
	loop, err := ReadLoop(loopID)
	if err != nil {
		return StatusResult{OK: false}, err
	}
	result := StatusResult{OK: true, Loop: loop, AttemptCount: len(loop.Attempts)}
	if len(loop.Attempts) > 0 {
		result.LastVerdict = loop.Attempts[len(loop.Attempts)-1].Verdict
	}
	return result, nil
}

func withLoopLock(loopID string, fn func() error) error {
	if _, err := normalizeLoopID(loopID); err != nil {
		return err
	}
	db, err := openStore()
	if err != nil {
		return err
	}
	return db.WithSpan(fn)
}

func normalizeRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func normalizeMaxAttempts(maxAttempts int) (int, error) {
	if maxAttempts == 0 {
		return defaultMaxAttempts, nil
	}
	if maxAttempts < 0 || maxAttempts > maxMaxAttempts {
		return 0, fmt.Errorf("max_attempts_invalid")
	}
	return maxAttempts, nil
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func redactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, redactFreeform(value))
	}
	return out
}

var secretAssignmentPattern = regexp.MustCompile(`(?i)\b(token|secret|password|api[_-]?key|access[_-]?key)\s*[:=]\s*["']?([^\s"',}]+)`)

func redactFreeform(value string) string {
	return strings.TrimSpace(secretAssignmentPattern.ReplaceAllString(value, "$1=<redacted>"))
}
