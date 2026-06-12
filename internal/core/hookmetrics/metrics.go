// Package hookmetrics records and aggregates per-event hook telemetry
// (quality program Q2 phase 2): hooks run on every tool call with a 5s
// budget, so latency and gate-hit rates must be observable to catch
// regressions and enforcement blindness.
package hookmetrics

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	corestate "agent-harness/internal/core/state"
)

const hookMetricsLogFile = "hook-metrics.jsonl"

type HookMetricEvent struct {
	Timestamp  string `json:"timestamp,omitempty"`
	Hook       string `json:"hook"`
	Host       string `json:"host,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	// Decision is set by enforcement gates when they block ("block"); empty
	// for ordinary completions.
	Decision string `json:"decision,omitempty"`
}

type HookMetricRecordResult struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

type HookLatencyStats struct {
	Count  int   `json:"count"`
	P50MS  int64 `json:"p50_ms"`
	P95MS  int64 `json:"p95_ms"`
	MaxMS  int64 `json:"max_ms"`
	Blocks int   `json:"blocks"`
}

type HookMetricsStats struct {
	OK      bool                        `json:"ok"`
	Path    string                      `json:"path"`
	Total   int                         `json:"total"`
	ByHook  map[string]HookLatencyStats `json:"by_hook,omitempty"`
	Last24h int                         `json:"last_24h"`
}

type HookMetricsPruneResult struct {
	OK     bool   `json:"ok"`
	Path   string `json:"path"`
	Pruned int    `json:"pruned"`
	Kept   int    `json:"kept"`
}

func HookMetricsLogPath() string {
	return filepath.Join(corestate.StateDir(), hookMetricsLogFile)
}

func RecordHookMetricEvent(event HookMetricEvent) (HookMetricRecordResult, error) {
	path := HookMetricsLogPath()
	result := HookMetricRecordResult{OK: false, Path: path}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return result, err
	}
	line, err := json.Marshal(event)
	if err != nil {
		return result, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return result, err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return result, err
	}
	result.OK = true
	return result, nil
}

func SummarizeHookMetricsLog() (HookMetricsStats, error) {
	path := HookMetricsLogPath()
	stats := HookMetricsStats{OK: true, Path: path, ByHook: map[string]HookLatencyStats{}}

	events, err := readHookMetricEvents(path)
	if os.IsNotExist(err) {
		return stats, nil
	}
	if err != nil {
		stats.OK = false
		return stats, err
	}

	now := time.Now().UTC()
	durations := map[string][]int64{}
	for _, event := range events {
		stats.Total++
		hook := event.Hook
		if hook == "" {
			hook = "unknown"
		}
		entry := stats.ByHook[hook]
		entry.Count++
		if event.Decision == "block" {
			entry.Blocks++
		}
		if event.DurationMS > entry.MaxMS {
			entry.MaxMS = event.DurationMS
		}
		stats.ByHook[hook] = entry
		durations[hook] = append(durations[hook], event.DurationMS)
		if ts, err := time.Parse(time.RFC3339Nano, event.Timestamp); err == nil && now.Sub(ts) <= 24*time.Hour {
			stats.Last24h++
		}
	}
	for hook, ds := range durations {
		slices.Sort(ds)
		entry := stats.ByHook[hook]
		entry.P50MS = percentileMS(ds, 50)
		entry.P95MS = percentileMS(ds, 95)
		stats.ByHook[hook] = entry
	}
	return stats, nil
}

func PruneHookMetricsLog(maxAge time.Duration) (HookMetricsPruneResult, error) {
	path := HookMetricsLogPath()
	result := HookMetricsPruneResult{OK: false, Path: path}

	events, err := readHookMetricEvents(path)
	if os.IsNotExist(err) {
		result.OK = true
		return result, nil
	}
	if err != nil {
		return result, err
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	kept := make([]HookMetricEvent, 0, len(events))
	for _, event := range events {
		ts, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err == nil && ts.Before(cutoff) {
			result.Pruned++
			continue
		}
		kept = append(kept, event)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+hookMetricsLogFile+"-*.tmp")
	if err != nil {
		return result, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	for _, event := range kept {
		line, err := json.Marshal(event)
		if err != nil {
			_ = tmp.Close()
			return result, err
		}
		if _, err := tmp.Write(append(line, '\n')); err != nil {
			_ = tmp.Close()
			return result, err
		}
	}
	if err := tmp.Close(); err != nil {
		return result, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return result, err
	}
	result.OK = true
	result.Kept = len(kept)
	return result, nil
}

func readHookMetricEvents(path string) ([]HookMetricEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []HookMetricEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event HookMetricEvent
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

func percentileMS(sorted []int64, pct int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := max((len(sorted)*pct+99)/100, 1)
	if idx > len(sorted) {
		idx = len(sorted)
	}
	return sorted[idx-1]
}
