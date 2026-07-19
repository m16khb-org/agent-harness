package issueopscli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/sqlstore"
)

func TestIssueOpsForceReleaseCASFlagsRequirePair(t *testing.T) {
	err := runIssueOpsForceRelease([]string{
		"--id", "io-0123456789ab",
		"--reason", "sealed reconciliation release",
		"--expected-raw-sha256", strings.Repeat("a", 64),
	})
	if err == nil || !strings.Contains(err.Error(), "must be provided together") {
		t.Fatalf("expected paired digest flag rejection, got %v", err)
	}
}

func TestIssueOpsForceReleaseCASEmptyFlagsDoNotFallBackToOrdinaryRelease(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
		Repo: t.TempDir(), Branch: "77-force-release-cas-cli-empty",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = runIssueOpsForceRelease([]string{
		"--id", record.ID,
		"--reason", "sealed reconciliation release",
		"--expected-raw-sha256", "",
		"--expected-canonical-sha256", "",
		"--json",
	})
	if err == nil || !strings.Contains(err.Error(), "64 lowercase") {
		t.Fatalf("expected empty CAS digest rejection, got %v", err)
	}
	current, readErr := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if current.Phase == core.IssueOpsPhaseDone {
		t.Fatal("empty CAS flags fell back to ordinary force-release")
	}
}

func TestIssueOpsForceReleaseCASFlagsRejectDriftWithoutOrdinaryRelease(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
		Repo: t.TempDir(), Branch: "78-force-release-cas-cli",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = runIssueOpsForceRelease([]string{
		"--id", record.ID,
		"--reason", "sealed reconciliation release",
		"--expected-raw-sha256", strings.Repeat("a", 64),
		"--expected-canonical-sha256", strings.Repeat("b", 64),
		"--json",
	})
	if err == nil || !strings.Contains(err.Error(), "raw SHA-256") {
		t.Fatalf("expected CAS digest rejection, got %v", err)
	}
	current, readErr := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if current.Phase == core.IssueOpsPhaseDone {
		t.Fatal("CAS CLI drift rejection fell back to ordinary force-release")
	}
}

func TestIssueOpsForceReleaseCASFlagsReturnLockedProof(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{
		Repo: t.TempDir(), Branch: "79-force-release-cas-cli-proof",
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(core.IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok {
		t.Fatalf("read raw record: ok=%v err=%v", ok, err)
	}
	rawSum := sha256.Sum256(raw)
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		t.Fatal(err)
	}
	canonical := bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))
	canonicalSum := sha256.Sum256(canonical)
	rawSHA := hex.EncodeToString(rawSum[:])
	canonicalSHA := hex.EncodeToString(canonicalSum[:])

	out := captureStdoutForContract(t, func() error {
		return runIssueOpsForceRelease([]string{
			"--id", record.ID,
			"--reason", "sealed reconciliation release",
			"--expected-raw-sha256", rawSHA,
			"--expected-canonical-sha256", canonicalSHA,
			"--json",
		})
	})
	var result core.ForceReleaseCASResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Record.Phase != core.IssueOpsPhaseDone || result.BeforeRawSHA256 != rawSHA || !result.BindingAbsenceVerified {
		t.Fatalf("unexpected force-release CAS proof: %+v", result)
	}
}
