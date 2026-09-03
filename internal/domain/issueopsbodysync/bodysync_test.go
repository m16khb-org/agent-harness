package issueopsbodysync

import (
	"errors"
	"strings"
	"testing"

	contract "agent-harness/internal/contract/issueopsbodysync"
)

const (
	completionBlock = "<!-- issueops:completion:start -->\n## 완료 기록\n<!-- issueops:completion:end -->"
	advocateBlock   = "<!-- issueops:devils-advocate:start -->\n- 반론\n<!-- issueops:devils-advocate:end -->"
	createMarker    = "<!-- agent-harness:issue-create:0123456789abcdef0123456789abcdef -->"
)

func TestSHA256BodyIsStableAndDistinct(t *testing.T) {
	if got, want := SHA256Body(""), SHA256Body(""); got != want {
		t.Fatalf("sha must be deterministic")
	}
	if len(SHA256Body("본문")) != 64 {
		t.Fatalf("sha must be 64 hex characters, got %q", SHA256Body("본문"))
	}
	if SHA256Body("a") == SHA256Body("b") {
		t.Fatalf("distinct bodies must hash differently")
	}
}

func TestMergePreservesEveryManagedRegion(t *testing.T) {
	tests := []struct {
		name  string
		live  string
		names []string
	}{
		{"none", "## 문제\n옛 본문\n", nil},
		{"completion", "## 문제\n옛 본문\n\n" + completionBlock + "\n", []string{"issueops:completion"}},
		{"advocate", "## 문제\n옛 본문\n\n" + advocateBlock + "\n", []string{"issueops:devils-advocate"}},
		{"create marker", createMarker + "\n\n## 문제\n옛 본문\n", []string{"agent-harness:issue-create"}},
		{
			"all three in appearance order",
			createMarker + "\n\n## 문제\n옛 본문\n\n" + advocateBlock + "\n\n" + completionBlock + "\n",
			[]string{"agent-harness:issue-create", "issueops:devils-advocate", "issueops:completion"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, preserved, err := Merge(tt.live, "## 문제\n새 본문\n")
			if err != nil {
				t.Fatalf("merge: %v", err)
			}
			if !strings.Contains(merged, "새 본문") {
				t.Fatalf("merged body must carry the proposal: %q", merged)
			}
			if strings.Contains(merged, "옛 본문") {
				t.Fatalf("merged body must drop the previous authored content: %q", merged)
			}
			if len(preserved) != len(tt.names) {
				t.Fatalf("preserved %d regions, want %d: %+v", len(preserved), len(tt.names), preserved)
			}
			for i, name := range tt.names {
				if preserved[i].Name != name {
					t.Fatalf("region %d is %q, want %q", i, preserved[i].Name, name)
				}
				if !strings.Contains(merged, preserved[i].Block) {
					t.Fatalf("merged body dropped region %q", name)
				}
			}
		})
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	live := createMarker + "\n\n## 문제\n옛 본문\n\n" + completionBlock + "\n"
	proposed := "## 문제\n새 본문\n"
	once, _, err := Merge(live, proposed)
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	twice, _, err := Merge(once, proposed)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if once != twice {
		t.Fatalf("merge must be idempotent:\nfirst:  %q\nsecond: %q", once, twice)
	}
}

func TestMergeRejectsUnusableProposals(t *testing.T) {
	tests := []struct {
		name     string
		proposed string
		want     string
	}{
		{"empty", "   \n\t\n", "body is empty"},
		{"carries completion marker", "## 문제\n" + completionBlock, "managed section"},
		{"carries advocate marker", "## 문제\n" + advocateBlock, "managed section"},
		{"carries create marker", createMarker + "\n## 문제\n본문", "managed section"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Merge("## 문제\n옛 본문\n", tt.proposed)
			if err == nil {
				t.Fatalf("expected a refusal for %s", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q must mention %q", err, tt.want)
			}
		})
	}
}

func TestClassifyDrift(t *testing.T) {
	tests := []struct {
		name                   string
		recorded, live, merged string
		want                   contract.Drift
	}{
		{"no baseline and unchanged", "", "a", "a", contract.DriftInSync},
		{"no baseline and changed", "", "a", "b", contract.DriftStale},
		{"recorded matches live and merged", "a", "a", "a", contract.DriftInSync},
		{"recorded matches live only", "a", "a", "b", contract.DriftStale},
		{"recorded differs from live", "a", "b", "c", contract.DriftRemoteEdited},
		{"recorded differs but merged equals live", "a", "b", "b", contract.DriftRemoteEdited},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyDrift(tt.recorded, tt.live, tt.merged); got != tt.want {
				t.Fatalf("drift = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildPlanReportsDigestsAndRegions(t *testing.T) {
	live := "## 문제\n옛 본문\n\n" + completionBlock + "\n"
	plan, err := BuildPlan(SHA256Body(live), live, "## 문제\n새 본문\n")
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Drift != contract.DriftStale {
		t.Fatalf("drift = %q, want stale", plan.Drift)
	}
	if plan.RemoteBodySHA256 != SHA256Body(live) {
		t.Fatalf("remote sha must hash the live body")
	}
	if plan.MergedBodySHA256 != SHA256Body(plan.MergedBody) {
		t.Fatalf("merged sha must hash the merged body")
	}
	if len(plan.PreservedSections) != 1 || plan.PreservedSections[0] != "issueops:completion" {
		t.Fatalf("preserved sections = %v", plan.PreservedSections)
	}
}

func TestValidateWriteIsFailClosed(t *testing.T) {
	stale := contract.Plan{Drift: contract.DriftStale, RemoteBodySHA256: "live"}
	edited := contract.Plan{Drift: contract.DriftRemoteEdited, RemoteBodySHA256: "live"}
	inSync := contract.Plan{Drift: contract.DriftInSync, RemoteBodySHA256: "live"}

	if err := ValidateWrite(stale, "", false); err == nil ||
		!strings.Contains(err.Error(), "expected-body-sha256") {
		t.Fatalf("a missing expectation must be refused, got %v", err)
	}
	if err := ValidateWrite(stale, "other", false); err == nil ||
		!strings.Contains(err.Error(), "changed") {
		t.Fatalf("a stale expectation must be refused, got %v", err)
	}
	if err := ValidateWrite(edited, "live", false); err == nil ||
		!strings.Contains(err.Error(), "accept-remote-edits") {
		t.Fatalf("an unacknowledged remote edit must be refused, got %v", err)
	}
	if err := ValidateWrite(edited, "live", true); err != nil {
		t.Fatalf("an acknowledged remote edit must pass, got %v", err)
	}
	if err := ValidateWrite(stale, "live", false); err != nil {
		t.Fatalf("a matching expectation must pass, got %v", err)
	}
	if err := ValidateWrite(inSync, "live", false); !errors.Is(err, ErrAlreadyInSync) {
		t.Fatalf("an in-sync plan must report ErrAlreadyInSync, got %v", err)
	}
}

func TestNormalizeBodyFoldsProviderRoundTrips(t *testing.T) {
	local := "## 문제\n본문"
	stored := "## 문제\r\n본문\r\n\r\n"
	if NormalizeBody(stored) != local {
		t.Fatalf("normalize(%q) = %q, want %q", stored, NormalizeBody(stored), local)
	}
	if SHA256Body(local) != SHA256Body(stored) {
		t.Fatalf("a CRLF round trip must not read as an edit")
	}
}

func TestMergedBodySurvivesAProviderRoundTrip(t *testing.T) {
	live := "## 문제\n옛 본문\n\n" + completionBlock + "\n"
	plan, err := BuildPlan(SHA256Body(live), live, "## 문제\n새 본문\n")
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	stored := strings.ReplaceAll(plan.MergedBody, "\n", "\r\n") + "\r\n"
	next, err := BuildPlan(plan.MergedBodySHA256, stored, "## 문제\n새 본문\n")
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}
	if next.Drift != contract.DriftInSync {
		t.Fatalf("re-syncing an unchanged artifact must be in_sync, got %q", next.Drift)
	}
}
