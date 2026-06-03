# Install Command TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the public `install-native` UX with a first-run `./install.sh` and `agent-harness install` flow that explains and applies PATH, Codex, Claude, MCP, and optional upstream tool settings without requiring manual environment exports.

**Architecture:** Keep install policy in the existing Go install path and host adapters. Use `./install.sh` only as the binary-missing first-run entrypoint, then delegate to `bin/agent-harness install`. Keep `bootstrap`/`update` as refresh surfaces that reuse the same installer instead of duplicating host config logic in shell.

**Tech Stack:** Go standard library CLI in `cmd/harness`, existing install core in `internal/core` and `internal/adapter/{codex,claude}`, POSIX shell for `./install.sh`, existing golden and adapter contract tests.

---

## File Map

- Create: `install.sh` — first-run entrypoint that works before `agent-harness` is on PATH.
- Modify: `scripts/install-native.sh` — either rename internally or turn into a thin compatibility wrapper that calls `install.sh` or `agent-harness install`.
- Modify: `cmd/harness/main.go` — dispatch `install`, remove or deprecate `install-native`, and preserve compatibility only if intentionally chosen.
- Modify: `cmd/harness/install_native.go` — rename user-facing command implementation to install, add interactive/dry-run flags, and keep host-neutral request construction.
- Modify: `cmd/harness/update_bootstrap.go` — make `bootstrap`/`update` call the unified install script or install command with explicit mode semantics.
- Modify: `internal/adapter/cli/usage.go` and `cmd/harness/testdata/usage.golden.txt` — update public usage text.
- Modify: `internal/adapter/install_contract_matrix_test.go` and `internal/adapter/testdata/native_install_contract_matrix.golden.json` only if install output contract changes.
- Modify: `.agent-harness/operations/install.md`, `.agent-harness/OPERATIONS.md`, `.agent-harness/TECH_STACK.md`, `.agent-harness/TESTING.md`, `README.md` — document first-run and installed-update paths.

## Domain Contract

Use these command meanings throughout the implementation:

```text
./install.sh
  First-run source checkout installer. It builds bin/agent-harness if needed and invokes bin/agent-harness install.

agent-harness install
  Public installer. It owns PATH shim setup, shell rc PATH guidance/write, Codex/Claude skills/hooks/MCP config, and HARNESS_ROOT injection into host configs.

agent-harness bootstrap
  Compatibility refresh command for user-level harness integration. It must not be documented as the first command a new user can run before the binary exists.

agent-harness update
  Already-installed checkout refresh. It rebuilds and refreshes host integration.
```

Environment rules:

```text
HARNESS_ROOT
  Installer writes this into Codex/Claude MCP config. Users should not need to export it manually for normal installation.

CODEX_HOME
  Installer reads it if already set. Otherwise default to ~/.codex and show that default in the plan/TUI.

PATH
  Installer may add ~/.local/bin to shell rc only after explaining the change and receiving the selected mode.
```

### Task 1: Lock Current Install Contract With Tests

**Files:**
- Modify: `cmd/harness/install_native.go`
- Modify: `cmd/harness/main_test.go` or create `cmd/harness/install_test.go`
- Test: `go test ./cmd/harness -run Install -count=1`

- [ ] **Step 1: Write a failing CLI test for the new command name**

Add a test that invokes the command dispatcher with `install --dry-run --json` or directly tests the command catalog once the local test style is confirmed. The assertion must prove the public command name is `install`, not only `install-native`.

Expected behavior:

```text
agent-harness install --dry-run --json
  exits 0
  returns the same native integration plan currently returned by install-native
```

- [ ] **Step 2: Run the targeted test and confirm it fails**

Run:

```bash
go test ./cmd/harness -run Install -count=1
```

Expected: FAIL because `install` is not yet a top-level command.

- [ ] **Step 3: Add `install` dispatch**

In `cmd/harness/main.go`, add `case "install":` that calls the renamed/shared install runner. Keep `install-native` only as a compatibility alias if the issue owner chooses a deprecation window.

The dispatch shape should be equivalent to:

```go
case "install":
	if err := runInstall(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		os.Exit(1)
	}
case "install-native":
	if err := runInstall(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "install-native:", err)
		os.Exit(1)
	}
```

- [ ] **Step 4: Rename the runner without changing behavior**

Rename `runInstallNative` to `runInstall`, and keep all current request construction intact:

```go
req := core.DefaultNativeInstallRequest(harnessRoot(), home, codexHome, filepath.Join(harnessRoot(), "bin", "agent-harness"))
req.ProjectLocal = *projectLocal
req.DryRun = *dryRun
```

- [ ] **Step 5: Verify the targeted command path**

Run:

```bash
go test ./cmd/harness -run Install -count=1
./bin/agent-harness install --dry-run --json
```

Expected: tests pass, and the dry-run JSON contains Codex/Claude install plan data.

### Task 2: Add First-Run `./install.sh`

**Files:**
- Create: `install.sh`
- Modify: `scripts/install-native.sh`
- Test: `./install.sh --dry-run`

- [ ] **Step 1: Write the first-run script**

Create `install.sh` as a thin entrypoint. It must compute repo root from its own path, build `bin/agent-harness` unless `--skip-build` is provided, and call `bin/agent-harness install`.

The implementation should follow this shape:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$ROOT/bin/agent-harness"
SKIP_BUILD=0
ARGS=()

for arg in "$@"; do
  case "$arg" in
    --skip-build) SKIP_BUILD=1 ;;
    *) ARGS+=("$arg") ;;
  esac
done

if [[ "$SKIP_BUILD" != "1" ]]; then
  (cd "$ROOT" && go build -o bin/agent-harness ./cmd/harness)
fi

exec "$BIN" install "${ARGS[@]}"
```

- [ ] **Step 2: Make the script executable**

Run:

```bash
chmod +x install.sh
```

- [ ] **Step 3: Convert `scripts/install-native.sh` into compatibility**

Keep `scripts/install-native.sh` working for existing automation, but make it clearly call the new path. If the existing script still owns upstream tool setup, move that logic behind `agent-harness install` first instead of preserving two installers.

Minimum acceptable compatibility behavior:

```bash
exec "$(dirname "$0")/../install.sh" "$@"
```

- [ ] **Step 4: Verify first-run dry-run**

Run:

```bash
./install.sh --dry-run
```

Expected: it builds or confirms `bin/agent-harness`, then prints a dry-run install plan without writing host files.

### Task 3: Move PATH Shim And Shell RC Decisions Into `install`

**Files:**
- Modify: `cmd/harness/install_native.go`
- Modify: `internal/core/install.go` or add focused install helper in `cmd/harness` if the behavior is CLI-only
- Test: `go test ./cmd/harness -run Install -count=1`

- [ ] **Step 1: Write tests for PATH decision planning**

Add tests for three modes:

```text
--path-mode=auto
  writes or plans ~/.local/bin shim and shell rc PATH addition when ~/.local/bin is not on PATH

--path-mode=manual
  writes or plans shim but reports a command the user can run for the current shell

--path-mode=skip
  does not write shell rc PATH changes
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run:

```bash
go test ./cmd/harness -run Install -count=1
```

Expected: FAIL because `install` does not yet own PATH shim planning.

- [ ] **Step 3: Implement path mode flags**

Add:

```go
pathMode := fs.String("path-mode", "auto", "PATH setup mode: auto, manual, or skip")
```

Validation:

```go
switch *pathMode {
case "auto", "manual", "skip":
default:
	return fmt.Errorf("path-mode must be auto, manual, or skip")
}
```

- [ ] **Step 4: Preserve current symlink behavior**

Move the behavior currently in `ensure_agent_harness_command` into Go or call a narrowly scoped helper from `install.sh`. Preferred direction is Go-owned planning and writing so dry-run JSON can report the same changes.

The plan output must mention:

```text
~/.local/bin/agent-harness -> <repo>/bin/agent-harness
shell rc file considered: ~/.zshrc, ~/.bashrc, or ~/.profile
PATH line: export PATH="$HOME/.local/bin:$PATH"
```

- [ ] **Step 5: Verify PATH modes**

Run:

```bash
./bin/agent-harness install --dry-run --path-mode=auto
./bin/agent-harness install --dry-run --path-mode=manual
./bin/agent-harness install --dry-run --path-mode=skip
```

Expected: each mode prints a distinct plan and does not write files.

### Task 4: Add Interactive Install Flow

**Files:**
- Modify: `cmd/harness/install_native.go`
- Test: `go test ./cmd/harness -run Install -count=1`

- [ ] **Step 1: Add non-interactive flags before TUI behavior**

Add flags that mirror every interactive choice:

```text
--interactive
--path-mode=auto|manual|skip
--codex-home PATH
--with-codex=true|false
--with-claude=true|false
--with-upstream-tools
--skip-upstream-tools
--init-codegraph=true|false
```

Do not require a TUI package. Start with standard input prompts using the standard library. Add a third-party TUI dependency only after a separate decision if standard prompts are insufficient.

- [ ] **Step 2: Write tests for non-interactive parity**

Verify that explicit flags produce the same request fields as interactive defaults would choose.

Run:

```bash
go test ./cmd/harness -run Install -count=1
```

Expected: tests fail before flag parsing exists.

- [ ] **Step 3: Implement interactive prompts**

When `--interactive` is set and stdin is a terminal, show concise explanations before each decision:

```text
PATH 설정: ~/.local/bin에 agent-harness command shim을 둡니다.
Codex 설정: CODEX_HOME이 없으면 ~/.codex를 사용하고 MCP env에 HARNESS_ROOT를 기록합니다.
Claude 설정: user-scope MCP에 HARNESS_ROOT와 binary path를 등록합니다.
Upstream tools: llm-wiki, CodeGraph, claude-mem은 opt-in companion입니다.
```

Each prompt must have a default. Pressing enter should choose the safe recommended value.

- [ ] **Step 4: Keep CI automation deterministic**

If `--interactive` is set in a non-terminal, return a clear error:

```text
install: --interactive requires a terminal; use explicit flags for automation
```

- [ ] **Step 5: Verify interactive dry-run plan mode**

Run in a terminal:

```bash
./bin/agent-harness install --interactive --dry-run
```

Expected: prompts appear, then dry-run summary shows planned PATH, Codex, Claude, MCP, and upstream choices.

### Task 5: Update Bootstrap And Update Semantics

**Files:**
- Modify: `cmd/harness/update_bootstrap.go`
- Modify: `internal/adapter/cli/usage.go`
- Modify: `cmd/harness/testdata/usage.golden.txt`
- Test: `go test ./cmd/harness -run Golden -count=1`

- [ ] **Step 1: Decide compatibility behavior in code**

Set these semantics:

```text
bootstrap
  delegates to install with non-interactive defaults and no upstream tools unless --sync is present

update
  rebuilds and refreshes install; may include upstream tools only when explicitly requested or existing behavior requires compatibility
```

- [ ] **Step 2: Update command usage text**

Change usage from:

```text
agent-harness install-native [--project-local] [--dry-run] [--json]
agent-harness update [--dry-run] [--json]
agent-harness bootstrap [--sync] [--dry-run] [--json]
```

to:

```text
agent-harness install [--interactive] [--project-local] [--dry-run] [--json]
agent-harness update [--dry-run] [--json]
agent-harness bootstrap [--sync] [--dry-run] [--json]
```

- [ ] **Step 3: Regenerate golden usage if intended**

Run:

```bash
go test ./cmd/harness -run Golden -update -count=1
```

Expected: golden usage changes only for intended command naming and flags.

- [ ] **Step 4: Verify compatibility commands**

Run:

```bash
./bin/agent-harness bootstrap --dry-run --json
./bin/agent-harness update --dry-run --json
```

Expected: both still produce valid install plans.

### Task 6: Update Docs And IssueOps State

**Files:**
- Modify: `README.md`
- Modify: `.agent-harness/operations/install.md`
- Modify: `.agent-harness/OPERATIONS.md`
- Modify: `.agent-harness/TECH_STACK.md`
- Modify: `.agent-harness/TESTING.md`
- Modify: `docs/superpowers/plans/2026-06-03-install-command-tui.md`
- Test: grep and command smoke

- [ ] **Step 1: Replace first-run docs**

Document first install as:

```bash
git clone git@github.com:m16khb/agent-harness.git
cd agent-harness
./install.sh
```

Document dry-run first install as:

```bash
./install.sh --dry-run
```

- [ ] **Step 2: Replace public command name**

Use `agent-harness install` in public docs. Mention `install-native` only if the code keeps it as a deprecated compatibility alias.

- [ ] **Step 3: Clarify env ownership**

Add a concise note:

```text
Normal users should not export HARNESS_ROOT manually. The installer records HARNESS_ROOT in Codex/Claude MCP config. CODEX_HOME is read only when already set; otherwise ~/.codex is used.
```

- [ ] **Step 4: Link the plan in IssueOps state**

Run:

```bash
./bin/agent-harness issueops link-plan --id io-87747efaf5c8 --plan-path docs/superpowers/plans/2026-06-03-install-command-tui.md --json
```

Expected: state includes issue #32 and this plan path.

- [ ] **Step 5: Verify docs references**

Run:

```bash
rg -n "install-native|agent-harness bootstrap|./scripts/install-native.sh|HARNESS_ROOT|CODEX_HOME" README.md .agent-harness docs/superpowers/plans/2026-06-03-install-command-tui.md
```

Expected: remaining `install-native` references are either compatibility notes or test names that intentionally preserve legacy behavior.

### Task 7: Final Verification

**Files:**
- No new files unless previous tasks require fixes.

- [ ] **Step 1: Run targeted install tests**

Run:

```bash
go test ./cmd/harness -run Install -count=1
go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -count=1
```

Expected: PASS.

- [ ] **Step 2: Run contract tests**

Run:

```bash
go test ./cmd/harness -run Golden -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full Go tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Build binary**

Run:

```bash
go build -o bin/agent-harness ./cmd/harness
```

Expected: PASS.

- [ ] **Step 5: Run installer smokes**

Run:

```bash
./install.sh --dry-run
./bin/agent-harness install --dry-run --json
./bin/agent-harness bootstrap --dry-run --json
git diff --check
```

Expected: PASS, no unexpected writes from dry-run commands.

## Self-Review

- Spec coverage: The plan covers first-run `./install.sh`, `agent-harness install`, PATH/HARNESS_ROOT/CODEX_HOME ownership, optional upstream tools, non-interactive automation, docs, and compatibility verification.
- Placeholder scan: No placeholder markers or undefined follow-up placeholders remain.
- Type consistency: The plan consistently uses `install`, `install-native` only as a compatibility alias, and `io-87747efaf5c8` for IssueOps state linkage.
