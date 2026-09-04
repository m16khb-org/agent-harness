package issueops

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsartifactdomain "issueops/internal/domain/issueopsartifact"
)

func TestRequireStagedExecutionOwnerPlanArtifact(t *testing.T) {
	tests := []struct {
		name          string
		stage         map[string]string
		configure     func(*testing.T, *issueopscontract.IssueOpsRecord)
		wantIdentity  PlanIdentity
		wantError     bool
		wantNext      bool
		wantExactNext bool
	}{
		{name: "no staged artifacts", wantError: true},
		{name: "spec only", stage: map[string]string{"spec": "# Spec\n"}, wantError: true},
		{
			name:         "fresh staged plan needs no durable path",
			stage:        map[string]string{"plan": "# Plan\n"},
			wantIdentity: PlanIdentity{Digest: digestExecutionOwnerBytes([]byte("# Plan\n"))},
		},
		{
			name:  "prelinked plan is missing",
			stage: map[string]string{"plan": "# Plan\n"},
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "missing.md")
			},
			wantError: true,
		},
		{
			name:  "prelinked plan is a directory",
			stage: map[string]string{"plan": "# Plan\n"},
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = record.WorktreePath
			},
			wantError: true,
		},
		{
			name:  "prelinked plan is outside worktree",
			stage: map[string]string{"plan": "# Plan\n"},
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(t.TempDir(), "plan.md")
				writePlanArtifactTestFile(t, record.PlanPath, "# Plan\n")
			},
			wantError: true,
		},
		{
			name:  "prelinked plan digest mismatches staged plan",
			stage: map[string]string{"plan": "# Staged\n"},
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "plan.md")
				writePlanArtifactTestFile(t, record.PlanPath, "# Linked\n")
			},
			wantError: true,
		},
		{
			name:  "prelinked plan matches staged plan",
			stage: map[string]string{"plan": "# Plan\n"},
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "plan.md")
				writePlanArtifactTestFile(t, record.PlanPath, "# Plan\n")
			},
			wantIdentity: PlanIdentity{Digest: digestExecutionOwnerBytes([]byte("# Plan\n"))},
		},
		{
			name: "delegation parent plan alone does not satisfy readiness",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				parentPlan := filepath.Join(t.TempDir(), "parent.md")
				writePlanArtifactTestFile(t, parentPlan, "# Parent plan\n")
				record.Delegation = &issueopscontract.IssueOpsDelegationContract{ParentPlanPath: parentPlan}
			},
			wantError: true,
		},
		{
			name: "missing staged plan offers exact command for valid linked plan",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "plan with ' quote.md")
				writePlanArtifactTestFile(t, record.PlanPath, "# Linked plan\n")
			},
			wantError: true, wantNext: true, wantExactNext: true,
		},
		{
			name: "missing staged plan offers no command for empty linked file",
			configure: func(t *testing.T, record *issueopscontract.IssueOpsRecord) {
				record.WorktreePath = t.TempDir()
				record.PlanPath = filepath.Join(record.WorktreePath, "empty.md")
				writePlanArtifactTestFile(t, record.PlanPath, "")
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			if test.configure != nil {
				test.configure(t, &record)
			}
			for name, content := range test.stage {
				if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, name, []byte(content)); err != nil {
					t.Fatal(err)
				}
			}

			identity, err := RequireStagedExecutionOwnerPlan(stateRoot, record)
			if test.wantError {
				if err == nil {
					t.Fatalf("identity=%+v, want readiness error", identity)
				}
				fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
				if !ok {
					t.Fatalf("error %T does not expose IssueOps fields: %v", err, err)
				}
				got := fields.IssueOpsErrorFields()
				if got["code"] != "orca_plan_artifact_required" {
					t.Fatalf("code=%v want orca_plan_artifact_required", got["code"])
				}
				if !reflect.DeepEqual(got["missing"], []string{"plan"}) {
					t.Fatalf("missing=%#v want []string{\"plan\"}", got["missing"])
				}
				next, hasNext := got["next_command"].(string)
				if hasNext != test.wantNext {
					t.Fatalf("next_command=%q present=%t want present=%t", next, hasNext, test.wantNext)
				}
				if test.wantExactNext {
					want := "issueops artifact stage --id " + quoteExecutionOwnerArg(record.ID) + " --name plan --file " + quoteExecutionOwnerArg(record.PlanPath) + " --json"
					if next != want {
						t.Fatalf("next_command=%q want %q", next, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := test.wantIdentity
			if record.PlanPath != "" {
				want.Path = record.PlanPath
			}
			if identity != want {
				t.Fatalf("identity=%+v want %+v", identity, want)
			}
		})
	}
}

func TestExecutionOwnerPlanMaterializationRequiresDurableIdentity(t *testing.T) {
	tests := []struct {
		name      string
		stagePlan bool
		prelinked string
		wantError bool
	}{
		{name: "fresh plan", stagePlan: true},
		{name: "matching prelinked plan", stagePlan: true, prelinked: "matching"},
		{name: "mismatched prelinked plan", stagePlan: true, prelinked: "mismatch", wantError: true},
		{name: "missing staged copy", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateRoot, record := executionPrepareRecord(t)
			const plan = "# Owner plan\n"
			if test.stagePlan {
				if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte(plan)); err != nil {
					t.Fatal(err)
				}
			}
			worktree := t.TempDir()
			record.WorktreePath = worktree
			record.Execution = &issueopscontract.Execution{
				Mode:      issueopscontract.ExecutionModeOrca,
				Workspace: issueopscontract.Workspace{Root: worktree},
				Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
			}
			if test.prelinked != "" {
				record.PlanPath = filepath.Join(worktree, "plans", "linked.md")
				content := plan
				if test.prelinked == "mismatch" {
					content = "# Different plan\n"
				}
				writePlanArtifactTestFile(t, record.PlanPath, content)
			}

			identity, manifest, err := materializeExecutionOwnerArtifacts(stateRoot, record)
			if test.wantError {
				if err == nil {
					t.Fatalf("identity=%+v manifest=%+v, want failure", identity, manifest)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			wantPath := record.PlanPath
			if wantPath == "" {
				wantPath = filepath.Join(worktree, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
			}
			wantDigest := digestExecutionOwnerBytes([]byte(plan))
			if identity.Path != wantPath || identity.Digest != wantDigest || manifest["plan"] != wantDigest {
				t.Fatalf("identity=%+v manifest=%+v want path=%q digest=%q", identity, manifest, wantPath, wantDigest)
			}
			content, readErr := os.ReadFile(identity.Path)
			if readErr != nil || string(content) != plan {
				t.Fatalf("durable plan content=%q err=%v", content, readErr)
			}
		})
	}
}

func TestArtifactStagingReleasedRecoveryPredicate(t *testing.T) {
	holder := executionActor("codex", "artifact-holder")
	tests := []struct {
		name      string
		execution *issueopscontract.Execution
		artifact  string
		want      bool
	}{
		{name: "no execution", artifact: "plan", want: true},
		{name: "released clean Orca", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased), artifact: "plan", want: true},
		{name: "released Orca non-plan", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased), artifact: "spec"},
		{name: "released Orca with holder", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased), artifact: "plan"},
		{name: "released Orca with pending", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased), artifact: "plan"},
		{name: "released Orca with completion", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased), artifact: "plan"},
		{name: "active Orca", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusActive), artifact: "plan"},
		{name: "claimable Orca", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusClaimable), artifact: "plan"},
		{name: "revoking Orca", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusRevoking), artifact: "plan"},
		{name: "completed Orca", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased), artifact: "plan"},
		{name: "released direct", execution: artifactRecoveryExecution(issueopscontract.ExecutionModeDirect, issueopscontract.LeaseStatusReleased), artifact: "plan"},
	}
	tests[3].execution.Lease.Holder = &holder
	tests[4].execution.Pending = &issueopscontract.ExternalIntent{OperationID: "operation", Kind: "owner_launch", Marker: "marker", StartedAt: "2026-08-03T00:00:00Z"}
	completion := &issueopscontract.ExecutionCompletion{
		FinalHead: strings.Repeat("a", 40), VerificationReportPath: "report.json",
		Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://example.test/pr/1", CompletedAt: "2026-08-03T00:00:00Z",
	}
	tests[5].execution.Completion = completion
	tests[9].execution.Completion = completion

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := issueopscontract.IssueOpsRecord{Execution: test.execution}
			if got := issueopsartifactdomain.CanStage(record, test.artifact); got != test.want {
				t.Fatalf("CanStage=%t want %t", got, test.want)
			}
		})
	}
}

func TestReleasedArtifactStagingChangesOnlyNextResealInput(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	worktree := t.TempDir()
	record.WorktreePath = worktree
	record.Execution = artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased)
	record.Execution.Workspace.SourceRoot = record.Repo
	record.Execution.Workspace.Root = worktree
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = record.BranchPrepare.BaseSHA
	record.Execution.Workspace.LinkedAt = "2026-08-03T00:00:00Z"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	packetPath, _ := executionOwnerArtifactPaths(record)
	before := []byte("sealed generation packet\n")
	if err := writeExecutionOwnerArtifact(worktree, packetPath, before); err != nil {
		t.Fatal(err)
	}
	const recoveryPlan = "# Recovery plan\n"
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte(recoveryPlan)); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("released staging changed sealed packet: before=%q after=%q", before, after)
	}
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err != nil || staged["plan"] != recoveryPlan {
		t.Fatalf("staged=%+v err=%v", staged, err)
	}
}

func TestReleasedArtifactRecoveryLinksPlanBeforeStaging(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	worktree := t.TempDir()
	record.WorktreePath = worktree
	record.DesignReview = issueOpsDesignReviewForTest()
	record.Execution = artifactRecoveryExecution(issueopscontract.ExecutionModeOrca, issueopscontract.LeaseStatusReleased)
	record.Execution.Workspace.SourceRoot = record.Repo
	record.Execution.Workspace.Root = worktree
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = record.BranchPrepare.BaseSHA
	record.Execution.Workspace.LinkedAt = "2026-08-03T00:00:00Z"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(worktree, "plans", "recovery.md")
	writePlanArtifactTestFile(t, planPath, "# Recovery plan\n")
	linked, err := LinkIssueOpsPlanWithActor(stateRoot, record.ID, planPath, issueOpsActorForTest(worktree))
	if err != nil {
		t.Fatal(err)
	}
	if linked.PlanPath != planPath {
		t.Fatalf("linked plan=%q want %q", linked.PlanPath, planPath)
	}
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# Recovery plan\n")); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(worktree, "plans", "replacement.md")
	writePlanArtifactTestFile(t, replacement, "# Replacement\n")
	if _, err := LinkIssueOpsPlanWithActor(stateRoot, record.ID, replacement, issueOpsActorForTest(worktree)); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("released recovery replaced durable plan identity: %v", err)
	}
}

func TestArtifactReleasedNearMissRequiresReseedBeforeResume(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	worktree := t.TempDir()
	record.WorktreePath = worktree
	record.Execution = artifactRecoveryExecution(issueopscontract.ExecutionModeDirect, issueopscontract.LeaseStatusReleased)
	record.Execution.Workspace.SourceRoot = record.Repo
	record.Execution.Workspace.Root = worktree
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = record.BranchPrepare.BaseSHA
	record.Execution.Workspace.LinkedAt = "2026-08-03T00:00:00Z"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	_, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# Blocked\n"))
	if err == nil {
		t.Fatal("released direct execution accepted recovery staging")
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok {
		t.Fatalf("error %T does not expose structured fields: %v", err, err)
	}
	got := fields.IssueOpsErrorFields()
	if got["code"] != "artifact_stage_requires_reseed" || got["required_action"] != "execution replace --reseed" || !strings.Contains(err.Error(), "before resume") {
		t.Fatalf("fields=%+v err=%v", got, err)
	}
}

func artifactRecoveryExecution(mode issueopscontract.ExecutionMode, status issueopscontract.LeaseStatus) *issueopscontract.Execution {
	driver := "orca"
	if mode == issueopscontract.ExecutionModeDirect {
		driver = "git"
	}
	execution := &issueopscontract.Execution{
		Mode: mode,
		Workspace: issueopscontract.Workspace{
			SourceRoot: "/source", Root: "/worktree", Branch: "262-plan-readiness",
			BaseHead: strings.Repeat("a", 40), Driver: driver, LinkedAt: "2026-08-03T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{Generation: 3, Status: status},
	}
	switch status {
	case issueopscontract.LeaseStatusClaimable:
		execution.Lease.ClaimTokenSHA256 = strings.Repeat("b", 64)
	case issueopscontract.LeaseStatusActive, issueopscontract.LeaseStatusRevoking:
		holder := executionActor("codex", "artifact-holder")
		execution.Lease.Holder = &holder
		if status == issueopscontract.LeaseStatusActive {
			execution.Lease.ClaimedAt = "2026-08-03T00:00:00Z"
		}
	}
	return execution
}

func writePlanArtifactTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRequireStagedExecutionOwnerPlanRejectsStaleDevilsAdvocateReview(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	record.WorktreePath = t.TempDir()
	record.PlanPath = filepath.Join(record.WorktreePath, "plan.md")
	writePlanArtifactTestFile(t, record.PlanPath, "# Plan v2\n")
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# Plan v2\n")); err != nil {
		t.Fatal(err)
	}
	record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", Findings: []string{"attacked gate 3"}, ReviewerContext: "subagent",
		ReviewedPlanDigest: digestExecutionOwnerBytes([]byte("# Plan v1\n")), RecordedAt: "t",
	}
	_, err := RequireStagedExecutionOwnerPlan(stateRoot, record)
	if err == nil {
		t.Fatal("a verdict recorded against an older plan must not launch an owner")
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok {
		t.Fatalf("error %T does not expose IssueOps fields: %v", err, err)
	}
	got := fields.IssueOpsErrorFields()
	if got["code"] != "devils_advocate_review_stale" || !reflect.DeepEqual(got["missing"], []string{"devils_advocate_review_stale"}) {
		t.Fatalf("code/missing must both name the stale gate: %#v", got)
	}
	if next, _ := got["next_command"].(string); !strings.Contains(next, "issueops devils-advocate review --id "+quoteExecutionOwnerArg(record.ID)) || !strings.Contains(next, "--reviewer-context subagent") {
		t.Fatalf("next_command must show the re-record command: %#v", got)
	}

	record.DevilsAdvocateReview.ReviewedPlanDigest = digestExecutionOwnerBytes([]byte("# Plan v2\n"))
	if _, err := RequireStagedExecutionOwnerPlan(stateRoot, record); err != nil {
		t.Fatalf("a verdict bound to the staged plan must pass: %v", err)
	}
	record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{Verdict: "pass", Waived: true, WaiverRationale: "delegated:io-parent parent DA verdict pass", ReviewerPattern: "delegated-parent-review", RecordedAt: "t"}
	if _, err := RequireStagedExecutionOwnerPlan(stateRoot, record); err != nil {
		t.Fatalf("delegated parent verdicts are exempt from plan binding: %v", err)
	}
}

func TestRequireStagedExecutionOwnerPlanSkipsPlanBindingAfterImplementEntry(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# Plan edited during implementation\n")); err != nil {
		t.Fatal(err)
	}
	record.DevilsAdvocateReview = &issueopscontract.IssueOpsDevilsAdvocateReview{
		Verdict: "pass", Findings: []string{"attacked gate 3"}, ReviewerContext: "subagent",
		ReviewedPlanDigest: digestExecutionOwnerBytes([]byte("# Plan v1\n")), RecordedAt: "t",
	}
	// Before implement entry the mismatch blocks the first owner launch.
	if _, err := RequireStagedExecutionOwnerPlan(stateRoot, record); err == nil {
		t.Fatal("pre-implement mismatch must block the first owner launch")
	}
	// Owner replacement/reseed during implementation reseals the edited plan
	// without a fresh devil's-advocate round.
	for _, phase := range []issueopscontract.IssueOpsPhase{IssueOpsPhaseImplement, IssueOpsPhaseAISlopClean, IssueOpsPhasePR} {
		record.Phase = phase
		if _, err := RequireStagedExecutionOwnerPlan(stateRoot, record); err != nil {
			t.Fatalf("phase %s: plan binding must not gate owner replacement after implement entry: %v", phase, err)
		}
	}
}
