/**
 * Thin bridge from GJC 0.7.8's first-party HookAPI to agent-harness.
 * Native identity comes from HookContext; no environment or transcript data is
 * forwarded. PreToolUse is fail-closed, while advisory lifecycle relays stay
 * non-fatal when the harness binary is absent.
 */
import type { HookAPI } from "@gajae-code/coding-agent";
import { spawn } from "node:child_process";

type HarnessPayload = Record<string, unknown>;
type HarnessResult = Record<string, unknown> | undefined;
export type HarnessHookRunner = (
	subcommand: string,
	payload: HarnessPayload,
	enforce: boolean,
) => Promise<HarnessResult>;

const MAX_OUTPUT_BYTES = 1024 * 1024;
const HOOK_TIMEOUT_MS = 30_000;

function hookArgs(subcommand: string): string[] {
	const args = ["hook", subcommand];
	switch (subcommand) {
		case "pre-tool-use":
			args.push(
				"--host",
				"gjc",
				"--enforce-worktree",
				"--enforce-korean-remote-artifacts",
				"--enforce-vcs-issue-linking",
				"--enforce-staged-checks",
				"--enforce-gitops-kubectl",
			);
			break;
		case "session-start":
		case "user-prompt":
		case "post-tool-use":
		case "post-compact":
			args.push("--host", "gjc");
			break;
		case "stop":
			args.push(
				"--host",
				"gjc",
				"--enforce-numbered-next-actions",
				"--enforce-engelbart-canvas-sections",
				"--relay-next-action-judgement",
			);
	}
	return args;
}

export const runAgentHarnessHook: HarnessHookRunner = async (subcommand, payload, enforce) =>
	new Promise((resolve, reject) => {
		const child = spawn("agent-harness", hookArgs(subcommand), {
			cwd: typeof payload.cwd === "string" ? payload.cwd : undefined,
			stdio: ["pipe", "pipe", "pipe"],
		});
		let stdout = "";
		let stderr = "";
		let settled = false;
		const finish = (result?: HarnessResult, error?: Error) => {
			if (settled) return;
			settled = true;
			clearTimeout(timer);
			if (error && enforce) reject(error);
			else resolve(result);
		};
		const append = (current: string, chunk: unknown): string =>
			(current + String(chunk)).slice(0, MAX_OUTPUT_BYTES);
		child.stdout.on("data", (chunk) => {
			stdout = append(stdout, chunk);
		});
		child.stderr.on("data", (chunk) => {
			stderr = append(stderr, chunk);
		});
		child.on("error", (error) => finish(undefined, error));
		child.on("close", (code) => {
			if (code !== 0) {
				finish(undefined, new Error(`agent-harness hook ${subcommand} failed (${code}): ${stderr}`));
				return;
			}
			if (stdout.trim() === "") {
				finish({});
				return;
			}
			try {
				finish(JSON.parse(stdout) as Record<string, unknown>);
			} catch (error) {
				finish(undefined, error instanceof Error ? error : new Error(String(error)));
			}
		});
		const timer = setTimeout(() => {
			child.kill("SIGTERM");
			finish(undefined, new Error(`agent-harness hook ${subcommand} timed out`));
		}, HOOK_TIMEOUT_MS);
		child.stdin.end(JSON.stringify(payload));
	});

function commonPayload(ctx: { cwd: string; sessionManager: { getSessionId(): string } }): HarnessPayload {
	return {
		host: "gjc",
		session_id: ctx.sessionManager.getSessionId(),
		cwd: ctx.cwd,
		agent_type: "gjc",
	};
}

function contextFromResult(result: HarnessResult): string {
	if (!result) return "";
	if (typeof result.systemMessage === "string") return result.systemMessage;
	const specific = result.hookSpecificOutput;
	if (specific && typeof specific === "object") {
		const context = (specific as Record<string, unknown>).additionalContext;
		if (typeof context === "string") return context;
	}
	return "";
}

export function registerAgentHarnessHooks(pi: HookAPI, run: HarnessHookRunner = runAgentHarnessHook): void {
	pi.on("before_agent_start", async (event, ctx) => {
		const result = await run("user-prompt", { ...commonPayload(ctx), prompt: event.prompt }, false);
		const context = contextFromResult(result);
		if (!context) return undefined;
		return {
			message: {
				customType: "agent-harness-user-prompt",
				content: context,
				display: true,
				attribution: "agent",
			},
		};
	});

	pi.on("session_start", async (event, ctx) => {
		const result = await run("session-start", { ...commonPayload(ctx), source: event.type }, false);
		const context = contextFromResult(result);
		if (context) {
			pi.sendMessage({
				customType: "agent-harness-session-start",
				content: context,
				display: true,
				attribution: "agent",
			});
		}
	});

	pi.on("tool_call", async (event, ctx) => {
		try {
			const result = await run(
				"pre-tool-use",
				{
					...commonPayload(ctx),
					tool_name: event.toolName,
					tool_call_id: event.toolCallId,
					tool_input: event.input,
				},
				true,
			);
			if (result?.decision === "block" || result?.block === true) {
				return { block: true, reason: String(result.reason || "blocked by agent-harness") };
			}
			return undefined;
		} catch {
			return { block: true, reason: "agent-harness PreToolUse enforcement unavailable" };
		}
	});

	pi.on("turn_end", async (event, ctx) => {
		await run("stop", { ...commonPayload(ctx), source: event.type }, false);
	});
	pi.on("auto_compaction_start", async (event, ctx) => {
		await run("pre-compact", { ...commonPayload(ctx), source: event.type }, false);
	});
	pi.on("auto_compaction_end", async (event, ctx) => {
		await run("post-compact", { ...commonPayload(ctx), source: event.type }, false);
	});
	pi.on("tool_result", async (event, ctx) => {
		await run(
			"post-tool-use",
			{
				...commonPayload(ctx),
				tool_name: event.toolName,
				tool_call_id: event.toolCallId,
				tool_input: event.input,
				is_error: event.isError === true,
			},
			false,
		);
	});
}

export default function (pi: HookAPI): void {
	registerAgentHarnessHooks(pi);
}
