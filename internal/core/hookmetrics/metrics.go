// Package hookmetrics records and aggregates per-event hook telemetry
// (quality program Q2 phase 2): hooks run on every tool call with a 5s
// budget, so latency and gate-hit rates must be observable to catch
// regressions and enforcement blindness.
package hookmetrics

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"time"

	corestate "agent-harness/internal/adapter/outbound/state"
)

const (
	hookMetricsLogFile        = "hook-metrics.jsonl"
	defaultHookMetricsEntries = 10000
	defaultHookMetricsBytes   = 1 << 20
	defaultStaleTempAge       = time.Hour
)

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
	// Asks counts enforcement "ask" decisions; like Blocks they are real gate
	// interventions, so both feed GateHitRate (A2/G4).
	Asks int `json:"asks"`
	// GateHitRate = (Blocks+Asks)/Count: the fraction of invocations where the
	// gate actually intervened. A gate that is silently disabled keeps Count
	// rising while this drops to ~0, which absolute Blocks alone cannot reveal.
	GateHitRate float64 `json:"gate_hit_rate"`
}

type HookMetricsStats struct {
	OK     bool                        `json:"ok"`
	Path   string                      `json:"path"`
	Total  int                         `json:"total"`
	ByHook map[string]HookLatencyStats `json:"by_hook,omitempty"`
	// GateHitRate is the overall (Blocks+Asks)/Count across all hooks.
	GateHitRate float64 `json:"gate_hit_rate"`
	Last24h     int     `json:"last_24h"`
}

type HookMetricsPruneResult struct {
	OK               bool   `json:"ok"`
	Path             string `json:"path"`
	Pruned           int    `json:"pruned"`
	Kept             int    `json:"kept"`
	StaleTempRemoved int    `json:"stale_temp_removed,omitempty"`
}

type hookMetricsPruneLimits struct {
	MaxEntries   int
	MaxBytes     int64
	StaleTempAge time.Duration
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
	if err := corestate.WithKeyLock(context.Background(), filepath.Dir(path), "hook-metrics", func(context.Context) error {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(append(line, '\n'))
		return err
	}); err != nil {
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
		switch event.Decision {
		case "block":
			entry.Blocks++
		case "ask":
			entry.Asks++
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
	totalGateHits := 0
	for hook, ds := range durations {
		slices.Sort(ds)
		entry := stats.ByHook[hook]
		entry.P50MS = percentileMS(ds, 50)
		entry.P95MS = percentileMS(ds, 95)
		entry.GateHitRate = Rate(entry.Blocks+entry.Asks, entry.Count)
		stats.ByHook[hook] = entry
		totalGateHits += entry.Blocks + entry.Asks
	}
	stats.GateHitRate = Rate(totalGateHits, stats.Total)
	return stats, nil
}

func PruneHookMetricsLog(maxAge time.Duration) (HookMetricsPruneResult, error) {
	return pruneHookMetricsLog(maxAge, hookMetricsPruneLimits{
		MaxEntries:   defaultHookMetricsEntries,
		MaxBytes:     defaultHookMetricsBytes,
		StaleTempAge: defaultStaleTempAge,
	})
}

func pruneHookMetricsLog(maxAge time.Duration, limits hookMetricsPruneLimits) (HookMetricsPruneResult, error) {
	path := HookMetricsLogPath()
	result := HookMetricsPruneResult{OK: false, Path: path}
	err := corestate.WithKeyLock(context.Background(), filepath.Dir(path), "hook-metrics", func(context.Context) error {
		if removed, err := sweepStaleHookMetricTemps(filepath.Dir(path), limits.StaleTempAge); err != nil {
			return err
		} else {
			result.StaleTempRemoved = removed
		}

		events, err := readHookMetricEvents(path)
		if os.IsNotExist(err) {
			result.OK = true
			return nil
		}
		if err != nil {
			return err
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
		bounded, boundedPruned, err := boundHookMetricEvents(kept, limits)
		if err != nil {
			return err
		}
		kept = bounded
		result.Pruned += boundedPruned
		tmp, err := os.CreateTemp(filepath.Dir(path), "."+hookMetricsLogFile+"-*.tmp")
		if err != nil {
			return err
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		for _, event := range kept {
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
		if err := os.Rename(tmpName, path); err != nil {
			return err
		}
		result.OK = true
		result.Kept = len(kept)
		return nil
	})
	return result, err
}

func sweepStaleHookMetricTemps(dir string, staleAfter time.Duration) (int, error) {
	if staleAfter <= 0 {
		return 0, nil
	}
	matches, err := filepath.Glob(filepath.Join(dir, "."+hookMetricsLogFile+"-*.tmp"))
	if err != nil {
		return 0, err
	}
	now := time.Now()
	removed := 0
	for _, path := range matches {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return removed, err
		}
		if now.Sub(info.ModTime()) < staleAfter {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func boundHookMetricEvents(events []HookMetricEvent, limits hookMetricsPruneLimits) ([]HookMetricEvent, int, error) {
	original := len(events)
	if limits.MaxEntries > 0 && len(events) > limits.MaxEntries {
		events = events[len(events)-limits.MaxEntries:]
	}
	if limits.MaxBytes > 0 && len(events) > 0 {
		total := int64(0)
		start := len(events)
		for i := len(events) - 1; i >= 0; i-- {
			line, err := json.Marshal(events[i])
			if err != nil {
				return nil, 0, err
			}
			lineBytes := int64(len(line) + 1)
			if total+lineBytes > limits.MaxBytes && start < len(events) {
				break
			}
			total += lineBytes
			start = i
		}
		events = events[start:]
	}
	return events, original - len(events), nil
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

// Rate returns num/denom clamped to [0,1] and rounded to 4 decimals; denom<=0
// yields 0. The clamp guards the failure-rate join (A2/G5): a process that
// crashed between recording a failure (hook.go:25) and recording the invocation
// (hook.go:30) can transiently make failures exceed invocations, but a *_rate
// field must stay a valid [0,1] probability — the raw counts remain available
// alongside it for anomaly detection. gate_hit_rate cannot exceed 1 by
// construction (block+ask <= count), so the clamp is a no-op there.
func Rate(num, denom int) float64 {
	if denom <= 0 {
		return 0
	}
	r := float64(num) / float64(denom)
	if r > 1 {
		r = 1
	}
	return math.Round(r*10000) / 10000
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
