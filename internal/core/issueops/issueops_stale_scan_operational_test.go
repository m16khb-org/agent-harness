package issueops

import (
	"os"
	"slices"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/stalescan"
	"agent-harness/internal/core/operationalhealth"
	"agent-harness/internal/core/preflight"
)

func TestStaleScanOperationalDeadOwnerHeartbeatIsReportOnly(t *testing.T) {
	tests := []struct {
		name      string
		heartbeat func() string
		wantDead  bool
	}{
		{name: "fresh", heartbeat: func() string { return time.Now().UTC().Format(time.RFC3339Nano) }},
		{name: "missing", heartbeat: func() string { return "" }, wantDead: true},
		{name: "stale", heartbeat: func() string {
			return time.Now().Add(-operationalhealth.HeartbeatTTL - time.Minute).UTC().Format(time.RFC3339Nano)
		}, wantDead: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			stateRoot := IssueOpsStateRoot()
			_, dispatched, _ := dispatchedHandoffRecordAt(t, stateRoot)
			claimed, err := ClaimIssueOpsHandoff(stateRoot, handoffClaimRequest(dispatched))
			if err != nil {
				t.Fatal(err)
			}

			heartbeat := tt.heartbeat()
			claimed.ExecutionHandoff.LastHeartbeatAt = heartbeat
			claimed.LastHeartbeatAt = heartbeat
			if _, err := WriteIssueOps(stateRoot, claimed); err != nil {
				t.Fatal(err)
			}
			if err := BindIssueOpsSession(claimed.Repo, claimed.ID, claimed.Branch, claimed.WorktreePath); err != nil {
				t.Fatal(err)
			}

			// The apply path runs git worktree prune. Give the coordinator root a
			// valid repository without registering or deleting the live worker.
			if err := os.MkdirAll(claimed.Repo, 0o755); err != nil {
				t.Fatal(err)
			}
			if code, _, stderr := preflight.GitCmd(claimed.Repo, "init", "-q", "-b", "main"); code != 0 {
				t.Fatalf("git init coordinator failed: %s", stderr)
			}

			result := ScanStaleIssueOpsCycles(IssueOpsStaleScanRequest{Repo: claimed.Repo, Apply: true})
			if len(result.Errors) != 0 {
				t.Fatalf("operational stale scan errors: %v", result.Errors)
			}
			var finding *stalescan.Finding
			for i := range result.Findings {
				if result.Findings[i].ID == claimed.ID {
					finding = &result.Findings[i]
					break
				}
			}
			if !tt.wantDead {
				if finding != nil {
					t.Fatalf("fresh claimed heartbeat produced a stale finding: %+v", *finding)
				}
				return
			}
			if finding == nil {
				t.Fatalf("missing dead-owner finding: %+v", result.Findings)
			}
			if finding.Category != stalescan.CategoryNeedsReview {
				t.Fatalf("heartbeat-only finding category = %q, want needs-review", finding.Category)
			}
			if !slices.Contains(finding.Reasons, operationalhealth.FindingDeadOwner) {
				t.Fatalf("heartbeat-only finding reasons = %v, want %q", finding.Reasons, operationalhealth.FindingDeadOwner)
			}
			if finding.Releasable || slices.Contains(result.Released, claimed.ID) {
				t.Fatalf("heartbeat-only evidence became releasable: finding=%+v released=%v", *finding, result.Released)
			}
			after, err := ReadIssueOps(stateRoot, claimed.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.Phase == IssueOpsPhaseDone {
				t.Fatal("Apply=true released a heartbeat-only dead owner")
			}
		})
	}
}
