package issueopsinventory

import (
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsinventorycontract "agent-harness/internal/contract/issueopsinventory"
)

// ProjectEntry는 list CLI가 렌더하는 전부다. 레코드의 실행/원격/정리 상태가
// 엔트리 필드로 정확히 투영되는지 잠근다.
func TestProjectEntryProjectsExecutionAndRemoteState(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-proj", Repo: "/repo", Branch: "12-proj",
		Phase: issueopscontract.IssueOpsPhaseImplement, UpdatedAt: "2026-08-21T00:00:00Z",
		Execution: &issueopscontract.Execution{
			Mode:      issueopscontract.ExecutionModeDirect,
			Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/12-proj"},
			Lease: issueopscontract.WriteLease{
				Status: issueopscontract.LeaseStatusClaimable,
			},
			Orca:    &issueopscontract.OrcaBinding{OwnerModel: "gpt-5.6-sol"},
			Pending: &issueopscontract.ExternalIntent{Kind: "gh_run", StartedAt: "2026-08-21T01:00:00Z"},
			Failure: &issueopscontract.ExecutionFailure{Code: "E_TEST", At: "2026-08-21T02:00:00Z"},
		},
		RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{
			URL: "https://github.com/acme/repo/pull/12",
		},
	}
	entry := ProjectEntry(record)
	if entry.ID != "io-proj" || entry.Repo != "/repo" || entry.Branch != "12-proj" ||
		string(entry.Phase) != "implement" || entry.UpdatedAt != "2026-08-21T00:00:00Z" {
		t.Fatalf("base projection wrong: %+v", entry)
	}
	if entry.Mode != "direct" || entry.LeaseStatus != "claimable" || !entry.Claimable {
		t.Fatalf("lease projection wrong: %+v", entry)
	}
	if entry.WorkspaceRoot != "/repo.worktrees/12-proj" || entry.OwnerModel != "gpt-5.6-sol" {
		t.Fatalf("workspace/orca projection wrong: %+v", entry)
	}
	if entry.PendingKind != "gh_run" || entry.PendingSince != "2026-08-21T01:00:00Z" {
		t.Fatalf("pending projection wrong: %+v", entry)
	}
	if entry.FailureCode != "E_TEST" || entry.FailureAt != "2026-08-21T02:00:00Z" {
		t.Fatalf("failure projection wrong: %+v", entry)
	}
	if entry.RemoteArtifactURL != "https://github.com/acme/repo/pull/12" || !entry.CompletionUnreflected {
		t.Fatalf("remote artifact projection wrong: %+v", entry)
	}
	if entry.CleanupCandidate {
		t.Fatalf("implement phase must not be a cleanup candidate: %+v", entry)
	}
}

func TestProjectEntryProjectsHolderAndCompletionReflection(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-holder", Phase: issueopscontract.IssueOpsPhasePR,
		Execution: &issueopscontract.Execution{
			Lease: issueopscontract.WriteLease{
				Status: issueopscontract.LeaseStatusActive,
				Holder: &issueopscontract.NativeActor{Host: "omo", SessionID: "sess-1"},
			},
		},
		RemoteArtifact:   &issueopscontract.IssueOpsRemoteArtifactVerification{URL: "u"},
		RemoteCompletion: &issueopscontract.IssueOpsRemoteCompletion{ReflectedAt: "2026-08-21T03:00:00Z"},
	}
	entry := ProjectEntry(record)
	if entry.HolderHost != "omo" || entry.HolderSession != "sess-1" {
		t.Fatalf("holder projection wrong: %+v", entry)
	}
	if entry.CompletionUnreflected || entry.Claimable {
		t.Fatalf("reflected completion must clear unreflected flag: %+v", entry)
	}

	record.Phase = issueopsinventorycontract.PhaseDone
	if entry := ProjectEntry(record); !entry.CleanupCandidate {
		t.Fatalf("done phase must be a cleanup candidate: %+v", entry)
	}
}

func TestProjectEntryProjectsCleanupFailuresAndIssueIntent(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{
		ID: "io-cleanup", Phase: issueopscontract.IssueOpsPhasePR,
		CleanupFinishFailure: &issueopscontract.IssueOpsCleanupFinishFailure{
			Step: "close_issue", At: "t1",
		},
		IssueCreateIntent: &issueopscontract.IssueOpsIssueCreateIntent{Status: "pending"},
	}
	entry := ProjectEntry(record)
	if entry.CleanupFailureStep != "close_issue" || entry.CleanupFailureAt != "t1" {
		t.Fatalf("cleanup failure projection wrong: %+v", entry)
	}
	if entry.IssueCreateStatus != "pending" {
		t.Fatalf("issue create status projection wrong: %+v", entry)
	}
	record.CleanupFinishFailure = nil
	record.CleanupAbandonFailure = &issueopscontract.IssueOpsCleanupAbandonFailure{
		Step: "branch_delete", At: "t2",
	}
	entry = ProjectEntry(record)
	if entry.CleanupFailureStep != "branch_delete" || entry.CleanupFailureAt != "t2" {
		t.Fatalf("abandon failure projection wrong: %+v", entry)
	}
	// 완료된 issue create intent은 list에 노출하지 않는다.
	record.IssueCreateIntent.Status = "completed"
	if entry := ProjectEntry(record); entry.IssueCreateStatus != "" {
		t.Fatalf("completed intent must not surface: %+v", entry)
	}
}

func TestNormalizeID(t *testing.T) {
	for _, ok := range []string{"io-123", "  io-123  "} {
		if got, err := NormalizeID(ok); err != nil || got != "io-123" {
			t.Fatalf("NormalizeID(%q) = %q, %v", ok, got, err)
		}
	}
	for _, bad := range []string{"", "  ", "../etc", "a/b", "a\\b"} {
		if _, err := NormalizeID(bad); err == nil || !strings.Contains(err.Error(), "invalid issueops id") {
			t.Fatalf("NormalizeID(%q) must fail closed: %v", bad, err)
		}
	}
}
