package issueopsinventory

import (
	"context"
	"errors"
	"testing"
	"time"

	issueopsapplication "agent-harness/internal/application/issueopsinventory"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

type fakeScanner struct {
	scanCalls   int
	stateRoot   string
	records     []issueopsinventorycontract.Record
	diagnostics []issueopsinventorycontract.RecordDiagnostic
	err         error
}

func (scanner *fakeScanner) Scan(
	_ context.Context,
	stateRoot string,
) ([]issueopsinventorycontract.Record, []issueopsinventorycontract.RecordDiagnostic, error) {
	scanner.scanCalls++
	scanner.stateRoot = stateRoot
	return scanner.records, scanner.diagnostics, scanner.err
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type identityNormalizer struct{}

func (identityNormalizer) Normalize(value string) string { return value }

func TestListHandlerDelegatesScanAndProjectsResult(t *testing.T) {
	scanner := &fakeScanner{
		records: []issueopsinventorycontract.Record{{ID: "io-1"}, {ID: "io-2"}},
		diagnostics: []issueopsinventorycontract.RecordDiagnostic{
			{ID: "io-broken"},
		},
	}
	handler := NewListHandler(issueopsapplication.NewService(
		scanner,
		fixedClock{now: time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)},
		identityNormalizer{},
	))

	result, err := handler("/state", "/repo/example")
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if scanner.scanCalls != 1 || scanner.stateRoot != "/state" {
		t.Fatalf("scan received wrong call: calls=%d root=%q", scanner.scanCalls, scanner.stateRoot)
	}
	if !result.OK {
		t.Fatal("list result must be ok on successful scan")
	}
	if result.ScannedRecords != 3 || result.ReadErrors != 1 {
		t.Fatalf("counts mismatch: scanned=%d readErrors=%d", result.ScannedRecords, result.ReadErrors)
	}
}

func TestListHandlerPropagatesScanError(t *testing.T) {
	scanner := &fakeScanner{err: errors.New("state store unavailable")}
	handler := NewListHandler(issueopsapplication.NewService(
		scanner,
		fixedClock{},
		identityNormalizer{},
	))

	if _, err := handler("/state", "/repo"); err == nil {
		t.Fatal("scan failure must surface through handler")
	}
}
