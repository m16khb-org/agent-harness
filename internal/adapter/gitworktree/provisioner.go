package gitworktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/port"
)

type Provisioner struct{}

func New() Provisioner {
	return Provisioner{}
}

func (Provisioner) ProbeAccess(ctx context.Context, req port.ExecutionWorkspaceRequest, host string) (port.ExecutionWorkspaceAccessResult, error) {
	if err := ctx.Err(); err != nil {
		return port.ExecutionWorkspaceAccessResult{}, err
	}
	if _, err := validateRequest(req); err != nil {
		return port.ExecutionWorkspaceAccessResult{}, err
	}
	base := filepath.Dir(filepath.Clean(req.Root))
	created, err := ensureProbeBase(req.SourceRoot, base)
	if err == nil {
		err = probeDirectoryReadWrite(base)
	}
	if err == nil {
		return port.ExecutionWorkspaceAccessResult{Allowed: true}, nil
	}
	if created {
		_ = os.Remove(base)
	}
	command, commandErr := workspaceRelaunchCommand(host, req.SourceRoot, base)
	if commandErr != nil {
		return port.ExecutionWorkspaceAccessResult{}, commandErr
	}
	return port.ExecutionWorkspaceAccessResult{
		Allowed: false, Code: "canonical_worktree_base_inaccessible", RelaunchCommand: command,
	}, nil
}

func (Provisioner) Prepare(ctx context.Context, req port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	if err := ctx.Err(); err != nil {
		return port.ExecutionWorkspaceReceipt{}, err
	}
	receipt, err := validateRequest(req)
	if err != nil {
		return receipt, err
	}
	if existing, err := inspectExisting(receipt); err != nil || existing {
		receipt.Exists = existing
		return receipt, err
	}
	if !req.Confirm {
		return receipt, nil
	}
	if err := createRealDirectories(receipt.SourceRoot, filepath.Dir(receipt.Root)); err != nil {
		return receipt, err
	}
	args := worktreeAddArgs(receipt)
	if code, _, stderr := preflight.GitCmd(receipt.SourceRoot, args...); code != 0 {
		return receipt, fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	if _, err := inspectExisting(receipt); err != nil {
		return receipt, err
	}
	receipt.Exists = true
	return receipt, nil
}

func validateRequest(req port.ExecutionWorkspaceRequest) (port.ExecutionWorkspaceReceipt, error) {
	receipt := port.ExecutionWorkspaceReceipt{
		SourceRoot: filepath.Clean(req.SourceRoot), Root: filepath.Clean(req.Root),
		Branch: strings.TrimSpace(req.Branch), BaseHead: strings.TrimSpace(req.BaseHead), Driver: "git",
	}
	if req.LifecycleID == "" || receipt.SourceRoot == "" || receipt.Root == "" || receipt.Branch == "" || receipt.BaseHead == "" {
		return receipt, fmt.Errorf("lifecycle_id, source_root, root, branch, and base_head are required")
	}
	info, err := os.Lstat(receipt.SourceRoot)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return receipt, fmt.Errorf("source_root must be a real directory")
	}
	top := strings.TrimSpace(preflight.GitOut(receipt.SourceRoot, "rev-parse", "--show-toplevel"))
	if !sameResolvedPath(top, receipt.SourceRoot) {
		return receipt, fmt.Errorf("source_root must be the Git top-level")
	}
	expectedBase := receipt.SourceRoot + ".worktrees"
	if filepath.Clean(filepath.Dir(receipt.Root)) != filepath.Clean(expectedBase) {
		return receipt, fmt.Errorf("canonical worktree must use the sibling .worktrees base")
	}
	if receipt.Root == receipt.SourceRoot {
		return receipt, fmt.Errorf("canonical worktree must be isolated from source_root")
	}
	return receipt, nil
}

func inspectExisting(receipt port.ExecutionWorkspaceReceipt) (bool, error) {
	info, err := os.Lstat(receipt.Root)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("canonical worktree path is occupied by a non-directory or symlink")
	}
	top := preflight.GitOut(receipt.Root, "rev-parse", "--show-toplevel")
	branch := preflight.GitOut(receipt.Root, "branch", "--show-current")
	head := preflight.GitOut(receipt.Root, "rev-parse", "HEAD")
	if !sameResolvedPath(top, receipt.Root) || branch != receipt.Branch || head != receipt.BaseHead {
		return false, fmt.Errorf("existing canonical worktree identity does not match branch and base_head")
	}
	return true, nil
}

func worktreeAddArgs(receipt port.ExecutionWorkspaceReceipt) []string {
	if preflight.GitOut(receipt.SourceRoot, "show-ref", "--verify", "--hash", "refs/heads/"+receipt.Branch) != "" {
		return []string{"worktree", "add", "-q", receipt.Root, receipt.Branch}
	}
	for _, remote := range []string{"origin", "upstream"} {
		ref := "refs/remotes/" + remote + "/" + receipt.Branch
		if preflight.GitOut(receipt.SourceRoot, "show-ref", "--verify", "--hash", ref) != "" {
			return []string{"worktree", "add", "-q", "-b", receipt.Branch, receipt.Root, ref}
		}
	}
	return []string{"worktree", "add", "-q", "-b", receipt.Branch, receipt.Root, receipt.BaseHead}
}

func createRealDirectories(sourceRoot, target string) error {
	expected := sourceRoot + ".worktrees"
	if filepath.Clean(target) != filepath.Clean(expected) {
		return fmt.Errorf("refusing to create a non-canonical worktree base")
	}
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return os.Mkdir(target, 0o700)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("canonical worktree base must be a real directory")
	}
	return nil
}

func ensureProbeBase(sourceRoot, target string) (bool, error) {
	if filepath.Clean(target) != filepath.Clean(sourceRoot+".worktrees") {
		return false, fmt.Errorf("refusing to probe a non-canonical worktree base")
	}
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return true, os.Mkdir(target, 0o700)
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("canonical worktree base must be a real directory")
	}
	return false, nil
}

func probeDirectoryReadWrite(base string) error {
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	_ = entries
	file, err := os.CreateTemp(base, ".agent-harness-access-*")
	if err != nil {
		return err
	}
	path := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(path)
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

func workspaceRelaunchCommand(host, sourceRoot, base string) (string, error) {
	sourceRoot, base = shellQuotePath(sourceRoot), shellQuotePath(base)
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "codex":
		return "codex --cd " + sourceRoot + " --add-dir " + base, nil
	case "claude":
		return "claude --add-dir " + base, nil
	default:
		return "", fmt.Errorf("native actor host must be codex or claude")
	}
}

func shellQuotePath(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func sameResolvedPath(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

var _ port.ExecutionWorkspaceAccessProber = Provisioner{}
