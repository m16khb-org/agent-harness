# Headroom Agent-Harness Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Headroom token-saving activation explicit, durable, reproducible, and compatible with existing agent-harness Codex and Claude Code hooks.

**Architecture:** Headroom remains an upstream companion and is not reimplemented in agent-harness core. The repo provides an opt-in setup script and installer flag that configure both Codex and Claude Code, then merge existing agent-harness lifecycle hooks/settings back into user config. Runtime savings require a healthy Headroom proxy on `127.0.0.1:8787`.

**Tech Stack:** Go CLI/tests, Markdown project docs, Codex user config, Headroom CLI `0.22.4`, `pipx`, Homebrew Python 3.13.

---

### Task 1: Preserve The IssueOps Worktree Contract

**Files:**
- Verify only: repository checkout and worktree state

- [x] **Step 1: Verify worktree identity**

Run:

```bash
pwd
git branch --show-current
git rev-parse --short HEAD
git status --short
```

Expected:

```text
/tmp/agent-harness.worktrees/feature-13-headroom-agent-harness-integration
feature/13-headroom-agent-harness-integration
7f015f7
 M .agent-harness/ADR.md
 M .agent-harness/CAUTIONS.md
```

- [x] **Step 2: Verify baseline Headroom contract test**

Run:

```bash
go test ./internal/adapter -run 'TestInstallNativeUpstreamToolsUseHeadroom' -count=1
```

Expected:

```text
ok  	agent-harness/internal/adapter
```

### Task 2: Document Headroom Runtime Activation Without Reimplementing It

**Files:**
- Modify: `.agent-harness/ADR.md`
- Modify: `.agent-harness/CAUTIONS.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `scripts/install-native.sh`
- Create: `scripts/setup-headroom-runtime.sh`
- Modify: `internal/adapter/install_contract_matrix_test.go`

- [x] **Step 1: Update the ADR**

Change the 2026-06-03 Headroom ADR consequence from “manual proxy or wrap only” to include the durable opt-in route:

```markdown
- Consequences: Operators who want Headroom runtime savings must first install prerequisites such as pipx, install `headroom-ai[all]`, then choose an explicit runtime path. One-shot use can run `HEADROOM_TELEMETRY=off headroom wrap codex`. Durable Codex use can run `HEADROOM_TELEMETRY=off headroom init -g codex`, preserve any existing `agent-harness` hooks in `~/.codex/hooks.json`, and keep `HEADROOM_TELEMETRY=off headroom install start --profile init-user` healthy. The agent-harness default installer still must not auto-enable proxy, wrap, learn, or MCP routing.
```

- [x] **Step 2: Update the caution**

Append a concrete warning that `headroom init -g codex` can rewrite user Codex hook config:

```markdown
## 2026-06-03 — Headroom Codex init can overwrite existing hooks

- Kind: `caution`
- Source: codex-cli
- Summary: `headroom init -g codex` may rewrite `~/.codex/hooks.json`; preserve or merge existing `agent-harness` lifecycle hooks before restarting Codex.
- Evidence:
  - Before Headroom init, `~/.codex/hooks.json` contained `agent-harness hook post-compact`, `post-tool-use`, `pre-compact`, `pre-tool-use`, `session-start`, `stop`, and `user-prompt`.
  - After Headroom init, the file contained only Headroom `SessionStart` and `PreToolUse` hook entries until manually merged.
  - `python3 -m json.tool ~/.codex/hooks.json` verified the merged hook file is valid JSON.
- Resolution: Back up `~/.codex/config.toml` and `~/.codex/hooks.json` before running Headroom init, then verify both Headroom and agent-harness hook entries remain present with `rg -n "headroom|agent-harness" ~/.codex/hooks.json`.
```

- [x] **Step 3: Add a reproducible runtime setup path**

Add `scripts/setup-headroom-runtime.sh` and wire it into `scripts/install-native.sh --enable-headroom-runtime`.

Expected behavior:

```text
scripts/install-native.sh --with-upstream-tools --enable-headroom-runtime
```

sets up both Codex and Claude Code, preserves `agent-harness` hooks/settings, starts Headroom profile `init-user`, and verifies `http://127.0.0.1:8787/health`.

- [x] **Step 4: Update operations**

In `.agent-harness/OPERATIONS.md`, expand the Headroom paragraph with the durable opt-in command sequence:

~~~markdown
Durable Codex token-saving setup is explicit operator action:

```bash
scripts/install-native.sh --with-upstream-tools --enable-headroom-runtime
```

If Headroom init rewrites host config, use the setup script's merge logic and verify both Headroom and `agent-harness` entries remain before restarting Codex or Claude Code.
~~~

### Task 3: Activate And Verify Local Headroom Runtime

**Files:**
- Modify outside repo: `~/.codex/config.toml`
- Modify outside repo: `~/.codex/hooks.json`
- Modify outside repo: `~/.claude/settings.json`
- Modify outside repo: `~/.claude.json`
- Runtime state outside repo: Headroom persistent deployment profile `init-user`

- [x] **Step 1: Verify merged hook config**

Run:

```bash
python3 -m json.tool ~/.codex/hooks.json >/dev/null
rg -n "headroom|agent-harness" ~/.codex/hooks.json
```

Expected: JSON parses and both Headroom hook ensure commands and agent-harness lifecycle hook commands are present for Codex and Claude Code.

- [x] **Step 2: Start Headroom persistent proxy**

Run:

```bash
HEADROOM_TELEMETRY=off headroom install start --profile init-user
```

Expected: command exits 0 or reports a readiness timeout while the process continues warming up. If readiness times out, inspect logs and re-run status before changing configuration.

- [x] **Step 3: Verify proxy health**

Run:

```bash
HEADROOM_TELEMETRY=off headroom install status --profile init-user
ps aux | rg -i '[h]eadroom|HEADROOM'
```

Expected: status reports `Status: running` and `Healthy: yes`; process list shows a Headroom proxy/runtime process that is not only the status command.

### Task 4: Final Verification

**Files:**
- Verify only: changed docs and user-level runtime configuration

- [x] **Step 1: Verify changed repo contract**

Run:

```bash
go test ./internal/adapter -run 'TestInstallNativeUpstreamToolsUseHeadroom' -count=1
```

Expected:

```text
ok  	agent-harness/internal/adapter
```

- [x] **Step 2: Verify IssueOps readiness inputs**

Run:

```bash
agent-harness issueops pr-readiness --id io-c8856b8c0340 --json
```

Expected: reports linked `issue_url` and this plan path after `link-plan`.

- [ ] **Step 3: Commit**

Run:

```bash
git add .agent-harness/ADR.md .agent-harness/CAUTIONS.md .agent-harness/OPERATIONS.md docs/superpowers/plans/2026-06-03-headroom-agent-harness-integration.md
git commit -m "docs: document Headroom runtime integration"
```

Expected: one focused docs commit on `feature/13-headroom-agent-harness-integration`.
