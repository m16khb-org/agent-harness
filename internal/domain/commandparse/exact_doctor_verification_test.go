package commandparse

import "testing"

func TestExactDoctorVerificationAdmitsOnlyTheRepoLocalLiteralForm(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		want    bool
	}{
		{"absolute repo", "./bin/agent-harness doctor --repo '/tmp/repo.worktrees/439' --json", true},
		{"current repo", "./bin/agent-harness doctor --repo . --json", true},
		{"PATH binary", "agent-harness doctor --repo /tmp/repo --json", false},
		{"unknown flag", "./bin/agent-harness doctor --repo /tmp/repo --cleanup-stale --json", false},
		{"shell expansion", "./bin/agent-harness doctor --repo $PWD --json", false},
		{"redirect", "./bin/agent-harness doctor --repo /tmp/repo --json > result.json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExactDoctorVerification(tc.command); got != tc.want {
				t.Fatalf("ExactDoctorVerification(%q) = %t, want %t", tc.command, got, tc.want)
			}
		})
	}
}
