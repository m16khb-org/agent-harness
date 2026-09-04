package gitworktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	command, commandErr := workspaceRelaunchCommand(host, req.SourceRoot, req.Root, base)
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
	if code, _, stderr := GitCmd(receipt.SourceRoot, args...); code != 0 {
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
	top := strings.TrimSpace(GitOut(receipt.SourceRoot, "rev-parse", "--show-toplevel"))
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
	top := GitOut(receipt.Root, "rev-parse", "--show-toplevel")
	branch := GitOut(receipt.Root, "branch", "--show-current")
	head := GitOut(receipt.Root, "rev-parse", "HEAD")
	if !sameResolvedPath(top, receipt.Root) || branch != receipt.Branch || head != receipt.BaseHead {
		return false, fmt.Errorf("existing canonical worktree identity does not match branch and base_head")
	}
	return true, nil
}

func worktreeAddArgs(receipt port.ExecutionWorkspaceReceipt) []string {
	if GitOut(receipt.SourceRoot, "show-ref", "--verify", "--hash", "refs/heads/"+receipt.Branch) != "" {
		return []string{"worktree", "add", "-q", receipt.Root, receipt.Branch}
	}
	for _, remote := range []string{"origin", "upstream"} {
		ref := "refs/remotes/" + remote + "/" + receipt.Branch
		if GitOut(receipt.SourceRoot, "show-ref", "--verify", "--hash", ref) != "" {
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

// workspaceRelaunchCommand는 접근이 막혔을 때 사용자가 실행할 정확한 재기동
// 명령을 만든다.
//
// 착지점은 canonical worktree다. source root로 되돌리면 그 세션이 source
// checkout을 작업 대상으로 오인하기 쉽고, 실제로 codex와 omo는 source root로,
// claude는 아무 데도 가지 않아 이전 디렉터리에 머물렀다. 워크트리를 2단계가
// 미리 만드는 흐름에서는 그 경로가 이미 존재하므로 그리로 데려간다.
//
// 아직 만들어지지 않은 사이클(worktree를 prepare가 만드는 경로)에서 `cd`는
// 실패하고 `&&`가 끊겨 host가 아예 뜨지 않는다. 그래서 존재를 관측해 없으면
// source root로 되돌린다 — 되띄운 세션은 거기서 prepare를 다시 실행하면 된다.
func workspaceRelaunchCommand(host, sourceRoot, root, base string) (string, error) {
	landing := strings.TrimSpace(root)
	if info, err := os.Lstat(landing); landing == "" || err != nil || !info.IsDir() {
		landing = sourceRoot
	}
	landing, base = shellQuotePath(landing), shellQuotePath(base)
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "codex":
		return "codex --cd " + landing + " --add-dir " + base, nil
	case "claude":
		return "cd " + landing + " && claude --add-dir " + base, nil
	case "omo":
		return "cd " + landing + " && omo", nil
	default:
		return "", fmt.Errorf("native actor host must be codex, claude, or omo")
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
