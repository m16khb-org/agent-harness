package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func StateDoctor() (StateDoctorResult, error) {
	dir := StateDir()
	result := StateDoctorResult{
		OK:        false,
		Healthy:   false,
		StateDir:  dir,
		ValidKeys: []string{},
		Valid:     []StateListEntry{},
		Issues:    []StateDoctorIssue{},
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		result.OK = true
		result.Healthy = true
		return result, nil
	}
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)
		if entry.IsDir() {
			if isHarnessOwnedStateDirectory(name) {
				continue
			}
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     path,
				Severity: "warning",
				Code:     "unexpected_directory",
				Message:  "state directory contains an unexpected subdirectory",
			})
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			if isHarnessOwnedStateFile(name) {
				continue
			}
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     path,
				Severity: "warning",
				Code:     "unexpected_file",
				Message:  "state directory contains a non-json file",
			})
			continue
		}
		result.Checked++
		key := strings.TrimSuffix(name, ".json")
		if _, err := NormalizeStateKey(key); err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "invalid_filename",
				Message:  err.Error(),
			})
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "read_error",
				Message:  err.Error(),
			})
			continue
		}
		var record StateRecord
		if err := json.Unmarshal(b, &record); err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "invalid_json",
				Message:  err.Error(),
			})
			continue
		}
		recordFatalIssues := []StateDoctorIssue{}
		recordWarnings := []StateDoctorIssue{}
		if record.Key != key {
			recordFatalIssues = append(recordFatalIssues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "key_mismatch",
				Message:  fmt.Sprintf("record key %q does not match filename key %q", record.Key, key),
			})
		}
		if record.Bytes != len([]byte(record.Content)) {
			recordFatalIssues = append(recordFatalIssues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "byte_count_mismatch",
				Message:  fmt.Sprintf("record bytes=%d but content is %d bytes", record.Bytes, len([]byte(record.Content))),
			})
		}
		if _, err := parseStateTime(record.UpdatedAt); err != nil {
			recordFatalIssues = append(recordFatalIssues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "invalid_timestamp",
				Message:  err.Error(),
			})
		}
		switch {
		case record.SchemaVersion == 0:
			recordWarnings = append(recordWarnings, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "warning",
				Code:     "legacy_schema",
				Message:  fmt.Sprintf("record has legacy schema version 0; migrate to schema version %d", StateCurrentSchemaVersion),
			})
		case record.SchemaVersion < 0 || record.SchemaVersion > StateCurrentSchemaVersion:
			recordFatalIssues = append(recordFatalIssues, StateDoctorIssue{
				Path:     path,
				Key:      key,
				Severity: "error",
				Code:     "unsupported_schema",
				Message:  fmt.Sprintf("record schema version %d is unsupported; current schema version is %d", record.SchemaVersion, StateCurrentSchemaVersion),
			})
		}
		if len(recordFatalIssues) > 0 {
			result.Issues = append(result.Issues, recordFatalIssues...)
			continue
		}
		result.Issues = append(result.Issues, recordWarnings...)
		result.Valid = append(result.Valid, StateListEntry{
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
	return result, nil
}

func isHarnessOwnedStateDirectory(name string) bool {
	switch name {
	case "projects", "daemon", "worker", "issueops", "issueops-benchmarks":
		return true
	default:
		return false
	}
}

func isHarnessOwnedStateFile(name string) bool {
	switch name {
	case hookFailureLogFile:
		return true
	default:
		return false
	}
}
