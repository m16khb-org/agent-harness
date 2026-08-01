package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestOrcaIntentMarkerRoundTripsProviderAndPurposeIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity orcaIntentMarkerIdentity
		want     string
	}{
		{
			name: "github prepare",
			identity: orcaIntentMarkerIdentity{
				Purpose: orcaIntentPurposePrepare, LifecycleID: "io-aaaaaaaaaaaa",
				Generation: 1, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Provider: "github", Issue: 69,
			},
			want: "agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69",
		},
		{
			name: "gitlab resume",
			identity: orcaIntentMarkerIdentity{
				Purpose: orcaIntentPurposeResume, LifecycleID: "io-aaaaaaaaaaaa",
				Generation: 2, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Provider: "gitlab", Issue: 2646,
			},
			want: "agent-harness issueops-v1 resume lifecycle=io-aaaaaaaaaaaa generation=2 operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab issue=2646",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := renderOrcaIntentMarker(test.identity)
			if err != nil || got != test.want {
				t.Fatalf("render = %q err=%v, want %q", got, err, test.want)
			}
			parsed, err := parseOrcaIntentMarker(got)
			if err != nil || parsed != test.identity {
				t.Fatalf("parse = %#v err=%v, want %#v", parsed, err, test.identity)
			}
		})
	}
}

func TestOrcaIntentMarkerRejectsPartialDuplicateAndUnknownIdentity(t *testing.T) {
	for _, marker := range []string{
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb issue=69",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github provider=gitlab issue=69",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=0",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=bitbucket issue=69",
		"agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69 extra=value",
	} {
		if _, err := parseOrcaIntentMarker(marker); err == nil {
			t.Fatalf("invalid marker was accepted: %q", marker)
		}
	}
}

func TestLegacyOrcaIntentMarkerParserNeverInventsIssueIdentity(t *testing.T) {
	tests := []struct {
		marker     string
		purpose    string
		generation uint64
	}{
		{
			marker:     "agent-harness issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			purpose:    orcaIntentPurposePrepare,
			generation: 1,
		},
		{
			marker:     "agent-harness issueops-v1 resume lifecycle=io-aaaaaaaaaaaa generation=2 operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			purpose:    orcaIntentPurposeResume,
			generation: 2,
		},
	}
	for _, test := range tests {
		parsed, err := parseLegacyOrcaIntentMarker(test.marker)
		if err != nil {
			t.Fatalf("parse legacy %q: %v", test.marker, err)
		}
		if parsed.Purpose != test.purpose || parsed.Generation != test.generation ||
			parsed.Provider != "" || parsed.Issue != 0 {
			t.Fatalf("legacy parser invented identity: %#v", parsed)
		}
	}
}

func TestAuthoritativeOrcaIssueIdentityRequiresVerifiedMatchingRecord(t *testing.T) {
	valid := IssueOpsRecord{
		IssueURL: "https://gitlab.example.com/acme/repo/-/work_items/2646",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "gitlab", IssueURL: "https://gitlab.example.com/acme/repo/-/work_items/2646",
			LinkVerified: true,
		},
	}
	identity, err := authoritativeOrcaIssueIdentity(valid)
	if err != nil || identity != (orcaIssueIdentity{Provider: "gitlab", Issue: 2646}) {
		t.Fatalf("verified identity = %#v err=%v", identity, err)
	}

	tests := []struct {
		name   string
		mutate func(*IssueOpsRecord)
	}{
		{name: "missing branch prepare", mutate: func(record *IssueOpsRecord) { record.BranchPrepare = nil }},
		{name: "unverified link", mutate: func(record *IssueOpsRecord) { record.BranchPrepare.LinkVerified = false }},
		{name: "provider URL mismatch", mutate: func(record *IssueOpsRecord) { record.BranchPrepare.Provider = "github" }},
		{name: "record URL drift", mutate: func(record *IssueOpsRecord) {
			record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/2647"
		}},
		{name: "non-positive issue", mutate: func(record *IssueOpsRecord) {
			record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/0"
			record.BranchPrepare.IssueURL = record.IssueURL
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := valid
			branchPrepare := *valid.BranchPrepare
			record.BranchPrepare = &branchPrepare
			test.mutate(&record)
			if _, err := authoritativeOrcaIssueIdentity(record); err == nil {
				t.Fatalf("invalid record identity was accepted: %#v", record.BranchPrepare)
			}
		})
	}
}

func TestOrcaPrepareIssueIdentityAllowsOnlyUnverifiedGitHub(t *testing.T) {
	github := IssueOpsRecord{
		IssueURL: "https://github.com/acme/repo/issues/194",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/194",
		},
	}
	identity, err := orcaPrepareIssueIdentity(github)
	if err != nil || identity != (orcaIssueIdentity{Provider: "github", Issue: 194}) {
		t.Fatalf("GitHub prepare identity = %#v err=%v", identity, err)
	}

	gitlab := github
	gitlab.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/194"
	prepared := *github.BranchPrepare
	prepared.Provider = "gitlab"
	prepared.IssueURL = gitlab.IssueURL
	gitlab.BranchPrepare = &prepared
	if _, err := orcaPrepareIssueIdentity(gitlab); err == nil {
		t.Fatal("미검증 GitLab branch identity가 Orca prepare에 허용됐다")
	}
}

func TestOrcaIntentIssueIdentityRequiresVerifiedLinkForResume(t *testing.T) {
	record := IssueOpsRecord{
		ID:       "io-aaaaaaaaaaaa",
		IssueURL: "https://github.com/acme/repo/issues/194",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/194",
		},
	}
	payload := externalOrcaIntentPayload{
		Purpose: orcaIntentPurposeResume, LifecycleID: record.ID,
		Probe: intentContractProbeRequest(port.ExecutionOrcaProbeRequest{Provider: "github", Issue: 194}),
	}
	if err := validateOrcaIntentIssueIdentity(record, payload); err == nil {
		t.Fatal("미검증 branch link로 resume intent identity가 허용됐다")
	}
}

func TestOrcaIntentContractErrorExposesStableCodeWithoutDuplicatingDetail(t *testing.T) {
	err := &orcaIntentContractError{Code: "intent_marker_invalid", Detail: "duplicate provider"}
	if got := err.Error(); got != "intent_marker_invalid: duplicate provider" {
		t.Fatalf("error = %q", got)
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "intent_marker_invalid" || len(fields) != 1 {
		t.Fatalf("error fields = %#v", fields)
	}
}

func TestSealExternalOrcaIntentPayloadUsesTheVerifiedRecordIdentity(t *testing.T) {
	_, record := executionPrepareRecord(t)
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	payload := externalOrcaIntentPayload{
		SchemaVersion:   model.IssueOpsSchemaVersion,
		Purpose:         orcaIntentPurposePrepare,
		OperationID:     strings.Repeat("b", 32),
		LifecycleID:     record.ID,
		Generation:      1,
		Stage:           intentContractStage(port.ExecutionOrcaIntentWorktree),
		StartedAt:       "2026-07-30T00:00:00Z",
		InvocationState: orcaIntentNotInvoked,
		Workspace:       intentContractWorkspaceRequest(workspace),
		Probe: intentContractProbeRequest(port.ExecutionOrcaProbeRequest{
			Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
			Provider: "gitlab", Issue: 2646,
		}),
		IssueBodySHA256: strings.Repeat("a", 64),
	}

	sealed, err := sealExternalOrcaIntentPayload(record, payload)
	if err != nil {
		t.Fatal(err)
	}
	if sealed.Probe.Provider != "github" || sealed.Probe.Issue != 16 {
		t.Fatalf("sealed issue identity = %s#%d", sealed.Probe.Provider, sealed.Probe.Issue)
	}
	wantSuffix := " provider=github issue=16"
	if !strings.HasSuffix(sealed.Marker, wantSuffix) || sealed.Probe.Marker != sealed.Marker {
		t.Fatalf("sealed markers = payload:%q probe:%q", sealed.Marker, sealed.Probe.Marker)
	}
}

func TestBeginOrcaExecutionResumeIntentSealsGitLabIdentity(t *testing.T) {
	stateRoot, record, _ := reseededOrcaCycle(t)
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/2646"
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.IssueURL = record.IssueURL
	record, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := executionResumeArtifacts{
		claimTokenPath:  record.Execution.Workspace.Root + "/claim-token",
		issueBodySHA256: strings.Repeat("a", 64),
		packetPath:      record.Execution.Workspace.Root + "/context.json",
		packetSHA256:    strings.Repeat("b", 64),
		promptPath:      record.Execution.Workspace.Root + "/prompt.md",
		promptSHA256:    strings.Repeat("c", 64),
	}

	persisted, payload, err := beginOrcaExecutionResumeIntent(
		stateRoot, record, artifacts, record.Execution.Orca.RuntimeID, "", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := " provider=gitlab issue=2646"
	if !strings.HasSuffix(payload.Marker, wantSuffix) || payload.Probe.Marker != payload.Marker {
		t.Fatalf("resume markers = payload:%q probe:%q", payload.Marker, payload.Probe.Marker)
	}
	if persisted.Execution.Pending == nil || persisted.Execution.Pending.Marker != payload.Marker {
		t.Fatalf("persisted pending = %#v", persisted.Execution.Pending)
	}
}

func TestBeginOrcaExecutionIntentRejectsRecordIdentityDriftBeforePersistence(t *testing.T) {
	stateRoot, record := orcaPrepareRecord(t)
	workspace, err := executionWorkspaceRequest(record, true)
	if err != nil {
		t.Fatal(err)
	}
	passed := record
	record.IssueURL = "https://github.com/acme/repo/issues/17"
	record.BranchPrepare.IssueURL = record.IssueURL
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	probe := port.ExecutionOrcaProbeRequest{
		Repo: record.Repo, Host: "codex", Model: "gpt-5.6-terra", Effort: "xhigh",
		Provider: "github", Issue: 16,
	}
	snapshot := executionOwnerSnapshot{issue: executionOwnerIssue{BodySHA256: strings.Repeat("a", 64)}}

	_, _, err = beginOrcaExecutionIntent(stateRoot, passed, workspace, probe, ExecutionPrepareRequest{
		OwnerHost: "codex", OwnerModel: "gpt-5.6-terra", OwnerEffort: "xhigh",
	}, snapshot, nil)
	if err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("identity drift error = %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.Execution != nil {
		t.Fatalf("identity drift persisted execution: %#v", persisted.Execution)
	}
	rows, rowsErr := sqlstore.GetAllExisting(stateRoot, externalIntentBucket)
	if rowsErr != nil {
		t.Fatal(rowsErr)
	}
	if len(rows) != 0 {
		t.Fatalf("identity drift persisted external intents: %#v", rows)
	}
}
