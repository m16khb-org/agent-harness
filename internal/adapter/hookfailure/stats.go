package hookfailure

import (
	hookfailurecontract "agent-harness/internal/contract/hookfailure"
	"bufio"
	"encoding/json"
	"os"
	"time"
)

func SummarizeHookFailureLog() (hookfailurecontract.HookFailureStats, error) {
	path := HookFailureLogPath()
	stats := hookfailurecontract.HookFailureStats{OK: true, Path: path, ByHook: map[string]int{}}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return stats, nil
	}
	if err != nil {
		stats.OK = false
		return stats, err
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil {
		stats.SizeBytes = info.Size()
	}

	now := time.Now().UTC()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event hookfailurecontract.HookFailureEvent
		if json.Unmarshal(line, &event) != nil {
			// Count unparseable lines so interleaved/corrupt entries (the
			// documented PIPE_BUF risk) stay visible instead of vanishing.
			stats.Total++
			stats.ByHook["unparseable"]++
			continue
		}
		stats.Total++
		hook := event.Hook
		if hook == "" {
			hook = "unknown"
		}
		stats.ByHook[hook]++
		ts, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil {
			continue
		}
		if stats.Oldest == "" || event.Timestamp < stats.Oldest {
			stats.Oldest = event.Timestamp
		}
		if event.Timestamp > stats.Newest {
			stats.Newest = event.Timestamp
		}
		age := now.Sub(ts)
		if age <= 24*time.Hour {
			stats.Last24h++
		}
		if age <= 7*24*time.Hour {
			stats.Last7d++
		}
	}
	if err := scanner.Err(); err != nil {
		stats.OK = false
		return stats, err
	}
	return stats, nil
}
