/**
 * agent-harness GJC lifecycle hook shim.
 *
 * GJC's first-party HookAPI (`pi.on`) supports the full lifecycle event set, but
 * external CLI harnesses like agent-harness expose hooks as a shell CLI
 * (`agent-harness hook <event>`). This shim bridges the two: it subscribes to the
 * GJC events and spawns the matching agent-harness hook subcommand in each
 * handler.
 *
 * Event mapping (GJC -> agent-harness hook CLI):
 *   context              -> user-prompt      (UserPromptSubmit)
 *   session_start        -> session-start    (SessionStart)
 *   turn_end             -> stop             (Stop)
 *   auto_compaction_start-> pre-compact      (PreCompact)
 *   auto_compaction_end  -> post-compact     (PostCompact)
 *   tool_call            -> pre-tool-use     (PreToolUse)
 *   tool_result          -> post-tool-use    (PostToolUse)
 *
 * The handler is fire-and-forget: agent-harness hooks emit JSON on stdout used
 * for routing/audit, but this shim does not parse it to block/modify GJC events
 * yet. A missing `agent-harness` binary is swallowed so it never breaks the GJC
 * session. The shim resolves `agent-harness` from PATH (~/.local/bin shim ->
 * repo build), so `go build` rebuilds are picked up without reinstalling.
 *
 * Install: drop this file at ~/.gjc/agent/hooks/agent-harness.ts (GJC's
 * discoverAndLoadHooks loads it via native Bun import).
 */
import type { HookAPI } from "@gajae-code/coding-agent";
import { spawn } from "node:child_process";

function runAgentHarnessHook(subcommand: string): void {
	const child = spawn("agent-harness", ["hook", subcommand, "--json"], {
		stdio: ["ignore", "pipe", "pipe"],
	});
	child.on("error", () => {
		// Swallow: a missing/failed agent-harness must not break the GJC session.
	});
}

export default function (pi: HookAPI): void {
	pi.on("context", async () => {
		runAgentHarnessHook("user-prompt");
		return undefined;
	});

	pi.on("session_start", async () => {
		runAgentHarnessHook("session-start");
		return undefined;
	});

	pi.on("turn_end", async () => {
		runAgentHarnessHook("stop");
		return undefined;
	});

	pi.on("auto_compaction_start", async () => {
		runAgentHarnessHook("pre-compact");
		return undefined;
	});

	pi.on("auto_compaction_end", async () => {
		runAgentHarnessHook("post-compact");
		return undefined;
	});

	pi.on("tool_call", async () => {
		runAgentHarnessHook("pre-tool-use");
		return undefined;
	});

	pi.on("tool_result", async () => {
		runAgentHarnessHook("post-tool-use");
		return undefined;
	});
}
