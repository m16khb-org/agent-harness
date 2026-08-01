package issueopspublication

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	publicationcontract "agent-harness/internal/contract/issueopspublication"
	"agent-harness/internal/core/issueops"
)

func TestReconcileHandlerProjectsRawRecordAndPreservesResult(t *testing.T) {
	for _, test := range []struct {
		name       string
		serviceErr error
		reconciled bool
		wantOK     bool
		wantBranch string
	}{
		{name: "success", reconciled: true, wantOK: true, wantBranch: "195-publication"},
		{name: "structured failure", serviceErr: errors.New("remote reconcile found multiple candidates; intent retained"), wantBranch: "prior-publication"},
		{name: "reconciled with terminal error", serviceErr: errors.New("retry was not invoked"), reconciled: true, wantOK: true, wantBranch: "195-publication"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := fullCoreReconcileRequest()
			if err := json.Unmarshal(publicationRecordRaw(t), request.Snapshot); err != nil {
				t.Fatal(err)
			}
			request.Snapshot.Execution.Workspace.Branch = "prior-publication"
			service := &fakeReconcileService{
				t: t, expected: true, err: test.serviceErr,
				result: publicationcontract.ReconcileResult{
					Record:     publicationcontract.RecordSnapshot{ID: "io-195", Raw: publicationRecordRaw(t)},
					Reconciled: test.reconciled, Code: "remote_reconcile_adopted", ExternalStateInspected: true,
				},
			}
			got, err := NewReconcileHandler(service)(context.Background(), "/state", request)
			if err != test.serviceErr || service.id != "io-195" || got.OK != test.wantOK || got.ID != "io-195" ||
				got.Reconciled != test.reconciled || got.Code != "remote_reconcile_adopted" || !got.ExternalStateInspected {
				t.Fatalf("result=%#v serviceID=%q err=%v", got, service.id, err)
			}
			if got.Execution.Workspace.Branch != test.wantBranch || got.Pending == nil ||
				got.Pending.OperationID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || got.Pending != got.Execution.Pending {
				t.Fatalf("execution projection=%#v pending=%#v", got.Execution, got.Pending)
			}
		})
	}
}

func TestReconcileHandlerRejectsMalformedRecordProjection(t *testing.T) {
	service := &fakeReconcileService{
		t: t, expected: true,
		result: publicationcontract.ReconcileResult{Record: publicationcontract.RecordSnapshot{ID: "io-195", Raw: []byte("{")}},
	}
	got, err := NewReconcileHandler(service)(context.Background(), "/state", fullCoreReconcileRequest())
	if err == nil || !strings.Contains(err.Error(), "decode publication record snapshot") || got.ID != "io-195" {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}

func TestReconcileHandlerFailsClosedWithoutService(t *testing.T) {
	got, err := NewReconcileHandler(nil)(context.Background(), "/state", fullCoreReconcileRequest())
	if !errors.Is(err, issueops.ErrRemotePullRequestReconcileHandlerUnavailable) || got.ID != "io-195" || got.OK {
		t.Fatalf("result=%#v err=%v", got, err)
	}
}
