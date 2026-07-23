package issueops

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
)

func TestIssueOpsUsesOnlySchemaOneAndDedicatedNamespace(t *testing.T) {
	stateRoot := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: t.TempDir(), Branch: "69-v1-namespace"})
	if err != nil {
		t.Fatal(err)
	}
	if model.IssueOpsSchemaVersion != 1 {
		t.Fatalf("model schema constant=%d want 1", model.IssueOpsSchemaVersion)
	}
	if record.SchemaVersion != model.IssueOpsSchemaVersion {
		t.Fatalf("new record schema=%d want 1", record.SchemaVersion)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.Get("issueops_v1", record.ID); err != nil || !ok {
		t.Fatalf("v1 row missing from issueops_v1: ok=%t err=%v", ok, err)
	}
	if _, ok, err := db.Get("issueops", record.ID); err != nil || ok {
		t.Fatalf("v1 writer touched legacy issueops namespace: ok=%t err=%v", ok, err)
	}
}

func TestIssueOpsReaderIgnoresLegacyBucketAndFailsClosedOnOtherSchemas(t *testing.T) {
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	legacyID := "io-legacy"
	legacy := []byte(`{"schema_version":9,"id":"io-legacy","repo":"/repo","phase":"problem"}`)
	if err := db.Put("issueops", legacyID, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadIssueOps(stateRoot, legacyID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("v1 reader must ignore legacy bucket, got %v", err)
	}
	for _, version := range []int{0, 2, 9} {
		id := "io-schema-" + strings.Repeat("x", version+1)
		raw, _ := json.Marshal(map[string]any{"schema_version": version, "id": id, "repo": "/repo", "phase": "problem"})
		if err := db.Put("issueops_v1", id, raw); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadIssueOps(stateRoot, id); err == nil || !strings.Contains(err.Error(), "schema_version") {
			t.Fatalf("schema %d must fail closed, got %v", version, err)
		}
	}
}

func TestIssueOpsRejectsLegacyExecutionAuthorityPayload(t *testing.T) {
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"execution_handoff", "execution_workspace", "ownership", "remote_create_claim"} {
		id := "io-legacy-" + strings.ReplaceAll(field, "_", "-")
		raw, _ := json.Marshal(map[string]any{
			"schema_version": 1, "id": id, "repo": "/repo", "phase": "problem", field: map[string]any{"legacy": true},
		})
		if err := db.Put("issueops_v1", id, raw); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadIssueOps(stateRoot, id); err == nil || !strings.Contains(err.Error(), "legacy execution authority") {
			t.Fatalf("legacy field %s must fail closed, got %v", field, err)
		}
	}
}

func TestIssueOpsDefaultStateRootIsSchemaSpecific(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if filepath.Base(IssueOpsStateRoot()) != "issueops_v1" {
		t.Fatalf("default state root is not the dedicated v1 namespace: %s", IssueOpsStateRoot())
	}
}
