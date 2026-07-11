package handoff

import (
	"fmt"
	"sort"
	"strings"

	"agent-harness/internal/port"
)

const (
	MaxBaselineIDs             = port.OrcaMaxBaselineIDs
	MaxExternalIDBytes         = 256
	MaxWorktreeBaselineIDBytes = 8192
	MaxBaselineTotalBytes      = 256 * 1024
)

func CanonicalBaselineIDs(kind string, values []string) ([]string, error) {
	seen := map[string]struct{}{}
	canonical := make([]string, 0, len(values))
	maxIDBytes, err := baselineIDByteLimit(kind)
	if err != nil {
		return nil, err
	}
	totalBytes := 0
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(value) > maxIDBytes {
			return nil, fmt.Errorf("%s baseline id exceeds %d bytes", kind, maxIDBytes)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		canonical = append(canonical, value)
		totalBytes += len(value)
		if totalBytes > MaxBaselineTotalBytes {
			return nil, fmt.Errorf("%s baseline exceeds %d total bytes", kind, MaxBaselineTotalBytes)
		}
	}
	if len(canonical) > MaxBaselineIDs {
		return nil, fmt.Errorf("%s baseline exceeds %d ids", kind, MaxBaselineIDs)
	}
	if len(canonical) == 0 {
		return nil, nil
	}
	sort.Strings(canonical)
	return canonical, nil
}

func RequireBaselineDeltaHeadroom(kind string, values []string) error {
	if len(values) >= MaxBaselineIDs {
		return fmt.Errorf("%s baseline has no delta headroom below the %d-id observation limit", kind, MaxBaselineIDs)
	}
	return nil
}

func baselineIDByteLimit(kind string) (int, error) {
	switch strings.TrimSpace(kind) {
	case "worktree":
		return MaxWorktreeBaselineIDBytes, nil
	case "terminal", "task":
		return MaxExternalIDBytes, nil
	default:
		return 0, fmt.Errorf("unknown baseline kind %q", kind)
	}
}
