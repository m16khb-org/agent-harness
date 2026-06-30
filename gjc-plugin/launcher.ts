#!/usr/bin/env bun
// GJC plugin bundle launcher for the agent-harness MCP server.
//
// The GJC plugin manifest schema does not carry an `env` field, so this launcher
// injects HARNESS_MCP_DIRECT=1 (direct stdio mode avoids daemon-proxy startup
// races with GJC's stdio client). The binary is resolved from PATH
// (~/.local/bin/agent-harness -> the repo's current build), and cwd is set to the
// agent-harness repo so HARNESS_ROOT is auto-detected by the binary. This keeps
// the plugin live across `go build` rebuilds without re-installing the plugin.
import { spawn } from "node:child_process";

// Resolve the agent-harness repo root from the PATH shim
// (~/.local/bin/agent-harness -> <repo>/bin/agent-harness), so the launcher is
// machine-independent: the copied bundle inside ~/.gjc/agent/gjc-plugins/ stays
// correct across workstations and `go build` rebuilds without re-installing.
// AGENT_HARNESS_ROOT env wins when set; cwd is the final fallback.
import { readlinkSync } from "node:fs";
import os from "node:os";
import path from "node:path";

function resolveHarnessRoot(): string {
	if (process.env.AGENT_HARNESS_ROOT) return process.env.AGENT_HARNESS_ROOT;
	const shim = path.join(os.homedir(), ".local", "bin", "agent-harness");
	try {
		// <repo>/bin/agent-harness -> <repo>
		const target = readlinkSync(shim);
		return path.dirname(path.dirname(target));
	} catch {
		return process.cwd();
	}
}

const AGENT_HARNESS_ROOT = resolveHarnessRoot();

const child = spawn("agent-harness", ["mcp"], {
	cwd: AGENT_HARNESS_ROOT,
	env: { ...process.env, HARNESS_MCP_DIRECT: "1" },
	stdio: ["inherit", "inherit", "inherit"],
});

child.on("error", err => {
	console.error(String(err));
	process.exit(1);
});
child.on("exit", code => process.exit(code ?? 0));
