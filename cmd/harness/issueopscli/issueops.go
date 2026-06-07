package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkcmd"
	"agent-harness/cmd/harness/issueopscli/feedbackcleanup"
	"flag"
	"fmt"

	"agent-harness/internal/core"
)

func runIssueOps(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		issueOpsUsage()
		return nil
	}
	switch args[0] {
	case "start":
		fs := flag.NewFlagSet("issueops start", flag.ContinueOnError)
		repo := fs.String("repo", "", "repository path")
		branch := fs.String("branch", "", "working branch")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
		return printIssueOpsResult(record, *jsonOut, err)
	case "status":
		fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		return printIssueOpsResult(record, *jsonOut, err)
	case "intent":
		return runIssueOpsIntent(args[1:])
	case "design":
		return runIssueOpsDesign(args[1:])
	case "link-issue":
		fs := flag.NewFlagSet("issueops link-issue", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), *id, *issueURL)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-plan":
		fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		planPath := fs.String("plan-path", "", "issue-driven plan path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), *id, *planPath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-worktree":
		fs := flag.NewFlagSet("issueops link-worktree", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		worktreePath := fs.String("worktree-path", "", "issue-driven worktree path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), *id, *worktreePath)
		return printIssueOpsResult(record, *jsonOut, err)
	case "link-child":
		fs := flag.NewFlagSet("issueops link-child", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		childURL := fs.String("child-url", "", "GitHub sub-issue or GitLab child item URL")
		title := fs.String("title", "", "optional child issue title")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		if err := verifyIssueOpsChildIssueBeforeLink(*childURL); err != nil {
			return printIssueOpsResult(core.IssueOpsRecord{OK: false}, *jsonOut, err)
		}
		record, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), *id, *childURL, *title)
		return printIssueOpsResult(record, *jsonOut, err)
	case "branch":
		return runIssueOpsBranch(args[1:])
	case "worktree":
		return runIssueOpsWorktree(args[1:])
	case "phase":
		fs := flag.NewFlagSet("issueops phase", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		to := fs.String("to", "", "target phase: problem, grill, plan, implement, ai-slop-clean, feedback, pr, done")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), *id, *to)
		return printIssueOpsResult(record, *jsonOut, err)
	case "feedback":
		return runIssueOpsFeedback(args[1:])
	case "cleanup":
		return runIssueOpsCleanup(args[1:])
	case "benchmark":
		return benchmarkcmd.Run(args[1:])
	case "remote":
		return runIssueOpsRemote(args[1:])
	case "remote-score":
		return runIssueOpsRemote(append([]string{"score"}, args[1:]...))
	case "pr-readiness":
		fs := flag.NewFlagSet("issueops pr-readiness", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		strict := fs.Bool("strict", false, "verify git cleanliness, upstream sync, plan path, and linked worktree path")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
		if err != nil {
			if *jsonOut {
				if printErr := printIssueOpsErrorJSON(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		readiness := core.IssueOpsPRReadiness(record)
		if *strict {
			readiness = core.IssueOpsStrictPRReadiness(record)
		}
		if *jsonOut {
			return printJSON(readiness)
		}
		fmt.Printf("ready: %v\n", readiness.Ready)
		for _, missing := range readiness.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		return nil
	default:
		return fmt.Errorf("unknown issueops subcommand %q", args[0])
	}
}

func parseIssueOpsFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func runIssueOpsFeedback(args []string) error {
	return feedbackcleanup.RunFeedback(args, issueOpsFeedbackCleanupDeps())
}

func runIssueOpsCleanup(args []string) error {
	return feedbackcleanup.RunCleanup(args, issueOpsFeedbackCleanupDeps())
}

func issueOpsFeedbackCleanupDeps() feedbackcleanup.Deps {
	return feedbackcleanup.Deps{
		ParseFlags:   parseIssueOpsFlags,
		PrintResult:  printIssueOpsResult,
		PrintJSON:    printJSON,
		PrintError:   printIssueOpsErrorJSON,
		VerifyMerged: verifyIssueOpsRemoteArtifactMergedLive,
	}
}

func issueOpsCleanupMerged(id string, requested bool) bool {
	return feedbackcleanup.CleanupMerged(id, requested, issueOpsFeedbackCleanupDeps())
}
