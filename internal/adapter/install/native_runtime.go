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
	// 경로가 같아도 파일이 교체됐으면 실행 중인 프로세스는 교체 이전 세대를
	// 계속 쓴다. 그 상태에서 새 typed command를 쓰면 이전 세대 hook이 그것을
	// 모르고 차단해 복구가 교착된다(#328). 경로 비교로는 잡히지 않으므로
	// 빌드 세대를 함께 본다.
	running, expected := RunningBuildGeneration(), FileBuildGeneration(result.Expected)
	result.ObservedGeneration, result.ExpectedGeneration = running.String(), expected.String()
	if !result.Stale && !SameGeneration(running, expected) {
		result.GenerationSkew = true
		result.RestartRequired = true
	}
	return result, nil
}

func NativeRuntimeDiagnosticMessage(diagnostic installcontract.NativeRuntimeDiagnostic, err error) (string, bool) {
	if err != nil {
		return fmt.Sprintf(
			"native hook runtime verification failed: observed=%s error=%v; reinstall hooks and restart the host session",
			diagnostic.Observed, err,
		), true
	}
	if diagnostic.Stale {
		return fmt.Sprintf(
			"cached native hook runtime is stale: observed=%s expected=%s; reinstall hooks and restart the host session",
			diagnostic.Observed, diagnostic.Expected,
		), true
	}
	if diagnostic.GenerationSkew {
		// 경로가 같으므로 재설치는 이미 끝났을 수 있다. 남은 것은 세션이
		// 이전 세대를 계속 쓰고 있다는 사실이며, 그 복구는 세션 재시작이다.
		return fmt.Sprintf(
			"native hook runtime is a different build than the installed binary: running=%s installed=%s at %s; "+
				"restart the host session so it loads the current generation before using new typed commands",
			diagnostic.ObservedGeneration, diagnostic.ExpectedGeneration, diagnostic.Expected,
		), true
	}
	return "", false
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
