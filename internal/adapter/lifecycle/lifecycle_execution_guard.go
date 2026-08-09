package lifecycle

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	commandparsecontract "agent-harness/internal/contract/commandparse"
	issueopscontract "agent-harness/internal/contract/issueops"
	lifecyclecontract "agent-harness/internal/contract/lifecycle"

	"agent-harness/internal/adapter/lifecycle/worktreeguard"
	"agent-harness/internal/domain/commandparse"
	"agent-harness/internal/domain/searchrouting"
)

// 관찰 권한은 동시 execution 중 owner를 먼저 고르는 절차에 의존하지 않는다.
func executionObservation(req lifecyclecontract.HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		return explicitIssueOpsReadOnlyTool(req.Tool)
	}
	if commandparse.ExactReadOnlyShellCommand(req.Command) {
		return true
	}
	// child host smoke는 관찰 명령은 아니지만, 완료된 child의 lease를 release한
	// 뒤 merge 전에 실행해야 한다. 일반 mutation fence를 적용하면 그 정상
	// 순서에서는 writer가 존재할 수 없어 명령이 영구 차단된다. provider linked
	// branch 생성과 같은 이유로 정확한 coordinator form만 여기서 admit하고,
	// HEAD/ref 일치와 user-scope 활성화·복구는 runner가 fail-closed로 검증한다.
	if exactCoordinatorChildHostSmoke(req) {
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
	case "status", "execution status", "pr-readiness", "cleanup status", "branch await-link":
		// cleanup status의 --merged가 원격을 조회하지만 그것도 읽기다.
		// cleanup remote-branch --preview가 같은 자격으로 원격 OID를 관측하는
		// 선례가 있다.
		//
		// branch await-link는 같은 관측을 주기적으로 되풀이할 뿐 아무것도
		// 쓰지 않는다. 이것이 관찰로 인정되지 않으면 owner는 pre-link 창을
		// 기다릴 수 없고, 그 창을 terminal 실패로 다루게 된다(#319).
		id, ok := oneFlag(flags, "--id")
		return ok && strings.TrimSpace(id) != ""
	case "child status":
		parent, ok := oneFlag(flags, "--parent")
		_, repair := flags["--repair"]
		return ok && strings.TrimSpace(parent) != "" && !repair
	case "child list":
		parent, ok := oneFlag(flags, "--parent")
		return ok && strings.TrimSpace(parent) != ""
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
	default:
		return false
	}
}

func generatedIssueOpsExecutableBlock(req lifecyclecontract.HookToolUseLifecycleRequest) (string, *lifecyclecontract.IssueOpsDenyReason) {
	if !searchrouting.IsShellTool(req.Tool) {
		return "", nil
	}
	commandText := strings.TrimSpace(req.Command)
	hasEnvelope := strings.Contains(commandText, commandparsecontract.GeneratedByExecutableFlag) ||
		strings.Contains(commandText, commandparsecontract.GeneratedBySHA256Flag) ||
		strings.Contains(commandText, commandparsecontract.GeneratedForGenerationFlag)
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok {
		if hasEnvelope {
			return "generated IssueOps command provenance is malformed or uses an unsafe shell form", &lifecyclecontract.IssueOpsDenyReason{Code: "generated_command_provenance_invalid"}
		}
		return "", nil
	}
	if !filepath.IsAbs(command.Tokens[0]) {
		return "", nil
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return "generated IssueOps command uses an unsupported absolute executable", &lifecyclecontract.IssueOpsDenyReason{Code: "generated_command_executable_untrusted"}
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return "generated IssueOps command provenance flags are invalid", &lifecyclecontract.IssueOpsDenyReason{Code: "generated_command_provenance_invalid"}
	}
	id, ok := oneFlag(flags, commandparse.IssueOpsLifecycleIDFlag(command.Path))
	if !ok || strings.TrimSpace(id) == "" {
		return "generated IssueOps command requires an exact lifecycle id", &lifecyclecontract.IssueOpsDenyReason{Code: "generated_command_provenance_invalid"}
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), strings.TrimSpace(id))
	if err != nil {
		return "generated IssueOps command executable cannot be matched to durable execution state", &lifecyclecontract.IssueOpsDenyReason{Code: "generated_command_executable_untrusted"}
	}
	authority, delegatedBootstrap := generatedIssueOpsAuthorityRecord(command.Path, record)
	if authority.Execution == nil {
		return "generated IssueOps command executable cannot be matched to durable execution state", &lifecyclecontract.IssueOpsDenyReason{Code: "generated_command_executable_untrusted"}
	}
	if err := issueopscontract.ValidateExecution(*authority.Execution); err != nil {
		return "generated IssueOps command executable cannot trust an invalid execution record", executionDeny(authority, "generated_command_executable_untrusted", executionStatusCommand(authority.ID))
	}
	if delegatedBootstrap && !generatedDelegatedBootstrapMatchesParent(req, command.Path, flags, record, authority) {
		return "generated delegated child bootstrap does not match the current parent authority", executionDeny(authority, "generated_command_provenance_invalid", executionStatusCommand(authority.ID))
	}
	executable, err := filepath.EvalSymlinks(command.Tokens[0])
	if err != nil || cleanAbsPath(executable) != cleanAbsPath(command.Tokens[0]) {
		return "generated IssueOps command executable must be an existing canonical path", executionDeny(authority, "generated_command_executable_untrusted", executionStatusCommand(authority.ID))
	}
	trusted := []string{
		filepath.Join(authority.Execution.Workspace.Root, "bin", "agent-harness"),
		filepath.Join(authority.Execution.Workspace.SourceRoot, "bin", "agent-harness"),
	}
	if parent := strings.TrimSpace(authority.Execution.Workspace.ParentWorktree); parent != "" {
		trusted = append(trusted, filepath.Join(parent, "bin", "agent-harness"))
	}
	for _, candidate := range trusted {
		resolved, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr == nil && cleanAbsPath(resolved) == cleanAbsPath(executable) {
			return "", nil
		}
	}
	return "generated IssueOps command executable is outside the durable worktree and trusted installed targets", executionDeny(authority, "generated_command_executable_untrusted", executionStatusCommand(authority.ID))
}

func generatedIssueOpsAuthorityRecord(path string, record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, bool) {
	if record.Execution != nil {
		return record, false
	}
	if path != "branch prepare" && path != "execution prepare" || record.Delegation == nil {
		return issueopscontract.IssueOpsRecord{}, false
	}
	parentID := strings.TrimSpace(record.Delegation.ParentCycleID)
	if parentID == "" {
		return issueopscontract.IssueOpsRecord{}, false
	}
	parent, err := ReadIssueOps(IssueOpsStateRoot(), parentID)
	if err != nil || parent.Execution == nil || !issueOpsParentReferencesChild(parent, record.ID) {
		return issueopscontract.IssueOpsRecord{}, false
	}
	return parent, true
}

func issueOpsParentReferencesChild(parent issueopscontract.IssueOpsRecord, childID string) bool {
	childID = strings.TrimSpace(childID)
	for _, child := range parent.ChildCycles {
		if strings.TrimSpace(child.CycleID) == childID {
			return true
		}
	}
	return false
}

func generatedDelegatedBootstrapMatchesParent(req lifecyclecontract.HookToolUseLifecycleRequest, path string, flags map[string][]string, child, parent issueopscontract.IssueOpsRecord) bool {
	execution := parent.Execution
	if execution == nil || execution.Lease.Status != issueopscontract.LeaseStatusActive || execution.Lease.Holder == nil ||
		!executionActorMatches(req, execution.Lease.Holder) {
		return false
	}
	host, hostOK := oneFlag(flags, "--host")
	sessionID, sessionOK := oneFlag(flags, "--session-id")
	cwd, cwdOK := oneFlag(flags, "--cwd")
	generation, generationOK := oneFlag(flags, commandparsecontract.GeneratedForGenerationFlag)
	parsedGeneration, err := strconv.ParseUint(generation, 10, 64)
	if !hostOK || !sessionOK || !cwdOK || !generationOK || err != nil || parsedGeneration != execution.Lease.Generation ||
		!strings.EqualFold(strings.TrimSpace(host), strings.TrimSpace(execution.Lease.Holder.Host)) ||
		strings.TrimSpace(sessionID) != strings.TrimSpace(execution.Lease.Holder.SessionID) ||
		!sameExecutionPath(cwd, execution.Workspace.Root) {
		return false
	}
	agentID, hasAgentID := oneFlag(flags, "--agent-id")
	if strings.TrimSpace(execution.Lease.Holder.AgentID) != strings.TrimSpace(agentID) ||
		(strings.TrimSpace(execution.Lease.Holder.AgentID) != "" && !hasAgentID) {
		return false
	}
	switch path {
	case "branch prepare":
		branch, branchOK := oneFlag(flags, "--branch")
		baseBranch, baseOK := oneFlag(flags, "--base-branch")
		parentWorktree, worktreeOK := oneFlag(flags, "--parent-worktree")
		return branchOK && baseOK && worktreeOK && strings.TrimSpace(branch) == strings.TrimSpace(child.Branch) &&
			strings.TrimSpace(baseBranch) == strings.TrimSpace(parent.Branch) && sameExecutionPath(parentWorktree, execution.Workspace.Root)
	case "execution prepare":
		prepared := child.BranchPrepare
		return prepared != nil && strings.TrimSpace(prepared.Branch) == strings.TrimSpace(child.Branch) &&
			strings.TrimSpace(prepared.BaseBranch) == strings.TrimSpace(parent.Branch) && sameExecutionPath(prepared.ParentWorktree, execution.Workspace.Root)
	default:
		return false
	}
}

func exactCoordinatorChildHostSmoke(req lifecyclecontract.HookToolUseLifecycleRequest) bool {
	commandText := strings.TrimSpace(req.Command)
	if commandText == "" || commandparse.HasUnquotedControlOperator(commandText) ||
		commandparse.HasActiveCommandSubstitution(commandText) ||
		commandparse.HasActiveInputRedirect(commandText) ||
		commandparse.HasActiveOutputRedirect(commandText) ||
		commandparse.HasActiveParameterOrTildeExpansion(commandText) ||
		commandparse.HasActivePathnameExpansion(commandText) ||
		commandparse.HasActiveShellSpecialQuoting(commandText) ||
		commandparse.HasActiveZshEqualsExpansion(commandText) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(commandText)
	if len(tokens) < 2 || !filepath.IsAbs(tokens[0]) {
		return false
	}
	flags, ok := commandparse.ExactFlags(
		commandparse.ExactIssueOpsCommand{Path: "child host smoke", Tokens: tokens, Start: 1},
		map[string]bool{
			"--issue": true, "--source-root": true, "--child-root": true,
			"--head": true, "--remote-ref": true, "--json-out": true,
		},
		map[string]bool{"--confirm-user-activation": true},
		map[string]bool{},
	)
	if !ok {
		return false
	}
	issue, issueOK := oneFlag(flags, "--issue")
	sourceRoot, sourceOK := oneFlag(flags, "--source-root")
	childRoot, childOK := oneFlag(flags, "--child-root")
	head, headOK := oneFlag(flags, "--head")
	remoteRef, refOK := oneFlag(flags, "--remote-ref")
	jsonOut, outputOK := oneFlag(flags, "--json-out")
	_, confirmed := flags["--confirm-user-activation"]
	if !issueOK || !positiveIssueNumber(issue) || !sourceOK || !childOK || !headOK ||
		!refOK || !outputOK || !confirmed {
		return false
	}
	if !canonicalRealDirectory(sourceRoot) || !canonicalRealDirectory(childRoot) ||
		!canonicalAbsolutePath(jsonOut) || sameExecutionPath(sourceRoot, childRoot) {
		return false
	}
	coordinatorRoot, ok := delegatedChildSmokeCoordinator(sourceRoot, childRoot, issue)
	if !ok {
		return false
	}
	scriptPath := filepath.Join(coordinatorRoot, "scripts", "verify-child-host-smoke.sh")
	if tokens[0] != scriptPath {
		return false
	}
	sourceAuthority := strings.TrimSpace(req.SourceCheckout)
	if sourceAuthority != "" {
		// 명시적 SourceCheckout이 있으면 불일치를 Repo로 우회하지 않는다.
		if !sameExecutionPath(sourceRoot, sourceAuthority) {
			return false
		}
	}
	requestBase := hookRequestPathBase(req)
	if !sameExecutionPath(requestBase, sourceRoot) && !sameExecutionPath(requestBase, coordinatorRoot) {
		return false
	}
	if repo := strings.TrimSpace(req.Repo); repo != "" &&
		!sameExecutionPath(repo, sourceRoot) && !sameExecutionPath(repo, coordinatorRoot) {
		return false
	}
	if !trustedIssueOpsCheckout(coordinatorRoot, sourceRoot) ||
		!trustedIssueOpsCheckout(childRoot, sourceRoot) || sameExecutionPath(coordinatorRoot, childRoot) {
		return false
	}
	scriptInfo, err := os.Lstat(scriptPath)
	if err != nil || !scriptInfo.Mode().IsRegular() || scriptInfo.Mode()&0o111 == 0 {
		return false
	}
	if len(head) != 40 || head != strings.ToLower(head) {
		return false
	}
	if _, err := hex.DecodeString(head); err != nil {
		return false
	}
	branch := strings.TrimPrefix(remoteRef, "refs/heads/")
	if branch == remoteRef || branch == "" || branch != filepath.Base(childRoot) ||
		!strings.HasPrefix(branch, issue+"-") || !validFlatSmokeBranch(branch) {
		return false
	}
	outputParent := filepath.Dir(jsonOut)
	parentInfo, err := os.Lstat(outputParent)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || parentInfo.Mode().Perm() != 0o700 {
		return false
	}
	if resolvedParent, err := filepath.EvalSymlinks(outputParent); err != nil || resolvedParent != outputParent {
		return false
	}
	if outputInfo, err := os.Lstat(jsonOut); err == nil {
		if !outputInfo.Mode().IsRegular() || outputInfo.Mode().Perm() != 0o600 {
			return false
		}
	} else if !os.IsNotExist(err) {
		return false
	}
	return true
}

func childHostSmokeInvocation(req lifecyclecontract.HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	if len(tokens) == 0 {
		return false
	}
	if childHostSmokeScriptToken(tokens[0]) {
		return true
	}
	if len(tokens) < 2 || !childHostSmokeScriptToken(tokens[1]) {
		return false
	}
	switch searchrouting.SearchTokenName(tokens[0]) {
	case "bash", "sh", "zsh":
		return true
	default:
		return false
	}
}

func childHostSmokeScriptToken(token string) bool {
	if token == "scripts/verify-child-host-smoke.sh" {
		return true
	}
	cleaned := filepath.Clean(token)
	return filepath.IsAbs(cleaned) && filepath.Base(cleaned) == "verify-child-host-smoke.sh" &&
		filepath.Base(filepath.Dir(cleaned)) == "scripts"
}

func delegatedChildSmokeCoordinator(sourceRoot, childRoot, issue string) (string, bool) {
	ids, err := issueOpsDeps.ListIssueOpsIDs(IssueOpsStateRoot())
	if err != nil {
		return "", false
	}
	var child *issueopscontract.IssueOpsRecord
	for _, id := range ids {
		record, err := ReadIssueOps(IssueOpsStateRoot(), id)
		if err != nil {
			return "", false
		}
		if record.Execution == nil || !sameExecutionPath(record.Execution.Workspace.Root, childRoot) {
			continue
		}
		if child != nil {
			return "", false
		}
		candidate := record
		child = &candidate
	}
	if child == nil || child.Execution == nil || child.Delegation == nil ||
		strings.TrimSpace(child.Delegation.ParentCycleID) == "" ||
		child.Execution.Lease.Status != issueopscontract.LeaseStatusReleased ||
		!sameExecutionPath(child.Repo, sourceRoot) ||
		!sameExecutionPath(child.Execution.Workspace.SourceRoot, sourceRoot) ||
		filepath.Base(strings.TrimRight(child.IssueURL, "/")) != issue {
		return "", false
	}
	if err := issueopscontract.ValidateExecution(*child.Execution); err != nil {
		return "", false
	}
	parent, err := ReadIssueOps(IssueOpsStateRoot(), child.Delegation.ParentCycleID)
	if err != nil || parent.Execution == nil ||
		parent.Execution.Lease.Status != issueopscontract.LeaseStatusReleased ||
		!sameExecutionPath(parent.Repo, sourceRoot) ||
		!sameExecutionPath(parent.Execution.Workspace.SourceRoot, sourceRoot) {
		return "", false
	}
	if err := issueopscontract.ValidateExecution(*parent.Execution); err != nil {
		return "", false
	}
	linked := false
	for _, ref := range parent.ChildCycles {
		if ref.CycleID == child.ID && ref.Branch == child.Branch &&
			(ref.ChildIssueURL == "" || ref.ChildIssueURL == child.IssueURL) {
			linked = true
			break
		}
	}
	coordinatorRoot := parent.Execution.Workspace.Root
	if !linked || !trustedIssueOpsCheckout(coordinatorRoot, sourceRoot) ||
		!trustedIssueOpsCheckout(childRoot, sourceRoot) || sameExecutionPath(coordinatorRoot, childRoot) {
		return "", false
	}
	return coordinatorRoot, true
}

func canonicalAbsolutePath(path string) bool {
	return path != "" && path == strings.TrimSpace(path) && filepath.IsAbs(path) && filepath.Clean(path) == path
}

func canonicalRealDirectory(path string) bool {
	if !canonicalAbsolutePath(path) {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && resolved == path
}

func validFlatSmokeBranch(branch string) bool {
	if branch == "" || strings.Contains(branch, "..") {
		return false
	}
	for _, char := range branch {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func trustedIssueOpsCheckout(root, sourceRoot string) bool {
	root, sourceRoot = cleanAbsPath(root), cleanAbsPath(sourceRoot)
	if !canonicalRealDirectory(root) || !canonicalRealDirectory(sourceRoot) {
		return false
	}
	if !sameExecutionPath(root, sourceRoot) && filepath.Dir(root) != sourceRoot+".worktrees" {
		return false
	}
	gitInfo, err := os.Lstat(filepath.Join(root, ".git"))
	return err == nil && gitInfo.Mode()&os.ModeSymlink == 0 && (gitInfo.IsDir() || gitInfo.Mode().IsRegular())
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
		return exactGitHubIssueRead(tokens) || exactGitHubLinkedBranchMutation(tokens)
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

// exactGitHubIssueRead는 IssueOps 실행에 필요한 두 조회 형태만 인정한다:
//
//	gh api repos/<owner>/<repo>/issues/<number> --jq .node_id
//	gh api repos/<owner>/<repo>/issues/<number> --jq .body
//
// node id는 linked branch 생성에, body는 봉인된 원격 digest 검증에 필요하다.
// 정확한 GET 경로와 jq projection만 허용해 다른 gh api 표면은 계속 차단한다.
func exactGitHubIssueRead(tokens []string) bool {
	if len(tokens) != 5 || tokens[3] != "--jq" ||
		(tokens[4] != ".node_id" && tokens[4] != ".body") {
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
				"--dispatch-id", "--dispatch-capability", "--outcome",
				"--files-modified", "--report-path", "--phase",
			),
			exactFlagNames("--json"),
			nil,
		)
		if !ok || !nonemptyExactFlags(flags, "--type", "--subject") ||
			!exactOrcaOwnerRecipient(flags) {
			return false
		}
		messageType, _ := oneFlag(flags, "--type")
		outcome, hasOutcome := oneFlag(flags, "--outcome")
		switch messageType {
		case "heartbeat":
			return !hasOutcome && nonemptyExactFlags(flags, "--task-id", "--dispatch-id", "--phase")
		case "worker_done":
			if !nonemptyExactFlags(flags, "--body", "--task-id", "--dispatch-id") {
				return false
			}
			_, capabilityRoute := flags["--dispatch-capability"]
			if !capabilityRoute {
				return !hasOutcome || outcome == "succeeded" || outcome == "failed"
			}
			return hasOutcome && (outcome == "succeeded" || outcome == "failed")
		case "escalation":
			return !hasOutcome && nonemptyExactFlags(flags, "--body", "--task-id")
		case "decision_gate", "reply":
			return !hasOutcome && nonemptyExactFlags(flags, "--body")
		default:
			return false
		}
	case "ask":
		flags, ok := commandparse.ExactFlags(
			exact,
			exactFlagNames("--to", "--from", "--dispatch-capability", "--question", "--options", "--timeout-ms"),
			exactFlagNames("--json"),
			nil,
		)
		if !ok || !nonemptyExactFlags(flags, "--from", "--question") ||
			!exactOrcaOwnerRecipient(flags) {
			return false
		}
		timeout, hasTimeout := oneFlag(flags, "--timeout-ms")
		return !hasTimeout || positiveMilliseconds(timeout)
	case "check":
		flags, ok := commandparse.ExactFlags(
			exact,
			exactFlagNames("--terminal", "--timeout-ms"),
			exactFlagNames("--wait", "--unread", "--inject", "--json"),
			nil,
		)
		if !ok {
			return false
		}
		terminal, hasTerminal := oneFlag(flags, "--terminal")
		_, unread := flags["--unread"]
		_, inject := flags["--inject"]
		if (hasTerminal && strings.TrimSpace(terminal) == "") ||
			(!hasTerminal && !unread) ||
			(inject && !unread) {
			return false
		}
		timeout, hasTimeout := oneFlag(flags, "--timeout-ms")
		_, wait := flags["--wait"]
		return !hasTimeout || wait && positiveMilliseconds(timeout)
	default:
		return false
	}
}

// exactOrcaOwnerRecipient는 예전 terminal 주소와 현재 Dispatch capability 중
// 정확히 하나만 허용한다. capability는 bearer authority이므로 발신 terminal도
// 함께 있어야 하며, 값의 원문은 이 계층에서 기록하지 않는다.
func exactOrcaOwnerRecipient(flags map[string][]string) bool {
	to, hasTo := oneFlag(flags, "--to")
	capability, hasCapability := oneFlag(flags, "--dispatch-capability")
	if hasTo == hasCapability {
		return false
	}
	if hasTo {
		return strings.TrimSpace(to) != ""
	}
	return strings.TrimSpace(capability) != "" && nonemptyExactFlags(flags, "--from")
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

func executionTypedControlPlane(req lifecyclecontract.HookToolUseLifecycleRequest) bool {
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
	case "execution resume":
		id, idOK := oneFlag(flags, "--id")
		generation, generationOK := oneFlag(flags, "--expected-generation")
		parsedGeneration, generationErr := strconv.ParseUint(strings.TrimSpace(generation), 10, 64)
		_, confirm := flags["--confirm"]
		actorFlags := []string{"--host", "--session-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"}
		actorExplicit := false
		for _, name := range append(actorFlags, "--agent-id") {
			_, actorExplicit = flags[name]
			if actorExplicit {
				break
			}
		}
		if actorExplicit && !nonemptyExactFlags(flags, actorFlags...) {
			return false
		}
		return idOK && strings.TrimSpace(id) != "" && generationOK &&
			generationErr == nil && parsedGeneration > 0 && confirm
	case "execution sync-base":
		return exactExecutionSyncBaseTyped(req, flags)
	case "execution prepare", "execution claim", "execution release", "execution replace", "execution reconcile", "execution complete", "execution switch-mode",
		"cleanup orphan":
		id, ok := oneFlag(flags, "--id")
		return ok && strings.TrimSpace(id) != ""
	default:
		return false
	}
}

func executionSyncBaseInvocation(req lifecyclecontract.HookToolUseLifecycleRequest) bool {
	if !searchrouting.IsShellTool(req.Tool) {
		return false
	}
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(req.Command))
	for index := 0; index+3 < len(tokens); index++ {
		if (tokens[index] == "agent-harness" || tokens[index] == "bin/agent-harness" || tokens[index] == "./bin/agent-harness") &&
			tokens[index+1] == "issueops" && tokens[index+2] == "execution" && tokens[index+3] == "sync-base" {
			return true
		}
	}
	return false
}

func exactExecutionSyncBaseTyped(req lifecyclecontract.HookToolUseLifecycleRequest, flags map[string][]string) bool {
	id, idOK := oneFlag(flags, "--id")
	if !idOK || strings.TrimSpace(id) == "" {
		return false
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil || record.Execution == nil || record.Execution.Pending != nil ||
		!exactExecutionSyncBaseHookCWD(req, flags, record.Execution.Workspace.Root) {
		return false
	}
	if _, jsonOut := flags["--json"]; !jsonOut || !exactExecutionSyncBaseMode(flags) {
		return false
	}
	switch record.Execution.Lease.Status {
	case issueopscontract.LeaseStatusActive:
		holder := record.Execution.Lease.Holder
		if holder == nil || !sameExecutionActorRequest(holder, req) {
			return false
		}
	case issueopscontract.LeaseStatusReleased:
		completion := record.Execution.Completion
		generation, generationOK := oneFlag(flags, "--completion-generation")
		parsedGeneration, generationErr := strconv.ParseUint(strings.TrimSpace(generation), 10, 64)
		if completion == nil || completion.Generation == 0 || !generationOK || generationErr != nil || parsedGeneration != completion.Generation {
			return false
		}
	default:
		return false
	}
	if _, preview := flags["--preview"]; preview {
		_, confirm := flags["--confirm"]
		_, fingerprint := flags["--fingerprint"]
		return !confirm && !fingerprint
	}
	return exactExecutionSyncBaseMutationActor(req, flags, record.Execution.Workspace.Root)
}

func exactExecutionSyncBaseHookCWD(req lifecyclecontract.HookToolUseLifecycleRequest, flags map[string][]string, root string) bool {
	if sameExecutionPath(req.CWD, root) {
		return true
	}
	// Codex 0.146의 stable Bash hook payload는 exec_command의 workdir를
	// 전달하지 않고 turn cwd와 command만 전달한다. 이 transport-blind 경로는
	// 앞선 generated-executable 검증을 통과한 absolute command에만 열고,
	// 실제 process cwd 일치는 CLI가 core mutation 전에 다시 검증한다.
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok || len(command.Tokens) == 0 || !filepath.IsAbs(command.Tokens[0]) {
		return false
	}
	commandCWD, ok := oneFlag(flags, "--cwd")
	return ok && sameExecutionPath(commandCWD, root)
}

func exactExecutionSyncBaseMode(flags map[string][]string) bool {
	selected := 0
	for _, name := range []string{"--preview", "--apply", "--finalize", "--abort"} {
		if _, ok := flags[name]; ok {
			selected++
		}
	}
	if selected != 1 {
		return false
	}
	_, apply := flags["--apply"]
	_, confirm := flags["--confirm"]
	fingerprint, hasFingerprint := oneFlag(flags, "--fingerprint")
	if apply {
		if !confirm || !hasFingerprint || len(fingerprint) != 64 {
			return false
		}
		_, err := hex.DecodeString(fingerprint)
		return err == nil
	}
	return !confirm && !hasFingerprint
}

func exactExecutionSyncBaseMutationActor(req lifecyclecontract.HookToolUseLifecycleRequest, flags map[string][]string, root string) bool {
	if !nonemptyExactFlags(flags, "--host", "--session-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd") {
		return false
	}
	host, _ := oneFlag(flags, "--host")
	sessionID, _ := oneFlag(flags, "--session-id")
	pid, _ := oneFlag(flags, "--session-pid")
	cwd, _ := oneFlag(flags, "--cwd")
	parsedPID, err := strconv.Atoi(pid)
	if err != nil || parsedPID <= 0 || strings.ToLower(strings.TrimSpace(host)) != strings.ToLower(strings.TrimSpace(req.Host)) ||
		strings.TrimSpace(sessionID) != strings.TrimSpace(req.SessionID) || !sameExecutionPath(cwd, root) {
		return false
	}
	agentID, hasAgentID := oneFlag(flags, "--agent-id")
	requestAgentID := strings.TrimSpace(req.AgentID)
	return (requestAgentID == "" && !hasAgentID) || (hasAgentID && strings.TrimSpace(agentID) == requestAgentID)
}

func sameExecutionActorRequest(holder *issueopscontract.NativeActor, req lifecyclecontract.HookToolUseLifecycleRequest) bool {
	if holder == nil || strings.ToLower(strings.TrimSpace(holder.Host)) != strings.ToLower(strings.TrimSpace(req.Host)) ||
		strings.TrimSpace(holder.SessionID) != strings.TrimSpace(req.SessionID) || strings.TrimSpace(holder.AgentID) != strings.TrimSpace(req.AgentID) {
		return false
	}
	return true
}

func executionTypedPreLinkBlock(req lifecyclecontract.HookToolUseLifecycleRequest) (string, *lifecyclecontract.IssueOpsDenyReason) {
	if !req.EnforceWorktree {
		return "", nil
	}
	id, ok := executionTypedMutationID(req)
	if !ok {
		return "", nil
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), id)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "IssueOps authority state could not be read (often transient state-store contention); retry once, and if it persists run `agent-harness doctor --repo " + cleanAbsPath(req.Repo) + " --json`", nil
	}
	if record.Execution == nil || record.Execution.Lease.Status != issueopscontract.LeaseStatusActive ||
		!orcaBranchLinkVerificationRequired(record) {
		return "", nil
	}
	if exactOrcaLeaseRelease(req.Command, record) || typedLeaseReleaseAction(req, record) {
		// 반납은 이 창의 안전한 출구다. 막으면 진행도 반납도 못 하는 덫이 된다.
		// shell과 MCP 두 표면이 같은 판정을 해야 한다 — 한쪽만 열면 owner가
		// 어느 표면을 쓰느냐에 따라 회수 가능성이 갈린다.
		return "", nil
	}
	return orcaBranchLinkDenyReason(record), executionDeny(record, "branch_link_verification_required", executionStatusCommand(record.ID))
}

func executionTypedMutationID(req lifecyclecontract.HookToolUseLifecycleRequest) (string, bool) {
	if !searchrouting.IsShellTool(req.Tool) {
		action, actionOK := req.ToolInput["action"].(string)
		id, idOK := req.ToolInput["id"].(string)
		switch strings.TrimSpace(action) {
		case issueopscontract.ExecutionActionPrepare, issueopscontract.ExecutionActionClaim,
			issueopscontract.ExecutionActionRelease, issueopscontract.ExecutionActionReplace,
			issueopscontract.ExecutionActionResume, issueopscontract.ExecutionActionReconcile,
			issueopscontract.ExecutionActionComplete:
			return strings.TrimSpace(id), actionOK && idOK && strings.TrimSpace(id) != ""
		default:
			return "", false
		}
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok {
		return "", false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return "", false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return "", false
	}
	id, ok := oneFlag(flags, "--id")
	return strings.TrimSpace(id), ok && strings.TrimSpace(id) != ""
}

func executionMutationDecision(req lifecyclecontract.HookToolUseLifecycleRequest) (bool, string, *lifecyclecontract.IssueOpsDenyReason) {
	if !req.EnforceWorktree {
		return false, "", nil
	}
	unsafeReason := executionUnsafeMutationReason(req)
	if unsafeReason == "" && searchrouting.IsShellTool(req.Tool) && !executionTypedControlPlane(req) {
		if command, ok := commandparse.ParseExactIssueOpsCommand(req.Command); ok && command.Path == "execution resume" {
			unsafeReason = "unclassified IssueOps execution resume command is blocked; use the exact generation-bound confirmed control-plane form"
		}
	}
	resourceWaitRoot, exactResourceWait := exactOwnedResourceWait(req.Command)
	atomicWorkflowRoot, exactAtomicWorkflow := exactAtomicCommitWorkflowScript(req)
	releasedPlanRecoveryID, exactReleasedPlanRecovery := exactReleasedPlanArtifactStage(req.Command)
	if !exactReleasedPlanRecovery {
		releasedPlanRecoveryID, exactReleasedPlanRecovery = exactReleasedPlanLink(req.Command)
	}
	atomicWorkflowRelativeScript := exactAtomicWorkflow && atomicCommitWorkflowUsesRelativeScript(req.Command)
	temporaryBuildOutput, exactTemporaryBuild := "", false
	if unsafeReason == "" {
		temporaryBuildOutput, exactTemporaryBuild = exactTemporaryAgentHarnessBuildOutput(req.Command)
	}
	mayMutate := toolUseMayMutateLifecycleFiles(req.Tool, req.Command)
	if searchrouting.IsShellTool(req.Tool) && !mayMutate {
		mayMutate = true
		if unsafeReason == "" && !exactIssueOpsOwnerMutation(req.Command) && !exactReleasedPlanRecovery && !exactResourceWait && !exactAtomicWorkflow &&
			!commandparse.ExactSelfVerifyVerification(req.Command) {
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
	if exactTemporaryBuild {
		// 출력 파일만 보면 active lifecycle을 찾을 수 없다. canonical cwd와
		// command에서 봉인한 출력 경로를 함께 판정해 holder 검사를 유지한다.
		targets = append(targets, temporaryBuildOutput, cleanAbsPath(req.CWD))
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
		if err := issueopscontract.ValidateExecution(*record.Execution); err != nil {
			return true, "invalid IssueOps execution v1 record: " + err.Error(), nil
		}
		lease := record.Execution.Lease
		root := record.Execution.Workspace.Root
		if atomicWorkflowRelativeScript && !sameExecutionPath(atomicWorkflowRoot, root) {
			reason := "relative atomic commit workflow scripts must run from the canonical IssueOps worktree root"
			return true, reason, executionDeny(record, "unsafe_mutation", executionStatusCommand(record.ID))
		}
		targetsAuthorized := executionRequestTargetsStayInside(req, targets, root)
		if exactTemporaryBuild {
			targetsAuthorized = executionTemporaryBuildTargetsAuthorized(req, targets, root, temporaryBuildOutput)
		}
		if lease.Status == issueopscontract.LeaseStatusReleased && unsafeReason == "" &&
			executionSyncBaseResolutionAllows(req, *record.Execution, targets, root) {
			return true, "", nil
		}
		if exactReleasedPlanRecovery && record.Execution.Mode == issueopscontract.ExecutionModeOrca &&
			releasedPlanRecoveryID == record.ID &&
			lease.Status == issueopscontract.LeaseStatusReleased && lease.Holder == nil &&
			record.Execution.Pending == nil && record.Execution.Completion == nil && targetsAuthorized {
			return true, "", nil
		}
		if lease.Status == issueopscontract.LeaseStatusActive && executionActorMatches(req, lease.Holder) &&
			targetsAuthorized {
			if orcaBranchLinkVerificationRequired(record) && !exactOrcaBranchLinkRecorder(req.Command, record) &&
				!exactOrcaBranchLinkWait(req.Command, record) && !exactOrcaLeaseRelease(req.Command, record) {
				return true, orcaBranchLinkDenyReason(record), executionDeny(record, "branch_link_verification_required", executionStatusCommand(record.ID))
			}
			return true, "", nil
		}
		if lease.Status == issueopscontract.LeaseStatusActive && lease.Holder != nil && !executionActorMatches(req, lease.Holder) {
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

func executionSyncBaseResolutionAllows(req lifecyclecontract.HookToolUseLifecycleRequest, execution issueopscontract.Execution, targets []string, root string) bool {
	resolution := execution.SyncBaseResolution
	if resolution == nil || execution.Lease.Status != issueopscontract.LeaseStatusReleased ||
		resolution.Generation != execution.Lease.Generation || !executionActorMatches(req, &resolution.Actor) || len(targets) == 0 {
		return false
	}
	allowed := map[string]bool{}
	for _, relative := range resolution.ConflictFiles {
		if target := resolveHookTargetPath(root, relative); target != "" {
			allowed[target] = true
		}
	}
	for _, target := range targets {
		if !allowed[cleanAbsPath(target)] {
			return false
		}
	}
	return true
}

func exactIssueOpsMutationHelpObservation(path string, tokens []string, start int) bool {
	switch path {
	case "remote create-child", "remote create-pr", "remote verify-artifact":
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
	_, ok = commandparse.ExactIssueOpsOwnerMutation(command)
	return ok
}

func exactReleasedPlanArtifactStage(commandText string) (string, bool) {
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "artifact stage" {
		return "", false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return "", false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return "", false
	}
	id, idOK := oneFlag(flags, "--id")
	name, nameOK := oneFlag(flags, "--name")
	file, fileOK := oneFlag(flags, "--file")
	_, jsonOK := flags["--json"]
	exact := idOK && strings.TrimSpace(id) != "" && nameOK && name == "plan" &&
		fileOK && strings.TrimSpace(file) != "" && jsonOK
	return strings.TrimSpace(id), exact
}

func exactReleasedPlanLink(commandText string) (string, bool) {
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "link-plan" {
		return "", false
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return "", false
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return "", false
	}
	id, found := oneFlag(flags, "--id")
	if !found || strings.TrimSpace(id) == "" {
		return "", false
	}
	for _, name := range []string{"--plan-path", "--host", "--session-id", "--cwd"} {
		value, found := oneFlag(flags, name)
		if !found || strings.TrimSpace(value) == "" {
			return "", false
		}
	}
	_, jsonOK := flags["--json"]
	return strings.TrimSpace(id), jsonOK
}

func orcaBranchLinkVerificationRequired(record issueopscontract.IssueOpsRecord) bool {
	return record.Execution != nil && record.Execution.Mode == issueopscontract.ExecutionModeOrca &&
		(record.BranchPrepare == nil || !record.BranchPrepare.LinkVerified)
}

func exactOrcaBranchLinkRecorder(commandText string, record issueopscontract.IssueOpsRecord) bool {
	prepared := record.BranchPrepare
	if prepared == nil {
		return false
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "branch prepare" {
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
	if _, verified := flags["--link-verified"]; !verified {
		return false
	}
	required := map[string]string{
		"--id": record.ID, "--provider": strings.ToLower(strings.TrimSpace(prepared.Provider)),
		"--issue-url": strings.TrimSpace(prepared.IssueURL), "--branch": strings.TrimSpace(prepared.Branch),
		"--base-branch": strings.TrimSpace(prepared.BaseBranch),
	}
	for name, want := range required {
		got, found := oneFlag(flags, name)
		if !found || strings.TrimSpace(got) != want {
			return false
		}
	}
	for _, optional := range []struct{ name, want string }{
		{"--base-sha", prepared.BaseSHA},
		{"--parent-worktree", prepared.ParentWorktree},
		{"--remote-branch-url", prepared.RemoteBranchURL},
	} {
		got, found := oneFlag(flags, optional.name)
		want := strings.TrimSpace(optional.want)
		if found != (want != "") || found && strings.TrimSpace(got) != want {
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
func exactAtomicCommitWorkflowScript(req lifecyclecontract.HookToolUseLifecycleRequest) (string, bool) {
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
	if root == "" {
		return "", false
	}
	if root != cwd && !selfDescribingAtomicWorkflowInvocation(tokens, root) {
		return "", false
	}
	return root, true
}

// selfDescribingAtomicWorkflowInvocation은 script와 repo 인자가 **둘 다 절대
// 경로이고 같은 root를 지목하는** 호출인지 보고한다.
//
// Codex 0.146의 stable hook payload는 exec_command의 workdir를 전달하지 않아
// hook이 보는 cwd는 언제나 turn의 source checkout이다. 그 상태에서 cwd 일치를
// 요구하면 canonical worktree의 봉인된 preflight를 실행할 방법이 없어진다(#331).
//
// 관측할 수 없는 workdir를 추측하는 대신, 명령 자체가 대상을 완전히 기술할 때만
// 연다. script 경로의 skill root와 repo 인자가 정확히 같아야 하므로 다른 repo를
// 지목하거나 설치본 script로 worktree를 겨누는 형태는 이 조건에 걸리지 않는다.
// 그 뒤의 holder·worktree fence는 그대로 적용된다.
func selfDescribingAtomicWorkflowInvocation(tokens []string, root string) bool {
	if len(tokens) != 3 || !filepath.IsAbs(filepath.Clean(tokens[1])) || !filepath.IsAbs(filepath.Clean(tokens[2])) {
		return false
	}
	// `<root>/skills/atomic-commit-push/scripts/<name>.py` 에서 root를 되짚는다.
	scriptRoot := cleanAbsPath(tokens[1])
	for range 4 {
		scriptRoot = filepath.Dir(scriptRoot)
	}
	return scriptRoot != "" && scriptRoot != string(filepath.Separator) && sameExecutionPath(scriptRoot, root)
}

// atomicCommitWorkflowCWD는 Codex exec_command가 실제로 사용하는 workdir와
// Claude Bash가 전달하는 top-level cwd를 구분한다. exec_command의 명시적
// workdir는 절대 경로일 때만 받아 host별 상대 경로 해석 차이를 열지 않는다.
func atomicCommitWorkflowCWD(req lifecyclecontract.HookToolUseLifecycleRequest) (string, bool) {
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

func atomicCommitWorkflowScriptPath(req lifecyclecontract.HookToolUseLifecycleRequest, path string) bool {
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
func atomicCommitWorkflowInstallBases(req lifecyclecontract.HookToolUseLifecycleRequest) []string {
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

func executionMutationTargets(req lifecyclecontract.HookToolUseLifecycleRequest) []string {
	targets := []string{}
	base := hookRequestPathBase(req)
	nonTargetPaths := exactIssueOpsOwnerNonTargetPaths(base, req.Command)
	for _, path := range req.Paths {
		if target := resolveHookTargetPath(base, path); target != "" {
			if nonTargetPaths[target] {
				continue
			}
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 && searchrouting.IsShellTool(req.Tool) {
		for _, path := range shellCommandWorktreeGuardPaths(base, req.Command) {
			if target := resolveHookTargetPath(base, path); target != "" {
				if nonTargetPaths[target] {
					continue
				}
				targets = append(targets, target)
			}
		}
	}
	return targets
}

func exactTemporaryAgentHarnessBuildOutput(commandText string) (string, bool) {
	tokens := commandparse.SplitCommandTokens(strings.TrimSpace(commandText))
	if len(tokens) < 4 || tokens[0] != "go" || tokens[1] != "build" {
		return "", false
	}
	output := ""
	for index := 2; index < len(tokens); index++ {
		switch {
		case tokens[index] == "--":
			index = len(tokens)
		case tokens[index] == "-o" && index+1 < len(tokens):
			if output != "" {
				return "", false
			}
			index++
			output = tokens[index]
		case strings.HasPrefix(tokens[index], "-o="):
			if output != "" {
				return "", false
			}
			output = strings.TrimPrefix(tokens[index], "-o=")
		}
	}
	if !filepath.IsAbs(output) {
		return "", false
	}
	output = cleanAbsPath(output)
	base := filepath.Base(output)
	if !strings.HasPrefix(base, "agent-harness-") || base == "agent-harness-" {
		return "", false
	}
	parent := filepath.Dir(output)
	allowedParent := false
	for _, root := range []string{os.TempDir(), "/tmp"} {
		if sameExecutionPath(parent, root) {
			allowedParent = true
			break
		}
	}
	if !allowedParent {
		return "", false
	}
	info, err := os.Lstat(output)
	switch {
	case err == nil:
		// 기존 symlink나 device를 따라 canonical 경계 밖 파일을 덮어쓰지 않는다.
		return output, info.Mode().IsRegular()
	case os.IsNotExist(err):
		return output, true
	default:
		return "", false
	}
}

func executionTemporaryBuildTargetsAuthorized(req lifecyclecontract.HookToolUseLifecycleRequest, targets []string, root, output string) bool {
	if !sameExecutionPath(req.CWD, root) {
		return false
	}
	foundOutput := false
	for _, target := range targets {
		if cleanAbsPath(target) == output {
			foundOutput = true
			continue
		}
		if !executionResolvedTargetInside(target, root) {
			return false
		}
	}
	return foundOutput
}

// exactIssueOpsOwnerNonTargetPaths는 owner 명령이 기록만 하는 경로 메타데이터를
// 변경 대상에서 제외한다. session executable은 holder identity 영수증이고,
// branch prepare의 parent worktree는 Orca lineage이므로 실제 mutation root가
// 아니다. 나머지 절대 경로는 기존 canonical root fence가 본다.
func exactIssueOpsOwnerNonTargetPaths(base, commandText string) map[string]bool {
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
	names := []string{"--session-executable", commandparsecontract.GeneratedByExecutableFlag}
	if command.Path == "branch prepare" {
		names = append(names, "--parent-worktree")
	}
	paths := map[string]bool{}
	for _, name := range names {
		value, found := oneFlag(flags, name)
		if !found {
			continue
		}
		if target := resolveHookTargetPath(base, value); target != "" {
			paths[target] = true
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

func executionRequestTargetsStayInside(req lifecyclecontract.HookToolUseLifecycleRequest, targets []string, root string) bool {
	effectiveCWD := hookRequestPathBase(req)
	if exactIssueOpsOwnerMutation(req.Command) && !sameExecutionPath(effectiveCWD, root) && !exactIssueOpsOwnerHookCWD(req, root) {
		return false
	}
	if len(targets) == 0 {
		return sameExecutionPath(effectiveCWD, root) || exactIssueOpsOwnerHookCWD(req, root)
	}
	return allExecutionTargetsInside(targets, root)
}

func exactIssueOpsOwnerHookCWD(req lifecyclecontract.HookToolUseLifecycleRequest, root string) bool {
	command, ok := commandparse.ParseExactIssueOpsCommand(req.Command)
	if !ok || len(command.Tokens) == 0 || !filepath.IsAbs(command.Tokens[0]) {
		return false
	}
	flags, ok := commandparse.ExactIssueOpsOwnerMutation(command)
	if !ok {
		return false
	}
	commandCWD, ok := oneFlag(flags, "--cwd")
	return ok && sameExecutionPath(commandCWD, root)
}

func executionUnsafeMutationReason(req lifecyclecontract.HookToolUseLifecycleRequest) string {
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

func executionGuardRecords(req lifecyclecontract.HookToolUseLifecycleRequest, targets []string) ([]issueopscontract.IssueOpsRecord, error) {
	records := []issueopscontract.IssueOpsRecord{}
	ids, err := issueOpsDeps.ListIssueOpsIDs(IssueOpsStateRoot())
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

func executionRecordTouchesRequest(record issueopscontract.IssueOpsRecord, req lifecyclecontract.HookToolUseLifecycleRequest, targets []string) bool {
	if record.Execution == nil {
		return false
	}
	return requestTouchesExecution(req, targets, *record.Execution)
}

func requestTouchesExecution(req lifecyclecontract.HookToolUseLifecycleRequest, targets []string, execution issueopscontract.Execution) bool {
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
func executionActorMismatchAxis(req lifecyclecontract.HookToolUseLifecycleRequest, holder *issueopscontract.NativeActor) string {
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

func executionActorMatches(req lifecyclecontract.HookToolUseLifecycleRequest, holder *issueopscontract.NativeActor) bool {
	if holder == nil || holder.SessionProcess == nil || !strings.EqualFold(strings.TrimSpace(req.Host), holder.Host) ||
		strings.TrimSpace(req.SessionID) == "" || strings.TrimSpace(req.SessionID) != holder.SessionID ||
		strings.TrimSpace(req.AgentID) != holder.AgentID {
		return false
	}
	for _, observed := range req.NativeProcessAncestry {
		if observed.PID == holder.SessionProcess.PID &&
			observed.StartedAt == holder.SessionProcess.StartedAt &&
			observed.Executable == holder.SessionProcess.Executable {
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

func executionMutationDenyReason(record issueopscontract.IssueOpsRecord) (string, *lifecyclecontract.IssueOpsDenyReason) {
	execution := record.Execution
	root := execution.Workspace.Root
	generation := execution.Lease.Generation
	switch execution.Lease.Status {
	case issueopscontract.LeaseStatusRevoking:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is revoking and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_revoking", next)
	case issueopscontract.LeaseStatusClaimable:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is claimable and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_claimable", next)
	case issueopscontract.LeaseStatusReleased:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("IssueOps execution %s generation %d is released and has no writer; inspect with `%s`", record.ID, generation, next), executionDeny(record, "lease_released", next)
	default:
		next := executionStatusCommand(record.ID)
		return fmt.Sprintf("mutation requires the current write lease for IssueOps execution %s generation %d and canonical root %s; inspect with `%s`", record.ID, generation, root, next), executionDeny(record, "write_lease_required", next)
	}
}

func executionDeny(record issueopscontract.IssueOpsRecord, code, nextCommand string) *lifecyclecontract.IssueOpsDenyReason {
	return &lifecyclecontract.IssueOpsDenyReason{
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
	case "prompt":
		return !hasJudgeFile
	case "file":
		return hasJudgeFile && strings.TrimSpace(judgeFile) != ""
	default:
		return false
	}
}

// exactOrcaBranchLinkWait은 pre-link 창에서 owner가 쓸 수 있는 유일한 대기
// reader를 인정한다(#319).
//
// 이 창이 존재하는 이유는 순서가 뒤집을 수 없기 때문이다. Orca는 항상 새
// branch를 만들므로 원격 branch가 먼저 있으면 prepare 자체가 실패한다. 따라서
// coordinator의 createLinkedBranch는 owner 기동 **뒤**에 와야 하고, owner는
// 그 사이를 기다릴 수단이 있어야 한다. 기다릴 수단이 없으면 owner는 시작
// 시점의 부재를 terminal 실패로 다루고, 이 경로는 완주할 수 없다.
//
// 인정하는 것은 이 lifecycle을 지목한 정확한 한 형태뿐이다. 읽기 전용이므로
// 허용해도 mutation 경계는 그대로다.
func exactOrcaBranchLinkWait(commandText string, record issueopscontract.IssueOpsRecord) bool {
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "branch await-link" {
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
	id, found := oneFlag(flags, "--id")
	return found && strings.TrimSpace(id) == record.ID
}

// orcaBranchLinkDenyReason은 pre-link 창의 차단 사유다. 두 차단 지점이 같은
// 문장을 쓰도록 한 곳에서 만든다.
//
// 차단 사유만 말하고 기다릴 방법을 말하지 않으면 owner는 그것을 terminal
// 실패로 다룬다 — 실측된 실패가 정확히 그랬다(#319, task_e3946ef93086).
func orcaBranchLinkDenyReason(record issueopscontract.IssueOpsRecord) string {
	return "active Orca owner must await and record the exact verified branch link before any other mutation; " +
		"`agent-harness issueops branch await-link --id " + record.ID + "` waits for the coordinator to create it, " +
		"and `agent-harness issueops execution release --id " + record.ID + " --generation " +
		strconv.FormatUint(leaseGenerationOf(record), 10) + "` hands the lease back if you must stop"
}

// exactOrcaLeaseRelease는 pre-link 창에서 현재 holder의 lease 반납을 인정한다.
//
// 이 창에서 owner는 링크가 기록되기 전 어떤 mutation도 할 수 없다. 그런데
// 반납까지 막으면 진행도 반납도 불가능한 덫이 된다 — 실제로 그랬다. blocker를
// 보고한 owner가 lease를 든 채 종료했고, 프로세스가 살아 있는 한 typed
// 회수(`replace --finalize-preview`)는 정당하게 거부하며, 프로세스 종료는
// 하네스의 비목표라 자동화하지 않는다. 남은 회수 수단이 사람뿐이었다(#319).
//
// 반납은 위험이 아니라 안전한 출구다. 쓰기 권한을 내려놓을 뿐이므로 이 창에서
// 허용해도 mutation 경계는 넓어지지 않는다. 인정하는 것은 이 lifecycle과 현재
// generation을 지목한 정확한 한 형태뿐이다.
func exactOrcaLeaseRelease(commandText string, record issueopscontract.IssueOpsRecord) bool {
	if record.Execution == nil {
		return false
	}
	command, ok := commandparse.ParseExactIssueOpsCommand(commandText)
	if !ok || command.Path != "execution release" {
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
	id, idOK := oneFlag(flags, "--id")
	if !idOK || strings.TrimSpace(id) != record.ID {
		return false
	}
	generation, generationOK := oneFlag(flags, "--generation")
	if !generationOK {
		return false
	}
	parsed, err := strconv.ParseUint(strings.TrimSpace(generation), 10, 64)
	return err == nil && parsed == record.Execution.Lease.Generation
}

// leaseGenerationOf는 진단 문구가 참조할 현재 generation이다.
func leaseGenerationOf(record issueopscontract.IssueOpsRecord) uint64 {
	if record.Execution == nil {
		return 0
	}
	return record.Execution.Lease.Generation
}

// typedLeaseReleaseAction은 MCP 표면의 반납 요청을 인정한다. shell 쪽
// exactOrcaLeaseRelease와 같은 근거이며, 대상 lifecycle을 지목한 release
// 하나만 통과시킨다.
func typedLeaseReleaseAction(req lifecyclecontract.HookToolUseLifecycleRequest, record issueopscontract.IssueOpsRecord) bool {
	if searchrouting.IsShellTool(req.Tool) {
		return false
	}
	action, actionOK := req.ToolInput["action"].(string)
	id, idOK := req.ToolInput["id"].(string)
	return actionOK && idOK && strings.TrimSpace(action) == issueopscontract.ExecutionActionRelease &&
		strings.TrimSpace(id) == record.ID
}
