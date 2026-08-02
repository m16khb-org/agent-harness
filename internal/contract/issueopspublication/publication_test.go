package issueopspublication

import (
	"strings"
	"testing"
)

func TestIntentClonePreservesAllAuthorityWithoutAliasing(t *testing.T) {
	original := Intent{
		Record:      RecordSnapshot{ID: "io-1", Raw: []byte("{\"schema_version\":1}")},
		OperationID: "op-1",
		Generation:  7,
		Provider:    "github",
		Kind:        "pr",
		Request: ProviderCreateRequest{
			Repo: "/repo", ProjectKey: "github.com/acme/repo", Title: "title", Body: "body",
			HeadBranch: "195-branch", BaseBranch: "117-parent", Labels: []string{"enhancement"},
			Assignees: []string{"maintainer"}, Draft: true, ExpectedHeadSHA: strings.Repeat("a", 40), Confirm: true,
			Host: "codex", SessionID: "session", AgentID: "agent", CWD: "/repo.worktrees/195-branch",
		},
		InvocationState: InvocationUnknown,
		RetryCount:      0,
		Eligibility: CreateEligibility{
			Provider: "github", Kind: "pr", Confirm: true, PhasePR: true,
			ExecutionActive: true, NoPending: true, NoArtifact: true,
			BranchAuthority: true, CanonicalLabelsAssignees: true,
		},
		Raw: []byte("{\"schema_version\":1}"),
	}
	cloned := original.Clone()

	original.Record.Raw[0] = 'x'
	original.Raw[0] = 'x'
	original.Request.Labels[0] = "changed"
	original.Request.Assignees[0] = "changed"

	if string(cloned.Record.Raw) != "{\"schema_version\":1}" ||
		string(cloned.Raw) != "{\"schema_version\":1}" ||
		cloned.Request.Labels[0] != "enhancement" ||
		cloned.Request.Assignees[0] != "maintainer" {
		t.Fatalf("clone aliased mutable input: %#v", cloned)
	}
}

func TestMutableProjectionClonesDoNotAliasInput(t *testing.T) {
	process := &ProcessReceipt{PID: 42, StartedAt: "2026-08-01T00:00:00Z", Executable: "/usr/bin/codex"}
	command := CreateCommand{
		ID: "io-1", Provider: "github", Labels: []string{"enhancement"}, Assignees: []string{"maintainer"},
		Actor: Actor{
			Host: "codex", SessionID: "session", SessionProcess: process,
			ProcessAncestry: []ProcessReceipt{{PID: 41, StartedAt: "2026-07-31T23:59:59Z", Executable: "/usr/bin/parent"}},
		},
	}
	commandClone := command.Clone()
	command.Labels[0] = "changed"
	command.Assignees[0] = "changed"
	command.Actor.SessionProcess.PID = 99
	command.Actor.ProcessAncestry[0].PID = 98
	if commandClone.Labels[0] != "enhancement" ||
		commandClone.Assignees[0] != "maintainer" ||
		commandClone.Actor.SessionProcess.PID != 42 ||
		commandClone.Actor.ProcessAncestry[0].PID != 41 {
		t.Fatalf("create command clone aliased mutable input: %#v", commandClone)
	}

	prepared := PreparedCreate{
		Request: ProviderCreateRequest{Labels: []string{}, Assignees: []string{"maintainer"}},
	}
	preparedClone := prepared.Clone()
	prepared.Request.Assignees[0] = "changed"
	if preparedClone.Request.Labels == nil || preparedClone.Request.Assignees[0] != "maintainer" {
		t.Fatalf("prepared create clone lost slice shape or aliased input: %#v", preparedClone)
	}

	inventory := Inventory{Candidates: []Candidate{{
		URL: "https://github.com/acme/repo/pull/1", Labels: []string{"enhancement"}, Assignees: []string{"maintainer"},
	}}}
	inventoryClone := inventory.Clone()
	inventory.Candidates[0].URL = "changed"
	inventory.Candidates[0].Labels[0] = "changed"
	inventory.Candidates[0].Assignees[0] = "changed"
	if inventoryClone.Candidates[0].URL != "https://github.com/acme/repo/pull/1" ||
		inventoryClone.Candidates[0].Labels[0] != "enhancement" ||
		inventoryClone.Candidates[0].Assignees[0] != "maintainer" {
		t.Fatalf("inventory clone aliased mutable input: %#v", inventoryClone)
	}

	result := ReconcileResult{Record: RecordSnapshot{ID: "io-1", Raw: []byte("{\"ok\":true}")}}
	resultClone := result.Clone()
	result.Record.Raw[0] = 'x'
	if string(resultClone.Record.Raw) != "{\"ok\":true}" {
		t.Fatalf("reconcile result clone aliased record bytes: %#v", resultClone)
	}
}
