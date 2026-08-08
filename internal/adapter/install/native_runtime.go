package install

import (
	installcontract "agent-harness/internal/contract/install"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiagnoseNativeRuntime compares the currently executing installer-owned binary
// with the stable source checkout runtime. It also recognizes a removed managed
// worktree from the source.worktrees/<name> layout after Git metadata is gone.
func DiagnoseNativeRuntime(executable string) (installcontract.NativeRuntimeDiagnostic, error) {
	observed := absClean(executable)
	result := installcontract.NativeRuntimeDiagnostic{Observed: observed}
	if observed == "" {
		return result, fmt.Errorf("native runtime executable is required")
	}
	if info, err := os.Lstat(observed); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if resolved, err := filepath.EvalSymlinks(observed); err == nil {
			observed = absClean(resolved)
			result.Observed = observed
		}
	}
	if filepath.Base(observed) != nativeBinaryName || filepath.Base(filepath.Dir(observed)) != "bin" {
		return installcontract.NativeRuntimeDiagnostic{}, nil
	}

	invokingRoot := filepath.Dir(filepath.Dir(observed))
	stableRoot, err := ResolveStableNativeRoot(invokingRoot)
	if err != nil {
		return result, err
	}
	if stableRoot == invokingRoot {
		if managedRoot, ok := removedManagedWorktreeSource(invokingRoot); ok {
			stableRoot = managedRoot
		}
	}
	result.Expected = filepath.Join(stableRoot, "bin", nativeBinaryName)
	result.Stale = result.Observed != result.Expected
	result.RestartRequired = result.Stale
	return result, nil
}

func NativeRuntimeDiagnosticMessage(diagnostic installcontract.NativeRuntimeDiagnostic, err error) (string, bool) {
	if err != nil {
		return fmt.Sprintf(
			"native hook runtime verification failed: observed=%s error=%v; reinstall hooks and restart the host session",
			diagnostic.Observed, err,
		), true
	}
	if !diagnostic.Stale {
		return "", false
	}
	return fmt.Sprintf(
		"cached native hook runtime is stale: observed=%s expected=%s; reinstall hooks and restart the host session",
		diagnostic.Observed, diagnostic.Expected,
	), true
}

func removedManagedWorktreeSource(root string) (string, bool) {
	worktreesDir := filepath.Dir(root)
	if !strings.HasSuffix(worktreesDir, ".worktrees") {
		return "", false
	}
	source := strings.TrimSuffix(worktreesDir, ".worktrees")
	info, err := os.Lstat(filepath.Join(source, ".git"))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", false
	}
	return filepath.Clean(source), true
}
