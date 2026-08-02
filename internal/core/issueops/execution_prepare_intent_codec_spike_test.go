package issueops

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"agent-harness/internal/contract/issueops"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestIntentCodecSpikeRoundTripsAndRecoversPrepareAndResume(t *testing.T) {
	const issueBody = "## acceptance criteria\n\n- [ ] AC-01: first\n- [ ] AC-23: last\n\n## 검증 명령\n\n```bash\ngo test ./... -count=1\ngo vet ./...\n```\n"
	tests := []struct {
		name    string
		fixture func(*testing.T) (string, issueops.IssueOpsRecord, externalOrcaIntentPayload)
	}{
		{name: "prepare", fixture: legacyPrepareIntentFixture},
		{name: "resume", fixture: func(t *testing.T) (string, issueops.IssueOpsRecord, externalOrcaIntentPayload) {
			return legacyResumeIntentFixture(t, "github", 16)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record, payload := test.fixture(t)
			assertIntentCodecSpikeRoundTrip(t, payload.OperationID, rawExternalIntentRow(t, stateRoot, payload.OperationID))
			record, payload = writeLegacyNotInvokedIntent(t, stateRoot, record, payload, func(_ *issueops.IssueOpsRecord, intent *externalOrcaIntentPayload) {
				if normalizedOrcaIntentPurpose(*intent) == orcaIntentPurposePrepare {
					intent.IssueBodySHA256 = digestExecutionOwnerBytes([]byte(issueBody))
				}
			})

			legacyRoot := cloneIntentCodecSpikeState(t, stateRoot, record.ID)
			bridgeRoot := cloneIntentCodecSpikeState(t, stateRoot, record.ID)
			legacyRecord, err := ReadIssueOps(legacyRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			legacyRecord, legacyIntent, legacyMigrated, err := reconcileCanonicalOrcaIntent(legacyRoot, legacyRecord)
			if err != nil || !legacyMigrated {
				t.Fatalf("legacy canonicalize migrated=%t err=%v", legacyMigrated, err)
			}
			bridge, err := CanonicalizeExecutionReconcileIntent(bridgeRoot, record.ID, nil)
			if err != nil || !bridge.Migrated {
				t.Fatalf("bridge canonicalize migrated=%t err=%v", bridge.Migrated, err)
			}
			assertIntentCodecSpikeRows(t, legacyRoot, bridgeRoot, record.ID, payload.OperationID)

			readIssue := func(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
				return port.ExecutionIssueSnapshot{URL: request.URL, Body: issueBody, State: "opened"}, nil
			}
			now := func() time.Time { return time.Date(2026, time.August, 2, 1, 0, 0, 0, time.UTC) }
			for step := 0; bridge.Pending; step++ {
				if step >= 6 {
					t.Fatal("recovery exceeded the fixed six-stage bound")
				}
				legacyRequest, err := executionOrcaIntentRequest(legacyRecord, legacyIntent)
				if err != nil {
					t.Fatal(err)
				}
				bridgeRequest, err := ExecutionReconcileIntentRequest(bridge)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(legacyRequest, bridgeRequest) {
					t.Fatalf("recovery request drifted\nlegacy=%#v\nbridge=%#v", legacyRequest, bridgeRequest)
				}
				receipt := successfulExecutionOrcaIntentReceipt(t, bridgeRequest)
				legacyRecord, legacyIntent, err = advanceOrcaIntentReceipt(context.Background(), legacyRoot, legacyRecord, legacyIntent, receipt, readIssue, now)
				if err != nil {
					t.Fatal(err)
				}
				bridge, err = ApplyExecutionReconcileIntentReceipt(context.Background(), bridgeRoot, bridge, receipt, readIssue, now)
				if err != nil {
					t.Fatal(err)
				}
				assertIntentCodecSpikeRows(t, legacyRoot, bridgeRoot, record.ID, payload.OperationID)
			}
			if legacyRecord.Execution == nil || legacyRecord.Execution.Pending != nil || bridge.Record.Execution == nil || bridge.Record.Execution.Pending != nil {
				t.Fatalf("recovery did not settle: legacy=%#v bridge=%#v", legacyRecord.Execution, bridge.Record.Execution)
			}
		})
	}
}

func TestIntentCodecSpikeRecoveryBridgeDoesNotCallPrepareWrapper(t *testing.T) {
	source, err := os.ReadFile("execution_reconcile_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("PrepareExecution("), []byte("invokeExecutionPrepareHandler(")} {
		if bytes.Contains(source, forbidden) {
			t.Fatalf("reconcile bridge calls forbidden prepare wrapper %q", forbidden)
		}
	}
}

func assertIntentCodecSpikeRoundTrip(t *testing.T, operationID string, raw []byte) {
	t.Helper()
	decoded, err := (preparationcontract.IntentCodec{}).Decode(operationID, raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := (preparationcontract.IntentCodec{}).Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, raw) {
		t.Fatalf("intent bytes drifted\nwant=%s\n got=%s", raw, encoded)
	}
}

func cloneIntentCodecSpikeState(t *testing.T, sourceRoot, id string) string {
	t.Helper()
	record, err := ReadIssueOps(sourceRoot, id)
	if err != nil {
		t.Fatal(err)
	}
	source, err := sqlstore.Open(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	mutations := make([]sqlstore.Mutation, 0, 3)
	for _, row := range []struct{ bucket, key string }{
		{bucket: issueOpsBucket, key: id},
		{bucket: externalIntentBucket, key: record.Execution.Pending.OperationID},
		{bucket: artifactStageBucket, key: id},
	} {
		data, ok, getErr := source.Get(row.bucket, row.key)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if ok {
			mutations = append(mutations, sqlstore.Mutation{Bucket: row.bucket, ID: row.key, Data: data})
		}
	}
	destinationRoot := t.TempDir()
	destination, err := sqlstore.Open(destinationRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := destination.Apply(context.Background(), mutations); err != nil {
		t.Fatal(err)
	}
	return destinationRoot
}

func assertIntentCodecSpikeRows(t *testing.T, leftRoot, rightRoot, id, operationID string) {
	t.Helper()
	leftRecord, leftRecordOK := intentCodecSpikeRow(t, leftRoot, issueOpsBucket, id)
	rightRecord, rightRecordOK := intentCodecSpikeRow(t, rightRoot, issueOpsBucket, id)
	if leftRecordOK != rightRecordOK || !bytes.Equal(leftRecord, rightRecord) {
		t.Fatalf("record bytes drifted\nleft=%s\nright=%s", leftRecord, rightRecord)
	}
	leftIntent, leftIntentOK := intentCodecSpikeRow(t, leftRoot, externalIntentBucket, operationID)
	rightIntent, rightIntentOK := intentCodecSpikeRow(t, rightRoot, externalIntentBucket, operationID)
	if leftIntentOK != rightIntentOK || !bytes.Equal(leftIntent, rightIntent) {
		t.Fatalf("intent bytes drifted\nleft=%s\nright=%s", leftIntent, rightIntent)
	}
}

func intentCodecSpikeRow(t *testing.T, stateRoot, bucket, id string) ([]byte, bool) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	data, ok, err := db.Get(bucket, id)
	if err != nil {
		t.Fatal(err)
	}
	return data, ok
}
