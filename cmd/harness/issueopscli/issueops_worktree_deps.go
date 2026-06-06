package issueopscli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
