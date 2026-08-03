package state

import (
	"sort"
	"strings"

	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/domain/statepath"
)

const storeMaintainSentinel = ".last-store-maintain"

type DoctorEntry struct {
	Name  string
	Path  string
	IsDir bool
}

type DoctorRow struct {
	Key  string
	Path string
	Data []byte
}

func Doctor(dir string, entries []DoctorEntry, rows []DoctorRow) statecontract.StateDoctorResult {
	result := statecontract.StateDoctorResult{
		OK:        false,
		Healthy:   false,
		StateDir:  dir,
		ValidKeys: []string{},
		Valid:     []statecontract.StateListEntry{},
		Issues:    []statecontract.StateDoctorIssue{},
	}
	for _, entry := range entries {
		if issue, ok := inspectDoctorEntry(entry); ok {
			result.Issues = append(result.Issues, issue)
		}
	}
	for _, row := range rows {
		result.Checked++
		record, issue := inspectDoctorRecord(row)
		if issue != nil {
			result.Issues = append(result.Issues, *issue)
			continue
		}
		result.Valid = append(result.Valid, statecontract.StateListEntry{
			Key:           record.Key,
			UpdatedAt:     record.UpdatedAt,
			Bytes:         record.Bytes,
			SchemaVersion: record.SchemaVersion,
		})
		result.ValidKeys = append(result.ValidKeys, record.Key)
	}
	sort.Strings(result.ValidKeys)
	sort.Slice(result.Valid, func(i, j int) bool { return result.Valid[i].Key < result.Valid[j].Key })
	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Path != result.Issues[j].Path {
			return result.Issues[i].Path < result.Issues[j].Path
		}
		if result.Issues[i].Code != result.Issues[j].Code {
			return result.Issues[i].Code < result.Issues[j].Code
		}
		return result.Issues[i].Message < result.Issues[j].Message
	})
	result.OK = true
	result.Healthy = len(result.Issues) == 0
	return result
}

func inspectDoctorEntry(entry DoctorEntry) (statecontract.StateDoctorIssue, bool) {
	if entry.IsDir {
		if harnessOwnedStateDirectory(entry.Name) {
			return statecontract.StateDoctorIssue{}, false
		}
		return statecontract.StateDoctorIssue{Path: entry.Path, Severity: "warning", Code: "unexpected_directory", Message: "state directory contains an unexpected subdirectory"}, true
	}
	if harnessOwnedStateFile(entry.Name) {
		return statecontract.StateDoctorIssue{}, false
	}
	return statecontract.StateDoctorIssue{Path: entry.Path, Severity: "warning", Code: "unexpected_file", Message: "state directory contains an unexpected file"}, true
}

func inspectDoctorRecord(row DoctorRow) (statecontract.RecordEnvelope, *statecontract.StateDoctorIssue) {
	if _, err := statepath.NormalizeKey(row.Key); err != nil {
		return statecontract.RecordEnvelope{}, &statecontract.StateDoctorIssue{Path: row.Path, Key: row.Key, Severity: "error", Code: "invalid_key", Message: err.Error()}
	}
	record, err := DecodeRecord(row.Key, row.Data)
	if err == nil {
		_, err = statepath.ParseTime(record.UpdatedAt)
	}
	if err != nil {
		return statecontract.RecordEnvelope{}, &statecontract.StateDoctorIssue{Path: row.Path, Key: row.Key, Severity: "error", Code: "invalid_state", Message: "invalid state"}
	}
	return record, nil
}

func harnessOwnedStateDirectory(name string) bool {
	switch name {
	case "projects", "daemon", "worker", "loop", "issueops-benchmarks", "issueops_v1", "native-activation", "audit":
		return true
	default:
		return false
	}
}

func harnessOwnedStateFile(name string) bool {
	for _, base := range []string{"harness.db", "harness.lock.db"} {
		if name == base || strings.HasPrefix(name, base+"-") {
			return true
		}
	}
	switch name {
	case statecontract.HookFailureLogFile, "hook-metrics.jsonl", storeMaintainSentinel:
		return true
	default:
		return false
	}
}
