package issueops

import (
	"agent-harness/internal/core/state"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ReadIssueOps(stateRoot, id string) (IssueOpsRecord, error) {
	id, err := normalizeIssueOpsID(id)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	path := issueopsPath(stateRoot, id)
	b, err := os.ReadFile(path)
	if err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	var record IssueOpsRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	if record.ID != id {
		return IssueOpsRecord{OK: false, ID: id}, fmt.Errorf("issueops id mismatch: file has %q", record.ID)
	}
	if err := normalizeIssueOpsSchemaVersion(&record); err != nil {
		return IssueOpsRecord{OK: false, ID: id}, err
	}
	record.OK = true
	return record, nil
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
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		record.OK = false
		return record, err
	}
	record.OK = true
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		record.OK = false
		return record, err
	}
	path := issueopsPath(stateRoot, record.ID)
	tmp, err := os.CreateTemp(stateRoot, "."+record.ID+"-*.tmp")
	if err != nil {
		record.OK = false
		return record, err
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
		record.OK = false
		return record, writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		record.OK = false
		return record, err
	}
	return record, nil
}

func WriteIssueOps(stateRoot string, record IssueOpsRecord) (IssueOpsRecord, error) {
	return writeIssueOps(stateRoot, record)
}

func issueopsPath(stateRoot, id string) string {
	return filepath.Join(stateRoot, id+".json")
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
