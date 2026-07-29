package issueopscli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	cliadapter "agent-harness/internal/adapter/cli"
	"agent-harness/internal/core"
)

func issueOpsUsage() {
	fmt.Fprint(os.Stderr, issueOpsUsageText())
}

// issueOpsUsageText는 `issueops` 서브커맨드 usage 원문을 반환한다.
//
// 줄 자체는 여기 없다 — `internal/adapter/cli`의 카탈로그가 유일한 원본이고 최상위
// usage는 같은 카탈로그를 축약 키로 걸러 렌더한다(#188). 전에는 같은 줄을 두 곳에
// 손으로 유지했고, 한쪽 누락은 parity 테스트가 잡았지만 **양쪽에 아예 없으면**
// 검사할 대상이 없어 `execution switch-mode`(#167)가 그 구멍으로 살아남았다.
func issueOpsUsageText() string {
	return "Usage:\n" +
		strings.Join(cliadapter.IssueOpsUsageLines(), "\n") + "\n\n" +
		cliadapter.IssueOpsActorFlagLegend + "\n"
}

const issueOpsBranchPrepareUsage = "Usage: agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--parent-worktree PATH] [--remote-branch-url URL] [--link-verified] [--json]"

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
	parentWorktree := fs.String("parent-worktree", "", "optional canonical parent worktree for Orca lineage")
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
		ParentWorktree:  *parentWorktree,
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
