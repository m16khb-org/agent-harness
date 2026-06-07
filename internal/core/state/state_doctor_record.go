package state

import (
	"encoding/json"
	"fmt"
	"os"

	"agent-harness/internal/core/state/statepath"
)

type stateDoctorRecordInspection struct {
	Valid  bool
	Record StateRecord
	Issues []StateDoctorIssue
}

func inspectStateDoctorRecord(path, key string) stateDoctorRecordInspection {
	if _, err := NormalizeStateKey(key); err != nil {
		return stateDoctorRecordInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "invalid_filename",
			Message:  err.Error(),
		}}}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return stateDoctorRecordInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "read_error",
			Message:  err.Error(),
		}}}
	}
	var record StateRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return stateDoctorRecordInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "invalid_json",
			Message:  err.Error(),
		}}}
	}
	return validateStateDoctorRecord(path, key, record)
}

func validateStateDoctorRecord(path, key string, record StateRecord) stateDoctorRecordInspection {
	fatalIssues := []StateDoctorIssue{}
	warnings := []StateDoctorIssue{}
	if record.Key != key {
		fatalIssues = append(fatalIssues, StateDoctorIssue{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "key_mismatch",
			Message:  fmt.Sprintf("record key %q does not match filename key %q", record.Key, key),
		})
	}
	if record.Bytes != len([]byte(record.Content)) {
		fatalIssues = append(fatalIssues, StateDoctorIssue{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "byte_count_mismatch",
			Message:  fmt.Sprintf("record bytes=%d but content is %d bytes", record.Bytes, len([]byte(record.Content))),
		})
	}
	if _, err := statepath.ParseTime(record.UpdatedAt); err != nil {
		fatalIssues = append(fatalIssues, StateDoctorIssue{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "invalid_timestamp",
			Message:  err.Error(),
		})
	}
	switch {
	case record.SchemaVersion == 0:
		warnings = append(warnings, StateDoctorIssue{
			Path:     path,
			Key:      key,
			Severity: "warning",
			Code:     "legacy_schema",
			Message:  fmt.Sprintf("record has legacy schema version 0; migrate to schema version %d", StateCurrentSchemaVersion),
		})
	case record.SchemaVersion < 0 || record.SchemaVersion > StateCurrentSchemaVersion:
		fatalIssues = append(fatalIssues, StateDoctorIssue{
			Path:     path,
			Key:      key,
			Severity: "error",
			Code:     "unsupported_schema",
			Message:  fmt.Sprintf("record schema version %d is unsupported; current schema version is %d", record.SchemaVersion, StateCurrentSchemaVersion),
		})
	}
	if len(fatalIssues) > 0 {
		return stateDoctorRecordInspection{Issues: fatalIssues}
	}
	return stateDoctorRecordInspection{Valid: true, Record: record, Issues: warnings}
}
