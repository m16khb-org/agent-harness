package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

func TestConfirmedHandoffStartCommandReplaysSealedContextOptions(t *testing.T) {
	record := IssueOpsRecord{
		ID:         "io-current",
		Repo:       "/repo/source",
		CycleState: IssueOpsCycleActive,
		Ownership: &IssueOpsOwnershipLedger{ActiveAttempt: 1, Attempts: []IssueOpsOwnershipAttempt{{
			Number: 1, Handoff: &IssueOpsExecutionHandoff{WorkspaceEpoch: "workspace-current"},
		}}},
	}
	options := handoff.ContextOptions{
		CriteriaIDs:               []string{"identity", "verification"},
		RequiredDocs:              []string{"AGENTS.md"},
		RequiredSkills:            []string{"issueops"},
		WorkerScope:               "only issue 51",
		VerificationCommands:      []string{"go test ./..."},
		HeartbeatCadence:          "after each gate",
		StopConditions:            []string{"stop on mismatch"},
		ResultFormat:              "Korean evidence report",
		AllowCodexHookTrustBypass: true,
	}

	command := confirmedHandoffStartCommand(record, "term_current", model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-current"}, options, strings.Repeat("a", 64))
	for _, fragment := range []string{
		"--criteria-id 'identity'", "--criteria-id 'verification'",
		"--required-doc 'AGENTS.md'", "--required-skill 'issueops'",
		"--worker-scope 'only issue 51'", "--verification 'go test ./...'",
		"--heartbeat-cadence 'after each gate'", "--stop-condition 'stop on mismatch'",
		"--result-format 'Korean evidence report'", "--allow-codex-hook-trust-bypass",
	} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("confirmed command omitted sealed context option %q: %s", fragment, command)
		}
	}
}

func TestProjectHandoffStartReportsSealedLaunchProfile(t *testing.T) {
	record := IssueOpsRecord{ID: "io-current", CycleState: IssueOpsCycleActive, Ownership: &IssueOpsOwnershipLedger{ActiveAttempt: 1, Attempts: []IssueOpsOwnershipAttempt{{
		Number: 1, Handoff: &IssueOpsExecutionHandoff{
			State: "ownership_dispatching", Attempt: 1, Agent: "codex",
			LaunchProfile: &model.IssueOpsAgentLaunchProfile{Agent: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high"},
		},
	}}}}

	result := projectHandoffStart(record, true, "plan-sha")
	if result.Agent != "codex" || result.Model != "gpt-5.6-terra" || result.ReasoningEffort != "high" {
		t.Fatalf("projected launch profile = %#v", result)
	}
}

func TestSealedHandoffLaunchProfileRejectsMissingAndMismatchedProfiles(t *testing.T) {
	for _, handoff := range []*IssueOpsExecutionHandoff{
		{Agent: "codex"},
		{Agent: "codex", LaunchProfile: &model.IssueOpsAgentLaunchProfile{Agent: "codex", Model: "gpt-5.6-sol", ReasoningEffort: "high"}},
	} {
		if _, err := sealedHandoffLaunchProfile(handoff); err == nil {
			t.Fatalf("invalid launch profile accepted: %#v", handoff)
		}
	}
}
