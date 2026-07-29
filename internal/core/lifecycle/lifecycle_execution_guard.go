package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/commandparse"
	issueopscore "agent-harness/internal/core/issueops"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/lifecycle/worktreeguard"
	"agent-harness/internal/core/searchrouting"
)

// 관찰 권한은 동시 execution 중 owner를 먼저 고르는 절차에 의존하지 않는다.
func executionObservation(req HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		return explicitIssueOpsReadOnlyTool(req.Tool)
	}
	if commandparse.ExactReadOnlyShellCommand(req.Command) {
		return true
	}
	if exactOrcaObservation(req.Command) {
		return true
	}
	if exactOrcaOwnerControlPlane(req.Command) {
		return true
	}
	// gh issue develop은 관측이 아니다 — provider에 브랜치를 만든다. 그런데 이
	// 목록에 없어서 authority 활성 중 unclassified로 막혔고, #163이 정한 orca
	// 순서(orca 준비 뒤 링크 부착)를 canonical worktree에서 실행할 수 없었다.
	// branch prepare의 fallback_api가 그 명령을 안내하므로 안내와 실행 가능성이
	// 어긋나 있었다(이슈 #177).
	//
	// ExactReadOnlyShellCommand에 넣지 않는 이유는 그 이름이 계약이기 때문이다.
	// 여기서 별도로 판정하고, 통과 근거는 "읽기라서"가 아니라 "IssueOps가 그
	// 명령을 지시하고 형태를 정확히 고정할 수 있어서"다.
	if exactProviderBranchLink(req.Command) {
		return true
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok {
		return false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return false
	}
	// 원격 mutation 명령에 help flag 하나만 전달하면 확인한 Go flag parser가
	// 실제 동작 전에 flag.ErrHelp로 종료한다. mutation 이름을 가졌더라도 이
	// 정확한 형태는 상태를 읽거나 쓰지 않는 CLI 표면 조회다.
	if exactIssueOpsMutationHelpObservation(command.Path, command.Tokens, command.Start) {
		return true
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return false
	}
	// 관찰로 인정하는 기준: core 구현이 상태를 쓰지 않고, 파괴 작업의 preview
	// 단계도 아닌 것. 명령 이름이 아니라 그 구현의 성질로 판단한다 — cleanup
	// status와 cleanup finish는 경로 문자열이 prefix를 공유하지만 전자만 읽기다.
	//
	// 이 목록이 명시적 열거인 것은 fail-closed의 근거다. 규칙 기반 판정으로
	// 바꾸면 분류 누락이 차단이 아니라 통과로 새어 나간다(#135).
	switch command.Path {
	case "status", "execution status", "pr-readiness", "cleanup status":
		// cleanup status의 --merged가 원격을 조회하지만 그것도 읽기다.
		// cleanup remote-branch --preview가 같은 자격으로 원격 OID를 관측하는
		// 선례가 있다.
		id, ok := oneFlag(flags, "--id")
		return ok && strings.TrimSpace(id) != ""
	case "list":
		// 다중 사이클을 훑는 read-only 집계다. --id를 받지 않으므로 식별자
		// 검사가 없고, --repo는 선택이다.
		return true
	case "execution whoami":
		// claim identity 부트스트랩: owner가 자기 native receipt를 관측할
		// 유일한 admitted 경로다. 읽기 전용이고 인자를 받지 않는다.
		return true
	case "remote score":
		return exactRemoteScoreObservation(flags)
	case "execution replace":
		_, preview := flags["--preview"]
		_, finalizePreview := flags["--finalize-preview"]
		_, confirm := flags["--confirm"]
		_, revoke := flags["--revoke"]
		_, finalize := flags["--finalize"]
		_, reseed := flags["--reseed"]
		id, idOK := oneFlag(flags, "--id")
		generation, generationOK := oneFlag(flags, "--expected-generation")
		return idOK && strings.TrimSpace(id) != "" && generationOK && strings.TrimSpace(generation) != "" &&
			preview != finalizePreview && !confirm && !revoke && !finalize && !reseed
	case "execution reconcile":
		_, preview := flags["--preview"]
		_, confirm := flags["--confirm"]
		id, idOK := oneFlag(flags, "--id")
		return idOK && strings.TrimSpace(id) != "" && preview && !confirm
	case "reset-legacy":
		// --preview와 --status는 schema 상태를 읽기만 한다. 그것이 가드 어디에도
		// 없어서 mutation authority가 활성인 동안 unclassified로 막혔고, 상태를
		// 진단할 수단이 하나 사라져 있었다(이슈 #170).
		//
		// mutation 경로(--confirm, --drain-cycle, --reconcile-remote)는 넣지
		// 않는다. 그것들은 schema v0 사이클을 다루는 마이그레이션 조작이고, v1
		// lease가 갇힌 상태를 풀지 못한다 — 열어 줄 이유가 없다.
		schema, schemaOK := oneFlag(flags, "--target-schema")
		_, preview := flags["--preview"]
		_, status := flags["--status"]
		_, confirm := flags["--confirm"]
		_, drain := flags["--drain-cycle"]
		_, reconcile := flags["--reconcile-remote"]
		return schemaOK && strings.TrimSpace(schema) != "" && (preview != status) && !confirm && !drain && !reconcile
	default:
		return false
	}
}

// exactProviderBranchLink는 `gh issue develop`의 정확한 두 형태만 인정한다.
//
//	gh issue develop <number> --repo <slug> --base <branch> --name <branch>
//	gh issue develop --list <number> --repo <slug>
//
// develop 하나를 열면서 issue 표면 전체가 열리지 않게 서브커맨드와 플래그를
// 열거한다. create·close·edit·comment는 통과하지 않고, 열거 밖 플래그가 하나라도
// 붙으면 거부한다(#177).
func exactProviderBranchLink(command string) bool {
	if commandparse.HasUnquotedControlOperator(command) ||
		commandparse.HasActiveCommandSubstitution(command) ||
		commandparse.HasActiveOutputRedirect(command) ||
		commandparse.HasActiveInputRedirect(command) ||
		commandparse.HasActiveParameterOrTildeExpansion(command) ||
		commandparse.HasActivePathnameExpansion(command) ||
		commandparse.HasActiveShellSpecialQuoting(command) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(command)
	if len(tokens) < 4 || searchrouting.SearchTokenName(tokens[0]) != "gh" {
		return false
	}
	// `gh api` 두 형태는 #176이 도입한 base-pinned 링크 경로다. `gh issue develop`은
	// --base를 브랜치 이름으로만 받아 GitHub이 그 시점 HEAD를 oid로 쓰므로, 봉인된
	// base에 못박으려면 createLinkedBranch를 직접 호출해야 한다. branch prepare가
	// 그 두 명령을 안내하므로 여기서도 분류해야 안내와 실행 가능성이 맞는다.
	if tokens[1] == "api" {
		return exactGitHubIssueNodeRead(tokens) || exactGitHubLinkedBranchMutation(tokens)
	}
	if tokens[1] != "issue" || tokens[2] != "develop" {
		return false
	}
	rest := tokens[3:]
	if rest[0] == "--list" {
		// 조회 형태: --list <number> --repo <slug>
		if len(rest) != 4 || rest[2] != "--repo" {
			return false
		}
		return positiveIssueNumber(rest[1]) && strings.TrimSpace(rest[3]) != ""
	}
	// 생성 형태: <number> --repo <slug> --base <branch> --name <branch>
	if len(rest) != 7 || !positiveIssueNumber(rest[0]) {
		return false
	}
	for _, pair := range [][2]int{{1, 2}, {3, 4}, {5, 6}} {
		if strings.TrimSpace(rest[pair[1]]) == "" || strings.HasPrefix(rest[pair[1]], "--") {
			return false
		}
	}
	return rest[1] == "--repo" && rest[3] == "--base" && rest[5] == "--name"
}

func positiveIssueNumber(value string) bool {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && number > 0
}

// exactGitHubIssueNodeRead는 node id 조회 한 형태만 인정한다:
//
//	gh api repos/<owner>/<repo>/issues/<number> --jq .node_id
//
// 읽기지만 ExactReadOnlyShellCommand의 gh 분기는 pr·run만 다루므로 여기서 함께
// 판정한다 — 이 명령과 그 뒤 mutation이 하나의 안내를 이루기 때문이다(#176).
func exactGitHubIssueNodeRead(tokens []string) bool {
	if len(tokens) != 5 || tokens[3] != "--jq" || tokens[4] != ".node_id" {
		return false
	}
	parts := strings.Split(tokens[2], "/")
	if len(parts) != 5 || parts[0] != "repos" || parts[3] != "issues" {
		return false
	}
	return strings.TrimSpace(parts[1]) != "" && strings.TrimSpace(parts[2]) != "" && positiveIssueNumber(parts[4])
}

// exactGitHubLinkedBranchMutation은 createLinkedBranch 호출 한 형태만 인정한다:
//
//	gh api graphql -f query=<mutation> -F issueId=<id> -F oid=<sha> -F name=<branch>
//
// query 본문이 그 mutation을 담고 있는지 확인하므로 임의 GraphQL이 통과하지
// 않는다. 플래그 위치와 개수도 고정한다(#176).
func exactGitHubLinkedBranchMutation(tokens []string) bool {
	if len(tokens) != 11 || tokens[2] != "graphql" {
		return false
	}
	if tokens[3] != "-f" || !strings.HasPrefix(tokens[4], "query=") ||
		!strings.Contains(tokens[4], "createLinkedBranch") {
		return false
	}
	for _, pair := range [][2]int{{5, 6}, {7, 8}, {9, 10}} {
		if tokens[pair[0]] != "-F" {
			return false
		}
		if strings.TrimSpace(tokens[pair[1]]) == "" || strings.HasPrefix(tokens[pair[1]], "-") {
			return false
		}
	}
	return strings.HasPrefix(tokens[6], "issueId=") &&
		strings.HasPrefix(tokens[8], "oid=") &&
		strings.HasPrefix(tokens[10], "name=")
}

func exactOrcaObservation(command string) bool {
	if commandparse.HasUnquotedControlOperator(command) ||
		commandparse.HasActiveCommandSubstitution(command) ||
		commandparse.HasActiveInputRedirect(command) ||
		commandparse.HasActiveParameterOrTildeExpansion(command) ||
		commandparse.HasActivePathnameExpansion(command) ||
		commandparse.HasActiveShellSpecialQuoting(command) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(command)
	if len(tokens) < 2 || searchrouting.SearchTokenName(tokens[0]) != "orca" {
		return false
	}
	switch tokens[1] {
	case "status":
		return true
	case "terminal":
		return len(tokens) >= 3 && (tokens[2] == "list" || tokens[2] == "show" || tokens[2] == "read")
	case "repo", "worktree":
		return len(tokens) >= 3 && (tokens[2] == "list" || tokens[2] == "show")
	case "skills":
		return len(tokens) >= 3 && (tokens[2] == "get" || tokens[2] == "list")
	case "orchestration":
		return len(tokens) >= 3 && (tokens[2] == "task-list" || tokens[2] == "dispatch-show")
	default:
		return false
	}
}

// exactOrcaOwnerControlPlane은 injected worker contract가 요구하는 coordinator
// 제어면만 인정한다. 이 명령들은 sealed Git worktree가 아니라 Orca orchestration
// ledger를 갱신하므로 lease 상태와 무관하게 실행 가능해야 한다.
//
// 모든 flag와 message type을 열거해 알 수 없는 mutation, shell expansion,
// redirect, detached composition은 계속 fail-closed로 유지한다.
func exactOrcaOwnerControlPlane(command string) bool {
	if commandparse.HasUnquotedControlOperator(command) ||
		commandparse.HasActiveCommandSubstitution(command) ||
		commandparse.HasActiveInputRedirect(command) ||
		commandparse.HasActiveOutputRedirect(command) ||
		commandparse.HasActiveParameterOrTildeExpansion(command) ||
		commandparse.HasActivePathnameExpansion(command) ||
		commandparse.HasActiveShellSpecialQuoting(command) ||
		commandparse.HasActiveZshEqualsExpansion(command) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(command)
	if len(tokens) < 4 || searchrouting.SearchTokenName(tokens[0]) != "orca" ||
		tokens[1] != "orchestration" {
		return false
	}
	exact := commandparse.ExactIssueOpsCommand{
		Path: "orca orchestration " + tokens[2], Tokens: tokens, Start: 3,
	}
	switch tokens[2] {
	case "send":
		flags, ok := commandparse.ExactFlags(
			exact,
			exactFlagNames(
				"--to", "--from", "--type", "--subject", "--body", "--task-id",
				"--dispatch-id", "--files-modified", "--report-path", "--phase",
			),
			exactFlagNames("--json"),
			nil,
		)
		if !ok || !nonemptyExactFlags(flags, "--to", "--type", "--subject") {
			return false
		}
		messageType, _ := oneFlag(flags, "--type")
		switch messageType {
		case "heartbeat":
			return nonemptyExactFlags(flags, "--task-id", "--dispatch-id", "--phase")
		case "worker_done":
			return nonemptyExactFlags(flags, "--body", "--task-id", "--dispatch-id")
		case "escalation":
			return nonemptyExactFlags(flags, "--body", "--task-id")
		case "decision_gate", "reply":
			return nonemptyExactFlags(flags, "--body")
		default:
			return false
		}
	case "ask":
		flags, ok := commandparse.ExactFlags(
			exact,
			exactFlagNames("--to", "--from", "--question", "--options", "--timeout-ms"),
			exactFlagNames("--json"),
			nil,
		)
		if !ok || !nonemptyExactFlags(flags, "--to", "--from", "--question") {
			return false
		}
		timeout, hasTimeout := oneFlag(flags, "--timeout-ms")
		return !hasTimeout || positiveMilliseconds(timeout)
	case "check":
		flags, ok := commandparse.ExactFlags(
			exact,
			exactFlagNames("--terminal", "--timeout-ms"),
			exactFlagNames("--wait", "--json"),
			nil,
		)
		if !ok || !nonemptyExactFlags(flags, "--terminal") {
			return false
		}
		timeout, hasTimeout := oneFlag(flags, "--timeout-ms")
		_, wait := flags["--wait"]
		return !hasTimeout || wait && positiveMilliseconds(timeout)
	default:
		return false
	}
}

func exactFlagNames(names ...string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		result[name] = true
	}
	return result
}

func nonemptyExactFlags(flags map[string][]string, names ...string) bool {
	for _, name := range names {
		value, ok := oneFlag(flags, name)
		if !ok || strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func positiveMilliseconds(value string) bool {
	milliseconds, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && milliseconds > 0
}

func executionTypedControlPlane(req HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		tool := strings.TrimSpace(req.Tool)
		return (tool == "issueops_execution" || tool == "mcp__agent_harness__issueops_execution") && req.ToolInput != nil
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok {
		return false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return false
	}
	switch command.Path {
	// sync-base는 워크트리 cwd에서 sealed topology 가드에 걸리는 유일한 합법
	// 표면이므로 typed 등록이 필요하다(설계 v2 F1 — "가드 무변경"은 오기).
	// typed 등록은 훅의 mutation 가드 블록 전체를 스킵시키므로(F14) lease·권위
	// 검사는 core(execution_sync_base.go)가 100% 책임진다.
	// switch-mode도 같은 이유로 typed 등록이 필요하다. 전환은 lease가 writer를
	// 쥐고 있지 않을 때만 가능한데(core가 강제한다), 그 상태에서도 다른 lifecycle의
	// mutation authority가 활성이면 훅이 이 명령을 unclassified로 막는다 — 사용자가
	// 갇힌다(이슈 #167).
	// cleanup orphan도 같은 이유로 typed 등록이 필요하다. 그 명령의 대상은 정식
	// phase를 밟지 못한 사이클의 자원이고, 그런 사이클은 정의상 mutation authority가
	// 활성인 채로 남는다 — 즉 이 명령이 필요한 순간에 정확히 막혔다. cleanup
	// abandon의 orca_residue_error가 그것을 안내하기까지 한다(이슈 #177).
	//
	// 안전은 그 명령의 fingerprint와 --apply --confirm 게이트가 본다. typed 등록은
	// 훅의 mutation 가드 블록을 스킵시킬 뿐이고 lease·권위 검사는 core 책임이다(F14).
	case "execution prepare", "execution claim", "execution release", "execution replace", "execution reconcile", "execution complete", "execution sync-base", "execution switch-mode",
		"cleanup orphan":
		id, ok := oneFlag(flags, "--id")
		return ok && strings.TrimSpace(id) != ""
	default:
		return false
	}
}

func executionMutationDecision(req HookToolUseLifecycleRequest) (bool, string, *IssueOpsDenyReason) {
	if !req.EnforceWorktree {
		return false, "", nil
	}
	unsafeReason := executionUnsafeMutationReason(req)
	resourceWaitRoot, exactResourceWait := exactOwnedResourceWait(req.Command)
	atomicWorkflowRoot, exactAtomicWorkflow := exactAtomicCommitWorkflowScript(req)
	atomicWorkflowRelativeScript := exactAtomicWorkflow && atomicCommitWorkflowUsesRelativeScript(req.Command)
	mayMutate := toolUseMayMutateLifecycleFiles(req.Tool, req.Command)
	if searchrouting.IsShellTool(req.Tool) && !mayMutate {
		mayMutate = true
		if unsafeReason == "" && !exactIssueOpsOwnerMutation(req.Command) && !exactResourceWait && !exactAtomicWorkflow {
			unsafeReason = "unclassified shell command is blocked while IssueOps mutation authority is active; use an exact listed reader or a statically classified foreground mutation command"
		}
	}
	if !mayMutate {
		return false, "", nil
	}
	targets := executionMutationTargets(req)
	if exactResourceWait {
		targets = append(targets, resourceWaitRoot)
	}
	if exactAtomicWorkflow {
		targets = []string{atomicWorkflowRoot}
	}
	records, err := executionGuardRecords(req, targets)
	if err != nil {
		return true, "IssueOps authority state could not be read (often transient state-store contention); retry once, and if it persists run `agent-harness doctor --repo " + cleanAbsPath(req.Repo) + " --json`", nil
	}
	if len(records) == 0 && exactAtomicWorkflow {
		// 명시적 workdir가 외부를 가리킬 때 target만 조회하면 현재 lifecycle을
		// 찾지 못해 일반 명령으로 빠질 수 있다. cwd/repo anchor는 허용 근거로
		// 쓰지 않고, 활성 lifecycle에서 빠져나가는 misdirect 차단에만 쓴다.
		anchors := []string{cleanAbsPath(req.CWD), cleanAbsPath(req.Repo)}
		anchorRecords, anchorErr := executionGuardRecords(req, anchors)
		if anchorErr != nil {
			return true, "IssueOps authority state could not be read (often transient state-store contention); retry once, and if it persists run `agent-harness doctor --repo " + cleanAbsPath(req.Repo) + " --json`", nil
		}
		if len(anchorRecords) > 0 {
			record := anchorRecords[0]
			reason := "atomic commit workflow must run with an effective shell workdir equal to the canonical IssueOps worktree"
			return true, reason, executionDeny(record, "unsafe_mutation", executionStatusCommand(record.ID))
		}
	}
	if len(records) == 0 {
		if exactResourceWait {
			return true, "resource wait requires an exact canonical IssueOps worktree owned by the current lifecycle", nil
		}
		return false, "", nil
	}
	for _, record := range records {
		if record.Execution == nil {
			continue
		}
		if !requestTouchesExecution(req, targets, *record.Execution) {
			continue
		}
		if unsafeReason != "" {
			return true, unsafeReason, executionDeny(record, "unsafe_mutation", executionStatusCommand(record.ID))
		}
		if err := issueopsmodel.ValidateExecution(*record.Execution); err != nil {
			return true, "invalid IssueOps execution v1 record: " + err.Error(), nil
		}
		lease := record.Execution.Lease
		root := record.Execution.Workspace.Root
		if atomicWorkflowRelativeScript && !sameExecutionPath(atomicWorkflowRoot, root) {
			reason := "relative atomic commit workflow scripts must run from the canonical IssueOps worktree root"
			return true, reason, executionDeny(record, "unsafe_mutation", executionStatusCommand(record.ID))
		}
		if lease.Status == issueopsmodel.LeaseStatusActive && executionActorMatches(req, lease.Holder) &&
			executionRequestTargetsStayInside(req, targets, root) {
			return true, "", nil
		}
		if lease.Status == issueopsmodel.LeaseStatusActive && lease.Holder != nil && !executionActorMatches(req, lease.Holder) {
			axis := executionActorMismatchAxis(req, lease.Holder)
			deny := executionDeny(record, "holder_identity_mismatch", executionStatusCommand(record.ID))
			deny.IdentityMismatch = axis
			deny.ObservedActor = fmt.Sprintf("host=%s session_id=%s agent_id=%s",
				strings.TrimSpace(req.Host), strings.TrimSpace(req.SessionID), strings.TrimSpace(req.AgentID))
			return true, fmt.Sprintf(
				"active write lease for IssueOps execution %s generation %d is held by a different native identity (mismatch axis: %s); the durable holder must re-establish identity, not retry",
				record.ID, lease.Generation, axis), deny
		}
		reason, deny := executionMutationDenyReason(record)
		return true, reason, deny
	}
	return false, "", nil
}

func exactIssueOpsMutationHelpObservation(path string, tokens []string, start int) bool {
	switch path {
	case "remote create-pr", "remote verify-artifact":
	default:
		return false
	}
	return len(tokens) == start+1 &&
		(tokens[start] == "--help" || tokens[start] == "-h")
}

func exactIssueOpsOwnerMutation(commandText string) bool {
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok {
		return false
	}
	switch command.Path {
	// decision add는 record.Decisions에 append만 하고 phase·lease·execution을 건드리지
	// 않는다(append-only 계약은 TestDecisionAddTouchesOnlyTheDecisionList가 고정한다).
	// 구현 중 설계 결정이 바뀌는 것은 정상인데 그 기록 경로가 implement 단계에서 막혀
	// 있어, #152에서 preview 계약 변경 결정을 문서에만 남겨야 했다 — durable state에
	// 담기지 않은 결정은 나중 사이클의 plan-prep prior-decisions 조회에 들어오지
	// 않는다(이슈 #158).
	case "link-plan", "compatibility review", "devils-advocate review", "phase",
		"decision add", "ai-slop-clean record", "feedback mark-issue-updated", "feedback resolve",
		"implementation-review record", "branch prepare", "intent record", "domain-review record", "regress",
		"remote create-pr", "remote verify-artifact", "remote reflect-devils-advocate":
	default:
		return false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return false
	}
	for _, name := range []string{"--id", "--host", "--session-id", "--cwd"} {
		value, found := oneFlag(flags, name)
		if !found || strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func exactOwnedResourceWait(commandText string) (string, bool) {
	commandText = strings.TrimSpace(commandText)
	if commandText == "" || commandparse.HasUnquotedControlOperator(commandText) ||
		commandparse.HasActiveCommandSubstitution(commandText) ||
		commandparse.HasActiveInputRedirect(commandText) ||
		commandparse.HasActiveOutputRedirect(commandText) ||
		commandparse.HasActiveParameterOrTildeExpansion(commandText) ||
		commandparse.HasActivePathnameExpansion(commandText) ||
		commandparse.HasActiveShellSpecialQuoting(commandText) ||
		commandparse.HasActiveZshEqualsExpansion(commandText) {
		return "", false
	}
	tokens := commandparse.SplitCommandTokens(commandText)
	if len(tokens) < 3 ||
		(tokens[0] != "agent-harness" && tokens[0] != "bin/agent-harness" && tokens[0] != "./bin/agent-harness") ||
		tokens[1] != "resource" || tokens[2] != "wait" {
		return "", false
	}
	values := map[string]bool{
		"--workspace-root": true,
		"--profile":        true,
		"--timeout":        true,
		"--interval":       true,
		"--progress":       true,
	}
	booleans := map[string]bool{"--json": true}
	flags, ok := commandparse.ExactFlags(
		commandparse.ExactIssueOpsCommand{Path: "resource wait", Tokens: tokens, Start: 3},
		values,
		booleans,
		map[string]bool{},
	)
	if !ok {
		return "", false
	}
	root, rootOK := oneFlag(flags, "--workspace-root")
	profile, profileOK := oneFlag(flags, "--profile")
	timeout, timeoutOK := oneFlag(flags, "--timeout")
	interval, intervalOK := oneFlag(flags, "--interval")
	progress, progressOK := oneFlag(flags, "--progress")
	_, jsonOK := flags["--json"]
	if !rootOK || !filepath.IsAbs(root) || !profileOK || profile != "e2e" ||
		!timeoutOK || !positiveDuration(timeout) || !intervalOK || !positiveDuration(interval) ||
		!progressOK || (progress != "none" && progress != "jsonl") || !jsonOK {
		return "", false
	}
	return cleanAbsPath(root), true
}

// exactAtomicCommitWorkflowScript는 atomic-commit-push 스킬이 필수로 실행하는
// 두 Python gate만 현재 holder의 foreground workflow로 인정한다. 저장소가
// 제공하거나 설치 경로에 연결된 Python 코드는 일반 관찰 권한으로 승격하지 않고,
// 대상 저장소만 기존 canonical worktree fence로 다시 검증한다.
func exactAtomicCommitWorkflowScript(req HookToolUseLifecycleRequest) (string, bool) {
	if !searchrouting.IsShellTool(req.Tool) {
		return "", false
	}
	commandText := strings.TrimSpace(req.Command)
	if commandText == "" || commandparse.HasUnquotedControlOperator(commandText) ||
		commandparse.HasActiveCommandSubstitution(commandText) ||
		commandparse.HasActiveInputRedirect(commandText) ||
		commandparse.HasActiveOutputRedirect(commandText) ||
		commandparse.HasActiveParameterOrTildeExpansion(commandText) ||
		commandparse.HasActivePathnameExpansion(commandText) ||
		commandparse.HasActiveShellSpecialQuoting(commandText) ||
		commandparse.HasActiveZshEqualsExpansion(commandText) {
		return "", false
	}
	tokens := commandparse.SplitCommandTokens(commandText)
	if (len(tokens) != 2 && len(tokens) != 3) || tokens[0] != "python3" ||
		!losslessAtomicWorkflowToken(tokens[1]) ||
		!atomicCommitWorkflowScriptPath(req, tokens[1]) {
		return "", false
	}
	cwd, ok := atomicCommitWorkflowCWD(req)
	if !ok {
		return "", false
	}
	root := cwd
	if len(tokens) == 3 {
		if !losslessAtomicWorkflowToken(tokens[2]) || strings.HasPrefix(tokens[2], "-") {
			return "", false
		}
		root = resolveHookTargetPath(cwd, tokens[2])
	}
	if root == "" || root != cwd {
		return "", false
	}
	return root, true
}

// atomicCommitWorkflowCWD는 Codex exec_command가 실제로 사용하는 workdir와
// Claude Bash가 전달하는 top-level cwd를 구분한다. exec_command의 명시적
// workdir는 절대 경로일 때만 받아 host별 상대 경로 해석 차이를 열지 않는다.
func atomicCommitWorkflowCWD(req HookToolUseLifecycleRequest) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(req.Tool), "exec_command") {
		value, exists := req.ToolInput["workdir"]
		if exists {
			workdir, ok := value.(string)
			if !ok || !losslessAtomicWorkflowToken(workdir) || !filepath.IsAbs(workdir) {
				return "", false
			}
			root := cleanAbsPath(workdir)
			return root, root != ""
		}
	}
	root := cleanAbsPath(req.CWD)
	return root, root != ""
}

func losslessAtomicWorkflowToken(token string) bool {
	return token != "" && token == strings.TrimSpace(token)
}

func atomicCommitWorkflowScriptPath(req HookToolUseLifecycleRequest, path string) bool {
	clean := filepath.Clean(path)
	for _, relative := range []string{
		"skills/atomic-commit-push/scripts/git_preflight.py",
		"skills/atomic-commit-push/scripts/api_doc_gate.py",
	} {
		if !filepath.IsAbs(clean) && filepath.ToSlash(clean) == relative {
			return true
		}
		if !filepath.IsAbs(clean) {
			continue
		}
		target := cleanAbsPath(clean)
		for _, base := range atomicCommitWorkflowInstallBases(req) {
			if target == filepath.Join(base, filepath.FromSlash(relative)) {
				return true
			}
		}
	}
	return false
}

func atomicCommitWorkflowUsesRelativeScript(commandText string) bool {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(commandText))
	return len(tokens) >= 2 && !filepath.IsAbs(filepath.Clean(tokens[1]))
}

// atomicCommitWorkflowInstallBases는 하네스 설치기가 실제로 만드는 skill
// root만 돌려준다. 임의의 `/tmp/.../skills` suffix는 이 목록에 들어오지 않는다.
func atomicCommitWorkflowInstallBases(req HookToolUseLifecycleRequest) []string {
	candidates := []string{
		req.ExpectedWorktree,
		req.SourceCheckout,
		os.Getenv("HARNESS_ROOT"),
		os.Getenv("CODEX_HOME"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".codex"), filepath.Join(home, ".claude"))
	}
	for _, root := range []string{req.ExpectedWorktree, req.SourceCheckout} {
		if filepath.IsAbs(strings.TrimSpace(root)) {
			candidates = append(candidates, filepath.Join(root, ".claude"))
		}
	}

	bases := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate != strings.TrimSpace(candidate) || !filepath.IsAbs(candidate) {
			continue
		}
		base := cleanAbsPath(candidate)
		if base == "" || seen[base] {
			continue
		}
		seen[base] = true
		bases = append(bases, base)
	}
	return bases
}

func positiveDuration(value string) bool {
	duration, err := time.ParseDuration(value)
	return err == nil && duration > 0
}

func executionMutationTargets(req HookToolUseLifecycleRequest) []string {
	targets := []string{}
	base := hookRequestPathBase(req)
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(base, path); target != "" {
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && searchrouting.IsShellTool(req.Tool) {
		receiptPaths := exactIssueOpsOwnerReceiptPaths(base, req.Command)
		for _, path := range shellCommandWorktreeGuardPaths(base, req.Command) {
			if target := resolveHookTargetPath(base, path); target != "" {
				if receiptPaths[target] {
					continue
				}
				targets = append(targets, target)
			}
		}
	}
	return targets
}

// exactIssueOpsOwnerReceiptPaths는 native process 영수증에 든 실행 파일 경로를
// 변경 대상에서 제외한다. 이 값은 holder identity를 증명하는 관찰값이며 실제
// 파일 접근 대상이 아니다. 나머지 절대 경로는 기존 canonical root fence가 본다.
func exactIssueOpsOwnerReceiptPaths(base, commandText string) map[string]bool {
	if !exactIssueOpsOwnerMutation(commandText) {
		return nil
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok {
		return nil
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return nil
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return nil
	}
	executable, ok := oneFlag(flags, "--session-executable")
	if !ok {
		return nil
	}
	target := resolveHookTargetPath(base, executable)
	if target == "" {
		return nil
	}
	return map[string]bool{target: true}
}

func executionRequestTargetsStayInside(req HookToolUseLifecycleRequest, targets []string, root string) bool {
	if len(targets) == 0 {
		return sameExecutionPath(req.CWD, root)
	}
	return allExecutionTargetsInside(targets, root)
}

func executionUnsafeMutationReason(req HookToolUseLifecycleRequest) string {
	if !searchrouting.IsShellTool(req.Tool) {
		if toolUseMayMutateLifecycleFiles(req.Tool, req.Command) && len(req.Paths) == 0 {
			return "filesystem mutation target is unresolved; provide one exact path inside the canonical IssueOps worktree"
		}
		return ""
	}
	command := strings.TrimSpace(req.Command)
	if commandparse.HasUnquotedBackgroundOperator(command) || executionDetachedShellCommand(command) {
		return "background or detached mutation is blocked; run the command in the foreground and observe it to completion in the holder session"
	}
	upstreamBranch, exactUpstream := worktreeguard.MatchingOriginUpstreamBranch(command)
	if worktreeguard.SealedGitTopologyMutation(command) ||
		(exactUpstream && upstreamBranch != gitBranchFromHead(req.CWD)) {
		return "the IssueOps branch and worktree identity are sealed; direct switch/reset/rebase/merge/force-push/worktree mutation is blocked"
	}
	if commandparse.HasUnquotedControlOperator(command) || commandparse.HasActiveCommandSubstitution(command) ||
		commandparse.HasActiveInputRedirect(command) || commandparse.HasActiveParameterOrTildeExpansion(command) ||
		commandparse.HasActivePathnameExpansion(command) || commandparse.HasActiveShellSpecialQuoting(command) ||
		commandparse.HasActiveZshEqualsExpansion(command) || executionEvalWrapper(command) {
		return "shell substitution or wrapper target is not statically resolvable; use one exact foreground command with literal paths"
	}
	return ""
}

func executionDetachedShellCommand(command string) bool {
	for _, token := range commandparse.SplitCommandTokens(command) {
		switch searchrouting.SearchTokenName(token) {
		case "nohup", "daemonize", "setsid", "disown":
			return true
		}
		value := strings.ToLower(strings.TrimSpace(token))
		for _, flag := range []string{"--detach", "--detached", "--daemon", "--daemonize", "--background"} {
			if value == flag || strings.HasPrefix(value, flag+"=") {
				return true
			}
		}
	}
	return false
}

func executionEvalWrapper(command string) bool {
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		switch searchrouting.SearchTokenName(token) {
		case "bash", "sh", "zsh":
			for _, arg := range tokens[i+1:] {
				if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(strings.TrimPrefix(arg, "-"), "c") {
					return true
				}
			}
		case "python", "python3":
			if containsExecutionToken(tokens[i+1:], "-c") {
				return true
			}
		case "node":
			if containsExecutionToken(tokens[i+1:], "-e") || containsExecutionToken(tokens[i+1:], "--eval") {
				return true
			}
		}
	}
	return false
}

func containsExecutionToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func executionGuardRecords(req HookToolUseLifecycleRequest, targets []string) ([]IssueOpsRecord, error) {
	records := []IssueOpsRecord{}
	ids, err := issueopscore.ListIssueOpsIDs(IssueOpsStateRoot())
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		record, readErr := ReadIssueOps(IssueOpsStateRoot(), id)
		if readErr != nil {
			return nil, readErr
		}
		if executionRecordTouchesRequest(record, req, targets) {
			records = append(records, record)
		}
	}
	return records, nil
}

func executionRecordTouchesRequest(record IssueOpsRecord, req HookToolUseLifecycleRequest, targets []string) bool {
	if record.Execution == nil {
		return false
	}
	return requestTouchesExecution(req, targets, *record.Execution)
}

func requestTouchesExecution(req HookToolUseLifecycleRequest, targets []string, execution issueopsmodel.Execution) bool {
	root := cleanAbsPath(execution.Workspace.Root)
	for _, path := range targets {
		if pathWithin(cleanAbsPath(path), root) {
			return true
		}
	}
	return len(targets) == 0 && pathWithin(cleanAbsPath(req.CWD), root)
}

// executionActorMismatchAxis는 훅 관측 identity와 holder가 처음 어긋난 축을
// 보고한다. executionActorMatches의 비교 순서와 동일해야 진단이 정확하다.
func executionActorMismatchAxis(req HookToolUseLifecycleRequest, holder *issueopsmodel.NativeActor) string {
	switch {
	case holder.SessionProcess == nil:
		return "holder_session_process_missing"
	case !strings.EqualFold(strings.TrimSpace(req.Host), holder.Host):
		return "host"
	case strings.TrimSpace(req.SessionID) != holder.SessionID:
		return "session_id"
	case strings.TrimSpace(req.AgentID) != holder.AgentID:
		return "agent_id"
	default:
		return "session_process_ancestry"
	}
}

func executionActorMatches(req HookToolUseLifecycleRequest, holder *issueopsmodel.NativeActor) bool {
	if holder == nil || holder.SessionProcess == nil || !strings.EqualFold(strings.TrimSpace(req.Host), holder.Host) ||
		strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.SessionID) != holder.SessionID ||
		strings.TrimSpace(req.AgentID) != holder.AgentID {
		return false
	}
	for _, observed := range req.NativeProcessAncestry {
		if observed == *holder.SessionProcess {
			return true
		}
	}
	return false
}

func allExecutionTargetsInside(targets []string, root string) bool {
	if len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		if !executionResolvedTargetInside(target, root) {
			return false
		}
	}
	return true
}

func executionResolvedTargetInside(target, root string) bool {
	resolvedRoot, err := filepath.EvalSymlinks(cleanAbsPath(root))
	if err != nil {
		return false
	}
	resolvedTarget, ok := executionResolveExistingAncestor(target)
	return ok && pathWithin(resolvedTarget, resolvedRoot)
}

func executionResolveExistingAncestor(path string) (string, bool) {
	current := cleanAbsPath(path)
	if current == "" {
		return "", false
	}
	suffix := []string{}
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", false
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return cleanAbsPath(resolved), true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func sameExecutionPath(left, right string) bool {
	left, right = cleanAbsPath(left), cleanAbsPath(right)
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	return left != "" && left == right
}

func executionMutationDenyReason(record IssueOpsRecord) (string, *IssueOpsDenyReason) {
	execution := record.Execution
	root := execution.Workspace.Root
	generation := execution.Lease.Generation
	switch execution.Lease.Status {
	case issueopsmodel.LeaseStatusRevoking:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is revoking and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_revoking", next)
	case issueopsmodel.LeaseStatusClaimable:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is claimable and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_claimable", next)
	case issueopsmodel.LeaseStatusReleased:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is released and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_released", next)
	default:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("mutation requires the current write lease for IssueOps execution %s generation %d and canonical root %s; inspect with `%s`", record.ID, generation, root, next), executionDeny(record, "write_lease_required", next)
	}
}

func executionDeny(record IssueOpsRecord, code, nextCommand string) *IssueOpsDenyReason {
	return &IssueOpsDenyReason{
		Code: code, LifecycleID: record.ID, ExpectedRoot: record.Execution.Workspace.Root,
		CurrentGeneration: record.Execution.Lease.Generation, NextCommand: nextCommand,
	}
}

func executionStatusCommand(id string) string {
	return fmt.Sprintf("agent-harness issueops execution status --id %s --json", id)
}

func exactRemoteScoreObservation(flags map[string][]string) bool {
	input, ok := oneFlag(flags, "--input")
	if !ok || strings.TrimSpace(input) == "" {
		return false
	}
	judge, hasJudge := oneFlag(flags, "--judge")
	judgeFile, hasJudgeFile := oneFlag(flags, "--judge-file")
	if !hasJudge {
		return !hasJudgeFile
	}
	switch judge {
	case "none":
		return !hasJudgeFile
	case "file":
		return hasJudgeFile && strings.TrimSpace(judgeFile) != ""
	default:
		return false
	}
}
