package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

func TestConfirmedHandoffStartCommandReplaysSealedContextOptions(t *testing.T) {
	record := IssueOpsRecord{
		ID:   "io-current",
		Repo: "/repo/source",
		ExecutionHandoff: &IssueOpsExecutionHandoff{
			WorkspaceEpoch: "workspace-current",
		},
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
