package hookcli

import (
	hookfailurecontract "agent-harness/internal/contract/hookfailure"
	hookmetricscontract "agent-harness/internal/contract/hookmetrics"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	coreinstall "agent-harness/internal/adapter/install"
)

var ResolveTarget = func(arg string) string {
	if arg == "" {
		if env := os.Getenv("CLAUDE_PROJECT_DIR"); env != "" {
			arg = env
		} else if env := os.Getenv("PWD"); env != "" {
			arg = env
		} else if cwd, err := os.Getwd(); err == nil {
			arg = cwd
		} else {
			arg = "."
		}
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return arg
	}
	return abs
}

var DiagnoseCurrentNativeRuntime = func() (coreinstall.NativeRuntimeDiagnostic, error) {
	executable, err := os.Executable()
	if err != nil {
		return coreinstall.NativeRuntimeDiagnostic{}, err
	}
	return coreinstall.DiagnoseNativeRuntime(executable)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// hook metric/failure 로그 연산은 composition root가 설치한다. hookcatalog에
// 넘기는 Config도 여기서 조립한다.
var (
	RecordHookMetricEvent func(hookmetricscontract.HookMetricEvent) (hookmetricscontract.HookMetricRecordResult, error)
	PruneHookFailureLog   func(maxAge time.Duration) (hookfailurecontract.HookFailurePruneResult, error)
	PruneHookMetricsLog   func(maxAge time.Duration) (hookmetricscontract.HookMetricsPruneResult, error)
)
