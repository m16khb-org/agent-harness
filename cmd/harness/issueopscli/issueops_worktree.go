package issueopscli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"agent-harness/internal/core"
)

func runIssueOpsWorktree(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops worktree prepare-tools --id ID [--json]")
		return nil
	}
	if args[0] != "prepare-tools" {
		return fmt.Errorf("unknown issueops worktree subcommand")
	}
	fs := flag.NewFlagSet("issueops worktree prepare-tools", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
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
	result, err := prepareIssueOpsWorktreeTools(record)
	if err != nil {
		if *jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("worktree: %s\n", result.WorktreePath)
	if result.DependenciesChecked {
		fmt.Printf("package_manager: %s\n", result.PackageManager)
		fmt.Printf("dependencies_ready: %v\n", result.DependenciesReady)
		if result.DependenciesAction != "" {
			fmt.Printf("dependencies_action: %s\n", result.DependenciesAction)
		}
	}
	fmt.Printf("codegraph_project_path: %s\n", result.CodeGraphProjectPath)
	fmt.Printf("codegraph_ready: %v\n", result.CodeGraphReady)
	for _, message := range result.Messages {
		fmt.Printf("- %s\n", message)
	}
	return nil
}

func prepareIssueOpsWorktreeTools(record core.IssueOpsRecord) (issueOpsWorktreeToolPrepareResult, error) {
	worktree := strings.TrimSpace(record.WorktreePath)
	result := issueOpsWorktreeToolPrepareResult{
		OK:                   true,
		ID:                   record.ID,
		WorktreePath:         worktree,
		CodeGraphProjectPath: worktree,
	}
	if worktree == "" {
		result.OK = false
		return result, fmt.Errorf("worktree_path is required")
	}
	if info, err := os.Stat(worktree); err != nil || !info.IsDir() {
		result.OK = false
		return result, fmt.Errorf("worktree_path does not exist or is not a directory: %s", worktree)
	}
	if err := prepareIssueOpsWorktreeDependencies(worktree, &result); err != nil {
		result.OK = false
		return result, err
	}
	result.CodeGraphChecked = true
	if err := exec.Command("codegraph", "status", worktree).Run(); err == nil {
		result.CodeGraphReady = true
		result.Messages = append(result.Messages, "CodeGraph index already ready for IssueOps worktree")
	} else if out, err := exec.Command("codegraph", "init", "-i", worktree).CombinedOutput(); err != nil {
		result.OK = false
		return result, fmt.Errorf("initialize CodeGraph for IssueOps worktree: %w: %s", err, strings.TrimSpace(string(out)))
	} else {
		result.CodeGraphInitialized = true
		result.CodeGraphReady = true
		result.Messages = append(result.Messages, "initialized CodeGraph index for IssueOps worktree")
	}
	return result, nil
}
