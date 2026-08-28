package issueopscli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"

	cliadapter "agent-harness/internal/domain/cli"
)

func issueOpsUsage() {
	fmt.Fprint(os.Stderr, issueOpsUsageText())
}

// issueOpsUsageText는 `issueops` 서브커맨드 usage 원문을 반환한다.
//
// 줄 자체는 여기 없다 — `internal/domain/cli`의 카탈로그가 유일한 원본이고 최상위
// usage는 같은 카탈로그를 축약 키로 걸러 렌더한다(#188). 전에는 같은 줄을 두 곳에
// 손으로 유지했고, 한쪽 누락은 parity 테스트가 잡았지만 **양쪽에 아예 없으면**
// 검사할 대상이 없어 `execution switch-mode`(#167)가 그 구멍으로 살아남았다.
func issueOpsUsageText() string {
	return "Usage:\n" +
		strings.Join(cliadapter.IssueOpsUsageLines(), "\n") + "\n\n" +
		cliadapter.IssueOpsActorFlagLegend + "\n"
}

const issueOpsBranchPrepareUsage = "Usage: agent-harness issueops branch prepare --id ID --provider github|gitlab --issue-url URL --branch NAME --base-branch REF [--base-sha SHA] [--parent-worktree PATH] [--remote-branch-url URL] [--link-verified] [--json]\n       agent-harness issueops branch await-link --id ID [--timeout DURATION] [--json]\n       agent-harness issueops branch retarget --id ID --base-branch REF --reason TEXT [--json]"

// issueOpsChildUsageText는 canonical catalog에서 child 하위 명령만 골라 렌더한다.
// usage 문장을 다시 적지 않아 parser/help 계약의 별도 drift를 막는다(#207).
func issueOpsChildUsageText() string {
	var lines []string
	for _, line := range cliadapter.IssueOpsUsageLines() {
		switch cliadapter.IssueOpsUsageKey(line) {
		case "child start", "child status", "child list",
			"child accept", "child reject", "child drop":
			lines = append(lines, line)
		}
	}
	return "Usage:\n" +
		strings.Join(lines, "\n") + "\n\n" +
		cliadapter.IssueOpsActorFlagLegend
}

func runIssueOpsBranch(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(issueOpsBranchPrepareUsage)
		return nil
	}
	if args[0] == "await-link" {
		return runIssueOpsBranchAwaitLink(args[1:])
	}
	if args[0] == "retarget" {
		return runIssueOpsBranchRetarget(args[1:])
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
	record, err := issueOpsCLIDeps.PrepareIssueOpsBranchWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, issueopscontract.IssueOpsBranchPrepareRequest{
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

func printIssueOpsResult(record issueopscontract.IssueOpsRecord, jsonOut bool, err error) error {
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

// runIssueOpsBranchAwaitLink는 coordinator가 만들 linked branch가 나타날
// 때까지 경계 있게 기다린다(#319).
//
// 읽기 전용이다. GitHub Orca 경로에서 owner는 링크가 아직 없는 시점에
// 시작하므로, 그 시작 창을 terminal 실패로 다루지 않으려면 owner가 기다릴
// 수단이 필요하다. 대기를 프롬프트 지시가 아니라 명령으로 두어야 간격과
// 상한이 값으로 고정된다.
// runIssueOpsBranchRetarget은 provider가 실제로 보여주는 PR/MR target으로 준비
// base를 옮긴다. cleanup finish의 base_branch_drifted는 레코드에 없는 base 변경을
// 거부하므로, 정당한 재타깃은 finish 전에 이 명령으로 기록한다.
func runIssueOpsBranchRetarget(args []string) error {
	fs := flag.NewFlagSet("issueops branch retarget", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	baseBranch := fs.String("base-branch", "", "new base branch; must equal the PR/MR target the provider shows")
	reason := fs.String("reason", "", "why the base changed")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueOpsCLIDeps.RetargetIssueOpsBranchWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id,
		issueopscontract.IssueOpsBranchRetargetRequest{BaseBranch: *baseBranch, Reason: *reason}, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsBranchAwaitLink(args []string) error {
	fs := flag.NewFlagSet("issueops branch await-link", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	timeout := fs.String("timeout", "", "bounded wait, e.g. 10m (default 10m, max 30m)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	if issueOpsCLIDeps.AwaitIssueOpsBranchLink == nil {
		return fmt.Errorf("issueops branch await-link is unavailable")
	}
	result, err := issueOpsCLIDeps.AwaitIssueOpsBranchLink(context.Background(), issueOpsCLIDeps.IssueOpsStateRoot(),
		issueopscontract.AwaitBranchLinkRequest{ID: *id, Timeout: *timeout})
	if err != nil {
		if *jsonOut {
			if printErr := printJSON(result); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	switch {
	case result.AlreadyVerified:
		fmt.Printf("branch link already recorded: branch=%s\n", result.Branch)
	default:
		fmt.Printf("branch link observed: branch=%s oid=%s attempts=%d\n", result.Branch, result.ObservedOID, result.Attempts)
		if result.NextCommand != "" {
			fmt.Printf("next: %s\n", result.NextCommand)
		}
	}
	return nil
}
