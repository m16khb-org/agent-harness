package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type issueOpsWorktreeToolPrepareResult struct {
	OK                   bool     `json:"ok"`
	ID                   string   `json:"id"`
	WorktreePath         string   `json:"worktree_path"`
	PackageManager       string   `json:"package_manager,omitempty"`
	DependenciesChecked  bool     `json:"dependencies_checked,omitempty"`
	DependenciesReady    bool     `json:"dependencies_ready,omitempty"`
	DependenciesAction   string   `json:"dependencies_action,omitempty"`
	CodeGraphProjectPath string   `json:"codegraph_project_path"`
	CodeGraphChecked     bool     `json:"codegraph_checked"`
	CodeGraphInitialized bool     `json:"codegraph_initialized,omitempty"`
	CodeGraphReady       bool     `json:"codegraph_ready"`
	Messages             []string `json:"messages,omitempty"`
}

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

func prepareIssueOpsWorktreeDependencies(worktree string, result *issueOpsWorktreeToolPrepareResult) error {
	manager := issueOpsWorktreePackageManager(worktree)
	if manager == "" {
		return nil
	}
	result.PackageManager = manager
	result.DependenciesChecked = true
	if info, err := os.Stat(filepath.Join(worktree, "node_modules")); err == nil && info.IsDir() {
		result.DependenciesReady = true
		result.DependenciesAction = "already_present"
		result.Messages = append(result.Messages, "node_modules already present in IssueOps worktree")
		return nil
	}
	switch manager {
	case "pnpm":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "pnpm", "install", "--frozen-lockfile", "--prefer-offline")
		cmd.Dir = worktree
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("install pnpm dependencies for IssueOps worktree timed out")
		}
		if err != nil {
			return fmt.Errorf("install pnpm dependencies for IssueOps worktree: %w: %s", err, strings.TrimSpace(string(out)))
		}
		result.DependenciesReady = true
		result.DependenciesAction = "pnpm_install"
		result.Messages = append(result.Messages, "installed pnpm dependencies for IssueOps worktree")
		return nil
	default:
		result.DependenciesAction = "manual"
		result.Messages = append(result.Messages, "detected "+manager+" dependencies; install them in the IssueOps worktree before running tests")
		return nil
	}
}

func issueOpsWorktreePackageManager(worktree string) string {
	if _, err := os.Stat(filepath.Join(worktree, "package.json")); err != nil {
		return ""
	}
	switch {
	case fileExists(filepath.Join(worktree, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(worktree, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(worktree, "package-lock.json")):
		return "npm"
	default:
		return ""
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
