package issueopscli

import (
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

func issueOpsUsage() {
	fmt.Fprint(os.Stderr, issueOpsUsageText())
}

// issueOpsUsageText는 issueops 서브커맨드 usage 원문을 반환한다. 최상위
// adapter usage(internal/adapter/cli)와의 라인 단위 일치는 테스트로 고정한다.
func issueOpsUsageText() string {
	return `Usage:
  agent-harness issueops start --repo PATH [--branch NAME] [--json]
  agent-harness issueops status --id ID [--json]
  agent-harness issueops list [--repo PATH] [--json]
  agent-harness issueops intent record --id ID --raw-request TEXT --interpreted-intent TEXT --success-criteria TEXT [--constraint TEXT] [--ambiguity TEXT] [--non-goal TEXT] [--intent-class CLASS] [--json]
  agent-harness issueops plan-prep record --id ID [--decisions-evidence TEXT | --decisions-waive REASON] [--related-score-ref TEXT | --related-waive REASON] [--web-research-evidence TEXT | --web-research-waive REASON] [--codebase-survey-evidence TEXT | --codebase-survey-waive REASON] [--json]
  agent-harness issueops domain-review record --id ID --model-fit TEXT [--terminology TEXT] [--risk TEXT] [--uncertainty TEXT] [--json]
  agent-harness issueops decision add --id ID --title TEXT --body TEXT --kind product|architecture|implementation|test|review|scope|follow-up [--rationale TEXT] [--alternative TEXT] [--affected-link URL] [--affected-artifact issue|plan|test|implementation|review|pr_mr|follow-up] [--json]
  agent-harness issueops link-issue --id ID --issue-url URL [--json]
  agent-harness issueops link-child --id ID --child-url URL [--title TEXT] [--json]
  agent-harness issueops link-related --id ID --type depends-on|blocks|supersedes|follows-up|duplicates|splits-from|implements --related-url URL [--title TEXT] [--json]
  agent-harness issueops child start --parent ID --branch BRANCH --title TEXT --scope TEXT --acceptance TEXT [--acceptance TEXT...] [--child-issue-url URL] [--json]
  agent-harness issueops child status --parent ID [--repair] [--json]
  agent-harness issueops child list --parent ID [--json]
  agent-harness issueops child accept --parent ID --child ID --evidence TEXT [--evidence TEXT...] [--json]
  agent-harness issueops child reject --parent ID --child ID --reason REASON [--json]
  agent-harness issueops child drop --parent ID --child ID --reason REASON [--json]
  agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--remote-branch-url URL] [--link-verified] [--json]
  agent-harness issueops link-worktree --id ID --worktree-path PATH [--json]
  agent-harness issueops design review --id ID --problem-summary TEXT --proposed-design TEXT --verification TEXT [--refactor-plan TEXT] [--alternative TEXT] [--risk TEXT] [--open-question TEXT] [--approved] [--json]
  agent-harness issueops compatibility review --id ID --backward-compatibility TEXT --side-effect TEXT --rollback-plan TEXT --verification TEXT [--blocker TEXT] [--approved] [--json]
  agent-harness issueops devils-advocate review --id ID --verdict pass|revise|stop [--finding TEXT]... [--waive --waiver-rationale TEXT] [--json]
  agent-harness issueops link-plan --id ID --plan-path PATH [--json]
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
  agent-harness issueops reset-legacy --target-schema 1 (--preview|--status|--reconcile-remote --id ID --claim-id CLAIM --confirm|--drain-cycle --id ID --confirm|--confirm) [--expected-fingerprint SHA256] [--json]
  agent-harness issueops phase --id ID --to problem|grill|plan|compatibility-review|implement|ai-slop-clean|feedback|pr [--force] [--json]
  agent-harness issueops ai-slop-clean record --id ID --category TEXT --verification TEXT [--json]
  agent-harness issueops implementation-review record --id ID --verdict pass|revise|stop --finding TEXT... --evidence TEXT... [--reviewer-host codex|claude] [--reviewer-model MODEL] [--reviewer-effort EFFORT] [--json]
  agent-harness issueops regress --id ID --reason TEXT [--json]
  agent-harness issueops record-routing --id ID --phase PHASE --skill SKILL [--json]
  agent-harness issueops routing-score --id ID --expect phase:skill,... [--json]
  agent-harness issueops feedback add --id ID --source TEXT --body TEXT [--classification TEXT] [--json]
  agent-harness issueops feedback mark-issue-updated --id ID [--json]
  agent-harness issueops feedback resolve --id ID --index N --resolution valid-defect|question-answered|noise-dismissed [--json]
  agent-harness issueops prune [--max-age DURATION] [--confirm] [--json]
  agent-harness issueops pr-readiness --id ID [--strict] [--json]
  agent-harness issueops cleanup status --id ID [--merged] [--json]
  agent-harness issueops cleanup close-children --id ID --merged [--confirm] [--json]
  agent-harness issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME --provider github|gitlab --kind pr|mr --artifact-url URL [--apply --confirm --fingerprint SHA256] [--json]
  agent-harness issueops cleanup finish --id ID [--provider github|gitlab] (--preview | --apply --confirm --fingerprint SHA256) [--json]
  agent-harness issueops remote score --input PATH [--judge none|file] [--judge-file PATH] [--json]
  agent-harness issueops remote-score --input PATH [--judge none|file] [--judge-file PATH] [--json]
  agent-harness issueops remote render-template --kind issue|child|pr --template KIND --title TEXT --provider github|gitlab --field key=value... [--score-file PATH] [--json]
  agent-harness issueops remote create-issue --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  agent-harness issueops remote create-child --id ID --title TEXT [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... [--confirm] [--json]
  agent-harness issueops remote create-pr --id ID --expected-generation N --title TEXT --head BRANCH --base BRANCH [--body TEXT|--body-file PATH] [--template KIND --field key=value...] [--label LABEL]... [--assignee USER]... ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops remote verify-artifact --id ID --provider github|gitlab --kind pr|mr --url URL --target-branch BRANCH --label LABEL --assignee USER ACTOR_FLAGS [--json]
  agent-harness issueops remote reflect-completion --id ID [--provider github|gitlab] [--confirm] [--json]
  agent-harness issueops remote close-issue --id ID [--provider github|gitlab] [--confirm] [--json]
  agent-harness issueops benchmark run --fixtures PATH [--judge none|file] [--judge-file PATH] [--json]
  agent-harness issueops benchmark compare --baseline KEY --candidate KEY [--json]
  agent-harness issueops benchmark gate --baseline KEY --candidate KEY --candidate-file PATH [--changed-path PATH]... [--json]
`
}

const issueOpsBranchPrepareUsage = "Usage: agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--remote-branch-url URL] [--link-verified] [--json]"

const issueOpsChildUsage = `Usage:
  agent-harness issueops child start --parent ID --branch BRANCH --title TEXT --scope TEXT --acceptance TEXT [--acceptance TEXT...] [--child-issue-url URL] [--json]
  agent-harness issueops child status --parent ID [--repair] [--json]
  agent-harness issueops child list --parent ID [--json]
  agent-harness issueops child accept --parent ID --child ID --evidence TEXT [--evidence TEXT...] [--json]
  agent-harness issueops child reject --parent ID --child ID --reason REASON [--json]
  agent-harness issueops child drop --parent ID --child ID --reason REASON [--json]`

func runIssueOpsBranch(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(issueOpsBranchPrepareUsage)
		return nil
	}
	if args[0] != "prepare" {
		return fmt.Errorf("unknown issueops branch subcommand")
	}
	fs := flag.NewFlagSet("issueops branch prepare", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	provider := fs.String("provider", "", "remote provider: github or gitlab")
	issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
	branch := fs.String("branch", "", "provider-linked issue-number branch name")
	baseBranch := fs.String("base-branch", "", "remote base branch or ref")
	baseSHA := fs.String("base-sha", "", "optional resolved base commit SHA")
	remoteBranchURL := fs.String("remote-branch-url", "", "optional provider branch URL after creation")
	linkVerified := fs.Bool("link-verified", false, "record that the provider issue shows the branch link")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := core.PrepareIssueOpsBranchWithActor(core.IssueOpsStateRoot(), *id, core.IssueOpsBranchPrepareRequest{
		Provider:        *provider,
		IssueURL:        *issueURL,
		Branch:          *branch,
		BaseBranch:      *baseBranch,
		BaseSHA:         *baseSHA,
		RemoteBranchURL: *remoteBranchURL,
		LinkVerified:    *linkVerified,
	}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func printIssueOpsResult(record core.IssueOpsRecord, jsonOut bool, err error) error {
	if err != nil {
		if jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if jsonOut {
		return printJSON(record)
	}
	fmt.Printf("%s %s %s\n", record.ID, record.Phase, record.Repo)
	return nil
}

func printIssueOpsErrorJSON(err error) error {
	if err == nil {
		return nil
	}
	payload := map[string]any{
		"ok":    false,
		"error": err.Error(),
	}
	if structured, ok := err.(interface{ IssueOpsErrorFields() map[string]any }); ok {
		for key, value := range structured.IssueOpsErrorFields() {
			if value != nil && value != "" {
				payload[key] = value
			}
		}
	}
	return printJSON(payload)
}
