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

func TestValidateNativeActorAcceptsOmo(t *testing.T) {
	err := ValidateNativeActor(NativeActor{
		Host:      "omo",
		SessionID: "019ff5b8-7d62-707a-a693-5e7a5e8a3187",
		SessionProcess: &NativeProcessReceipt{
			PID: 42, StartedAt: "2026-08-12T00:00:00Z", Executable: "/Users/test/Library/pnpm/bin/omo",
		},
	})
	if err != nil {
		t.Fatalf("Omo native actor must be valid: %v", err)
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

func TestValidateExecutionAcceptsCompleteOrEmptyOrcaArtifactIdentity(t *testing.T) {
	execution := validOrcaExecutionForTest()
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("legacy empty artifact identity must remain readable: %v", err)
	}
	execution.Orca.ArtifactIdentityVersion = OrcaArtifactIdentityVersion
	execution.Orca.IssueBodySHA256 = strings.Repeat("a", 64)
	execution.Orca.ContextPacketSHA256 = strings.Repeat("b", 64)
	execution.Orca.OwnerPromptSHA256 = strings.Repeat("c", 64)
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("complete artifact identity must be valid: %v", err)
	}
}

func TestValidateExecutionRejectsPostUpgradeEmptyOrcaArtifactIdentity(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.ArtifactIdentityVersion = OrcaArtifactIdentityVersion
	if err := ValidateExecution(execution); err == nil || !strings.Contains(err.Error(), "version requires a complete sealed artifact identity") {
		t.Fatalf("post-upgrade empty artifact identity must fail as an invariant violation: %v", err)
	}
}

func TestValidateExecutionRejectsUnversionedCurrentOrcaArtifactIdentity(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.IssueBodySHA256 = strings.Repeat("a", 64)
	execution.Orca.ContextPacketSHA256 = strings.Repeat("b", 64)
	execution.Orca.OwnerPromptSHA256 = strings.Repeat("c", 64)
	if err := ValidateExecution(execution); err == nil || !strings.Contains(err.Error(), "requires artifact identity version") {
		t.Fatalf("unversioned current artifact identity must fail as an invariant violation: %v", err)
	}
}

func TestValidateExecutionRejectsPartialOrcaArtifactIdentity(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Orca.ArtifactIdentityVersion = OrcaArtifactIdentityVersion
	execution.Orca.OwnerPromptSHA256 = strings.Repeat("c", 64)
	if err := ValidateExecution(execution); err == nil || !strings.Contains(err.Error(), "complete sealed artifact identity") {
		t.Fatalf("partial artifact identity error=%v", err)
	}
}

func TestValidateExecutionAcceptsCompletionHistory(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.CompletionHistory = []ExecutionCompletionHistory{{
		Generation: 1,
		Completion: ExecutionCompletion{
			Generation: 1, FinalHead: strings.Repeat("b", 40), TuringReportPath: ".agent-harness/turing/report.json",
			Verification: []string{"go test ./... -count=1"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
		},
		Reason: "new verified HEAD", ReopenedAt: "2026-08-04T00:00:00Z",
	}}
	if err := ValidateExecution(execution); err != nil {
		t.Fatalf("valid completion history rejected: %v", err)
	}
}

func TestValidateExecutionRejectsInvalidCompletionHistory(t *testing.T) {
	valid := ExecutionCompletionHistory{
		Generation: 1,
		Completion: ExecutionCompletion{
			Generation: 1, FinalHead: strings.Repeat("b", 40), TuringReportPath: ".agent-harness/turing/report.json",
			Verification: []string{"go test ./... -count=1"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
		},
		Reason: "new verified HEAD", ReopenedAt: "2026-08-04T00:00:00Z",
	}
	for _, test := range []struct {
		name   string
		mutate func(*ExecutionCompletionHistory)
	}{
		{name: "generation", mutate: func(entry *ExecutionCompletionHistory) { entry.Generation = 0 }},
		{name: "completion", mutate: func(entry *ExecutionCompletionHistory) { entry.Completion.Verification = nil }},
		{name: "blank verification", mutate: func(entry *ExecutionCompletionHistory) { entry.Completion.Verification = []string{" "} }},
		{name: "generation conflict", mutate: func(entry *ExecutionCompletionHistory) { entry.Completion.Generation = 2 }},
		{name: "current generation", mutate: func(entry *ExecutionCompletionHistory) {
			entry.Generation = 2
			entry.Completion.Generation = 2
		}},
		{name: "future generation", mutate: func(entry *ExecutionCompletionHistory) {
			entry.Generation = 3
			entry.Completion.Generation = 3
		}},
		{name: "reason", mutate: func(entry *ExecutionCompletionHistory) { entry.Reason = " " }},
		{name: "reopened at", mutate: func(entry *ExecutionCompletionHistory) { entry.ReopenedAt = " " }},
	} {
		t.Run(test.name, func(t *testing.T) {
			entry := valid
			test.mutate(&entry)
			execution := validOrcaExecutionForTest()
			execution.CompletionHistory = []ExecutionCompletionHistory{entry}
			if err := ValidateExecution(execution); err == nil {
				t.Fatal("invalid completion history accepted")
			}
		})
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

func TestValidateExecutionSelectionRejectsExplicitDirectFallbackCode(t *testing.T) {
	execution := validOrcaExecutionForTest()
	execution.Mode = ExecutionModeDirect
	execution.Workspace.Driver = "git"
	execution.Orca = nil
	execution.Selection = &ExecutionSelection{
		RequestedMode: "direct", ResolvedMode: "direct",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:00Z",
		ExplicitDirectReason: "manual recovery",
	}
	for _, fallback := range []string{"orca_unready", " "} {
		execution.Selection.FallbackCode = fallback
		if err := ValidateExecution(execution); err == nil {
			t.Fatalf("explicit direct selection with fallback_code %q accepted", fallback)
		}
	}
}
