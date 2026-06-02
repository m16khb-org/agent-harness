# Headroom Upstream Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Headroom as an explicit opt-in upstream companion tool alongside LLM Wiki, CodeGraph, and claude-mem.

**Architecture:** Keep Headroom outside harness core. The installer may install or update the upstream CLI/package during `--with-upstream-tools`, but it must not enable proxying, wrapping, learning, hooks, or repo-local config by default.

**Tech Stack:** Bash installer, Go static installer-contract tests, project docs, existing `go test` and dry-run bootstrap checks.

**Issue:** https://github.com/m16khb/agent-harness/issues/2

**Branch / Worktree:** `feature/2-integrate-headroom-upstream-companion` at `/Users/m16khb/Workspace/agent-harness.worktrees/feature-2-integrate-headroom-upstream-companion`

**Baseline note:** `go test ./... -count=1` currently fails before new edits in `cmd/harness` `TestResponseContractsGolden` because `response_contracts.golden.json` is stale. Do not attribute that failure to Headroom unless a focused Headroom check also fails.

---

## File Structure

- Modify `internal/adapter/install_contract_matrix_test.go`
  - Add a static contract test for Headroom optional upstream setup.
- Modify `scripts/install-native.sh`
  - Add a small helper that installs or updates `headroom-ai` through `pipx` when explicitly syncing upstream tools.
  - Update dry-run wording.
- Modify `.agent-harness/OPERATIONS.md`
  - Add Headroom to the upstream dependency table and note opt-in behavior.
- Modify `.agent-harness/TESTING.md`
  - Add Headroom smoke checks to optional upstream companion verification.
- Modify `README.md`
  - Keep the public English and Korean upstream companion tables consistent with project docs.

---

### Task 1: Confirm Headroom upstream contract test

**Files:**
- Modify: `internal/adapter/install_contract_matrix_test.go`

- [x] **Step 1: Add static contract test**

Add `TestInstallNativeUpstreamToolsUseHeadroom`:

```go
func TestInstallNativeUpstreamToolsUseHeadroom(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	for _, want := range []string{
		"install_headroom_cli",
		"headroom-ai[all]",
		"pipx install --python python3.13 \"headroom-ai[all]\"",
		"pipx upgrade headroom-ai",
		"HEADROOM_TELEMETRY=off",
		"Headroom",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install-native.sh missing Headroom upstream contract %q", want)
		}
	}
	for _, gone := range []string{
		"headroom wrap codex",
		"headroom wrap claude",
		"headroom proxy --port",
		"headroom learn",
	} {
		if strings.Contains(script, gone) {
			t.Fatalf("install-native.sh must not auto-enable Headroom runtime behavior %q", gone)
		}
	}
}
```

- [x] **Step 2: Verify contract test**

Run:

```bash
go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseHeadroom -count=1
```

Expected: PASS once Headroom installer wiring is present.

---

### Task 2: Implement optional Headroom installer wiring

**Files:**
- Modify: `scripts/install-native.sh`

- [x] **Step 1: Add helper**

Add a helper near the upstream setup helpers:

```bash
install_headroom_cli() {
  if ! command -v pipx >/dev/null 2>&1; then
    log "pipx not found; skipping Headroom setup"
    return 0
  fi
  log "installing/updating Headroom with telemetry opt-out guidance: HEADROOM_TELEMETRY=off"
  if command -v headroom >/dev/null 2>&1; then
    pipx upgrade headroom-ai >/dev/null || log "warning: failed to upgrade Headroom; continuing"
  else
    pipx install --python python3.13 "headroom-ai[all]" >/dev/null || log "warning: failed to install Headroom; continuing"
  fi
}
```

- [x] **Step 2: Call helper from explicit upstream path**

At the end of `install_upstream_tools`, after CodeGraph setup, call:

```bash
install_headroom_cli
```

- [x] **Step 3: Update dry-run wording**

Change the dry-run log to mention Headroom:

```bash
log "dry-run: would install/update upstream tools: llm-wiki, codegraph, claude-mem, Headroom; would remove legacy agentmemory plugin wiring"
```

---

### Task 3: Update project docs and smoke checks

**Files:**
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/TESTING.md`

- [x] **Step 1: Add Headroom to upstream dependency table**

Add a row:

```markdown
| Headroom | `chopratejas/headroom` / `headroom-ai` | `pipx install --python python3.13 "headroom-ai[all]"`로 CLI를 설치/갱신한다. 자동 proxy/wrap/learn은 실행하지 않는다. |
```

- [x] **Step 2: Add safety note**

Document that Headroom must remain explicit opt-in and that operators should use `HEADROOM_TELEMETRY=off` for experiments.

- [x] **Step 3: Add smoke checks**

Add:

```bash
command -v headroom
HEADROOM_TELEMETRY=off headroom --help
```

---

### Task 4: Update README upstream tables

**Files:**
- Modify: `README.md`

- [x] **Step 1: Add English Headroom row**

Add this row after the claude-mem row in the English upstream tools table:

```markdown
| Headroom | `chopratejas/headroom` / `headroom-ai` | Installs or upgrades the CLI with `pipx install --python python3.13 "headroom-ai[all]"`; it does not automatically run proxy, wrap, learn, or MCP setup. |
```

- [x] **Step 2: Add Korean Headroom row**

Add this row after the claude-mem row in the Korean upstream tools table:

```markdown
| Headroom | `chopratejas/headroom` / `headroom-ai` | `pipx install --python python3.13 "headroom-ai[all]"`로 CLI를 설치/갱신하며, proxy/wrap/learn/MCP 설정은 자동 실행하지 않습니다. |
```

- [x] **Step 3: Add telemetry and manual-use notes**

After each table, add a short note that Headroom runtime use remains manual and experiments can set `HEADROOM_TELEMETRY=off`.

---

### Task 5: Verify

**Files:**
- No new files.

- [x] **Step 1: Run targeted test**

Run:

```bash
go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseHeadroom -count=1
```

Expected: PASS.

- [x] **Step 2: Run dry-run installer smoke**

Run:

```bash
./scripts/install-native.sh --with-upstream-tools --dry-run
```

Expected: output mentions Headroom and does not perform installation.

- [x] **Step 3: Run bootstrap dry-run smoke**

Run:

```bash
./bin/agent-harness bootstrap --sync --dry-run
```

Expected: output mentions Headroom through upstream dry-run path.

- [ ] **Step 4: Run final tests**

Run:

```bash
go test ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

Expected: `go build` passes. `go test ./... -count=1` and `go test ./cmd/harness -run Golden -count=1` are currently blocked by the pre-existing `TestResponseContractsGolden` mismatch recorded in the baseline note above.
