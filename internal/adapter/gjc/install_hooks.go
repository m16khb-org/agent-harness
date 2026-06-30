package gjc

import (
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

// hookShimRelPath is the in-repo TypeScript shim source that bridges GJC's
// first-party HookAPI (pi.on) to the agent-harness hook CLI. It lives next to
// the GJC plugin bundle launcher under gjc-plugin/.
const hookShimRelPath = "gjc-plugin/hook.ts"

// writeGJCHookShim copies the TS hook shim from the repo into GJC's first-party
// hooks directory so GJC's discoverAndLoadHooks loads it via native Bun import.
// GJC's HookAPI supports the full lifecycle event set (context, session_start,
// turn_end, auto_compaction_*, tool_call, tool_result); the shim subscribes to
// each and spawns the matching `agent-harness hook <event>` subcommand.
//
// This replaces an earlier pre/post-tool shell-script approach: GJC's
// shell-script discovery surface is limited to tool pre/post, while the TS
// HookAPI covers all seven agent-harness lifecycle hooks.
func writeGJCHookShim(hooksDir, repoRoot string, req port.NativeInstallRequest) (port.InstallFile, error) {
	src := filepath.Join(repoRoot, hookShimRelPath)
	content, err := os.ReadFile(src)
	if err != nil {
		return port.InstallFile{Path: src, Kind: "gjc_hook_shim_source_missing"},
			fmt.Errorf("GJC hook shim source not found at %s: %w; run install from the agent-harness repo root", src, err)
	}
	dest := filepath.Join(hooksDir, "agent-harness.ts")
	return installutil.WriteTextPlan(dest, "gjc_hook_shim", string(content), 0o644, req.DryRun)
}
