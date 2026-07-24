package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/core/state"
)

// 현재 schema는 legacy row를 해석하지 않도록 물리 namespace까지 분리한다.
// namespace 이름은 model.IssueOpsSchemaVersion에서 파생된다.
var issueOpsBucket = fmt.Sprintf("issueops_v%d", model.IssueOpsSchemaVersion)

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
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
func ReadIssueOpsForStopSuppression(stateRoot, id string) (IssueOpsRecord, error) {
	return ReadIssueOpsExisting(stateRoot, id)
}

// ReadIssueOpsExisting reads exactly one existing record without creating,
// repairing, migrating, or changing permissions on the state store.
func ReadIssueOpsExisting(stateRoot, id string) (IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	b, ok, err := sqlstore.GetExisting(stateRoot, issueOpsBucket, id)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if !ok {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
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
	return decodeIssueOpsRecord(id, b)
}

func decodeIssueOpsRecord(id string, b []byte) (IssueOpsRecord, error) {
	var header struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
	}
	if err := json.Unmarshal(b, &header); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if header.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: record has %q", header.ID)
	}
	if schemaErr := issueOpsSchemaVersionError(header.SchemaVersion); schemaErr != nil {
		record, projectionErr := decodeInvalidIssueOpsProjection(b)
		if projectionErr != nil {
			return IssueOpsRecord{OK: false, ID: id}, projectionErr
		}
		record.OK = false
		record.Invalid = true
		record.InvalidReason = boundedIssueOpsInvalidReason(schemaErr.Error())
		return record, schemaErr
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	for _, name := range []string{"execution_handoff", "execution_workspace", "ownership", "remote_create_claim"} {
		raw := fields[name]
		if rawIssueOpsAuthorityPresent(raw) {
			return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("legacy execution authority %s is forbidden in IssueOps v1", name)
		}
	}
	var record IssueOpsRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: record has %q", record.ID)
	}
	record.OK = true
	return record, nil
}

func rawIssueOpsAuthorityPresent(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && !bytes.Equal(raw, []byte("null"))
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

// deleteIssueOps removes the cycle record for id; deleting an absent record is
// not an error.
func deleteIssueOps(stateRoot, id string) error {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return err
	}
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
	return db.Apply(context.Background(), []sqlstore.Mutation{
		{Bucket: artifactStageBucket, ID: id, Delete: true},
		{Bucket: issueOpsBucket, ID: id, Delete: true},
	})
}

func IssueOpsStateRoot() string {
	return filepath.Join(state.StateDir(), issueOpsBucket)
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
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		record.OK = false
		return record, err
	}
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

func encodeIssueOpsRecord(record IssueOpsRecord) (IssueOpsRecord, []byte, error) {
	if _, err := normalizeIssueOpsID(record.ID); err != nil {
		record.OK = false
		return record, nil, err
	}
	if err := normalizeIssueOpsSchemaVersion(&record); err != nil {
		record.OK = false
		return record, nil, err
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
	if record.SchemaVersion == 0 {
		// In-memory constructors may omit the field; every persisted row is still v1.
		record.SchemaVersion = model.IssueOpsSchemaVersion
	}
	return issueOpsSchemaVersionError(record.SchemaVersion)
}

func issueOpsSchemaVersionError(version int) error {
	if version == model.IssueOpsSchemaVersion {
		return nil
	}
	return fmt.Errorf("unsupported issueops schema_version %d; current is %d", version, model.IssueOpsSchemaVersion)
}

func validateIssueOpsRecord(record IssueOpsRecord) error {
	if record.SchemaVersion != model.IssueOpsSchemaVersion {
		return issueOpsSchemaVersionError(record.SchemaVersion)
	}
	if record.Execution != nil {
		return model.ValidateExecution(*record.Execution)
	}
	return nil
}

func decodeInvalidIssueOpsProjection(raw []byte) (IssueOpsRecord, error) {
	var projection struct {
		SchemaVersion int           `json:"schema_version"`
		ID            string        `json:"id"`
		Repo          string        `json:"repo"`
		Branch        string        `json:"branch"`
		Phase         IssueOpsPhase `json:"phase"`
		WorktreePath  string        `json:"worktree_path"`
	}
	if err := json.Unmarshal(raw, &projection); err != nil {
		return IssueOpsRecord{}, err
	}
	record := IssueOpsRecord{
		SchemaVersion: projection.SchemaVersion,
		ID:            boundedIssueOpsIdentity(projection.ID, 128),
		Repo:          boundedIssueOpsIdentity(projection.Repo, 4096),
		Branch:        boundedIssueOpsIdentity(projection.Branch, 1024),
		Phase:         projection.Phase,
		WorktreePath:  boundedIssueOpsIdentity(projection.WorktreePath, 4096),
	}
	return record, nil
}

func boundedIssueOpsIdentity(value string, limit int) string {
	if len(value) > limit || strings.ContainsRune(value, 0) {
		return ""
	}
	return strings.TrimSpace(value)
}

func boundedIssueOpsInvalidReason(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		return value[:253] + "..."
	}
	return value
}
