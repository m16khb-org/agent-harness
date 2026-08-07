package issueops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

func TestReadExecutionResumeArtifactsUsesDurableIdentityAcrossTemplateUpgrade(t *testing.T) {
	record, want := sealedResumeIdentityFixture(t)
	original := executionOwnerPromptTemplate
	executionOwnerPromptTemplate = original + "\nThis line represents a later owner template version.\n"
	t.Cleanup(func() { executionOwnerPromptTemplate = original })

	got, err := readExecutionResumeArtifacts(record)
	if err != nil {
		t.Fatalf("resume intact prompt sealed by an older template: %v", err)
	}
	if got.issueBodySHA256 != want.issueBodySHA256 || got.packetSHA256 != want.packetSHA256 || got.promptSHA256 != want.promptSHA256 {
		t.Fatalf("artifacts=%+v want=%+v", got, want)
	}
}

func TestReadExecutionResumeArtifactsRejectsSealedIdentityDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, record *issueops.IssueOpsRecord, artifacts executionResumeArtifacts)
		want   string
	}{
		{name: "prompt bytes", want: "sealed owner prompt identity changed", mutate: func(t *testing.T, record *issueops.IssueOpsRecord, artifacts executionResumeArtifacts) {
			writePrivateResumeFixture(t, artifacts.promptPath, []byte("tampered prompt"))
		}},
		{name: "packet bytes", want: "sealed context packet identity changed", mutate: func(t *testing.T, record *issueops.IssueOpsRecord, artifacts executionResumeArtifacts) {
			writePrivateResumeFixture(t, artifacts.packetPath, []byte("{}\n"))
		}},
		{name: "issue digest", want: "sealed issue body identity changed", mutate: func(t *testing.T, record *issueops.IssueOpsRecord, _ executionResumeArtifacts) {
			record.Execution.Orca.IssueBodySHA256 = strings.Repeat("f", 64)
		}},
		{name: "prompt digest", want: "sealed owner prompt identity changed", mutate: func(t *testing.T, record *issueops.IssueOpsRecord, _ executionResumeArtifacts) {
			record.Execution.Orca.OwnerPromptSHA256 = strings.Repeat("f", 64)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, artifacts := sealedResumeIdentityFixture(t)
			tt.mutate(t, &record, artifacts)
			if _, err := readExecutionResumeArtifacts(record); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("drift error=%v want=%q", err, tt.want)
			}
		})
	}
}

func TestResumePlanIdentityRejectsUnsealedOrDriftedPlan(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, record *issueops.IssueOpsRecord)
	}{
		{name: "manifest plan missing", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			mutateResumePacket(t, record, func(packet *executionOwnerContextPacket) { delete(packet.ArtifactManifest, "plan") })
		}},
		{name: "manifest plan digest invalid", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			mutateResumePacket(t, record, func(packet *executionOwnerContextPacket) { packet.ArtifactManifest["plan"] = "invalid" })
		}},
		{name: "sealed plan artifact missing", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			if err := os.Remove(filepath.Join(record.Execution.Workspace.Root, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "sealed plan artifact digest mismatch", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			writePrivateResumeFixture(t, filepath.Join(record.Execution.Workspace.Root, filepath.FromSlash(IssueOpsArtifactDir), "plan.md"), []byte("# Tampered sealed plan\n"))
		}},
		{name: "durable plan missing", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			record.PlanPath = filepath.Join(record.Execution.Workspace.Root, "plans", "missing.md")
		}},
		{name: "durable plan outside worktree", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			record.PlanPath = filepath.Join(t.TempDir(), "outside.md")
			writePlanArtifactTestFile(t, record.PlanPath, "# Resume plan\n")
		}},
		{name: "durable plan digest mismatch", mutate: func(t *testing.T, record *issueops.IssueOpsRecord) {
			writePlanArtifactTestFile(t, record.PlanPath, "# Different durable plan\n")
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record, _ := sealedResumeIdentityFixture(t)
			test.mutate(t, &record)
			if _, err := readExecutionResumeArtifacts(record); err == nil {
				t.Fatal("invalid plan identity passed resume validation")
			} else {
				fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
				if !ok || fields.IssueOpsErrorFields()["code"] != "orca_plan_artifact_required" {
					t.Fatalf("error=%T %v want orca_plan_artifact_required", err, err)
				}
				next, ok := fields.IssueOpsErrorFields()["next_command"].(string)
				if !ok || !strings.Contains(next, "execution replace") || !strings.Contains(next, "--preview") {
					t.Fatalf("next_command=%q want replacement preview", next)
				}
			}
		})
	}
}

func TestResumePlanIdentityAcceptsMatchingSealedAndDurablePlan(t *testing.T) {
	record, _ := sealedResumeIdentityFixture(t)
	if _, err := readExecutionResumeArtifacts(record); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionWriterAbsentRecoveryRoutesUnversionedOrcaThroughReseed(t *testing.T) {
	legacy, _ := sealedResumeIdentityFixture(t)
	legacy.Execution.Orca.ArtifactIdentityVersion = 0
	legacy.Execution.Orca.IssueBodySHA256 = ""
	legacy.Execution.Orca.ContextPacketSHA256 = ""
	legacy.Execution.Orca.OwnerPromptSHA256 = ""
	legacyCommand := executionWriterAbsentRecoveryCommand(legacy)
	if !strings.Contains(legacyCommand, "execution replace") || !strings.Contains(legacyCommand, "--preview") || strings.Contains(legacyCommand, "execution resume") {
		t.Fatalf("legacy recovery command=%q", legacyCommand)
	}
	current, _ := sealedResumeIdentityFixture(t)
	currentCommand := executionWriterAbsentRecoveryCommand(current)
	if !strings.Contains(currentCommand, "execution resume") || strings.Contains(currentCommand, "--preview") {
		t.Fatalf("current recovery command=%q", currentCommand)
	}
	current.BranchPrepare.LinkVerified = false
	unverifiedGitHubCommand := executionWriterAbsentRecoveryCommand(current)
	if !strings.Contains(unverifiedGitHubCommand, "execution resume") {
		t.Fatalf("unverified GitHub launch recovery command=%q", unverifiedGitHubCommand)
	}
}

func TestResumeDispatchRetainsDurableArtifactIdentity(t *testing.T) {
	stateRoot, record, payload := resumeIntentFixture(t, "gitlab", 2646)
	wantIssue := payload.IssueBodySHA256
	wantPacket := payload.Launch.ContextPacketSHA256
	wantPrompt := payload.Launch.PromptSHA256
	steps := []port.ExecutionOrcaIntentReceipt{
		{TerminalPTYID: "pty-next"},
		{RunID: "run-next"},
		{RunID: "run-next", RunBound: true},
		{TaskID: "task-next"},
		{TaskID: "task-next", DispatchID: "dispatch-next"},
	}
	var err error
	for _, receipt := range steps {
		record, payload, err = advanceOrcaIntentReceipt(context.Background(), stateRoot, record, payload, receipt, nil, nil)
		if err != nil {
			t.Fatalf("advance resume stage: %v", err)
		}
	}
	binding := record.Execution.Orca
	if binding.ArtifactIdentityVersion != issueops.OrcaArtifactIdentityVersion {
		t.Fatalf("resumed binding artifact identity version=%d want=%d", binding.ArtifactIdentityVersion, issueops.OrcaArtifactIdentityVersion)
	}
	if binding.IssueBodySHA256 != wantIssue || binding.ContextPacketSHA256 != wantPacket || binding.OwnerPromptSHA256 != wantPrompt {
		t.Fatalf("resumed binding artifact identity=%+v", binding)
	}
}

func TestBeginOrcaExecutionResumeIntentAllowsUnverifiedGitHubLaunch(t *testing.T) {
	_, record, payload := resumeIntentFixtureWithLinkVerified(t, "github", 16, false)
	if payload.Probe.Provider != "github" || payload.Probe.Issue != 16 {
		t.Fatalf("resume identity = provider:%q issue:%d", payload.Probe.Provider, payload.Probe.Issue)
	}
	if record.Execution.Pending == nil || record.Execution.Pending.Marker != payload.Marker {
		t.Fatalf("persisted pending = %#v", record.Execution.Pending)
	}
}

func sealedResumeIdentityFixture(t *testing.T) (issueops.IssueOpsRecord, executionResumeArtifacts) {
	t.Helper()
	worktree := t.TempDir()
	issueBody := "## Acceptance\n- AC-01 sealed resume\n\n## Verification\n```bash\ngo test ./...\n```\n"
	record := issueops.IssueOpsRecord{
		SchemaVersion: issueops.IssueOpsSchemaVersion, ID: "io-aaaaaaaaaaaa", Repo: filepath.Join(t.TempDir(), "source"), Branch: "254-resume",
		IssueURL: "https://github.com/example/agent-harness/issues/254", WorktreePath: worktree,
		PlanPath:      filepath.Join(worktree, "plans", "linked.md"),
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/agent-harness/issues/254", Branch: "254-resume", BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true},
		Execution: &issueops.Execution{
			Mode:      issueops.ExecutionModeOrca,
			Workspace: issueops.Workspace{SourceRoot: filepath.Join(t.TempDir(), "source"), Root: worktree, Branch: "254-resume", BaseHead: strings.Repeat("a", 40), Driver: "orca", LinkedAt: "2026-08-03T00:00:00Z"},
			Lease:     issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusClaimable},
			Orca:      &issueops.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1, OwnerHost: "codex", OwnerModel: "gpt-5.6-sol", OwnerEffort: "high", TaskID: "task", DispatchID: "dispatch"},
		},
	}
	const plan = "# Resume plan\n"
	writePlanArtifactTestFile(t, record.PlanPath, plan)
	sealedPlanPath := filepath.Join(worktree, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
	if err := os.MkdirAll(filepath.Dir(sealedPlanPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writePrivateResumeFixture(t, sealedPlanPath, []byte(plan))
	token := "sealed-resume-token"
	record.Execution.Lease.ClaimTokenSHA256 = tokenSHA256(token)
	tokenPath := claimTokenPath(record)
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writePrivateResumeFixture(t, tokenPath, []byte(token+"\n"))
	snapshot := executionOwnerSnapshot{
		issue:          executionOwnerIssue{URL: record.IssueURL, Body: issueBody, BodySHA256: digestExecutionOwnerBytes([]byte(issueBody))},
		requiredSkills: []string{"issueops"}, acceptanceIDs: []string{"AC-01"}, verificationCommands: []string{"go test ./..."},
	}
	manifest := map[string]string{"plan": digestExecutionOwnerBytes([]byte(plan))}
	artifacts, err := buildExecutionOwnerArtifacts(record, ExecutionPrepareRequest{OwnerHost: "codex", OwnerModel: "gpt-5.6-sol", OwnerEffort: "high"}, snapshot, manifest)
	if err != nil {
		t.Fatal(err)
	}
	record.Execution.Orca.IssueBodySHA256 = snapshot.issue.BodySHA256
	record.Execution.Orca.ContextPacketSHA256 = artifacts.packetSHA256
	record.Execution.Orca.OwnerPromptSHA256 = artifacts.promptSHA256
	record.Execution.Orca.ArtifactIdentityVersion = issueops.OrcaArtifactIdentityVersion
	return record, executionResumeArtifacts{
		claimTokenPath: tokenPath, issueBodySHA256: snapshot.issue.BodySHA256,
		packetPath: artifacts.packetPath, packetSHA256: artifacts.packetSHA256,
		promptPath: artifacts.promptPath, promptSHA256: artifacts.promptSHA256,
	}
}

func mutateResumePacket(t *testing.T, record *issueops.IssueOpsRecord, mutate func(*executionOwnerContextPacket)) {
	t.Helper()
	packetPath, _ := executionOwnerArtifactPaths(*record)
	raw, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet executionOwnerContextPacket
	if err := json.Unmarshal(raw, &packet); err != nil {
		t.Fatal(err)
	}
	mutate(&packet)
	raw, err = json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	writePrivateResumeFixture(t, packetPath, raw)
	record.Execution.Orca.ContextPacketSHA256 = digestExecutionOwnerBytes(raw)
}

func writePrivateResumeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
