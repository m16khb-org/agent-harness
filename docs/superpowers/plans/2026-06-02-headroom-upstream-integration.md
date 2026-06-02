# Headroom Upstream Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Headroom as an explicit opt-in upstream companion tool alongside LLM Wiki, CodeGraph, and claude-mem.

**Architecture:** Keep Headroom outside harness core. The installer may install or update the upstream CLI/package during `--with-upstream-tools`, but it must not enable proxying, wrapping, learning, hooks, or repo-local config by default.

**Tech Stack:** Bash installer, Go static installer-contract tests, project docs, existing `go test` and dry-run bootstrap checks.

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

---

### Task 1: Lock Headroom upstream contract with a failing test

**Files:**
- Modify: `internal/adapter/install_contract_matrix_test.go`

- [ ] **Step 1: Add static contract test**

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

- [ ] **Step 2: Verify RED**

Run:

```bash
go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseHeadroom -count=1
```

Expected: FAIL because Headroom is not yet wired.

---

### Task 2: Implement optional Headroom installer wiring

**Files:**
- Modify: `scripts/install-native.sh`

- [ ] **Step 1: Add helper**

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

- [ ] **Step 2: Call helper from explicit upstream path**

At the end of `install_upstream_tools`, after CodeGraph setup, call:

```bash
install_headroom_cli
```

- [ ] **Step 3: Update dry-run wording**

Change the dry-run log to mention Headroom:

```bash
log "dry-run: would install/update upstream tools: llm-wiki, codegraph, claude-mem, Headroom; would remove legacy agentmemory plugin wiring"
```

---

### Task 3: Update docs and smoke checks

**Files:**
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/TESTING.md`

- [ ] **Step 1: Add Headroom to upstream dependency table**

Add a row:

```markdown
| Headroom | `chopratejas/headroom` / `headroom-ai` | `pipx install --python python3.13 "headroom-ai[all]"`로 CLI를 설치/갱신한다. 자동 proxy/wrap/learn은 실행하지 않는다. |
```

- [ ] **Step 2: Add safety note**

Document that Headroom must remain explicit opt-in and that operators should use `HEADROOM_TELEMETRY=off` for experiments.

- [ ] **Step 3: Add smoke checks**

Add:

```bash
command -v headroom
HEADROOM_TELEMETRY=off headroom --help
```

---

### Task 4: Verify

**Files:**
- No new files.

- [ ] **Step 1: Run targeted test**

Run:

```bash
go test ./internal/adapter -run TestInstallNativeUpstreamToolsUseHeadroom -count=1
```

Expected: PASS.

- [ ] **Step 2: Run dry-run installer smoke**

Run:

```bash
./scripts/install-native.sh --with-upstream-tools --dry-run
```

Expected: output mentions Headroom and does not perform installation.

- [ ] **Step 3: Run bootstrap dry-run smoke**

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

Expected: both pass.
