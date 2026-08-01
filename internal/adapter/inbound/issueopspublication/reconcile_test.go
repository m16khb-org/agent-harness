package issueopspublication

import (
	"context"
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
		wantOK     bool
	}{
		{name: "success", wantOK: true},
		{name: "structured failure", serviceErr: errors.New("remote reconcile found multiple candidates; intent retained")},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeReconcileService{
				t: t, expected: true, err: test.serviceErr,
				result: publicationcontract.ReconcileResult{
					Record:     publicationcontract.RecordSnapshot{ID: "io-195", Raw: publicationRecordRaw(t)},
					Reconciled: test.wantOK, Code: "remote_reconcile_adopted", ExternalStateInspected: true,
				},
			}
			got, err := NewReconcileHandler(service)(context.Background(), "/state", fullCoreReconcileRequest())
			if err != test.serviceErr || service.id != "io-195" || got.OK != test.wantOK || got.ID != "io-195" ||
				got.Reconciled != test.wantOK || got.Code != "remote_reconcile_adopted" || !got.ExternalStateInspected {
				t.Fatalf("result=%#v serviceID=%q err=%v", got, service.id, err)
			}
			if got.Execution.Workspace.Branch != "195-publication" || got.Pending == nil ||
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
