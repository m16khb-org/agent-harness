package issueops

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/sqlstore"
)

func TestLegacyIssueOpsRecordWithoutExecutionHandoffRemainsInline(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-legacy-inline"
	writeRawIssueOpsRecord(t, stateRoot, id, `{
  "ok": true,
  "id": "io-legacy-inline",
  "repo": "/repo/example",
  "branch": "16-demo",
  "phase": "implement",
  "created_at": "2026-07-11T00:00:00Z",
  "updated_at": "2026-07-11T00:00:00Z"
}`)

	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	if record.ExecutionHandoff != nil {
		t.Fatalf("legacy record unexpectedly gained execution handoff: %#v", record.ExecutionHandoff)
	}
	readiness := IssueOpsImplementationReadiness(record)
	if containsString(readiness.Missing, "handoff_worker_claim") {
		t.Fatalf("legacy inline readiness must not require worker claim: %#v", readiness.Missing)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "execution_handoff") {
		t.Fatalf("legacy inline JSON contains execution_handoff: %s", encoded)
	}
}

func TestIssueOpsImplementationReadinessRequiresClaimOnlyForHandoff(t *testing.T) {
	record := IssueOpsRecord{ExecutionHandoff: &IssueOpsExecutionHandoff{
		State:          "dispatched",
		Attempt:        1,
		OwnershipEpoch: "epoch-1",
	}}
	readiness := IssueOpsImplementationReadiness(record)
	if !containsString(readiness.Missing, "handoff_worker_claim") {
		t.Fatalf("unclaimed handoff missing keys = %#v, want handoff_worker_claim", readiness.Missing)
	}
	record.ExecutionHandoff.State = "claimed"
	readiness = IssueOpsImplementationReadiness(record)
	if containsString(readiness.Missing, "handoff_worker_claim") {
		t.Fatalf("claimed handoff still requires worker claim: %#v", readiness.Missing)
	}
}

// writeRawIssueOpsRecord inserts record bytes directly into the state store,
// bypassing WriteIssueOps normalization, to simulate legacy or foreign rows.
func writeRawIssueOpsRecord(t *testing.T, stateRoot, id, raw string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", id, []byte(raw)); err != nil {
		t.Fatal(err)
	}
}

func TestIssueOpsReadNormalizesLegacySchemaVersion(t *testing.T) {
	stateRoot := t.TempDir()
	id := "io-legacy-schema"
	writeRawIssueOpsRecord(t, stateRoot, id, `{
  "ok": true,
  "id": "io-legacy-schema",
  "repo": "/repo/example",
  "branch": "1-demo",
  "phase": "problem",
  "created_at": "2026-07-02T00:00:00Z",
  "updated_at": "2026-07-02T00:00:00Z"
}
`)

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
	writeRawIssueOpsRecord(t, stateRoot, id, `{
  "ok": true,
  "schema_version": 8,
  "id": "io-future-schema",
  "repo": "/repo/example",
  "branch": "1-demo",
  "phase": "problem",
  "created_at": "2026-07-02T00:00:00Z",
  "updated_at": "2026-07-02T00:00:00Z"
}
`)

	_, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "unsupported issueops schema_version 8") {
		t.Fatalf("expected future schema rejection, got %v", err)
	}
}

func TestRawSchemaV5RejectsRemoteCreateClaimWithoutRewriting(t *testing.T) {
	stateRoot, id := t.TempDir(), "io-v5-claim"
	raw := `{"ok":true,"schema_version":5,"id":"io-v5-claim","repo":"/repo/example","branch":"16-demo","phase":"pr","remote_create_claim":{"claim_id":"claim_00000000000000000000000000000000"}}`
	writeRawIssueOpsRecord(t, stateRoot, id, raw)
	before := rawIssueOpsBytesForTest(t, stateRoot, id)
	_, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "schema_version 5 cannot contain remote_create_claim") {
		t.Fatalf("raw schema-v5 claim error = %v", err)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, id); !bytes.Equal(after, before) {
		t.Fatal("raw schema-v5 claim row was rewritten")
	}
}

func TestRawSchemaV5RejectsCoordinatorSessionWithoutRewriting(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 5
	record.ExecutionHandoff.CoordinatorMailboxHandle = "term_coordinator"
	record.ExecutionHandoff.CoordinatorSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "copied-session", AgentID: "copied-agent"}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	invalid, err := ReadIssueOps(stateRoot, record.ID)
	if err == nil || !strings.Contains(err.Error(), "schema_version 5 cannot contain coordinator_session") {
		t.Fatalf("raw schema-v5 coordinator session error = %v", err)
	}
	if !invalid.Invalid || invalid.ExecutionHandoff == nil || invalid.ExecutionHandoff.WorkerRoot != record.ExecutionHandoff.WorkerRoot || invalid.Repo != record.Repo {
		t.Fatalf("raw schema-v5 coordinator session lost bounded handoff guard authority: %#v", invalid)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !bytes.Equal(after, before) {
		t.Fatal("raw schema-v5 coordinator session row was rewritten")
	}
}

func TestFrozenSchemaV5ReaderRejectsRawSchemaV6WithoutRewriting(t *testing.T) {
	setup := func(t *testing.T) (string, string, []byte) {
		t.Helper()
		stateRoot, record := handoffDispatchRecord(t)
		record.SchemaVersion = 6
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
		return stateRoot, record.ID, rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	}
	t.Run("rejects raw schema-v6", func(t *testing.T) {
		stateRoot, id, _ := setup(t)
		if err := legacyV5ReadModifyWriteFixture(stateRoot, id); err == nil || !strings.Contains(err.Error(), "schema_version 6") {
			t.Fatalf("frozen schema-v5 reader did not reject raw schema-v6 bytes: %v", err)
		}
	})
	t.Run("preserves raw bytes", func(t *testing.T) {
		stateRoot, id, before := setup(t)
		_ = legacyV5ReadModifyWriteFixture(stateRoot, id)
		if after := rawIssueOpsBytesForTest(t, stateRoot, id); !bytes.Equal(after, before) {
			t.Fatal("frozen schema-v5 reader rewrote raw schema-v6 bytes")
		}
	})
}

func TestFrozenSchemaV6ReaderRejectsRawSchemaV7WithoutRewriting(t *testing.T) {
	setup := func(t *testing.T) (string, string, []byte) {
		t.Helper()
		stateRoot, record := handoffDispatchRecord(t)
		record.SchemaVersion = 7
		raw, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
		return stateRoot, record.ID, rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	}
	t.Run("rejects raw schema-v7", func(t *testing.T) {
		stateRoot, id, _ := setup(t)
		if err := legacyV6ReadModifyWriteFixture(stateRoot, id); err == nil || !strings.Contains(err.Error(), "schema_version 7") {
			t.Fatalf("frozen schema-v6 reader did not reject raw schema-v7 bytes: %v", err)
		}
	})
	t.Run("preserves raw bytes", func(t *testing.T) {
		stateRoot, id, before := setup(t)
		_ = legacyV6ReadModifyWriteFixture(stateRoot, id)
		if after := rawIssueOpsBytesForTest(t, stateRoot, id); !bytes.Equal(after, before) {
			t.Fatal("frozen schema-v6 reader rewrote raw schema-v7 bytes")
		}
	})
}

func TestReadIssueOpsRejectsInvalidLegacyWorktreeMigrationSnapshot(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	record.LegacyWorktreeMigration = &IssueOpsLegacyWorktreeMigration{
		State:        IssueOpsLegacyWorktreeMigrationStateGitRemoved,
		WorktreePath: handoffPrepareWorktreePath(record),
		Branch:       record.Branch,
		Head:         record.BranchPrepare.BaseSHA,
		BaseRef:      "refs/remotes/origin/" + record.Branch,
		PreparedAt:   "2026-07-11T01:02:03Z",
		GitRemovedAt: "not-a-timestamp",
	}
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	got, err := ReadIssueOps(stateRoot, record.ID)
	if err == nil || !strings.Contains(err.Error(), "git removal timestamp") || !got.Invalid {
		t.Fatalf("invalid migration snapshot must fail closed: record=%#v err=%v", got, err)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !bytes.Equal(after, before) {
		t.Fatal("invalid migration snapshot was rewritten during read")
	}
}

func TestSchemaV5OldPublishReceiptRequiresReattestOrReconcileWithoutRewriting(t *testing.T) {
	stateRoot, id := t.TempDir(), "io-v5-receipt"
	raw := `{"ok":true,"schema_version":5,"id":"io-v5-receipt","repo":"/repo/example","branch":"16-demo","phase":"pr","execution_handoff":{"protocol_version":1,"state":"closed","closed_disposition":"accepted","coordinator_root":"/repo/example","coordinator_mailbox_handle":"term_coordinator","worker_root":"/repo/example.worktrees/16-demo","publish_receipt":{"provider":"github","remote":"origin","branch":"16-demo","remote_ref":"refs/heads/16-demo","final_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","verified_at":"2026-07-12T00:00:00Z"}}}`
	writeRawIssueOpsRecord(t, stateRoot, id, raw)
	before := rawIssueOpsBytesForTest(t, stateRoot, id)
	got, err := ReadIssueOps(stateRoot, id)
	if err == nil || !strings.Contains(err.Error(), "re-attest publication") || !strings.Contains(err.Error(), "dedicated remote-create reconcile") || len(err.Error()) > 512 {
		t.Fatalf("raw schema-v5 receipt diagnostic = %v", err)
	}
	if !got.Invalid || got.SchemaVersion != 5 || got.Repo != "/repo/example" || got.ExecutionHandoff == nil || got.ExecutionHandoff.PublishReceipt == nil || got.ExecutionHandoff.CoordinatorMailboxHandle != "term_coordinator" {
		t.Fatalf("raw schema-v5 receipt lost bounded hook authority projection: %#v", got)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, id); !bytes.Equal(after, before) {
		t.Fatal("raw schema-v5 receipt row was rewritten")
	}
}

func TestSchemaV5AndV6WithoutV7AuthorityMigrateInMemoryAndSchemaV7IsAccepted(t *testing.T) {
	for _, version := range []int{5, 6, 7} {
		stateRoot := t.TempDir()
		id := fmt.Sprintf("io-schema-v%d", version)
		raw := fmt.Sprintf(`{"ok":true,"schema_version":%d,"id":%q,"repo":"/repo/example","branch":"16-demo","phase":"problem"}`, version, id)
		writeRawIssueOpsRecord(t, stateRoot, id, raw)
		before := rawIssueOpsBytesForTest(t, stateRoot, id)
		got, err := ReadIssueOps(stateRoot, id)
		if err != nil || got.SchemaVersion != IssueOpsCurrentSchemaVersion {
			t.Fatalf("schema v%d read = %#v, %v", version, got, err)
		}
		if after := rawIssueOpsBytesForTest(t, stateRoot, id); !bytes.Equal(after, before) {
			t.Fatalf("schema v%d read rewrote stored bytes", version)
		}
	}
}

func TestIssueOpsSchemaV4PreservesSealedOrcaMailboxAuthorities(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.SchemaVersion = 4
	record.ExecutionHandoff.CoordinatorSession = nil
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	handoffDocument := document["execution_handoff"].(map[string]any)
	handoffDocument["coordinator_mailbox_handle"] = "term_coordinator"
	orcaDocument := handoffDocument["orca"].(map[string]any)
	orcaDocument["worker_terminal_handle"] = "term_live"
	orcaDocument["worker_mailbox_handle"] = "term_dispatched"
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))

	got, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"schema_version":7`, `"coordinator_mailbox_handle":"term_coordinator"`, `"worker_terminal_handle":"term_live"`, `"worker_mailbox_handle":"term_dispatched"`} {
		if !bytes.Contains(reencoded, []byte(want)) {
			t.Fatalf("schema-v4 authority %s was not preserved: %s", want, reencoded)
		}
	}
}

func TestIssueOpsReadUpgradesV1HandoffWithoutDataLoss(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 1
	wantHandoff := *record.ExecutionHandoff
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}

	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != IssueOpsCurrentSchemaVersion || upgraded.ExecutionHandoff == nil || !reflect.DeepEqual(*upgraded.ExecutionHandoff, wantHandoff) {
		t.Fatalf("v1 handoff was not preserved during read migration: %#v", upgraded)
	}
	if _, err := WriteIssueOps(stateRoot, upgraded); err != nil {
		t.Fatal(err)
	}
	rewritten, ok, err := db.Get(issueOpsBucket, record.ID)
	if err != nil || !ok {
		t.Fatalf("read upgraded row: ok=%v err=%v", ok, err)
	}
	var stored map[string]any
	if err := json.Unmarshal(rewritten, &stored); err != nil {
		t.Fatal(err)
	}
	if stored["schema_version"] != float64(IssueOpsCurrentSchemaVersion) || stored["execution_handoff"] == nil {
		t.Fatalf("v1 handoff write did not upgrade to current schema without data loss: %s", rewritten)
	}
}

func TestIssueOpsReadUpgradesV2HandoffWithoutDataLoss(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 2
	wantHandoff := *record.ExecutionHandoff
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != IssueOpsCurrentSchemaVersion || upgraded.ExecutionHandoff == nil || !reflect.DeepEqual(*upgraded.ExecutionHandoff, wantHandoff) {
		t.Fatalf("v2 handoff was not preserved during read migration: %#v", upgraded)
	}
}

func TestIssueOpsReadUpgradesV3LiveTerminalIdentityWithoutInventingCoordinator(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 3
	record.ExecutionHandoff.CoordinatorMailboxHandle = ""
	record.ExecutionHandoff.Orca.WorkerPTYID = "pty-legacy"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = ""
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-legacy"
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.SchemaVersion != IssueOpsCurrentSchemaVersion || upgraded.ExecutionHandoff.CoordinatorMailboxHandle != "" || upgraded.ExecutionHandoff.Orca.WorkerMailboxHandle != "" || upgraded.ExecutionHandoff.Orca.WorkerTerminalHandle != "term-legacy" {
		t.Fatalf("v3 authority migration invented or lost identity: %#v", upgraded.ExecutionHandoff)
	}
}

func TestIssueOpsReadUpgradesV3ContextV1WithoutChangingLegacyHashes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	options := handoff.ContextOptions{WorkerScope: "legacy v3 worker", VerificationCommands: []string{"go test ./... -count=1"}}
	packet, err := handoff.BuildContext(record, options)
	if err != nil {
		t.Fatal(err)
	}
	sourcePacket, err := handoff.BuildContext(record, handoff.ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	legacyContextSHA := legacyV3ContextProjectionSHA256(t, packet.Projection)
	legacySourceSHA := legacyV3ContextProjectionSHA256(t, sourcePacket.Projection)
	record.SchemaVersion = 3
	record.ExecutionHandoff.ContextVersion = handoff.ContextVersion
	record.ExecutionHandoff.ContextSHA256 = legacyContextSHA
	record.ExecutionHandoff.ContextSourceSHA256 = legacySourceSHA
	canonicalOptions := handoff.CanonicalContextOptions(options)
	record.ExecutionHandoff.ContextOptions = &canonicalOptions
	putRawIssueOpsRecordForTest(t, stateRoot, record)

	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateHandoffContextSource(upgraded); err != nil {
		t.Fatalf("migrated v3 context source became stale: %v", err)
	}
	rebuilt, err := handoff.BuildContext(upgraded, options)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.SHA256 != legacyContextSHA || rebuilt.SourceSHA256 != legacySourceSHA {
		t.Fatalf("migrated v3 ContextVersion-1 hashes changed: got=%s/%s want=%s/%s", rebuilt.SHA256, rebuilt.SourceSHA256, legacyContextSHA, legacySourceSHA)
	}
	if strings.Contains(rebuilt.Markdown, `"coordinator_recipient"`) {
		t.Fatalf("legacy empty coordinator changed rendered context bytes: %s", rebuilt.Markdown)
	}
}

func legacyV3ContextProjectionSHA256(t *testing.T, projection handoff.ContextProjection) string {
	t.Helper()
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	legacy := bytes.Replace(encoded, []byte(`,"coordinator_recipient":""`), nil, 1)
	sum := sha256.Sum256(legacy)
	return fmt.Sprintf("%x", sum)
}

func TestIssueOpsReadUpgradesV3CurrentAndPriorTerminalAuthority(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	h := record.ExecutionHandoff
	h.State = handoff.StateClosed
	h.ClosedDisposition = handoff.DispositionWorkerFailed
	h.WorkerSession = &IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-prior", AgentID: "agent-prior"}
	h.Orca.WorkerTerminalHandle = ""
	h.Orca.WorkerMailboxHandle = "term-prior-mailbox"
	h.Result = validFailedHandoffResultForTest(record)
	h.CompletedAt = "2026-07-11T00:30:00Z"
	prior, err := handoff.SnapshotPriorAttempt(h)
	if err != nil {
		t.Fatal(err)
	}

	h.State = handoff.StateCoordinatorPreparing
	h.ClosedDisposition = ""
	h.Attempt = 2
	h.OwnershipEpoch = "epoch-current"
	h.DeliveryMode = ""
	h.WorkerSession = nil
	h.Result = nil
	h.Failure = nil
	h.PendingOperation = nil
	h.DispatchedAt = ""
	h.ClaimedAt = ""
	h.LastHeartbeatAt = ""
	h.CompletedAt = ""
	h.AcceptedAt = ""
	h.Orca.WorkerTerminalHandle = ""
	h.Orca.WorkerMailboxHandle = "term-current-legacy"
	h.CoordinatorSession = nil
	h.Orca.DispatchID = ""
	h.PriorAttempts = []IssueOpsExecutionHandoffPriorAttempt{prior}
	record.SchemaVersion = 3
	putRawIssueOpsRecordForTest(t, stateRoot, record)

	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	current := upgraded.ExecutionHandoff
	if current.Orca.WorkerTerminalHandle != "term-current-legacy" || current.Orca.WorkerMailboxHandle != "" {
		t.Fatalf("v3 current attempt authority migration = %#v", current.Orca)
	}
	if len(current.PriorAttempts) != 1 || current.PriorAttempts[0].Orca == nil {
		t.Fatalf("v3 prior attempt was not retained: %#v", current.PriorAttempts)
	}
	priorOrca := current.PriorAttempts[0].Orca
	if priorOrca.WorkerTerminalHandle != "term-prior-mailbox" || priorOrca.WorkerMailboxHandle != "term-prior-mailbox" || priorOrca.DispatchID == "" {
		t.Fatalf("v3 dispatched prior authority migration = %#v", priorOrca)
	}
}

func TestIssueOpsReadUpgradesV3CancelledNoDispatchCurrentAndPrior(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	h := record.ExecutionHandoff
	h.State = handoff.StateClosed
	h.ClosedDisposition = handoff.DispositionCancelled
	h.Orca.WorkerTerminalHandle = ""
	h.Orca.WorkerMailboxHandle = "term-prior-legacy"
	h.Orca.TaskID = ""
	h.Orca.DispatchID = ""
	prior, err := handoff.SnapshotPriorAttempt(h)
	if err != nil {
		t.Fatal(err)
	}

	h.Attempt = 2
	h.OwnershipEpoch = "epoch-current"
	h.Orca.WorkerTerminalHandle = ""
	h.Orca.WorkerMailboxHandle = "term-current-legacy"
	h.PriorAttempts = []IssueOpsExecutionHandoffPriorAttempt{prior}
	record.SchemaVersion = 3
	putRawIssueOpsRecordForTest(t, stateRoot, record)

	upgraded, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	current := upgraded.ExecutionHandoff
	if current.Orca.WorkerTerminalHandle != "term-current-legacy" || current.Orca.WorkerMailboxHandle != "" {
		t.Fatalf("v3 cancelled current authority migration = %#v", current.Orca)
	}
	if len(current.PriorAttempts) != 1 || current.PriorAttempts[0].Orca == nil {
		t.Fatalf("v3 cancelled prior attempt was not retained: %#v", current.PriorAttempts)
	}
	priorOrca := current.PriorAttempts[0].Orca
	if priorOrca.WorkerTerminalHandle != "term-prior-legacy" || priorOrca.WorkerMailboxHandle != "" || priorOrca.DispatchID != "" {
		t.Fatalf("v3 cancelled prior authority migration = %#v", priorOrca)
	}
}

func TestIssueOpsSchemaV4DoesNotApplyLegacyMailboxRewrites(t *testing.T) {
	_, record := handoffDispatchRecord(t)
	record.SchemaVersion = 4
	record.ExecutionHandoff.State = handoff.StateCoordinatorPreparing
	record.ExecutionHandoff.Orca.DispatchID = ""
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = ""
	record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-sealed-v4"

	if err := normalizeIssueOpsSchemaVersion(&record); err != nil {
		t.Fatal(err)
	}
	if record.ExecutionHandoff.Orca.WorkerTerminalHandle != "" || record.ExecutionHandoff.Orca.WorkerMailboxHandle != "term-sealed-v4" {
		t.Fatalf("schema-v4 authority was rewritten as legacy: %#v", record.ExecutionHandoff.Orca)
	}
}

func TestIssueOpsSchemaV4MissingLiveTerminalFailsClosedWithoutBorrowingMailbox(t *testing.T) {
	stateRoot, record, _ := dispatchedHandoffRecord(t)
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = ""
	putRawIssueOpsRecordForTest(t, stateRoot, record)

	got, err := ReadIssueOps(stateRoot, record.ID)
	if err == nil || got.ExecutionHandoff == nil || got.ExecutionHandoff.Orca == nil {
		t.Fatalf("schema-v4 missing live terminal did not fail closed: got=%#v err=%v", got, err)
	}
	if got.ExecutionHandoff.Orca.WorkerTerminalHandle != "" || got.ExecutionHandoff.Orca.WorkerMailboxHandle != record.ExecutionHandoff.Orca.WorkerMailboxHandle {
		t.Fatalf("schema-v4 read borrowed or cleared sealed authority: %#v", got.ExecutionHandoff.Orca)
	}
}

func TestFutureSchemaReadPreservesBoundedHandoffIdentityAndInvalidMarker(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = IssueOpsCurrentSchemaVersion + 1
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	got, readErr := readIssueOpsUnchecked(stateRoot, record.ID)
	if readErr == nil || got.ExecutionHandoff == nil || got.ExecutionHandoff.WorkerRoot != record.ExecutionHandoff.WorkerRoot || !got.Invalid || got.InvalidReason == "" {
		t.Fatalf("future schema discarded bounded ownership identity: got=%#v err=%v", got, readErr)
	}
	if len(got.InvalidReason) > 256 || !strings.Contains(got.InvalidReason, "schema_version") {
		t.Fatalf("future schema invalid marker is unbounded or vague: %q", got.InvalidReason)
	}
}

func TestLegacyV1DecoderRejectsV2HandoffWithoutModifyingBytes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 2
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(issueOpsBucket, record.ID, raw); err != nil {
		t.Fatal(err)
	}
	before, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyV1ReadModifyWriteFixture(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "schema_version 2") {
		t.Fatalf("legacy v1 decoder did not reject v2 handoff: %v", err)
	}
	after, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("legacy v1 rejection modified durable bytes\nbefore=%s\n after=%s", before, after)
	}
}

func TestLegacyV2DecoderRejectsV3StableTerminalIdentityWithoutModifyingBytes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 3
	record.ExecutionHandoff.Orca.WorkerTabID = "tab-stable"
	record.ExecutionHandoff.Orca.WorkerLeafID = "leaf-stable"
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
	before, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyV2ReadModifyWriteFixture(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "schema_version 3") {
		t.Fatalf("legacy v2 decoder did not reject v3 stable terminal identity: %v", err)
	}
	after, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("legacy v2 rejection modified durable bytes\nbefore=%s\n after=%s", before, after)
	}
}

func TestLegacyV3DecoderRejectsV4MailboxAuthorityWithoutModifyingBytes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	record.SchemaVersion = 4
	record.ExecutionHandoff.CoordinatorMailboxHandle = "term_coordinator"
	record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term_live"
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeRawIssueOpsRecord(t, stateRoot, record.ID, string(raw))
	before, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyV3ReadModifyWriteFixture(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "schema_version 4") {
		t.Fatalf("legacy v3 decoder did not reject v4 mailbox authority: %v", err)
	}
	after, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("legacy v3 rejection modified durable bytes\nbefore=%s\n after=%s", before, after)
	}
}

func TestLegacyV4DecoderRejectsV6AuthorityWithoutModifyingBytes(t *testing.T) {
	stateRoot, record := handoffDispatchRecord(t)
	before, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyV4ReadModifyWriteFixture(stateRoot, record.ID); err == nil || !strings.Contains(err.Error(), "schema_version 6") {
		t.Fatalf("legacy v4 decoder did not reject v6 authority: %v", err)
	}
	after, err := rawIssueOpsRecordBytes(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("legacy v4 rejection modified durable bytes\nbefore=%s\n after=%s", before, after)
	}
}

func rawIssueOpsRecordBytes(stateRoot, id string) ([]byte, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	raw, ok, err := db.Get(issueOpsBucket, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("missing issueops row %s", id)
	}
	return raw, nil
}

// legacyV1ReadModifyWriteFixture models the v1 decoder/writer boundary with a
// record type that deliberately does not know execution_handoff.
func legacyV1ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		OK            bool          `json:"ok"`
		SchemaVersion int           `json:"schema_version"`
		ID            string        `json:"id"`
		Repo          string        `json:"repo"`
		Branch        string        `json:"branch,omitempty"`
		Phase         IssueOpsPhase `json:"phase"`
		UpdatedAt     string        `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 1 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 1", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v1-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func legacyV2ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 2 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 2", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v2-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func legacyV3ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 3 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 3", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v3-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func legacyV4ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 4 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 4", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v4-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func legacyV5ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 5 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 5", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v5-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
}

func legacyV6ReadModifyWriteFixture(stateRoot, id string) error {
	raw, err := rawIssueOpsRecordBytes(stateRoot, id)
	if err != nil {
		return err
	}
	var legacy struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
		UpdatedAt     string `json:"updated_at"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	if legacy.SchemaVersion > 6 {
		return fmt.Errorf("unsupported issueops schema_version %d; current is 6", legacy.SchemaVersion)
	}
	legacy.UpdatedAt = "legacy-v6-touch"
	rewritten, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		return err
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return err
	}
	return db.Put(issueOpsBucket, id, rewritten)
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
