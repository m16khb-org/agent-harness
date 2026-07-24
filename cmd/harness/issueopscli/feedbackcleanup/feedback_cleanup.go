package feedbackcleanup

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/core"
	issueopscore "agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/orphancleanup"
)

type Deps struct {
	ParseFlags    func(fs *flag.FlagSet, args []string) (bool, error)
	PrintResult   func(record core.IssueOpsRecord, jsonOut bool, err error) error
	PrintJSON     func(value any) error
	PrintError    func(err error) error
	VerifyMerged  func(core.IssueOpsRemoteArtifactVerification) error
	Provider      func(provider string) (core.IssueProvider, error)
	OrphanPreview func(context.Context, orphancleanup.Request) (orphancleanup.Result, error)
	OrphanApply   func(context.Context, orphancleanup.Request, orphancleanup.ApplyRequest) (orphancleanup.Result, error)
}

func RunFeedback(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops feedback add --id ID --source TEXT --body TEXT --host HOST --session-id SESSION --cwd PATH [--agent-id ID] [--classification TEXT] [--json]\n       agent-harness issueops feedback mark-issue-updated --id ID --host HOST --session-id SESSION --cwd PATH [--agent-id ID] [--json]")
		return nil
	}
	switch args[0] {
	case "add":
		fs := flag.NewFlagSet("issueops feedback add", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		source := fs.String("source", "", "feedback source")
		body := fs.String("body", "", "feedback body")
		classification := fs.String("classification", "", "optional feedback classification, such as contract_change, defect, question, or noise")
		host := fs.String("host", "", "native actor host")
		sessionID := fs.String("session-id", "", "native actor session id")
		agentID := fs.String("agent-id", "", "native actor agent id")
		cwd := fs.String("cwd", "", "canonical actor cwd")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.AddIssueOpsFeedbackWithActor(core.IssueOpsStateRoot(), *id, *source, *body, *classification, localActor(*host, *sessionID, *agentID, *cwd))
		return deps.PrintResult(record, *jsonOut, err)
	case "mark-issue-updated":
		fs := flag.NewFlagSet("issueops feedback mark-issue-updated", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		host := fs.String("host", "", "native actor host")
		sessionID := fs.String("session-id", "", "native actor session id")
		agentID := fs.String("agent-id", "", "native actor agent id")
		cwd := fs.String("cwd", "", "canonical actor cwd")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := core.MarkIssueOpsContractFeedbackIssueUpdatedWithActor(core.IssueOpsStateRoot(), *id, localActor(*host, *sessionID, *agentID, *cwd))
		return deps.PrintResult(record, *jsonOut, err)
	default:
		return fmt.Errorf("unknown issueops feedback subcommand")
	}
}

func localActor(host, sessionID, agentID, cwd string) core.IssueOpsActor {
	ancestry, _ := issueopscore.ObserveNativeProcessAncestry(os.Getpid())
	return core.IssueOpsActor{
		Host: host, SessionID: sessionID, AgentID: agentID, CWD: cwd,
		NativeProcessAncestry: ancestry,
	}
}

func RunCleanup(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops cleanup status --id ID [--merged] [--json]\n       agent-harness issueops cleanup close-children --id ID --merged [--confirm] [--json]\n       agent-harness issueops cleanup orphan --id ID --repo ROOT --worktree PATH --branch NAME --provider github|gitlab --kind pr|mr --artifact-url URL [--apply --confirm --fingerprint SHA256] [--json]")
		return nil
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("issueops cleanup status", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		merged := fs.Bool("merged", false, "confirm the remote PR/MR was verified merged before cleanup")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		verifiedMerged := CleanupMerged(*id, *merged, deps)
		status, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), *id, core.IssueOpsCleanupStatusRequest{Merged: verifiedMerged})
		if err != nil {
			if *jsonOut {
				if printErr := deps.PrintError(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *jsonOut {
			return deps.PrintJSON(status)
		}
		fmt.Printf("ready: %v\n", status.Ready)
		for _, missing := range status.Missing {
			fmt.Printf("- missing: %s\n", missing)
		}
		if len(status.Choices) > 0 {
			fmt.Println("선택지:")
			for _, choice := range status.Choices {
				fmt.Println(choice)
			}
		}
		return nil
	case "close-children":
		fs := flag.NewFlagSet("issueops cleanup close-children", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		merged := fs.Bool("merged", false, "confirm child PR/MR merge into the parent work branch before closing child tasks")
		confirm := fs.Bool("confirm", false, "execute remote child close and record verification; without this, dry-run preview only")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := deps.ParseFlags(fs, args[1:]); help || err != nil {
			return err
		}
		verifiedMerged := CleanupMerged(*id, *merged, deps)
		result, err := core.CloseIssueOpsChildren(core.IssueOpsStateRoot(), *id, core.IssueOpsCloseChildrenRequest{
			Merged:  verifiedMerged,
			Confirm: *confirm,
		}, deps.Provider)
		if err != nil {
			if *jsonOut {
				if printErr := deps.PrintError(err); printErr != nil {
					return printErr
				}
			}
			return err
		}
		if *jsonOut {
			return deps.PrintJSON(result)
		}
		fmt.Printf("closed children: %d\n", result.ClosedCount)
		for _, child := range result.Children {
			if child.Preview != "" {
				fmt.Println(child.Preview)
			} else {
				fmt.Printf("- %s closed=%t state=%s\n", child.URL, child.Closed, child.State)
			}
		}
		return nil
	case "orphan":
		return runOrphanCleanup(args[1:], deps)
	default:
		return fmt.Errorf("unknown issueops cleanup subcommand")
	}
}

func runOrphanCleanup(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops cleanup orphan", flag.ContinueOnError)
	id := fs.String("id", "", "missing IssueOps lifecycle id expected to have no record")
	repo := fs.String("repo", "", "exact canonical repository root")
	worktree := fs.String("worktree", "", "exact recordless worktree path")
	branch := fs.String("branch", "", "exact local branch checked out by the worktree")
	provider := fs.String("provider", "", "remote artifact provider: github or gitlab")
	kind := fs.String("kind", "", "remote artifact kind: pr or mr")
	artifactURL := fs.String("artifact-url", "", "merged remote pull or merge request URL")
	apply := fs.Bool("apply", false, "remove only the confirmed local worktree and local branch")
	confirm := fs.Bool("confirm", false, "confirm the exact preview fingerprint for --apply")
	fingerprint := fs.String("fingerprint", "", "ready preview fingerprint required for --apply")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	if *confirm && !*apply {
		return fmt.Errorf("orphan cleanup --confirm requires --apply")
	}
	if strings.TrimSpace(*fingerprint) != "" && !*apply {
		return fmt.Errorf("orphan cleanup --fingerprint requires --apply")
	}
	request := orphancleanup.Request{
		ID:           *id,
		RepoRoot:     *repo,
		WorktreePath: *worktree,
		Branch:       *branch,
		Artifact: core.IssueOpsRemoteArtifactVerification{
			Provider: *provider,
			Kind:     *kind,
			URL:      *artifactURL,
		},
	}
	var (
		result orphancleanup.Result
		err    error
	)
	if *apply {
		if !*confirm {
			return fmt.Errorf("orphan cleanup apply requires --confirm")
		}
		if deps.OrphanApply == nil {
			return fmt.Errorf("orphan cleanup apply is unavailable")
		}
		result, err = deps.OrphanApply(context.Background(), request, orphancleanup.ApplyRequest{Confirm: *confirm, Fingerprint: *fingerprint})
	} else {
		if deps.OrphanPreview == nil {
			return fmt.Errorf("orphan cleanup preview is unavailable")
		}
		result, err = deps.OrphanPreview(context.Background(), request)
	}
	if *jsonOut {
		if printErr := deps.PrintJSON(result); printErr != nil {
			return printErr
		}
	} else {
		printOrphanCleanupResult(result)
	}
	return err
}

func printOrphanCleanupResult(result orphancleanup.Result) {
	fmt.Printf("ready: %v\n", result.Ready)
	fmt.Printf("head: %s\n", result.HeadSHA)
	fmt.Printf("recovery path: %s\n", result.RecoveryPath)
	for _, missing := range result.Missing {
		fmt.Printf("- missing: %s\n", missing)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("- warning: %s\n", warning)
	}
	if result.Fingerprint != "" {
		fmt.Printf("fingerprint: %s\n", result.Fingerprint)
	}
	if result.RemoteBranchDeletion != "" {
		fmt.Printf("remote branch: %s\n", result.RemoteBranchDeletion)
	}
}

func CleanupMerged(id string, requested bool, deps Deps) bool {
	if !requested {
		return false
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil || record.RemoteArtifact == nil {
		return false
	}
	return deps.VerifyMerged(*record.RemoteArtifact) == nil
}
