package cli

import "strings"

// issueOpsUsageCatalog는 `issueops` 명령 usage 줄의 **유일한 원본**이다.
//
// 전에는 같은 줄이 두 곳에 있었다 — 여기의 최상위 축약 카탈로그와
// `cmd/issueops/issueopscli`의 전체 목록. 한쪽 누락은 parity 테스트가 잡았지만
// **양쪽에 아예 없으면** 검사할 대상이 없었고, `execution switch-mode`(#167)가 그
// 구멍으로 살아남았다(#188).
//
// 두 표면은 이제 이 카탈로그의 서로 다른 투영이다:
//
//   - `Usage()` — 최상위 lifecycle 명령 전체를 렌더하는 카탈로그
//   - `issueopscli.issueOpsUsageText()` — 전체 렌더
//
// 줄 순서가 곧 렌더 순서다. 새 명령은 여기 한 곳에만 추가하고, 최상위에도 노출할
// 것이면 축약 키에 그 명령 경로를 더한다.
const issueOpsUsageCatalog = `  issueops start --repo PATH [--branch NAME] [--json]
  issueops status --id ID [--json]
  issueops list [--repo PATH] [--json]
  issueops next [--id ID] [--cwd PATH] [--json]
  issueops intent record --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--intent-class CLASS] RECORD_ACTOR_FLAGS [--json]
  issueops plan-prep record --id ID [--decisions-evidence TEXT | --decisions-waive REASON] [--related-score-ref TEXT | --related-waive REASON] [--web-research-evidence TEXT | --web-research-waive REASON] [--codebase-survey-evidence TEXT | --codebase-survey-waive REASON] RECORD_ACTOR_FLAGS [--json]
  issueops domain-review record --id ID --model-fit TEXT [--terminology TEXT] [--risk TEXT] [--uncertainty TEXT] RECORD_ACTOR_FLAGS [--json]
  issueops decision add --id ID --title TEXT --body TEXT --kind product|architecture|implementation|test|review|scope|follow-up [--rationale TEXT] [--alternative TEXT] [--affected-link URL] [--affected-artifact issue|plan|test|implementation|review|pr_mr|follow-up] RECORD_ACTOR_FLAGS [--json]
  issueops link-issue --id ID --issue-url URL RECORD_ACTOR_FLAGS [--json]
  issueops link-child --id ID --child-url URL [--title TEXT] RECORD_ACTOR_FLAGS [--json]
  issueops link-related --id ID --type depends-on|blocks|supersedes|follows-up|duplicates|splits-from|implements --related-url URL [--title TEXT] RECORD_ACTOR_FLAGS [--json]
  issueops child start --parent ID --branch BRANCH --title TEXT --scope TEXT --acceptance TEXT [--acceptance TEXT...] [--child-issue-url URL] RECORD_ACTOR_FLAGS [--json]
  issueops child status --parent ID [--repair] RECORD_ACTOR_FLAGS [--json]
  issueops child list --parent ID RECORD_ACTOR_FLAGS [--json]
  issueops child accept --parent ID --child ID --evidence TEXT [--evidence TEXT...] RECORD_ACTOR_FLAGS [--json]
  issueops child reject --parent ID --child ID --reason REASON RECORD_ACTOR_FLAGS [--json]
  issueops child drop --parent ID --child ID --reason REASON RECORD_ACTOR_FLAGS [--json]
  issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--parent-worktree PATH] [--remote-branch-url URL] [--code-project-key HOST/PROJECT] [--link-verified] RECORD_ACTOR_FLAGS [--json]
  issueops branch await-link --id ID [--timeout DURATION] [--json]
  issueops branch retarget --id ID --base-branch REF --reason TEXT RECORD_ACTOR_FLAGS [--json]
  issueops link-worktree --id ID --worktree-path PATH RECORD_ACTOR_FLAGS [--json]
  issueops design review --id ID --problem-summary TEXT --proposed-design TEXT --verification TEXT [--refactor-plan TEXT] [--alternative TEXT] [--risk TEXT] [--open-question TEXT] [--approved] RECORD_ACTOR_FLAGS [--json]
  issueops compatibility review --id ID --backward-compatibility TEXT --side-effect TEXT --rollback-plan TEXT --verification TEXT [--blocker TEXT] [--approved] RECORD_ACTOR_FLAGS [--json]
  issueops devils-advocate review --id ID --verdict pass|revise|stop --reviewer-context subagent|inline [--finding TEXT]... [--waive --waiver-rationale TEXT] RECORD_ACTOR_FLAGS [--json]
  issueops link-plan --id ID --plan-path PATH RECORD_ACTOR_FLAGS [--json]
  issueops artifact stage --id ID --name plan|spec|verified-execution-loop --file PATH [--json]
  issueops artifact unstage --id ID --name plan|spec|verified-execution-loop [--json]
  issueops execution prepare --id ID --mode auto|direct|orca --owner-host codex|claude|omo [--owner-model MODEL] [--owner-effort EFFORT] [--direct-reason REASON] [--expected-readiness-fingerprint SHA256] [--issue-snapshot-file PATH] ACTOR_FLAGS [--confirm] [--json]
  issueops execution status --id ID [--json]
  issueops execution whoami [--json]
  issueops execution claim --id ID --generation N (--claim-current-token|--claim-token-file PATH) [--issue-body-sha256 SHA256 --context-packet-sha256 SHA256] [--issue-snapshot-file PATH] [ACTOR_FLAGS] [--json]
  issueops execution release --id ID --generation N ACTOR_FLAGS [--json]
  issueops execution replace --id ID --expected-generation N (--preview|--revoke|--finalize-preview|--finalize|--reseed) [--completion-generation N] [fingerprint/reason flags] [--issue-snapshot-file PATH] ACTOR_FLAGS [--confirm] [--json]
  issueops execution resume --id ID --expected-generation N [ACTOR_FLAGS] --confirm [--json]
  issueops execution reconcile --id ID (--preview|--confirm) [--issue-snapshot-file PATH] ACTOR_FLAGS [--json]
  issueops execution complete --id ID --generation N --final-head SHA --verification-report PATH --remote-artifact-url URL --verification TEXT... ACTOR_FLAGS --confirm [--json]
  issueops execution sync-base --id ID [--completion-generation N] (--preview | --apply --confirm --fingerprint SHA256 | --finalize | --abort) ACTOR_FLAGS [--json]
  issueops execution switch-mode --id ID --mode direct|orca [--apply --confirm --fingerprint SHA256] ACTOR_FLAGS [--json]
  issueops phase --id ID --to problem|grill|plan|compatibility-review|implement|ai-slop-clean|feedback|pr RECORD_ACTOR_FLAGS [--json]
  issueops ai-slop-clean record --id ID --category TEXT --verification TEXT RECORD_ACTOR_FLAGS [--json]
  issueops implementation-review record --id ID --verdict pass|revise|stop --finding TEXT... --evidence TEXT... [--reviewer-host codex|claude|omo] [--reviewer-model MODEL] [--reviewer-effort EFFORT] RECORD_ACTOR_FLAGS [--json]
  issueops project-docs-review record --id ID --verdict updated|no-change [--doc PATH...] --evidence TEXT... RECORD_ACTOR_FLAGS [--json]
  issueops schema-evidence record --id ID --measurement TEXT... --source TEXT... [--waive --waiver-rationale TEXT] RECORD_ACTOR_FLAGS [--json]
  issueops regress --id ID --reason TEXT RECORD_ACTOR_FLAGS [--json]
  issueops record-routing --id ID --phase PHASE --skill SKILL RECORD_ACTOR_FLAGS [--json]
  issueops routing-score --id ID --expect phase:skill,... [--json]
  issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] RECORD_ACTOR_FLAGS [--json]
  issueops feedback mark-issue-updated --id ID RECORD_ACTOR_FLAGS [--json]
  issueops feedback resolve --id ID --index N --resolution valid-defect|question-answered|noise-dismissed RECORD_ACTOR_FLAGS [--json]
  issueops prune [--max-age DURATION] [--confirm] [--json]
  issueops pr-readiness --id ID [--strict] [--json]
  issueops cleanup status --id ID [--merged] [--json]
  issueops cleanup close-children --id ID --merged [--confirm] [--json]
  issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME --provider github|gitlab --kind pr|mr --artifact-url URL [--apply --confirm --fingerprint SHA256] [--json]
  issueops cleanup remote-branch --id ID (--preview | --apply --confirm --fingerprint SHA256) [--superseded-by URL] [--json]
  issueops cleanup linked-branch --id ID (--preview | --apply --confirm --fingerprint SHA256) [--json]
  issueops cleanup finish --id ID [--provider github|gitlab] (--preview | --apply --confirm --fingerprint SHA256) [--superseded-by URL] [--keep-remote-branch] [--json]
  issueops cleanup abandon --id ID --reason TEXT [--close-pr] [--close-issue] [--delete-remote-branch] (--preview | --apply --confirm --fingerprint SHA256) [--json]
  issueops remote score --input PATH [--judge none|prompt|file] [--judge-file PATH] [--json]
  issueops remote-score --input PATH [--judge none|prompt|file] [--judge-file PATH] [--json]
  issueops remote render-template --kind issue|child|pr --template KIND --title TEXT --provider github|gitlab --field key=value... [--score-file PATH] [--json]
  issueops remote create-issue --id ID --title TEXT [--provider github|gitlab] [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  issueops remote reconcile-issue --id ID [--confirm] [--json]
  issueops remote sync-graph --id ID [--confirm] [--json]
  issueops remote sync-issue --id ID [--provider github|gitlab] [--url CHILD_URL] [--body TEXT|--body-file PATH] [--expected-body-sha256 SHA] [--accept-remote-edits] RECORD_ACTOR_FLAGS [--confirm] [--json]
  issueops remote sync-pr --id ID --expected-generation N [--provider github|gitlab] [--body TEXT|--body-file PATH] [--expected-body-sha256 SHA] [--accept-remote-edits] RECORD_ACTOR_FLAGS [--confirm] [--json]
  issueops remote create-child --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... --host codex|claude|omo --session-id SESSION [--agent-id ID] --cwd WORKER_PATH [--confirm] [--json]
  issueops remote create-pr --id ID --expected-generation N --title TEXT --head BRANCH --base BRANCH [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... ACTOR_FLAGS [--confirm] [--json]
  issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --target-branch BRANCH --label LABEL --assignee USER RECORD_ACTOR_FLAGS [--json]
  issueops remote reflect-devils-advocate --id ID [--provider github|gitlab] RECORD_ACTOR_FLAGS [--confirm] [--json]
  issueops remote reflect-completion --id ID [--provider github|gitlab] [--confirm] [--json]
  issueops remote close-issue --id ID [--provider github|gitlab] [--confirm] [--json]
  issueops benchmark run --fixtures PATH [--judge none|file] [--judge-file PATH] [--json]
  issueops benchmark compare --baseline KEY --candidate KEY [--json]
  issueops benchmark gate --baseline KEY --candidate KEY --candidate-file PATH [--changed-path PATH]... [--json]`

// IssueOpsActorFlagLegend는 두 usage 출력이 공유하는 actor 축약 정의다. 축약을 쓰는
// 출력은 같은 출력 안에서 그것을 정의해야 한다(#184).
const IssueOpsActorFlagLegend = `RECORD_ACTOR_FLAGS: --host codex|claude|omo --session-id ID [--agent-id ID] --cwd PATH
ACTOR_FLAGS: --host codex|claude|omo --session-id ID [--agent-id ID] --session-pid PID --session-started-at RFC3339 --session-executable PATH --cwd PATH

Durable-record mutations take RECORD_ACTOR_FLAGS; without them an active execution rejects the
call as a non-holder. execution claim accepts either no actor flags and observes the current
native session receipt, or one complete ACTOR_FLAGS set; partial actor flags fail closed. Other execution
lease transitions and generation-fenced publication verify the live session process with ACTOR_FLAGS.`

// IssueOpsUsageLines는 카탈로그를 줄 단위로 돌려준다. 각 줄은 선행 두 칸을 포함한다.
func IssueOpsUsageLines() []string {
	return strings.Split(issueOpsUsageCatalog, "\n")
}

// IssueOpsUsageKey는 usage 줄에서 `issueops ` 뒤의 명령 경로를 뽑는다.
// 선택적 플래그는 `[--repo PATH]`, 배타 그룹은 `(--preview|...)`로 표기되므로 그
// 문자로 시작하는 필드도 경로의 끝이다 — 끊지 않으면 `list [--repo`가 경로가 된다.
func IssueOpsUsageKey(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "issueops" {
		return ""
	}
	if !IsLifecycleCommand(fields[1]) {
		return ""
	}
	fields = fields[1:]
	limit := len(fields)
	if limit > 2 {
		limit = 2
	}
	for index, field := range fields[:limit] {
		if strings.HasPrefix(field, "-") || strings.HasPrefix(field, "[") || strings.HasPrefix(field, "(") {
			limit = index
			break
		}
	}
	return strings.Join(fields[:limit], " ")
}

// LifecycleCommands lists the root commands owned by the guarded lifecycle dispatcher.
func LifecycleCommands() []Command {
	return []Command{
		{Name: "ai-slop-clean", Description: "IssueOps ai-slop-clean"},
		{Name: "artifact", Description: "IssueOps artifact"},
		{Name: "benchmark", Description: "IssueOps benchmark"},
		{Name: "branch", Description: "IssueOps branch"},
		{Name: "child", Description: "IssueOps child"},
		{Name: "cleanup", Description: "IssueOps cleanup"},
		{Name: "compatibility", Description: "IssueOps compatibility"},
		{Name: "decision", Description: "IssueOps decision"},
		{Name: "design", Description: "IssueOps design"},
		{Name: "devils-advocate", Description: "IssueOps devils-advocate"},
		{Name: "domain-review", Description: "IssueOps domain-review"},
		{Name: "execution", Description: "IssueOps execution"},
		{Name: "feedback", Description: "IssueOps feedback"},
		{Name: "implementation-review", Description: "IssueOps implementation-review"},
		{Name: "intent", Description: "IssueOps intent"},
		{Name: "link-child", Description: "IssueOps link-child"},
		{Name: "link-issue", Description: "IssueOps link-issue"},
		{Name: "link-plan", Description: "IssueOps link-plan"},
		{Name: "link-related", Description: "IssueOps link-related"},
		{Name: "link-worktree", Description: "IssueOps link-worktree"},
		{Name: "list", Description: "IssueOps list"},
		{Name: "next", Description: "IssueOps next"},
		{Name: "phase", Description: "IssueOps phase"},
		{Name: "plan-prep", Description: "IssueOps plan-prep"},
		{Name: "pr-readiness", Description: "IssueOps pr-readiness"},
		{Name: "project-docs-review", Description: "IssueOps project-docs-review"},
		{Name: "prune", Description: "IssueOps prune"},
		{Name: "record-routing", Description: "IssueOps record-routing"},
		{Name: "regress", Description: "IssueOps regress"},
		{Name: "remote", Description: "IssueOps remote"},
		{Name: "remote-score", Description: "IssueOps remote-score"},
		{Name: "routing-score", Description: "IssueOps routing-score"},
		{Name: "schema-evidence", Description: "IssueOps schema-evidence"},
		{Name: "start", Description: "IssueOps start"},
		{Name: "status", Description: "IssueOps status"},
	}
}

func IsLifecycleCommand(name string) bool {
	for _, command := range LifecycleCommands() {
		if command.Name == name {
			return true
		}
	}
	return false
}
