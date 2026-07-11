package issueops

import (
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/core/state"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
)

// issueOpsBucket is the sqlstore bucket holding one row per cycle record.
const issueOpsBucket = "issueops"

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	record, err := readIssueOpsUnchecked(stateRoot, id)
	if err != nil {
		return record, err
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		record.OK = false
		return record, err
	}
	return record, nil
}

func readIssueOpsUnchecked(stateRoot, id string) (IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if !ok {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	var record IssueOpsRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: record has %q", record.ID)
	}
	if err := normalizeIssueOpsSchemaVersion(&record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	record.OK = true
	return record, nil
}

// ListIssueOpsIDs returns every cycle id stored under stateRoot in ascending
// order.
func ListIssueOpsIDs(stateRoot string) ([]string, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	return db.List(issueOpsBucket)
}

// deleteIssueOps removes the cycle record for id; deleting an absent record is
// not an error.
func deleteIssueOps(stateRoot, id string) error {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Delete(issueOpsBucket, id)
}

func IssueOpsStateRoot() string {
	return filepath.Join(state.StateDir(), "issueops")
}

func newIssueOpsID(repo, branch string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(branch)))
	return "io-" + hex.EncodeToString(sum[:])[:12]
}

func NewIssueOpsID(repo, branch string) string {
	return newIssueOpsID(repo, branch)
}

func touchAndWriteIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return writeIssueOps(stateRoot, record)
}

func writeIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, err
	}
	if err := normalizeIssueOpsSchemaVersion(&record); err != nil {
		record.OK = false
		return record, err
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		record.OK = false
		return record, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		record.OK = false
		return record, err
	}
	record.OK = true
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		record.OK = false
		return record, err
	}
	if err := db.Put(issueOpsBucket, record.ID, b); err != nil {
		record.OK = false
		return record, err
	}
	return record, nil
}

func WriteIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	return writeIssueOps(stateRoot, record)
}

func normalizeIssueOpsID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("id is required")
	}
	if !strings.HasPrefix(id, "io-") {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	if strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("invalid issueops id %q", id)
	}
	return id, nil
}

func normalizeIssueOpsSchemaVersion(record *IssueOpsRecord) error {
	switch {
	case record.SchemaVersion == 0:
		record.SchemaVersion = IssueOpsCurrentSchemaVersion
		return nil
	case record.SchemaVersion == IssueOpsCurrentSchemaVersion:
		return nil
	case record.SchemaVersion > IssueOpsCurrentSchemaVersion:
		return fmt.Errorf("unsupported issueops schema_version %d; current is %d", record.SchemaVersion, IssueOpsCurrentSchemaVersion)
	default:
		return fmt.Errorf("unsupported issueops schema_version %d", record.SchemaVersion)
	}
}
