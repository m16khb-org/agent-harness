# Claude-Mem Upstream Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Remove agentmemory as the configured upstream memory companion, install claude-mem latest for Claude Code and Codex, and verify the full issueops + memory-provider E2E path.

**Architecture:** `issueops` continues to own only shared CLI/MCP/hooks/install glue. Specialized memory remains upstream, but the upstream memory provider changes from `rohitg00/agentmemory` to `thedotmack/claude-mem`; hard deletion of existing local agentmemory state is an operator action, while the installer avoids reinstalling agentmemory and wires claude-mem through its upstream `npx claude-mem@latest install` entrypoint.

**Tech Stack:** Bash installer, Go tests, Codex/Claude plugin CLIs, `npx claude-mem@latest`, local worker runtime, existing `go test`/`go build` verification.

---

## File Structure

- Modify `scripts/install-native.sh`
  - Replace optional upstream memory setup from agentmemory to claude-mem.
  - Remove legacy agentmemory Codex/Claude plugin wiring when setting up upstream tools.
  - Use `npx -y claude-mem@latest install --ide <ide> --provider claude --runtime worker --no-auto-start` instead of global `npm install -g claude-mem`.
- Modify `internal/core/hook_prompt.go`
  - Change the memory routing hint from agentmemory to claude-mem.
- Modify `internal/core/hook_prompt_test.go`
  - Assert memory prompts recommend claude-mem instead of agentmemory.
- Modify `internal/adapter/install_contract_matrix_test.go`
  - Add a static installer contract test proving upstream setup invokes claude-mem and does not reinstall agentmemory.
- Modify docs: `AGENTS.md`, `CLAUDE.md`, `.issueops/TESTING.md`, `README.md`
  - Update upstream companion wording and smoke commands from agentmemory to claude-mem.

---

### Task 1: Lock the new memory-provider contract with failing tests

**Files:**
- Modify: `internal/core/hook_prompt_test.go`
- Modify: `internal/adapter/install_contract_matrix_test.go`

- [x] **Step 1: Write the failing hook hint test**

Change `TestBuildUserPromptMCPHintsRoutesMemoryToAgentmemory` to expect claude-mem:

```go
func TestBuildUserPromptMCPHintsRoutesMemoryToClaudeMem(t *testing.T) {
	got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "지난번에 이미 해결한 memory 찾아줘"})
	if !strings.Contains(got.AdditionalContext, "memory: use claude-mem only for previous-session/repeated-work recall") {
		t.Fatalf("expected claude-mem secondary hint:\n%s", got.AdditionalContext)
	}
	if strings.Contains(got.AdditionalContext, "agentmemory") {
		t.Fatalf("memory hint should not reference agentmemory after upstream migration:\n%s", got.AdditionalContext)
	}
}
```

- [x] **Step 2: Write the failing installer contract test**

Add this test to `internal/adapter/install_contract_matrix_test.go`:

```go
func TestInstallNativeUpstreamToolsUseClaudeMem(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	for _, want := range []string{
		"claude-mem",
		"npx -y claude-mem@latest install --ide \"$ide\" --provider claude --runtime worker --no-auto-start",
		"install_claude_mem_for_ide \"codex-cli\"",
		"install_claude_mem_for_ide \"claude-code\"",
		"ensure_codex_plugin \"claude-mem@claude-mem-local\"",
		"ensure_claude_plugin \"claude-mem@thedotmack\"",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install-native.sh missing %q", want)
		}
	}
	for _, gone := range []string{
		"npm install -g @agentmemory/agentmemory",
		"ensure_agentmemory_cli",
		"refresh_agentmemory_host_wiring",
		"ensure_codex_marketplace \"agentmemory\"",
		"ensure_claude_marketplace \"agentmemory\"",
	} {
		if strings.Contains(script, gone) {
			t.Fatalf("install-native.sh should not keep agentmemory upstream setup %q", gone)
		}
	}
}
```

- [x] **Step 3: Run tests to verify RED**

Run:

```bash
go test ./internal/core -run TestBuildUserPromptMCPHintsRoutesMemoryToClaudeMem -count=1
go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseClaudeMem -count=1
```

Expected: both fail because the current hook and installer still reference agentmemory.

---

### Task 2: Implement claude-mem upstream wiring

**Files:**
- Modify: `scripts/install-native.sh`
- Modify: `internal/core/hook_prompt.go`

- [x] **Step 1: Replace memory-provider helper functions**

In `scripts/install-native.sh`, remove `ensure_agentmemory_cli` and `refresh_agentmemory_host_wiring`. Add:

```bash
ensure_claude_mem_cli() {
  if ! command -v npm >/dev/null 2>&1; then
    log "npm not found; skipping claude-mem setup"
    return 0
  fi
  log "refreshing claude-mem upstream installer for Claude Code/Codex"
}

install_claude_mem_for_ide() {
  local ide="$1"
  if ! command -v npm >/dev/null 2>&1; then
    return 0
  fi
  log "installing claude-mem for ${ide}"
  npx -y claude-mem@latest install --ide "$ide" --provider claude --runtime worker --no-auto-start >/dev/null || log "warning: failed to install claude-mem for ${ide}; continuing"
}
```

- [x] **Step 2: Replace upstream setup calls**

In `install_upstream_tools`, change messages and setup flow:

```bash
log "setting up Codex plugins: llm-wiki, claude-mem"
remove_codex_plugin "agentmemory@agentmemory"
remove_codex_marketplace "agentmemory"
install_claude_mem_for_ide "codex-cli"
ensure_codex_plugin "claude-mem@claude-mem-local"

log "setting up Claude plugins: llm-wiki, claude-mem"
remove_claude_plugin "agentmemory@agentmemory"
remove_claude_marketplace "agentmemory"
install_claude_mem_for_ide "claude-code"
ensure_claude_plugin "claude-mem@thedotmack"
```

- [x] **Step 3: Update hook memory hint implementation**

In `internal/core/hook_prompt.go`, change the memory secondary tool from agentmemory to claude-mem:

```go
addPriority("claude-mem", "Secondary hint: consider claude-mem for previous-session memory or repeated-work questions.", hintPrioritySecondary)
```

and update helper branches so the compact hint becomes:

```go
memory: use claude-mem only for previous-session/repeated-work recall
```

- [x] **Step 4: Run tests to verify GREEN**

Run:

```bash
go test ./internal/core -run TestBuildUserPromptMCPHintsRoutesMemoryToClaudeMem -count=1
go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseClaudeMem -count=1
```

Expected: both pass.

---

### Task 3: Update docs and golden outputs

**Files:**
- Modify: `AGENTS.md`
- Modify: `CLAUDE.md`
- Modify: `.issueops/TESTING.md`
- Modify: `README.md`
- Modify: generated golden files if tests require it

- [x] **Step 1: Replace upstream companion wording**

Replace references to `agentmemory` as the default upstream memory provider with `claude-mem`. Keep historical mentions only when describing legacy removal.

- [x] **Step 2: Update smoke commands**

Use:

```bash
npx -y claude-mem@latest doctor
npx -y claude-mem@latest status
codex plugin list | grep -E 'wiki@llm-wiki|claude-mem@claude-mem-local'
claude plugin list | grep -E 'wiki@llm-wiki|claude-mem'
```

- [x] **Step 3: Run golden tests**

Run:

```bash
go test ./cmd/issueops -run Golden -count=1
```

If golden output changes only because the memory-provider text changed, update the golden through the repository’s established update command or targeted edit.

---

### Task 4: Hard-delete local agentmemory

**Files:**
- No repo files.
- User-level state/config under `$HOME`.

- [x] **Step 1: Stop running processes**

Run:

```bash
pkill -f 'agentmemory|@agentmemory|agentmemory-mcp|iii' || true
```

- [x] **Step 2: Remove Codex plugin/MCP/hooks/config**

Run:

```bash
codex plugin remove agentmemory@agentmemory || true
codex plugin marketplace remove agentmemory || true
python3 scripts/local-clean-agentmemory-codex.py
```

If no cleanup script exists, use a one-off Python script that removes `[plugins."agentmemory@agentmemory"]`, `[marketplaces.agentmemory]`, `[mcp_servers.agentmemory]`, `[mcp_servers.agentmemory.env]`, and `[hooks.state."agentmemory@agentmemory:..."]` blocks from `~/.codex/config.toml`, preserving all unrelated blocks.

- [x] **Step 3: Remove Claude plugin/MCP/config**

Run:

```bash
claude plugin uninstall agentmemory@agentmemory --keep-data=false -s user -y || true
claude plugin marketplace remove agentmemory --scope user || true
claude mcp remove agentmemory -s user || true
```

- [x] **Step 4: Remove package/cache/state files**

Run:

```bash
npm uninstall -g @agentmemory/agentmemory @agentmemory/mcp || true
rm -f ~/.local/bin/agentmemory
rm -rf ~/.agentmemory
rm -rf ~/.codex/plugins/cache/agentmemory
rm -rf ~/.codex/plugins/marketplaces/agentmemory
rm -rf ~/.claude/plugins/cache/agentmemory
rm -rf ~/.claude/plugins/marketplaces/agentmemory
```

- [x] **Step 5: Verify removal**

Run:

```bash
! command -v agentmemory
! pgrep -fl 'agentmemory|@agentmemory|agentmemory-mcp|iii'
! grep -R "agentmemory" ~/.codex/config.toml ~/.codex/hooks.json ~/.claude/settings.json 2>/dev/null
```

Expected: no agentmemory command, process, plugin cache, MCP config, or hook config remains.

---

### Task 5: Install claude-mem latest and verify E2E

**Files:**
- User-level Claude/Codex/claude-mem config.

- [x] **Step 1: Install latest claude-mem for both hosts**

Run:

```bash
npx -y claude-mem@latest install --ide claude-code --provider claude --runtime worker --no-auto-start
npx -y claude-mem@latest install --ide codex-cli --provider claude --runtime worker --no-auto-start
npx -y claude-mem@latest repair
npx -y claude-mem@latest start
```

- [x] **Step 2: Verify version/config**

Run:

```bash
npm view claude-mem version dist-tags.latest --json
npx -y claude-mem@latest version
claude plugin list | grep -E 'claude-mem'
codex plugin list | grep -E 'claude-mem@claude-mem-local'
npx -y claude-mem@latest status
npx -y claude-mem@latest doctor
```

- [x] **Step 3: Verify issueops E2E**

Run:

```bash
./scripts/install-native.sh --with-upstream-tools --dry-run
./scripts/install-native.sh --with-upstream-tools
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops inspect --json
./bin/issueops docs --json
./bin/issueops daemon status --json
./bin/issueops verify-work --json -- git status --short
```

Expected: issueops checks pass, claude-mem is latest and registered for both hosts, and agentmemory remains absent.

---

## Self-Review

- Spec coverage: The plan covers hard deletion, latest claude-mem setup, issueops upstream migration, TDD, and full E2E verification.
- Placeholder scan: No TBD/TODO/fill-in placeholders remain; one-off cleanup is specified with exact config blocks if no script exists.
- Type/signature consistency: Test names and exact expected strings match the implementation steps.
