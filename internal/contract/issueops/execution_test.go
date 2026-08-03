package issueops

import (
	"strings"
	"testing"
)

func validOrcaExecutionForTest() Execution {
	return Execution{
		Mode: ExecutionModeOrca,
		Workspace: Workspace{
			SourceRoot: "/repo",
			Root:       "/repo.worktrees/resume",
			Branch:     "resume",
			BaseHead:   strings.Repeat("a", 40),
			Driver:     "orca",
			LinkedAt:   "2026-07-30T00:00:00Z",
		},
		Lease: WriteLease{Generation: 2, Status: LeaseStatusReleased},
		Orca: &OrcaBinding{
			RuntimeID:  "runtime-1",
			RepoID:     "repo-1",
			WorktreeID: "worktree-1",
			OwnerHost:  "codex",
			OwnerModel: "gpt-5.6-terra",
			TaskID:     "task-1",
			DispatchID: "dispatch-1",
		},
	}
}

func TestValidateExecutionAcceptsOptionalOrcaLeaseGeneration(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.LeaseGeneration = 0
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("optional Orca lease generation must remain valid: %v", err)
	}
}

func TestValidateExecutionAcceptsOptionalOrcaRunID(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.RunID = ""
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("optional Orca run id must remain valid: %v", err)
	}
}

func TestValidateExecutionAcceptsSealedOrcaRunID(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.RunID = "run_issueops_1"
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("sealed Orca run id must be valid: %v", err)
	}
}

func TestValidateExecutionAcceptsOpaqueOrcaRunID(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.RunID = "run_legacy_local"

	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("syntactically valid opaque Orca Run identity must be valid: %v", err)
	}
}

func TestValidateExecutionRejectsBindingFromFutureLeaseGeneration(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.LeaseGeneration = 3
	if err := ValidateExecution(execution); err == nil ||
		!strings.Contains(err.Error(), "Orca binding lease_generation exceeds the lease generation") {
		t.Fatalf("future binding generation must fail closed: %v", err)
	}
}

func TestValidateExecutionSelectionReceipt(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Selection = &ExecutionSelection{
		RequestedMode:        "auto",
		ResolvedMode:         "orca",
		ProbeAttempted:       true,
		ProbeAvailable:       true,
		ProbeReady:           true,
		ProbeCode:            "ready",
		ReadinessFingerprint: strings.Repeat("b", 64),
		SelectedAt:           "2026-08-03T00:00:00Z",
	}
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("valid selection receipt rejected: %v", err)
	}

	invalid := []struct {
		name   string
		mutate func(*ExecutionSelection)
	}{
		{name: "mode mismatch", mutate: func(receipt *ExecutionSelection) { receipt.ResolvedMode = "direct" }},
		{name: "auto without probe", mutate: func(receipt *ExecutionSelection) {
			receipt.ProbeAttempted = false
			receipt.ProbeAvailable = false
			receipt.ProbeReady = false
			receipt.ProbeCode = ""
		}},
		{name: "ready without available", mutate: func(receipt *ExecutionSelection) { receipt.ProbeAvailable = false }},
		{name: "fallback on orca", mutate: func(receipt *ExecutionSelection) { receipt.FallbackCode = "orca_unready" }},
		{name: "direct reason on auto", mutate: func(receipt *ExecutionSelection) { receipt.ExplicitDirectReason = "manual recovery" }},
		{name: "missing fingerprint", mutate: func(receipt *ExecutionSelection) { receipt.ReadinessFingerprint = "" }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			candidate := execution
			receipt := *execution.Selection
			test.mutate(&receipt)
			candidate.Selection = &receipt
			if err := ValidateExecution(candidate); err == nil {
				t.Fatal("invalid selection receipt accepted")
			}
		})
	}
}

func TestValidateExecutionSelectionRequiresExactAutoFallbackCode(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Mode = ExecutionModeDirect
	execution.Workspace.Driver = "git"
	execution.Orca = nil
	execution.Selection = &ExecutionSelection{
		RequestedMode: "auto", ResolvedMode: "direct", ProbeAttempted: true,
		ProbeCode: "orca_unready", FallbackCode: "orca_unready",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:00Z",
	}
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("valid auto fallback rejected: %v", err)
	}
	for _, fallback := range []string{"", "different_code", " orca_unready "} {
		candidate := execution
		selection := *execution.Selection
		selection.FallbackCode = fallback
		candidate.Selection = &selection
		if err := ValidateExecution(candidate); err == nil {
			t.Fatalf("invalid fallback_code %q accepted", fallback)
		}
	}
}
