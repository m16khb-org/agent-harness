#!/usr/bin/env bun

import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

const hookArg = process.argv[2];
if (!hookArg) {
	throw new Error("usage: bun scripts/smoke-gjc-native-hook.ts <hook.ts>");
}

const hookPath = resolve(hookArg);
const mod = await import(`${pathToFileURL(hookPath).href}?smoke=${Date.now()}`);
if (typeof mod.registerAgentHarnessHooks !== "function") {
	throw new Error(`GJC hook does not export registerAgentHarnessHooks: ${hookPath}`);
}

type HookCall = {
	subcommand: string;
	payload: Record<string, unknown>;
	enforce: boolean;
};

const handlers = new Map<string, (event: Record<string, unknown>, ctx: unknown) => Promise<unknown>>();
const messages: Array<Record<string, unknown>> = [];
const calls: HookCall[] = [];
const pi = {
	on: (event: string, handler: (event: Record<string, unknown>, ctx: unknown) => Promise<unknown>) =>
		handlers.set(event, handler),
	sendMessage: (message: Record<string, unknown>) => messages.push(message),
};

mod.registerAgentHarnessHooks(
	pi,
	async (subcommand: string, payload: Record<string, unknown>, enforce: boolean) => {
		calls.push({ subcommand, payload, enforce });
		if (subcommand === "session-start") {
			return { hookSpecificOutput: { additionalContext: "claim-guidance" } };
		}
		if (subcommand === "pre-tool-use") {
			return { decision: "block", reason: "owned-by-other-session" };
		}
		return {};
	},
);

const ctx = {
	cwd: "/repo.worktrees/16-demo",
	sessionManager: { getSessionId: () => "gjc-session-1" },
};
const sessionStart = handlers.get("session_start");
const toolCall = handlers.get("tool_call");
if (!sessionStart || !toolCall) {
	throw new Error("GJC hook did not register session_start and tool_call handlers");
}

await sessionStart({ type: "session_start" }, ctx);
const blocked = (await toolCall(
	{ type: "tool_call", toolName: "edit", toolCallId: "call-1", input: { path: "x.go" } },
	ctx,
)) as { block?: boolean; reason?: string } | undefined;

if (!blocked?.block || blocked.reason !== "owned-by-other-session") {
	throw new Error(`wrong GJC block shape: ${JSON.stringify(blocked)}`);
}
if (!messages.some((message) => String(message.content).includes("claim-guidance"))) {
	throw new Error("GJC session guidance was not relayed");
}
if (handlers.has("context")) {
	throw new Error("GJC context must not be mapped as every user prompt");
}

const tool = calls.find((call) => call.subcommand === "pre-tool-use");
const input = tool?.payload.tool_input as Record<string, unknown> | undefined;
if (
	!tool?.enforce ||
	tool.payload.host !== "gjc" ||
	tool.payload.session_id !== "gjc-session-1" ||
	tool.payload.cwd !== ctx.cwd ||
	tool.payload.tool_name !== "edit" ||
	input?.path !== "x.go"
) {
	throw new Error(`GJC native payload missing: ${JSON.stringify(tool)}`);
}

console.log(
	JSON.stringify({
		ok: true,
		hook_path: hookPath,
		host: tool.payload.host,
		session_id: tool.payload.session_id,
		cwd: tool.payload.cwd,
		blocked: blocked.block,
		reason: blocked.reason,
	}),
);
