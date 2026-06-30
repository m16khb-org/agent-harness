#!/usr/bin/env bun
// GJC plugin bundle launcher for the agent-harness MCP server.
//
// The GJC plugin manifest schema does not carry an `env` field, so this launcher
// injects HARNESS_MCP_DIRECT=1 (direct stdio mode avoids daemon-proxy startup
// races with GJC's stdio client). The binary is resolved from PATH
// (~/.local/bin/agent-harness -> the repo's current build), and cwd is set to the
// agent-harness repo so HARNESS_ROOT is auto-detected by the binary. This keeps
// the plugin live across `go build` rebuilds without re-installing the plugin.
//
// AGENT_HARNESS_ROOT is machine-local by design: this is a personal harness on a
// single workstation. Update it if the repo moves.
import { spawn } from "node:child_process";

const AGENT_HARNESS_ROOT = "/Users/habin/workspace/agent-harness";

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
