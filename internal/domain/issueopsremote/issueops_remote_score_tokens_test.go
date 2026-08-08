package remote

import "testing"

func TestIssueOpsRemoteOverlapUsesSmallerTokenSet(t *testing.T) {
	left := map[string]bool{"refactor": true, "coverage": true, "issueops": true}
	right := map[string]bool{"coverage": true, "issueops": true}

	if got := issueOpsRemoteOverlap(left, right); got != 1 {
		t.Fatalf("issueOpsRemoteOverlap() = %v, want 1", got)
	}
}

func TestIssueOpsRemoteOverlapRejectsEmptyInputs(t *testing.T) {
	if got := issueOpsRemoteOverlap(nil, map[string]bool{"coverage": true}); got != 0 {
		t.Fatalf("issueOpsRemoteOverlap(left empty) = %v, want 0", got)
	}
	if got := issueOpsRemoteOverlap(map[string]bool{"coverage": true}, nil); got != 0 {
		t.Fatalf("issueOpsRemoteOverlap(right empty) = %v, want 0", got)
	}
}

func TestIssueOpsRemoteLabelHeuristicMatchesKnownLabels(t *testing.T) {
	tests := []struct {
		name   string
		tokens map[string]bool
		label  IssueOpsRemoteLabelCandidate
		want   float64
	}{
		{
			name:   "enhancement",
			tokens: map[string]bool{"feature": true},
			label:  IssueOpsRemoteLabelCandidate{Name: " Enhancement "},
			want:   1,
		},
		{
			name:   "bug",
			tokens: map[string]bool{"failure": true},
			label:  IssueOpsRemoteLabelCandidate{Name: "BUG"},
			want:   1,
		},
		{
			name:   "documentation",
			tokens: map[string]bool{"docs": true},
			label:  IssueOpsRemoteLabelCandidate{Name: "documentation"},
			want:   0.75,
		},
		{
			name:   "unknown",
			tokens: map[string]bool{"coverage": true},
			label:  IssueOpsRemoteLabelCandidate{Name: "refactor"},
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := issueOpsRemoteLabelHeuristic(tt.tokens, tt.label); got != tt.want {
				t.Fatalf("issueOpsRemoteLabelHeuristic() = %v, want %v", got, tt.want)
			}
		})
	}
}
