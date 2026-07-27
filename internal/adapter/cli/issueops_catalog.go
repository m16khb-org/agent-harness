package cli

import "strings"

// issueOpsUsageCatalog는 `issueops` 명령 usage 줄의 **유일한 원본**이다.
//
// 전에는 같은 줄이 두 곳에 있었다 — 여기의 최상위 축약 카탈로그와
// `cmd/harness/issueopscli`의 전체 목록. 한쪽 누락은 parity 테스트가 잡았지만
// **양쪽에 아예 없으면** 검사할 대상이 없었고, `execution switch-mode`(#167)가 그
// 구멍으로 살아남았다(#188).
//
// 두 표면은 이제 이 카탈로그의 서로 다른 투영이다:
//
//   - `Usage()` — `abridgedIssueOpsUsageKeys()`로 걸러 렌더하는 최상위 축약 카탈로그
//   - `issueopscli.issueOpsUsageText()` — 전체 렌더
//
// 줄 순서가 곧 렌더 순서다. 새 명령은 여기 한 곳에만 추가하고, 최상위에도 노출할
// 것이면 축약 키에 그 명령 경로를 더한다.
const issueOpsUsageCatalog = `  agent-harness issueops start --repo PATH [--branch NAME] [--json]
  agent-harness issueops status --id ID [--json]
  agent-harness issueops list [--repo PATH] [--json]
  agent-harness issueops intent record --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--intent-class CLASS] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops plan-prep record --id ID [--decisions-evidence TEXT | --decisions-waive REASON] [--related-score-ref TEXT | --related-waive REASON] [--web-research-evidence TEXT | --web-research-waive REASON] [--codebase-survey-evidence TEXT | --codebase-survey-waive REASON] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops domain-review record --id ID --model-fit TEXT [--terminology TEXT] [--risk TEXT] [--uncertainty TEXT] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops decision add --id ID --title TEXT --body TEXT --kind product|architecture|implementation|test|review|scope|follow-up [--rationale TEXT] [--alternative TEXT] [--affected-link URL] [--affected-artifact issue|plan|test|implementation|review|pr_mr|follow-up] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops link-issue --id ID --issue-url URL RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops link-child --id ID --child-url URL [--title TEXT] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops link-related --id ID --type depends-on|blocks|supersedes|follows-up|duplicates|splits-from|implements --related-url URL [--title TEXT] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops child start --parent ID --branch BRANCH --title TEXT --scope TEXT --acceptance TEXT [--acceptance TEXT...] [--child-issue-url URL] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops child status --parent ID [--repair] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops child list --parent ID RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops child accept --parent ID --child ID --evidence TEXT [--evidence TEXT...] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops child reject --parent ID --child ID --reason REASON RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops child drop --parent ID --child ID --reason REASON RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--remote-branch-url URL] [--link-verified] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops link-worktree --id ID --worktree-path PATH RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops design review --id ID --problem-summary TEXT --proposed-design TEXT --verification TEXT [--refactor-plan TEXT] [--alternative TEXT] [--risk TEXT] [--open-question TEXT] [--approved] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops compatibility review --id ID --backward-compatibility TEXT --side-effect TEXT --rollback-plan TEXT --verification TEXT [--blocker TEXT] [--approved] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops devils-advocate review --id ID --verdict pass|revise|stop [--finding TEXT]... [--waive --waiver-rationale TEXT] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops link-plan --id ID --plan-path PATH RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops artifact stage --id ID --name plan|spec|turing-loop --file PATH [--json]
  agent-harness issueops artifact unstage --id ID --name plan|spec|turing-loop [--json]
  agent-harness issueops execution prepare --id ID --mode auto|direct|orca --owner-host codex|claude [--owner-model MODEL] [--owner-effort EFFORT] ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops execution status --id ID [--json]
  agent-harness issueops execution whoami [--json]
  agent-harness issueops execution claim --id ID --generation N --claim-token-file PATH [--issue-body-sha256 SHA256 --context-packet-sha256 SHA256] ACTOR_FLAGS [--json]
  agent-harness issueops execution release --id ID --generation N ACTOR_FLAGS [--json]
  agent-harness issueops execution replace --id ID --expected-generation N (--preview|--revoke|--finalize-preview|--finalize|--reseed) [fingerprint/reason flags] ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops execution reconcile --id ID (--preview|--confirm) ACTOR_FLAGS [--json]
  agent-harness issueops execution complete --id ID --generation N --final-head SHA --turing-report PATH --remote-artifact-url URL --verification TEXT... ACTOR_FLAGS --confirm [--json]
  agent-harness issueops execution sync-base --id ID (--preview | --apply --confirm --fingerprint SHA256 | --finalize | --abort) ACTOR_FLAGS [--json]
  agent-harness issueops execution switch-mode --id ID --mode direct|orca [--apply --confirm --fingerprint SHA256] ACTOR_FLAGS [--json]
  agent-harness issueops reset-legacy --target-schema 1 (--preview|--status|--reconcile-remote --id ID --claim-id CLAIM --confirm|--drain-cycle --id ID --confirm|--confirm) [--expected-fingerprint SHA256] [--json]
  agent-harness issueops phase --id ID --to problem|grill|plan|compatibility-review|implement|ai-slop-clean|feedback|pr RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops ai-slop-clean record --id ID --category TEXT --verification TEXT RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops implementation-review record --id ID --verdict pass|revise|stop --finding TEXT... --evidence TEXT... [--reviewer-host codex|claude] [--reviewer-model MODEL] [--reviewer-effort EFFORT] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops regress --id ID --reason TEXT RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops record-routing --id ID --phase PHASE --skill SKILL RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops routing-score --id ID --expect phase:skill,... [--json]
  agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops feedback mark-issue-updated --id ID RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops feedback resolve --id ID --index N --resolution valid-defect|question-answered|noise-dismissed RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops prune [--max-age DURATION] [--confirm] [--json]
  agent-harness issueops pr-readiness --id ID [--strict] [--json]
  agent-harness issueops cleanup status --id ID [--merged] [--json]
  agent-harness issueops cleanup close-children --id ID --merged [--confirm] [--json]
  agent-harness issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME --provider github|gitlab --kind pr|mr --artifact-url URL [--apply --confirm --fingerprint SHA256] [--json]
  agent-harness issueops cleanup remote-branch --id ID (--preview | --apply --confirm --fingerprint SHA256) [--json]
  agent-harness issueops cleanup finish --id ID [--provider github|gitlab] (--preview | --apply --confirm --fingerprint SHA256) [--json]
  agent-harness issueops cleanup abandon --id ID --reason TEXT (--preview | --apply --confirm --fingerprint SHA256) [--json]
  agent-harness issueops remote score --input PATH [--judge none|prompt|file] [--judge-file PATH] [--json]
  agent-harness issueops remote-score --input PATH [--judge none|prompt|file] [--judge-file PATH] [--json]
  agent-harness issueops remote render-template --kind issue|child|pr --template KIND --title TEXT --provider github|gitlab --field key=value... [--score-file PATH] [--json]
  agent-harness issueops remote create-issue --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  agent-harness issueops remote create-child --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  agent-harness issueops remote create-pr --id ID --expected-generation N --title TEXT --head BRANCH --base BRANCH [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --target-branch BRANCH --label LABEL --assignee USER RECORD_ACTOR_FLAGS [--json]
  agent-harness issueops remote reflect-completion --id ID [--provider github|gitlab] [--confirm] [--json]
  agent-harness issueops remote close-issue --id ID [--provider github|gitlab] [--confirm] [--json]
  agent-harness issueops benchmark run --fixtures PATH [--judge none|file] [--judge-file PATH] [--json]
  agent-harness issueops benchmark compare --baseline KEY --candidate KEY [--json]
  agent-harness issueops benchmark gate --baseline KEY --candidate KEY --candidate-file PATH [--changed-path PATH]... [--json]`

// IssueOpsActorFlagLegend는 두 usage 출력이 공유하는 actor 축약 정의다. 축약을 쓰는
// 출력은 같은 출력 안에서 그것을 정의해야 한다(#184).
const IssueOpsActorFlagLegend = `RECORD_ACTOR_FLAGS: --host codex|claude --session-id ID [--agent-id ID] --cwd PATH
ACTOR_FLAGS: --host codex|claude --session-id ID [--agent-id ID] --session-pid PID --session-started-at RFC3339 --session-executable PATH --cwd PATH

Durable-record mutations take RECORD_ACTOR_FLAGS; without them an active execution rejects the
call as a non-holder. Execution lease transitions and generation-fenced publication additionally
verify the live session process, so they take the wider ACTOR_FLAGS.`

// abridgedIssueOpsMainKeys는 최상위 usage 본문에 노출할 명령 경로다. 렌더 순서는
// 카탈로그 순서를 따르므로 이 목록의 순서는 의미가 없다.
//
// 여기 없는 명령은 `issueops --help`에서만 보인다. 어느 명령을 최상위에 노출할지는
// 별개 판단이며 `#188`은 현재 선정을 그대로 옮겼다.
var abridgedIssueOpsMainKeys = []string{
	"start", "status", "list",
	"intent record", "link-issue", "link-child",
	"child start", "child status", "child list", "child accept", "child reject", "child drop",
	"branch prepare", "link-worktree",
	"design review", "compatibility review", "link-plan",
	"artifact stage", "artifact unstage",
	"execution prepare", "execution status", "execution whoami", "execution claim",
	"execution release", "execution replace", "execution reconcile", "execution complete",
	"execution sync-base", "execution switch-mode",
	"reset-legacy",
	"feedback add", "feedback mark-issue-updated",
	"pr-readiness",
	"cleanup status", "cleanup close-children", "cleanup orphan",
	"cleanup remote-branch", "cleanup finish", "cleanup abandon",
	"remote score", "remote render-template", "remote create-issue", "remote create-child",
	"remote create-pr",
	"benchmark run", "benchmark compare", "benchmark gate",
}

// abridgedIssueOpsTrailingKeys는 최상위 usage의 비-issueops 목록 **뒤에** 렌더되는
// 명령이다. 그 위치는 카탈로그 도입 전부터의 출력 형태이고 golden이 고정한다.
var abridgedIssueOpsTrailingKeys = []string{
	"devils-advocate review",
	"implementation-review record",
}

// IssueOpsUsageLines는 카탈로그를 줄 단위로 돌려준다. 각 줄은 선행 두 칸을 포함한다.
func IssueOpsUsageLines() []string {
	return strings.Split(issueOpsUsageCatalog, "\n")
}

// IssueOpsUsageKey는 usage 줄에서 `agent-harness issueops ` 뒤의 명령 경로를 뽑는다.
// 선택적 플래그는 `[--repo PATH]`, 배타 그룹은 `(--preview|...)`로 표기되므로 그
// 문자로 시작하는 필드도 경로의 끝이다 — 끊지 않으면 `list [--repo`가 경로가 된다.
func IssueOpsUsageKey(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 3 || fields[0] != "agent-harness" || fields[1] != "issueops" {
		return ""
	}
	fields = fields[2:]
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

func abridgedIssueOpsUsageKeys() []string {
	keys := make([]string, 0, len(abridgedIssueOpsMainKeys)+len(abridgedIssueOpsTrailingKeys))
	keys = append(keys, abridgedIssueOpsMainKeys...)
	keys = append(keys, abridgedIssueOpsTrailingKeys...)
	return keys
}

// renderIssueOpsUsage는 주어진 키에 해당하는 카탈로그 줄을 **카탈로그 순서로**
// 이어 붙인다. 키가 어느 줄과도 맞지 않으면 그 명령은 조용히 빠지므로
// `TestAbridgedIssueOpsKeysMatchExactlyOneCatalogLine`이 그것을 막는다.
func renderIssueOpsUsage(keys []string) string {
	wanted := make(map[string]bool, len(keys))
	for _, key := range keys {
		wanted[key] = true
	}
	var selected []string
	for _, line := range IssueOpsUsageLines() {
		if wanted[IssueOpsUsageKey(line)] {
			selected = append(selected, line)
		}
	}
	return strings.Join(selected, "\n")
}
