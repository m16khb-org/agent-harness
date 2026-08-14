package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/contract/issueops"
	statecontract "agent-harness/internal/contract/state"
	"agent-harness/internal/port"
)

// 현재 schema는 legacy row를 해석하지 않도록 물리 namespace까지 분리한다.
// namespace 이름은 issueops.IssueOpsSchemaVersion에서 파생된다.
var issueOpsBucket = fmt.Sprintf("issueops_v%d", issueops.IssueOpsSchemaVersion)

func ReadIssueOps(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	record, err := readIssueOpsUnchecked(stateRoot, id)
	if err != nil {
		return record, err
	}
	if err := validateIssueOpsRecord(record); err != nil {
		record.OK = false
		return record, err
	}
	return record, nil
}

// ReadIssueOpsForStopSuppression reads exactly one existing record through the
// bounded, non-creating sqlstore path used by the Stop-hook hot path.
func ReadIssueOpsForStopSuppression(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	return ReadIssueOpsExisting(stateRoot, id)
}

// ReadIssueOpsExisting reads exactly one existing record without creating,
// repairing, migrating, or changing permissions on the state store.
func ReadIssueOpsExisting(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	b, ok, err := sqlstore.GetExisting(stateRoot, issueOpsBucket, id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !ok {
		return issueops.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	record, err := decodeIssueOpsRecord(id, b)
	if err != nil {
		return record, err
	}
	if err := validateIssueOpsRecord(record); err != nil {
		record.OK = false
		return record, err
	}
	return record, nil
}

func readIssueOpsUnchecked(stateRoot, id string) (issueops.IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: id}, err
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false, ID: id}, err
	}
	if !ok {
		return issueops.IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	return decodeIssueOpsRecord(id, b)
}

func decodeIssueOpsRecord(id string, b []byte) (issueops.IssueOpsRecord, error) {
	invalid := issueops.IssueOpsRecord{OK: false, ID: id, Invalid: true, InvalidReason: statecontract.ErrInvalidState.Error()}
	var record issueops.IssueOpsRecord
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return invalid, statecontract.ErrInvalidState
	}
	if record.SchemaVersion != issueops.IssueOpsSchemaVersion || record.ID != id {
		return invalid, statecontract.ErrInvalidState
	}
	if err := validateIssueOpsRecord(record); err != nil {
		return invalid, statecontract.ErrInvalidState
	}
	record.OK = true
	return record, nil
}

// ListIssueOpsIDs returns every cycle id stored under stateRoot in ascending
// order.
func ListIssueOpsIDs(stateRoot string) ([]string, error) {
	ids, err := sqlstore.ListExisting(stateRoot, issueOpsBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func ScanIssueOps(stateRoot string) ([]issueops.IssueOpsRecord, error) {
	records, invalid, err := scanIssueOpsRows(stateRoot)
	if err != nil {
		return nil, err
	}
	if invalid {
		return nil, statecontract.ErrInvalidState
	}
	return records, nil
}

func ScanReadableIssueOps(stateRoot string) ([]issueops.IssueOpsRecord, error) {
	records, _, err := scanIssueOpsRows(stateRoot)
	return records, err
}

func scanIssueOpsRows(stateRoot string) ([]issueops.IssueOpsRecord, bool, error) {
	rows, err := sqlstore.GetAllExisting(stateRoot, issueOpsBucket)
	if errors.Is(err, fs.ErrNotExist) {
		return []issueops.IssueOpsRecord{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	records := make([]issueops.IssueOpsRecord, 0, len(rows))
	invalid := false
	for _, row := range rows {
		record, decodeErr := decodeIssueOpsRecord(row.ID, row.Data)
		if decodeErr != nil {
			invalid = true
			continue
		}
		records = append(records, record)
	}
	return records, invalid, nil
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
	// 스테이징 artifact는 레코드와 수명을 같이한다 — 레코드 삭제(prune,
	// cleanup finish)가 스테이지 blob을 고아로 남기지 않는다(C4a-F1 ②).
	return db.Apply(context.Background(), []port.RecordMutation{
		{Bucket: artifactStageBucket, ID: id, Delete: true},
		{Bucket: issueOpsBucket, ID: id, Delete: true},
	})
}

func IssueOpsStateRoot() string {
	return filepath.Join(StateDir(), issueOpsBucket)
}

func newIssueOpsID(repo, branch string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(branch)))
	return "io-" + hex.EncodeToString(sum[:])[:12]
}

func NewIssueOpsID(repo, branch string) string {
	return newIssueOpsID(repo, branch)
}

func touchAndWriteIssueOps(stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return writeIssueOps(stateRoot, record)
}

func writeIssueOps(stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	record, b, err := encodeIssueOpsRecord(record)
	if err != nil {
		return record, err
	}
	db, err := sqlstore.Open(stateRoot)
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

func encodeIssueOpsRecord(record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, []byte, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, nil, err
	}
	if record.SchemaVersion != issueops.IssueOpsSchemaVersion {
		record.OK = false
		return record, nil, statecontract.ErrInvalidState
	}
	if err := validateIssueOpsRecord(record); err != nil {
		record.OK = false
		return record, nil, err
	}
	record.OK = true
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		record.OK = false
		return record, nil, err
	}
	return record, b, nil
}

func WriteIssueOps(stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
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

func validateIssueOpsRecord(record issueops.IssueOpsRecord) error {
	if err := issueops.ValidateRecord(record); err != nil {
		return statecontract.ErrInvalidState
	}
	return nil
}
