package issueopsinventory

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestRepositoryListsAndStrictlyReadsRecords(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-valid",
		Repo:          "/repo",
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), record.ID, encoded); err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), "io-invalid", []byte(`{"schema_version":1,"id":"io-invalid","phase":"problem","unknown":true}`)); err != nil {
		t.Fatal(err)
	}

	repository := Repository{}
	records, diagnostics, err := repository.Scan(context.Background(), stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != record.ID || records[0].Repo != record.Repo {
		t.Fatalf("unexpected records: %+v", records)
	}
	if len(diagnostics) != 1 ||
		diagnostics[0].ID != "io-invalid" ||
		diagnostics[0].Code != "invalid_state" {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
}

func TestRepositoryHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repository := Repository{}
	if _, _, err := repository.Scan(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("scan error = %v", err)
	}
}

func TestRepositoryListIDsAndReadUnchecked(t *testing.T) {
	stateRoot := t.TempDir()
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	record := issueopscontract.IssueOpsRecord{
		OK:            true,
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-list",
		Repo:          "/repo",
		Phase:         issueopscontract.IssueOpsPhaseProblem,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Put(issueopsrecord.Bucket(), record.ID, encoded); err != nil {
		t.Fatal(err)
	}
	repository := Repository{}
	ids, err := repository.ListIDs(context.Background(), stateRoot)
	if err != nil || len(ids) != 1 || ids[0] != record.ID {
		t.Fatalf("ListIDs = %v, %v", ids, err)
	}
	got, err := repository.ReadUnchecked(context.Background(), stateRoot, record.ID)
	if err != nil || got.ID != record.ID || got.Repo != "/repo" {
		t.Fatalf("ReadUnchecked = %#v, %v", got, err)
	}
	// ListIDs는 문맥 취소를 존중해야 한다.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repository.ListIDs(ctx, stateRoot); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ListIDs error = %v", err)
	}
}

func TestCleanPathNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"/repo//worktrees/x/", "/repo/worktrees/x"},
	}
	for _, tc := range cases {
		if got := (CleanPath{}).Normalize(tc.in); got != tc.want {
			t.Fatalf("Normalize(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
	// 상대 경로는 절대 경로로 정규화된다(프로세스 cwd 기준).
	rel := (CleanPath{}).Normalize("relative/path")
	if !filepath.IsAbs(rel) || !strings.HasSuffix(rel, "relative/path") {
		t.Fatalf("relative normalization = %q", rel)
	}
}

func TestSystemClockNowAdvances(t *testing.T) {
	clock := SystemClock{}
	before := clock.Now()
	if after := clock.Now(); after.Before(before) {
		t.Fatalf("clock must not go backwards: %v < %v", after, before)
	}
}
