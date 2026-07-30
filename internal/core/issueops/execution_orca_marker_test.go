package issueops

import (
	"testing"

	"agent-harness/internal/core/issueops/model"
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
