package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
)

func TestMigrateV9CLIHelpIsExactRecordOnly(t *testing.T) {
	if err := runIssueOps([]string{"migrate-v9", "--help"}); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"migrate-v9", "--preview"},
		{"migrate-v9", "--id", "io-0123456789ab"},
		{"migrate-v9", "--id", "io-0123456789ab", "--preview", "--confirm"},
		{"migrate-v9", "--all", "--preview"},
	} {
		if err := runIssueOps(args); err == nil {
			t.Fatalf("expected exact-record flag rejection for %v", args)
		}
	}
}

func TestMigrateV9CLIPreviewsWithoutWritingThenConfirmsExactRecord(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := core.IssueOpsRecord{
		SchemaVersion: 8,
		ID:            "io-0123456789ab",
		Repo:          t.TempDir(),
		Branch:        "68-migrate-v9-cli",
		Phase:         core.IssueOpsPhaseImplement,
		CreatedAt:     "2026-07-22T00:00:00Z",
		UpdatedAt:     "2026-07-22T00:00:00Z",
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(core.IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", record.ID, raw); err != nil {
		t.Fatal(err)
	}

	preview := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"migrate-v9", "--id", record.ID, "--preview", "--json"})
	})
	if !strings.Contains(preview, `"classification": "active_without_owner"`) {
		t.Fatalf("unexpected preview: %s", preview)
	}
	stored, ok, err := db.Get("issueops", record.ID)
	var storedRecord core.IssueOpsRecord
	if decodeErr := json.Unmarshal(stored, &storedRecord); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if err != nil || !ok || storedRecord.SchemaVersion != 8 {
		t.Fatalf("preview changed v8 record: ok=%v err=%v raw=%s", ok, err, stored)
	}

	confirm := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"migrate-v9", "--id", record.ID, "--confirm", "--json"})
	})
	if !strings.Contains(confirm, `"raw_sha256"`) || !strings.Contains(confirm, `"canonical_sha256"`) {
		t.Fatalf("confirm omitted CAS proof: %s", confirm)
	}
	stored, ok, err = db.Get("issueops", record.ID)
	if decodeErr := json.Unmarshal(stored, &storedRecord); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if err != nil || !ok || storedRecord.SchemaVersion != 9 {
		t.Fatalf("confirm did not persist schema v9: ok=%v err=%v raw=%s", ok, err, stored)
	}
}
