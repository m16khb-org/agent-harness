package issueops

import (
	"os"
	"strings"
	"testing"
)

func TestIssueOpsReadNormalizesLegacySchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-legacy-schema"
	if err := os.WriteFile(issueopsPath(stateRoot, id), []byte(`{
  "ok": true,
  "id": "io-legacy-schema",
  "repo": "/repo/example",
  "branch": "1-demo",
  "phase": "problem",
  "created_at": "2026-07-02T00:00:00Z",
  "updated_at": "2026-07-02T00:00:00Z"
}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != IssueOpsCurrentSchemaVersion {
		t.Fatalf("legacy record schema version = %d, want %d", record.SchemaVersion, IssueOpsCurrentSchemaVersion)
	}
}

func TestIssueOpsReadRejectsFutureSchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-future-schema"
	if err := os.WriteFile(issueopsPath(stateRoot, id), []byte(`{
  "ok": true,
  "schema_version": 2,
  "id": "io-future-schema",
  "repo": "/repo/example",
  "branch": "1-demo",
  "phase": "problem",
  "created_at": "2026-07-02T00:00:00Z",
  "updated_at": "2026-07-02T00:00:00Z"
}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "unsupported issueops schema_version 2") {
		t.Fatalf("expected future schema rejection, got %v", err)
	}
}

func TestIssueOpsWriteStampsCurrentSchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := WriteIssueOps(stateRoot, IssueOpsRecord{
		ID:    "io-current-schema",
		Repo:  "/repo/example",
		Phase: IssueOpsPhaseProblem,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.SchemaVersion != IssueOpsCurrentSchemaVersion {
		t.Fatalf("written record schema version = %d, want %d", record.SchemaVersion, IssueOpsCurrentSchemaVersion)
	}

	reloaded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.SchemaVersion != IssueOpsCurrentSchemaVersion {
		t.Fatalf("reloaded record schema version = %d, want %d", reloaded.SchemaVersion, IssueOpsCurrentSchemaVersion)
	}
}

func TestIssueOpsWriteRejectsFutureSchemaVersion(t *testing.T) {
	_, err := WriteIssueOps(t.TempDir(), IssueOpsRecord{
		ID:            "io-write-future-schema",
		SchemaVersion: IssueOpsCurrentSchemaVersion + 1,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported issueops schema_version") {
		t.Fatalf("expected future schema write rejection, got %v", err)
	}
}
