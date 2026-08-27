package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func TestSealedArtifactDirUsesRecordFieldOrLegacy(t *testing.T) {
	legacy := issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/21"}
	if got := sealedArtifactDir(legacy); got != IssueOpsArtifactDir {
		t.Fatalf("no execution must resolve to legacy dir, got %s", got)
	}
	empty := issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/21", Execution: &issueops.Execution{}}
	if got := sealedArtifactDir(empty); got != IssueOpsArtifactDir {
		t.Fatalf("empty artifact_dir must resolve to legacy dir even with an issue number (old records), got %s", got)
	}
	filled := issueops.IssueOpsRecord{Execution: &issueops.Execution{Workspace: issueops.Workspace{ArtifactDir: ".agent-harness/issues/21/artifact"}}}
	if got := sealedArtifactPath(filled, "/wt", "plan"); got != filepath.Join("/wt", ".agent-harness", "issues", "21", "artifact", "plan.md") {
		t.Fatalf("artifact_dir must drive the sealed path, got %s", got)
	}
}

func TestIssueArtifactDirForUsesLinkedIssueNumber(t *testing.T) {
	if got := issueArtifactDirFor(issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/21"}); got != ".agent-harness/issues/21/artifact" {
		t.Fatalf("linked issue must pick the issue folder, got %q", got)
	}
	if got := issueArtifactDirFor(issueops.IssueOpsRecord{BranchPrepare: &issueops.IssueOpsBranchPrepare{IssueURL: "https://gitlab.example.com/g/p/-/work_items/7"}}); got != ".agent-harness/issues/7/artifact" {
		t.Fatalf("branch prepare issue URL must be a fallback, got %q", got)
	}
	if got := issueArtifactDirFor(issueops.IssueOpsRecord{}); got != "" {
		t.Fatalf("no issue number must leave artifact_dir empty (legacy), got %q", got)
	}
}

func TestWorkspaceFromReceiptRecordsArtifactDir(t *testing.T) {
	record := issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/480"}
	ws := workspaceFromReceipt(record, port.ExecutionWorkspaceReceipt{Root: "/wt", Branch: "b", Driver: "orca"}, "2026-08-27T00:00:00Z")
	if ws.ArtifactDir != ".agent-harness/issues/480/artifact" || ws.Root != "/wt" {
		t.Fatalf("workspace must carry artifact_dir: %+v", ws)
	}
}

func TestMaterializeStagedArtifactsWritesIntoRecordedArtifactDir(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	root := t.TempDir()
	record.WorktreePath = root
	record.Execution = artifactRecoveryExecution(issueops.ExecutionModeOrca, issueops.LeaseStatusReleased)
	record.Execution.Workspace.SourceRoot = record.Repo
	record.Execution.Workspace.Root = root
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = record.BranchPrepare.BaseSHA
	record.Execution.Workspace.LinkedAt = "2026-08-27T00:00:00Z"
	record.Execution.Workspace.ArtifactDir = ".agent-harness/issues/480/artifact"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# plan\n")); err != nil {
		t.Fatal(err)
	}
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	path := filepath.Join(root, ".agent-harness", "issues", "480", "artifact", "plan.md")
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("plan must be sealed 0600 at the recorded dir: %v %v", err, info)
	}
	if _, ok := manifest["plan"]; !ok {
		t.Fatalf("manifest must carry plan: %+v", manifest)
	}
	if _, err := os.Stat(filepath.Join(root, ".agent-harness", "artifact")); !os.IsNotExist(err) {
		t.Fatalf("legacy dir must not be created when artifact_dir is recorded")
	}
	// 재-materialize는 같은 내용이면 통과하고(불변 계약), 파일은 그대로다.
	if _, err := materializeStagedArtifacts(stateRoot, record); err != nil {
		t.Fatalf("idempotent re-materialize must pass: %v", err)
	}
}

func TestGatherCompletionSectionReportsMissingPlan(t *testing.T) {
	root := t.TempDir()
	record := issueops.IssueOpsRecord{Repo: root, Execution: &issueops.Execution{Workspace: issueops.Workspace{Root: root, ArtifactDir: ".agent-harness/issues/480/artifact"}}}
	completion := gatherCompletionSection(record)
	if strings.Join(completion.MissingArtifacts, ",") != "plan" {
		t.Fatalf("absent sealed plan must be reported, got %+v", completion.MissingArtifacts)
	}
	dir := filepath.Join(root, ".agent-harness", "issues", "480", "artifact")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.md"), []byte("# plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	completion = gatherCompletionSection(record)
	if len(completion.MissingArtifacts) != 0 || completion.PlanBody == "" {
		t.Fatalf("sealed plan at the recorded dir must be read: %+v", completion)
	}
}
