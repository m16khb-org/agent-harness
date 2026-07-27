package issueops

import (
	"testing"

	"agent-harness/internal/port"
)

// GitLab은 draft 상태를 제목 접두사로 표현하고 목록 API도 접두사를 포함해 반환한다.
// 접두사를 정규화하지 않으면 하네스가 자기 자신이 만든 draft MR도 채택하지 못한다.
func TestRemotePullRequestCandidateTitleStripsDraftPrefixOnlyForDrafts(t *testing.T) {
	title := "fix(gateway): guard bypass"
	cases := []struct {
		name      string
		candidate port.IssueProviderReconcilePullRequestCandidate
		want      string
	}{
		{"draft with prefix", port.IssueProviderReconcilePullRequestCandidate{Title: "Draft: " + title, Draft: true}, title},
		{"draft with legacy wip prefix", port.IssueProviderReconcilePullRequestCandidate{Title: "WIP: " + title, Draft: true}, title},
		{"draft without prefix", port.IssueProviderReconcilePullRequestCandidate{Title: title, Draft: true}, title},
		{"non draft keeps literal title", port.IssueProviderReconcilePullRequestCandidate{Title: "Draft: " + title, Draft: false}, "Draft: " + title},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := remotePullRequestCandidateTitle(tc.candidate); got != tc.want {
				t.Fatalf("title = %q, want %q", got, tc.want)
			}
		})
	}
}

// 이미 merged/closed된 아티팩트는 draft일 수 없으므로 draft 의도와의 불일치가
// 모순이 아니다. 아직 열려 있으면 기존대로 정확히 일치해야 한다.
func TestRemotePullRequestCandidateDraftMatchesAllowsSettledArtifacts(t *testing.T) {
	cases := []struct {
		name           string
		state          string
		candidateDraft bool
		expectedDraft  bool
		want           bool
	}{
		{"open draft matches draft intent", "opened", true, true, true},
		{"open non draft rejects draft intent", "opened", false, true, false},
		{"merged non draft accepts draft intent", "merged", false, true, true},
		{"closed non draft accepts draft intent", "closed", false, true, true},
		{"merged draft rejects non draft intent", "merged", true, false, false},
		{"unknown state rejects draft intent", "", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := port.IssueProviderReconcilePullRequestCandidate{Draft: tc.candidateDraft, State: tc.state}
			if got := remotePullRequestCandidateDraftMatches(candidate, tc.expectedDraft); got != tc.want {
				t.Fatalf("draft match = %v, want %v", got, tc.want)
			}
		})
	}
}
