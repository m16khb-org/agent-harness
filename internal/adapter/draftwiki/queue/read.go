package queue

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/domain/policy"
)

func CountLines(path string, limit int) (int, error) {
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

func ReadEvents(path string) ([]Event, []string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []Event{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	events := []Event{}
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
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			warnings = append(warnings, FormatMalformedWarning(lineNumber, line))
			continue
		}
		events = append(events, event)
	}
	return events, warnings, scanner.Err()
}

func FormatMalformedWarning(lineNumber int, line string) string {
	line = policy.RedactFreeform(line)
	const maxLineBytes = 120
	if len([]byte(line)) > maxLineBytes {
		line = string([]byte(line)[:maxLineBytes]) + "...[truncated]"
	}
	return fmt.Sprintf("malformed JSONL line %d skipped: %s", lineNumber, line)
}
