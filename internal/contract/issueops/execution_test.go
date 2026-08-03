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
