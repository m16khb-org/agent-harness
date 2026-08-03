package lifecycle

import (
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"
)

// AC-05: 하위 세션(owner)이 publication 전에 실행하는 implementation-review
// record는 4-flag 시그니처를 갖출 때만 owner mutation allowlist를 통과한다.
func TestExactIssueOpsOwnerMutationAdmitsImplementationReview(t *testing.T) {
	command := "agent-harness issueops implementation-review record --id io-000000000083 --verdict pass" +
		" --finding '경계 검토 완료' --evidence 'go test ok' --reviewer-host codex --reviewer-model gpt-5.6-sol" +
		" --host codex --session-id sess-1 --agent-id none --cwd /tmp/wt --json"
	if !exactIssueOpsOwnerMutation(command) {
		t.Fatalf("well-formed implementation-review record must pass the owner allowlist: %s", command)
	}
	for _, drop := range []string{"--session-id sess-1 ", "--host codex ", "--cwd /tmp/wt ", "--id io-000000000083 "} {
		broken := strings.Replace(command, drop, "", 1)
		if exactIssueOpsOwnerMutation(broken) {
			t.Fatalf("missing %s must fail the 4-flag signature: %s", strings.TrimSpace(drop), broken)
		}
	}
}

// 이슈 #90 도그푸드: handoff 시점에 branch_link_verified가 비어 있으면
// link-plan gate가 막히는데, active lease에서는 holder만 레코드를 고칠 수
// 있으므로 branch prepare도 4-flag owner mutation으로 admit되어야 한다.
func TestExactIssueOpsOwnerMutationAdmitsBranchPrepare(t *testing.T) {
	command := "agent-harness issueops branch prepare --id io-000000000089 --provider github" +
		" --issue-url 'https://github.com/acme/repo/issues/89' --branch 89-atomic --base-branch main" +
		" --base-sha 635303af758fae465d6e6fe30302fed9233180c5 --parent-worktree /tmp/repo.worktrees/main --link-verified" +
		" --host codex --session-id sess-1 --cwd /tmp/wt --json"
	if !exactIssueOpsOwnerMutation(command) {
		t.Fatalf("well-formed branch prepare must pass the owner allowlist: %s", command)
	}
	if exactIssueOpsOwnerMutation(strings.Replace(command, "--session-id sess-1 ", "", 1)) {
		t.Fatal("branch prepare without session-id must fail the 4-flag signature")
	}
}

func TestUnverifiedOrcaHolderMayRecordBranchLinkButCannotMutateProduction(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	record.BranchPrepare.LinkVerified = false
	record.Execution.Mode = issueopscontract.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &issueopscontract.OrcaBinding{
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree",
		OwnerHost: "claude", OwnerModel: "claude-opus-4-6",
		TaskID: "task", DispatchID: "dispatch",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	production := executionRequest(record, worker, "claude", "owner-session", "")
	production.AgentID = "owner-agent"
	production.Tool = "apply_patch"
	production.Paths = []string{filepath.Join(worker, "internal", "application", "issueops.go")}
	if got := BuildLifecyclePreToolUseDecision(production); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "branch_link_verification_required" {
		t.Fatalf("unverified Orca holder production mutation must be blocked: %+v", got)
	}

	phaseCommand := "agent-harness issueops phase --id " + record.ID + " --to implement" +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	phase := executionRequest(record, worker, "claude", "owner-session", phaseCommand)
	phase.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(phase); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "branch_link_verification_required" {
		t.Fatalf("unverified Orca holder implement transition must be blocked: %+v", got)
	}

	branchCommand := "agent-harness issueops branch prepare --id " + record.ID + " --provider github" +
		" --issue-url '" + record.IssueURL + "' --branch " + record.Branch + " --base-branch main" +
		" --link-verified --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	branch := executionRequest(record, worker, "claude", "owner-session", branchCommand)
	branch.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(branch); got.Decision != "allow" {
		t.Fatalf("exact branch-link recorder must remain available to the unverified holder: %+v", got)
	}
	driftedBranch := branch
	driftedBranch.Command = strings.Replace(branchCommand, "--base-branch main", "--base-branch other", 1)
	if got := BuildLifecyclePreToolUseDecision(driftedBranch); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "branch_link_verification_required" {
		t.Fatalf("branch-link recorder with topology drift must be blocked: %+v", got)
	}

	linkPlanCommand := "agent-harness issueops link-plan --id " + record.ID + " --plan-path " + filepath.Join(worker, "plan.md") +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	linkPlan := executionRequest(record, worker, "claude", "owner-session", linkPlanCommand)
	linkPlan.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(linkPlan); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "branch_link_verification_required" {
		t.Fatalf("non-recovery owner mutation must be blocked before link verification: %+v", got)
	}

	releaseCommand := "agent-harness issueops execution release --id " + record.ID + " --generation 1" +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	t.Run("shell typed control plane", func(t *testing.T) {
		release := executionRequest(record, worker, "claude", "owner-session", releaseCommand)
		release.AgentID = "owner-agent"
		if got := BuildLifecyclePreToolUseDecision(release); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "branch_link_verification_required" {
			t.Fatalf("typed release must not bypass branch-link verification: %+v", got)
		}
	})
	t.Run("MCP typed control plane", func(t *testing.T) {
		release := executionRequest(record, worker, "claude", "owner-session", "")
		release.AgentID = "owner-agent"
		release.Tool = "mcp__agent_harness__issueops_execution"
		release.ToolInput = map[string]any{"action": "release", "id": record.ID}
		if got := BuildLifecyclePreToolUseDecision(release); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "branch_link_verification_required" {
			t.Fatalf("MCP release must not bypass branch-link verification: %+v", got)
		}
	})

	record.BranchPrepare.LinkVerified = true
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if got := BuildLifecyclePreToolUseDecision(production); got.Decision != "allow" {
		t.Fatalf("verified Orca holder production mutation was blocked: %+v", got)
	}
}

func TestUnverifiedClaimableOrcaMayResumeOwnerLaunch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	record.BranchPrepare.LinkVerified = false
	record.Execution.Mode = issueopscontract.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Lease.Status = issueopscontract.LeaseStatusClaimable
	record.Execution.Lease.Holder = nil
	record.Execution.Lease.ClaimedAt = ""
	record.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
	record.Execution.Orca = &issueopscontract.OrcaBinding{
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree",
		OwnerHost: "claude", OwnerModel: "claude-opus-4-6",
		TaskID: "task", DispatchID: "dispatch",
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	resumeCommand := "agent-harness issueops execution resume --id " + record.ID + " --expected-generation 1" +
		" --host claude --session-id replacement-session --session-pid 4321" +
		" --session-started-at 2026-08-03T00:00:00Z --session-executable /usr/bin/claude" +
		" --cwd " + worker + " --confirm --json"
	resume := executionRequest(record, worker, "claude", "replacement-session", resumeCommand)
	if got := BuildLifecyclePreToolUseDecision(resume); got.Decision != "allow" {
		t.Fatalf("claimable Orca resume must remain available for owner recovery: %+v", got)
	}

	status := executionRequest(record, worker, "claude", "replacement-session", "")
	status.Tool = "mcp__agent_harness__issueops_execution"
	status.ToolInput = map[string]any{"action": "status", "id": record.ID}
	if got := BuildLifecyclePreToolUseDecision(status); got.Decision != "allow" {
		t.Fatalf("MCP execution status must remain observable: %+v", got)
	}
}

func TestExactIssueOpsOwnerMutationAdmitsDelegationCommands(t *testing.T) {
	actor := " --host codex --session-id sess-1 --cwd /tmp/parent-worktree --json"
	commands := []string{
		"agent-harness issueops child start --parent io-parent --branch 222-child --title child --scope regression" +
			" --acceptance barrier --acceptance mutation" + actor,
		"agent-harness issueops child status --parent io-parent --repair" + actor,
		"agent-harness issueops link-child --id io-parent --child-url https://github.com/acme/repo/issues/222 --title child" + actor,
		"agent-harness issueops remote create-child --id io-parent --title child --body body --label enhancement" +
			" --assignee octocat --confirm" + actor,
	}
	for _, command := range commands {
		if !exactIssueOpsOwnerMutation(command) {
			t.Errorf("delegation owner mutation was not admitted: %s", command)
		}
	}
}

func TestChildStartAllowsOnlyCurrentParentHolder(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	command := "agent-harness issueops child start --parent " + record.ID +
		" --branch 222-child --title child --scope regression --acceptance barrier" +
		" --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	holder := executionRequest(record, worker, "claude", "owner-session", command)
	holder.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("current parent holder child start was blocked: %+v", got)
	}
	foreign := executionRequest(record, worker, "claude", "other-session", command)
	foreign.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("foreign child start must fail the parent holder fence: %+v", got)
	}
}

func TestChildStatusSeparatesObservationFromRepairMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	actor := " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	status := "agent-harness issueops child status --parent " + record.ID + actor
	observer := executionRequest(record, worker, "claude", "observer-session", status)
	if got := BuildLifecyclePreToolUseDecision(observer); got.Decision != "allow" {
		t.Fatalf("read-only child status was blocked: %+v", got)
	}

	repair := "agent-harness issueops child status --parent " + record.ID + " --repair" + actor
	holder := executionRequest(record, worker, "claude", "owner-session", repair)
	holder.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("current holder child status --repair was blocked: %+v", got)
	}
	foreign := executionRequest(record, worker, "claude", "other-session", repair)
	foreign.AgentID = "owner-agent"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" || got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("foreign child status --repair must fail the holder fence: %+v", got)
	}
}

// 우산 worktree는 branch prepare가 기록할 lineage 메타데이터이며 현재 명령의
// mutation root가 아니다. 이미 release된 우산 lifecycle과 연결돼 있어도 자식
// holder가 자기 canonical root의 레코드를 갱신하는 작업을 가로채면 안 된다.
func TestBranchPrepareParentWorktreeMetadataDoesNotSelectForeignLease(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, child, childRoot := executionActiveLifecycleRecord(t)
	parent := linkIssueOpsWorktreeForGuardTest(t, source, "117-parent")
	parentRecord, err := ReadIssueOps(IssueOpsStateRoot(), parent.id)
	if err != nil {
		t.Fatal(err)
	}
	parentRecord.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: source, Root: parent.path, Branch: parentRecord.Branch,
			BaseHead: "0123456789012345678901234567890123456789", Driver: "git", LinkedAt: "2026-07-22T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusReleased, ReleasedAt: "2026-07-22T00:00:01Z",
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), parentRecord); err != nil {
		t.Fatal(err)
	}

	command := "agent-harness issueops branch prepare --id " + child.ID + " --provider github" +
		" --issue-url '" + child.IssueURL + "' --branch " + child.Branch +
		" --base-branch " + parentRecord.Branch +
		" --base-sha 8235f30cac338b444a99c918ffe9e11991e37a8f" +
		" --parent-worktree '" + parent.path + "' --link-verified" +
		" --host claude --session-id owner-session --agent-id owner-agent" +
		" --cwd '" + childRoot + "' --json"
	req := executionRequest(child, childRoot, "claude", "owner-session", command)
	req.AgentID = "owner-agent"
	// host adapter가 명령 인자에서 추출한 경로를 함께 넘기는 형상도 동일하게
	// metadata와 mutation root를 구분해야 한다.
	req.Paths = []string{parent.path, childRoot}

	for _, target := range executionMutationTargets(req) {
		if sameExecutionPath(target, parent.path) {
			t.Fatalf("parent-worktree metadata must not become a mutation target: %v", executionMutationTargets(req))
		}
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("현재 child holder의 branch prepare가 foreign parent lease에 가로채였다: %+v", got)
	}
}

// 현재 holder가 전달하는 session-executable은 native identity 영수증이지
// 워크트리 변경 대상이 아니다. 설치된 Codex/Claude 실행 파일은 보통 워크트리
// 밖에 있으므로 이 값을 경로 fence에 넣으면 정상 publication도 차단된다.
func TestRemoteCreatePRAllowsCurrentHolderWithExplicitWorkdirAndExternalSessionExecutable(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source, record, worker := executionActiveLifecycleRecord(t)
	executable := filepath.Join(t.TempDir(), "codex", "bin", "codex")
	command := "agent-harness issueops remote create-pr --id " + record.ID +
		" --expected-generation 1 --title 'IssueOps lease release differential vertical 검증'" +
		" --head 191-issueops-lease-differential-spike --base 117-hexagonal-architecture-migration" +
		" --body '현재 holder의 CLI/MCP governed preview 검증' --label enhancement --assignee m16khb" +
		" --host claude --session-id owner-session --session-pid 1234" +
		" --session-started-at 2026-07-22T00:00:00Z --session-executable " + executable +
		" --cwd " + worker + " --json"

	holder := executionRequest(record, source, "claude", "owner-session", command)
	holder.AgentID = "owner-agent"
	holder.ToolInput = map[string]any{"workdir": worker, "cmd": command}
	if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
		t.Fatalf("외부 session-executable 영수증을 가진 현재 holder의 create-pr preview가 차단됐다: %+v", got)
	}

	foreign := holder
	foreign.SessionID = "other-session"
	if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" ||
		got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
		t.Fatalf("같은 create-pr 명령을 실행한 비-holder는 identity fence에 차단돼야 한다: %+v", got)
	}
}

// execution prepare 이후에도 grill 재진입과 계획 보강에 필요한 레코더는
// 현재 holder가 사용할 수 있어야 한다. 각 명령은 등록된 플래그와 4-flag
// identity 시그니처를 모두 만족할 때만 owner mutation으로 분류한다.
func TestPlanningOwnerMutationsRemainAvailableAfterExecutionPrepare(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	_, record, worker := executionActiveLifecycleRecord(t)
	actorFlags := " --host claude --session-id owner-session --agent-id owner-agent --cwd " + worker + " --json"
	commands := map[string]string{
		"link-worktree": "agent-harness issueops link-worktree --id " + record.ID +
			" --worktree-path " + worker + actorFlags,
		"intent record": "agent-harness issueops intent record --id " + record.ID +
			" --raw-request '관측성 보강' --interpreted-intent 'breaker 원인과 상태를 노출'" +
			" --success-criteria '원인 분류를 검증'" + actorFlags,
		"domain-review record": "agent-harness issueops domain-review record --id " + record.ID +
			" --model-fit '기존 breaker 상태 모델을 유지' --terminology 'open state'" +
			" --risk '고카디널리티 방지'" + actorFlags,
		"design review": "agent-harness issueops design review --id " + record.ID +
			" --problem-summary '분류 누락' --proposed-design 'exact command 정합화'" +
			" --refactor-plan '기존 guard 경계 유지' --alternative 'bypass 거절'" +
			" --risk '권한 표면 확장' --verification '설계 검토 완료: 대안과 위험 확인' --approved" + actorFlags,
		"regress": "agent-harness issueops regress --id " + record.ID +
			" --reason 'Brooks revise 반영을 위해 grill로 복귀'" + actorFlags,
		"remote reflect-devils-advocate": "agent-harness issueops remote reflect-devils-advocate --id " + record.ID +
			" --provider gitlab --confirm" + actorFlags,
	}

	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			holder := executionRequest(record, worker, "claude", "owner-session", command)
			holder.AgentID = "owner-agent"
			if got := BuildLifecyclePreToolUseDecision(holder); got.Decision != "allow" {
				t.Fatalf("현재 holder의 %s 명령이 차단됐다: %+v", name, got)
			}

			foreign := holder
			foreign.SessionID = "other-session"
			if got := BuildLifecyclePreToolUseDecision(foreign); got.Decision != "block" ||
				got.Deny == nil || got.Deny.Code != "holder_identity_mismatch" {
				t.Fatalf("비-holder의 %s 명령은 identity fence에 차단돼야 한다: %+v", name, got)
			}
		})
	}
}
