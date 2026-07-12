package issueops

import (
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/core/state"
	"bytes"
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

// ReadIssueOpsForStopSuppression reads exactly one existing record through the
// bounded, non-creating sqlstore path used by the Stop-hook hot path.
func ReadIssueOpsForStopSuppression(stateRoot, id string) (IssueOpsRecord, error) {
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
	return decodeIssueOpsRecord(id, b)
}

func readRawIssueOpsBytes(stateRoot, id string) ([]byte, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return nil, err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	b, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("issueops record %s: %w", id, fs.ErrNotExist)
	}
	if len(b) > 4<<20 {
		return nil, fmt.Errorf("issueops raw record exceeds bounded migration size")
	}
	return append([]byte(nil), b...), nil
}

func decodeIssueOpsRecord(id string, b []byte) (IssueOpsRecord, error) {
	var header struct {
		SchemaVersion     int             `json:"schema_version"`
		ID                string          `json:"id"`
		RemoteCreateClaim json.RawMessage `json:"remote_create_claim"`
		ExecutionHandoff  *struct {
			CoordinatorSession json.RawMessage `json:"coordinator_session"`
			PublishReceipt     json.RawMessage `json:"publish_receipt"`
		} `json:"execution_handoff"`
	}
	if err := json.Unmarshal(b, &header); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if header.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: record has %q", header.ID)
	}
	if header.SchemaVersion <= 5 && rawIssueOpsAuthorityPresent(header.RemoteCreateClaim) {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops schema_version %d cannot contain remote_create_claim durable mutation authority", header.SchemaVersion)
	}
	if header.SchemaVersion <= 5 && header.ExecutionHandoff != nil && rawIssueOpsAuthorityPresent(header.ExecutionHandoff.CoordinatorSession) {
		record, projectionErr := decodeInvalidIssueOpsProjection(b)
		if projectionErr != nil {
			return IssueOpsRecord{OK: false, ID: id}, projectionErr
		}
		record.OK = false
		record.Invalid = true
		record.InvalidReason = boundedIssueOpsInvalidReason(fmt.Sprintf("issueops schema_version %d cannot contain coordinator_session durable mutation authority", header.SchemaVersion))
		return record, fmt.Errorf("issueops schema_version %d cannot contain coordinator_session durable mutation authority", header.SchemaVersion)
	}
	if header.SchemaVersion <= 5 && header.ExecutionHandoff != nil && rawIssueOpsAuthorityPresent(header.ExecutionHandoff.PublishReceipt) {
		record, projectionErr := decodeInvalidIssueOpsProjection(b)
		if projectionErr != nil {
			return IssueOpsRecord{OK: false, ID: id}, projectionErr
		}
		record.OK = false
		record.Invalid = true
		record.InvalidReason = boundedIssueOpsInvalidReason(fmt.Sprintf("issueops schema_version %d publish receipt predates v6 push-target authority", header.SchemaVersion))
		return record, fmt.Errorf("issueops schema_version %d publish receipt predates v6 push-target authority; re-attest publication or use the dedicated remote-create reconcile path", header.SchemaVersion)
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
	var record IssueOpsRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: record has %q", record.ID)
	}
	if err := normalizeIssueOpsSchemaVersion(&record); err != nil {
		record.OK = false
		record.Invalid = true
		record.InvalidReason = boundedIssueOpsInvalidReason(err.Error())
		return record, err
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
	if err := issueOpsSchemaVersionError(record.SchemaVersion); err != nil {
		return err
	}
	legacyIdentitySchema := record.SchemaVersion == 0 || record.SchemaVersion == 1 || record.SchemaVersion == 2 || record.SchemaVersion == 3
	legacySchema := legacyIdentitySchema || record.SchemaVersion == 4 || record.SchemaVersion == 5
	if legacySchema {
		record.SchemaVersion = IssueOpsCurrentSchemaVersion
		if legacyIdentitySchema && record.ExecutionHandoff != nil {
			migrateLegacyIssueOpsOrcaIdentity(record.ExecutionHandoff.Orca)
			for i := range record.ExecutionHandoff.PriorAttempts {
				migrateLegacyIssueOpsOrcaIdentity(record.ExecutionHandoff.PriorAttempts[i].Orca)
			}
		}
	}
	return nil
}

func migrateLegacyIssueOpsOrcaIdentity(identity *IssueOpsOrcaIdentity) {
	if identity == nil {
		return
	}
	if identity.WorkerTerminalHandle == "" {
		identity.WorkerTerminalHandle = identity.WorkerMailboxHandle
	}
	if identity.DispatchID == "" {
		identity.WorkerMailboxHandle = ""
	}
}

func issueOpsSchemaVersionError(version int) error {
	switch {
	case version == 0 || version == 1 || version == 2 || version == 3 || version == 4 || version == 5 || version == IssueOpsCurrentSchemaVersion:
		return nil
	case version > IssueOpsCurrentSchemaVersion:
		return fmt.Errorf("unsupported issueops schema_version %d; current is %d", version, IssueOpsCurrentSchemaVersion)
	default:
		return fmt.Errorf("unsupported issueops schema_version %d", version)
	}
}

func decodeInvalidIssueOpsProjection(raw []byte) (IssueOpsRecord, error) {
	var projection struct {
		SchemaVersion int           `json:"schema_version"`
		ID            string        `json:"id"`
		Repo          string        `json:"repo"`
		Branch        string        `json:"branch"`
		Phase         IssueOpsPhase `json:"phase"`
		WorktreePath  string        `json:"worktree_path"`
		Handoff       *struct {
			ProtocolVersion          int             `json:"protocol_version"`
			State                    string          `json:"state"`
			ClosedDisposition        string          `json:"closed_disposition"`
			Attempt                  int             `json:"attempt"`
			CoordinatorRoot          string          `json:"coordinator_root"`
			CoordinatorMailboxHandle string          `json:"coordinator_mailbox_handle"`
			WorkerRoot               string          `json:"worker_root"`
			PublishReceipt           json.RawMessage `json:"publish_receipt"`
		} `json:"execution_handoff"`
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
	if projection.Handoff != nil {
		record.ExecutionHandoff = &IssueOpsExecutionHandoff{
			ProtocolVersion:          projection.Handoff.ProtocolVersion,
			State:                    boundedIssueOpsIdentity(projection.Handoff.State, 64),
			ClosedDisposition:        boundedIssueOpsIdentity(projection.Handoff.ClosedDisposition, 64),
			Attempt:                  projection.Handoff.Attempt,
			CoordinatorRoot:          boundedIssueOpsIdentity(projection.Handoff.CoordinatorRoot, 4096),
			CoordinatorMailboxHandle: boundedIssueOpsIdentity(projection.Handoff.CoordinatorMailboxHandle, 256),
			WorkerRoot:               boundedIssueOpsIdentity(projection.Handoff.WorkerRoot, 4096),
		}
		if rawIssueOpsAuthorityPresent(projection.Handoff.PublishReceipt) {
			record.ExecutionHandoff.PublishReceipt = &IssueOpsExecutionHandoffPublishReceipt{}
		}
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
