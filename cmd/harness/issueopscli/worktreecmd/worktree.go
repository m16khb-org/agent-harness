package worktreecmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/cmd/harness/issueopscli/worktreetools"
	"agent-harness/internal/core"
)

type Deps struct {
	ParseFlags     func(*flag.FlagSet, []string) (bool, error)
	PrintJSON      func(any) error
	PrintError     func(error) error
	PrepareHandoff func(context.Context, string, core.IssueOpsHandoffPrepareRequest) (core.IssueOpsHandoffPrepareResult, error)
}

type PrepareResult = worktreetools.PrepareResult

func Run(args []string, deps Deps) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage:")
		fmt.Println("  agent-harness issueops worktree prepare --id ID [--orchestrator auto|orca|inline] [--agent NAME] [--confirm] [--json]")
		fmt.Println("  agent-harness issueops worktree prepare-tools --id ID [--json]")
		fmt.Println("  agent-harness issueops worktree verify --id ID [--json]")
		fmt.Println("  agent-harness issueops worktree cleanup-readiness --id ID [--merged] [--json]")
		return nil
	}
	switch args[0] {
	case "prepare":
		return runWorktreePrepare(args[1:], deps)
	case "prepare-tools":
		return runWorktreePrepareTools(args[1:], deps)
	case "verify":
		return runWorktreeVerify(args[1:], deps)
	case "cleanup-readiness":
		return runWorktreeCleanupReadiness(args[1:], deps)
	default:
		return fmt.Errorf("unknown issueops worktree subcommand %q", args[0])
	}
}

func runWorktreePrepareTools(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops worktree prepare-tools", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			if printErr := deps.PrintError(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	result, err := PrepareWorktreeTools(record)
	if err != nil {
		if *jsonOut {
			if printErr := deps.PrintError(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	updated, err := core.RecordIssueOpsWorktreeTools(core.IssueOpsStateRoot(), record.ID, result.IssueOpsWorktreeToolPreparation())
	if err != nil {
		if *jsonOut {
			if printErr := deps.PrintError(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if updated.WorktreeTools != nil {
		result.PreparedAt = updated.WorktreeTools.PreparedAt
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	fmt.Printf("worktree: %s\n", result.WorktreePath)
	if result.DependenciesChecked {
		fmt.Printf("package_manager: %s\n", result.PackageManager)
		fmt.Printf("dependencies_ready: %v\n", result.DependenciesReady)
		if result.DependenciesAction != "" {
			fmt.Printf("dependencies_action: %s\n", result.DependenciesAction)
		}
	}
	if result.Guidance != "" {
		fmt.Printf("guidance: %s\n", result.Guidance)
	}
	for _, message := range result.Messages {
		fmt.Printf("- %s\n", message)
	}
	return nil
}

func PrepareWorktreeTools(record core.IssueOpsRecord) (PrepareResult, error) {
	worktree := strings.TrimSpace(record.WorktreePath)
	result := PrepareResult{
		OK:           true,
		ID:           record.ID,
		WorktreePath: worktree,
		Guidance:     core.ExpectedWorktreeEnvGuidance(worktree),
	}
	if worktree == "" {
		result.OK = false
		return result, fmt.Errorf("worktree_path is required")
	}
	if info, err := os.Stat(worktree); err != nil || !info.IsDir() {
		result.OK = false
		return result, fmt.Errorf("worktree_path does not exist or is not a directory: %s", worktree)
	}
	if err := worktreetools.PrepareDependencies(worktree, &result); err != nil {
		result.OK = false
		return result, err
	}
	return result, nil
}

func runWorktreePrepare(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops worktree prepare", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	orchestrator := fs.String("orchestrator", "auto", "orchestrator: auto, orca, or inline")
	agent := fs.String("agent", "codex", "built-in Orca agent host")
	confirm := fs.Bool("confirm", false, "confirm Orca worktree creation")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	if deps.PrepareHandoff == nil {
		return fmt.Errorf("IssueOps handoff worktree dependency is unavailable")
	}
	result, err := deps.PrepareHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffPrepareRequest{
		ID: *id, Orchestrator: *orchestrator, Agent: *agent, Confirm: *confirm,
	})
	if err != nil {
		if *jsonOut {
			_ = deps.PrintError(err)
		}
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	fmt.Printf("worktree_path: %s\n", result.WorktreePath)
	fmt.Printf("branch: %s (base: %s)\n", result.Branch, result.BaseBranch)
	if result.State != "" {
		fmt.Printf("orchestrator: %s (state: %s)\n", result.ResolvedMode, result.State)
	}
	if result.Exists {
		fmt.Println("worktree already exists; link it with issueops link-worktree")
	} else if result.ResolvedMode == "inline" || result.ResolvedMode == "" {
		fmt.Printf("create with: git worktree add %s %s\n", result.WorktreePath, result.Branch)
	}
	return nil
}

func runWorktreeVerify(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops worktree verify", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			_ = deps.PrintError(err)
		}
		return err
	}
	readiness := core.IssueOpsStrictPRReadiness(record)
	if *jsonOut {
		return deps.PrintJSON(readiness)
	}
	fmt.Printf("ready: %v\n", readiness.Ready)
	for _, missing := range readiness.Missing {
		fmt.Printf("- missing: %s\n", missing)
	}
	for _, warn := range readiness.Warnings {
		fmt.Printf("warning: %s\n", warn)
	}
	return nil
}

func runWorktreeCleanupReadiness(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops worktree cleanup-readiness", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	merged := fs.Bool("merged", false, "whether the remote PR/MR was merged")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := deps.ParseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.IssueOpsCleanupStatusByID(core.IssueOpsStateRoot(), *id, core.IssueOpsCleanupStatusRequest{Merged: *merged})
	if err != nil {
		if *jsonOut {
			_ = deps.PrintError(err)
		}
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	fmt.Printf("ready: %v\n", result.Ready)
	for _, missing := range result.Missing {
		fmt.Printf("- missing: %s\n", missing)
	}
	for _, choice := range result.Choices {
		fmt.Printf("- choice: %s\n", choice)
	}
	return nil
}
