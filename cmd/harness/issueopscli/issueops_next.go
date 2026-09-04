package issueopscli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	issueopsnextcontract "agent-harness/internal/contract/issueopsnext"
)

// runIssueOpsNext는 현재 단계와 다음 명령을 출력한다. 읽기 전용이므로 어느
// 단계에서 실행해도 사이클을 바꾸지 않는다.
func runIssueOpsNext(args []string) error {
	fs := flag.NewFlagSet("issueops next", flag.ContinueOnError)
	id := fs.String("id", "", "select a cycle by id")
	cwd := fs.String("cwd", "", "classify as if run from this directory")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	directory := strings.TrimSpace(*cwd)
	if directory == "" {
		current, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve working directory: %w", err)
		}
		directory = current
	}
	result, err := issueOpsCLIDeps.IssueOpsNext(issueOpsCLIDeps.IssueOpsStateRoot(), directory, *id)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	printIssueOpsNextText(result)
	return nil
}

func printIssueOpsNextText(result issueopsnextcontract.Result) {
	fmt.Printf("stage %d/10 %s  cycle %s  phase %s  lease %s\n",
		result.Stage.Index, result.Stage.Key, issueOpsNextCycleID(result), issueOpsNextPhase(result), issueOpsNextLease(result))
	if len(result.Missing) > 0 {
		fmt.Printf("missing: %s\n", strings.Join(result.Missing, ", "))
	}
	for _, candidate := range result.Candidates {
		fmt.Printf("candidate: %s phase=%s branch=%s\n", candidate.ID, candidate.Phase, candidate.Branch)
	}
	if result.NextCommand != "" {
		fmt.Printf("next: %s\n", result.NextCommand)
	}
	if result.Exits.AbandonCommand != "" {
		fmt.Printf("exits: pause=%s abandon=%s takeover=%s\n",
			issueOpsNextExit(result.Exits.PauseCommand), result.Exits.AbandonCommand, issueOpsNextExit(result.Exits.TakeoverCommand))
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
}

func issueOpsNextCycleID(result issueopsnextcontract.Result) string {
	if result.Selected == nil {
		return "-"
	}
	return result.Selected.ID
}

func issueOpsNextPhase(result issueopsnextcontract.Result) string {
	if result.Selected == nil || result.Selected.Phase == "" {
		return "-"
	}
	return result.Selected.Phase
}

func issueOpsNextLease(result issueopsnextcontract.Result) string {
	if result.Lease.Status == "" {
		return "none"
	}
	// holder 표기는 lease 상태에서 파생한다. claimable·released는 홀더가 없고,
	// active·revoking은 이 세션이 아니면 다른 세션이 쥐고 있다.
	holder := "none"
	switch {
	case result.Lease.HolderIsSelf:
		holder = "self"
	case result.Lease.Status == string(issueopsnextcontract.LeaseStatusActive),
		result.Lease.Status == string(issueopsnextcontract.LeaseStatusRevoking):
		holder = "other"
	}
	return fmt.Sprintf("%s(gen %d, %s)", result.Lease.Status, result.Lease.Generation, holder)
}

func issueOpsNextExit(command string) string {
	if strings.TrimSpace(command) == "" {
		return "-"
	}
	return command
}
