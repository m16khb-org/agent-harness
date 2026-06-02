package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var stateKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

const StateCurrentSchemaVersion = 1

type StateRecord struct {
	SchemaVersion int    `json:"schema_version,omitempty"`
	Key           string `json:"key"`
	Content       string `json:"content"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
}

type StateResult struct {
	OK       bool        `json:"ok"`
	StateDir string      `json:"state_dir"`
	Path     string      `json:"path,omitempty"`
	Record   StateRecord `json:"record"`
}

type StateListEntry struct {
	Key           string `json:"key"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
	SchemaVersion int    `json:"schema_version"`
}

type StateListResult struct {
	OK       bool             `json:"ok"`
	StateDir string           `json:"state_dir"`
	Keys     []string         `json:"keys"`
	Records  []StateListEntry `json:"records"`
}

type StatePruneResult struct {
	OK          bool             `json:"ok"`
	StateDir    string           `json:"state_dir"`
	MaxAge      string           `json:"max_age"`
	Cutoff      string           `json:"cutoff"`
	Confirm     bool             `json:"confirm"`
	DryRun      bool             `json:"dry_run"`
	DeletedKeys []string         `json:"deleted_keys"`
	Pruned      []StateListEntry `json:"pruned"`
	KeptKeys    []string         `json:"kept_keys"`
	Kept        []StateListEntry `json:"kept"`
}

type StateDoctorIssue struct {
	Path     string `json:"path"`
	Key      string `json:"key,omitempty"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type StateDoctorResult struct {
	OK        bool               `json:"ok"`
	Healthy   bool               `json:"healthy"`
	StateDir  string             `json:"state_dir"`
	Checked   int                `json:"checked"`
	ValidKeys []string           `json:"valid_keys"`
	Valid     []StateListEntry   `json:"valid"`
	Issues    []StateDoctorIssue `json:"issues"`
}

type StateMigrateResult struct {
	OK            bool               `json:"ok"`
	StateDir      string             `json:"state_dir"`
	FromSchema    int                `json:"from_schema"`
	ToSchema      int                `json:"to_schema"`
	Confirm       bool               `json:"confirm"`
	DryRun        bool               `json:"dry_run"`
	CandidateKeys []string           `json:"candidate_keys"`
	Candidates    []StateListEntry   `json:"candidates"`
	MigratedKeys  []string           `json:"migrated_keys"`
	SkippedKeys   []string           `json:"skipped_keys"`
	Skipped       []StateListEntry   `json:"skipped"`
	Issues        []StateDoctorIssue `json:"issues"`
}

func StateWrite(key, content string) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return StateResult{OK: false, StateDir: dir}, err
	}
	record := StateRecord{
		SchemaVersion: StateCurrentSchemaVersion,
		Key:           key,
		Content:       content,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Bytes:         len([]byte(content)),
	}
	path, err := writeStateRecord(dir, key, record)
	if err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	return StateResult{OK: true, StateDir: dir, Path: path, Record: record}, nil
}

func StateRead(key string) (StateResult, error) {
	key, err := NormalizeStateKey(key)
	if err != nil {
		return StateResult{OK: false, StateDir: StateDir()}, err
	}
	dir := StateDir()
	path := statePath(dir, key)
	b, err := os.ReadFile(path)
	if err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	var record StateRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return StateResult{OK: false, StateDir: dir, Path: path}, err
	}
	if record.Key != key {
		return StateResult{OK: false, StateDir: dir, Path: path}, fmt.Errorf("state key mismatch: file has %q", record.Key)
	}
	if record.Bytes != len([]byte(record.Content)) {
		return StateResult{OK: false, StateDir: dir, Path: path}, fmt.Errorf("state byte count mismatch for %q", key)
	}
	if record.SchemaVersion < 0 || record.SchemaVersion > StateCurrentSchemaVersion {
		return StateResult{OK: false, StateDir: dir, Path: path}, fmt.Errorf("unsupported state schema version %d for %q", record.SchemaVersion, key)
	}
	return StateResult{OK: true, StateDir: dir, Path: path, Record: record}, nil
}

func StateList() (StateListResult, error) {
	dir := StateDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return StateListResult{OK: true, StateDir: dir, Keys: []string{}, Records: []StateListEntry{}}, nil
	}
	if err != nil {
		return StateListResult{OK: false, StateDir: dir}, err
	}
	records := []StateListEntry{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		if _, err := NormalizeStateKey(key); err != nil {
			continue
		}
		result, err := StateRead(key)
		if err != nil {
			continue
		}
		records = append(records, StateListEntry{
			Key:           result.Record.Key,
			UpdatedAt:     result.Record.UpdatedAt,
			Bytes:         result.Record.Bytes,
			SchemaVersion: result.Record.SchemaVersion,
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })
	keys := make([]string, 0, len(records))
	for _, record := range records {
		keys = append(keys, record.Key)
	}
	return StateListResult{OK: true, StateDir: dir, Keys: keys, Records: records}, nil
}

func StatePrune(maxAge time.Duration, confirm bool) (StatePruneResult, error) {
	dir := StateDir()
	result := StatePruneResult{
		OK:          false,
		StateDir:    dir,
		MaxAge:      maxAge.String(),
		Confirm:     confirm,
		DryRun:      !confirm,
		DeletedKeys: []string{},
		Pruned:      []StateListEntry{},
		KeptKeys:    []string{},
		Kept:        []StateListEntry{},
	}
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	list, err := StateList()
	if err != nil {
		return result, err
	}
	for _, record := range list.Records {
		updatedAt, err := parseStateTime(record.UpdatedAt)
		if err != nil || updatedAt.IsZero() || !updatedAt.Before(cutoff) {
			result.Kept = append(result.Kept, record)
			result.KeptKeys = append(result.KeptKeys, record.Key)
			continue
		}
		result.Pruned = append(result.Pruned, record)
		result.DeletedKeys = append(result.DeletedKeys, record.Key)
		if confirm {
			if err := os.Remove(statePath(dir, record.Key)); err != nil && !os.IsNotExist(err) {
				return result, err
			}
		}
	}
	sort.Strings(result.DeletedKeys)
	sort.Strings(result.KeptKeys)
	sort.Slice(result.Pruned, func(i, j int) bool { return result.Pruned[i].Key < result.Pruned[j].Key })
	sort.Slice(result.Kept, func(i, j int) bool { return result.Kept[i].Key < result.Kept[j].Key })
	result.OK = true
	return result, nil
}

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
			if name == "projects" || name == "daemon" || name == "worker" || name == "issueops" {
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

func StateMigrate(confirm bool) (StateMigrateResult, error) {
	dir := StateDir()
	result := StateMigrateResult{
		OK:            false,
		StateDir:      dir,
		FromSchema:    0,
		ToSchema:      StateCurrentSchemaVersion,
		Confirm:       confirm,
		DryRun:        !confirm,
		CandidateKeys: []string{},
		Candidates:    []StateListEntry{},
		MigratedKeys:  []string{},
		SkippedKeys:   []string{},
		Skipped:       []StateListEntry{},
		Issues:        []StateDoctorIssue{},
	}
	doctor, err := StateDoctor()
	if err != nil {
		return result, err
	}
	for _, issue := range doctor.Issues {
		if issue.Code != "legacy_schema" {
			result.Issues = append(result.Issues, issue)
		}
	}
	for _, entry := range doctor.Valid {
		read, err := StateRead(entry.Key)
		if err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     statePath(dir, entry.Key),
				Key:      entry.Key,
				Severity: "error",
				Code:     "read_error",
				Message:  err.Error(),
			})
			continue
		}
		if read.Record.SchemaVersion != 0 {
			result.Skipped = append(result.Skipped, entry)
			result.SkippedKeys = append(result.SkippedKeys, entry.Key)
			continue
		}
		result.CandidateKeys = append(result.CandidateKeys, entry.Key)
		result.Candidates = append(result.Candidates, entry)
		if !confirm {
			continue
		}
		migrated := read.Record
		migrated.SchemaVersion = StateCurrentSchemaVersion
		if _, err := writeStateRecord(dir, migrated.Key, migrated); err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     statePath(dir, migrated.Key),
				Key:      migrated.Key,
				Severity: "error",
				Code:     "write_error",
				Message:  err.Error(),
			})
			continue
		}
		result.MigratedKeys = append(result.MigratedKeys, migrated.Key)
	}
	sort.Strings(result.CandidateKeys)
	sort.Strings(result.MigratedKeys)
	sort.Strings(result.SkippedKeys)
	sort.Slice(result.Candidates, func(i, j int) bool { return result.Candidates[i].Key < result.Candidates[j].Key })
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Key < result.Skipped[j].Key })
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
	return result, nil
}

func parseStateTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("empty state timestamp")
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func NormalizeStateKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("state key is required")
	}
	if strings.Contains(key, "..") || strings.ContainsAny(key, `/\`) || !stateKeyRe.MatchString(key) {
		return "", fmt.Errorf("invalid state key %q; use [A-Za-z0-9._-] without path separators or '..', max 128 chars", key)
	}
	return key, nil
}

func StateDir() string {
	if env := os.Getenv("HARNESS_STATE_DIR"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "agent-harness-state")
	}
	return filepath.Join(home, ".local", "state", "agent-harness")
}

func statePath(dir, key string) string {
	return filepath.Join(dir, key+".json")
}

func writeStateRecord(dir, key string, record StateRecord) (string, error) {
	path := statePath(dir, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path, err
	}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return path, err
	}
	tmp, err := os.CreateTemp(dir, "."+key+"-*.tmp")
	if err != nil {
		return path, err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(b); err != nil {
			return err
		}
		if _, err := tmp.Write([]byte{'\n'}); err != nil {
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return path, writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return path, err
	}
	return path, nil
}
