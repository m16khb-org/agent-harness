package issueopscli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
)

func TestIssueOpsResetLegacyCLIPreviewUsesExactV1Contract(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	db, err := sqlstore.Open(filepath.Join(stateDir, "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", "io-aaaaaaaaaaaa", []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)); err != nil {
		t.Fatal(err)
	}

	out := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"reset-legacy", "--target-schema", "1", "--preview", "--json"})
	})
	var preview core.LegacyResetPreviewV1
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatalf("preview JSON: %v\n%s", err, out)
	}
	if !preview.OK || !preview.ResetRequired || !preview.CanConfirm || preview.RowCount != 1 || len(preview.Fingerprint) != 64 {
		t.Fatalf("preview = %#v", preview)
	}
}

func TestIssueOpsResetLegacyCLIRequiresOneExplicitAction(t *testing.T) {
	for _, args := range [][]string{
		{"reset-legacy", "--target-schema", "1"},
		{"reset-legacy", "--target-schema", "1", "--preview", "--status"},
		{"reset-legacy", "--target-schema", "2", "--preview"},
		{"reset-legacy", "--target-schema", "1", "--confirm"},
	} {
		if err := runIssueOps(args); err == nil {
			t.Fatalf("command unexpectedly succeeded: %v", args)
		} else if args[len(args)-1] == "--confirm" && !strings.Contains(err.Error(), "expected-fingerprint") {
			t.Fatalf("confirm error = %v", err)
		}
	}
}
