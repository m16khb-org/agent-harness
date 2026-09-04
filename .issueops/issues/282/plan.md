# Native Hook Stable Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure every Codex and Claude native hook installation targets the canonical source runtime, reports stale cached targets precisely, and activates the runtime binary without overwriting an executing inode.

**Architecture:** The host-neutral install core resolves and validates one stable source root before either host adapter runs. Codex and Claude keep their existing hook renderers, but share target extraction and drift diagnostics from `installutil`; the shell wrapper builds from the invoking checkout and atomically activates into the stable source runtime. Existing #261 regular-file adoption remains out of scope, while this plan pins the stable-target and inode-activation contracts it consumes.

**Tech Stack:** Go 1.26, Bash, Python 3 `os.replace`/`os.fsync`, Git worktree metadata, table-driven Go tests.

## Global Constraints

- Core policy stays host-neutral; Codex and Claude adapters remain thin projections.
- Installed hook targets must never resolve inside a linked IssueOps worktree.
- Existing third-party hook groups remain byte-semantically preserved.
- Dry-run must diagnose the same observed and expected targets without writing.
- Do not duplicate #261's regular-file adoption or PATH-link ownership behavior.
- Every behavior change follows RED, minimal GREEN, focused verification, then full verification.

---

### Task 1: Resolve the canonical stable native root

**Files:**
- Create: `internal/core/install/native_root.go`
- Create: `internal/core/install/native_root_test.go`
- Modify: `internal/core/install/install.go`

**Interfaces:**
- Produces: `ResolveStableNativeRoot(root string) (string, error)`.
- Produces: `ValidateStableNativeRuntime(root, binPath string) error`.
- Consumes: a normal checkout whose `.git` is a directory, or a linked worktree whose `.git` file points at a gitdir containing `commondir`.

- [x] **Step 1: Write failing resolver tests**

  Add table-driven tests proving a normal checkout stays unchanged, a linked worktree resolves through `gitdir` plus `commondir` to the source checkout, relative `gitdir:` and relative `commondir` paths resolve against their declaring files, malformed git metadata fails closed, and a runtime outside the resolved root is rejected. Add a `DefaultNativeInstallRequest` test whose input root and default binary are under a linked worktree and whose result is the source root plus `source/bin/issueops`.

- [x] **Step 2: Run the focused RED test**

  Run: `go test ./internal/core/install -run 'TestResolveStableNativeRoot|TestValidateStableNativeRuntime|TestDefaultNativeInstallRequestMapsLinkedWorktree' -count=1`

  Expected: FAIL because the resolver and validator do not exist and the default request preserves the worktree target.

- [x] **Step 3: Implement the minimum resolver and validator**

  Resolve only literal Git metadata: directory `.git` means the supplied root is stable; file `.git` must contain exactly one `gitdir:` path; the referenced gitdir must contain `commondir`; the resolved common directory must be a physical `.git` directory whose parent is the source checkout. In `DefaultNativeInstallRequest`, remap the binary only when it was empty or exactly the invoking root's default `bin/issueops`. In `InstallNative`, reject unresolved roots and binaries outside the stable root before invoking adapters.

- [x] **Step 4: Run focused GREEN and regression tests**

  Run: `go test ./internal/core/install ./internal/core -count=1`

  Expected: PASS.

### Task 2: Share stale hook-target diagnosis across hosts

**Files:**
- Create: `internal/adapter/installutil/hook_target.go`
- Create: `internal/adapter/installutil/hook_target_test.go`
- Modify: `internal/adapter/codex/install_hooks.go`
- Modify: `internal/adapter/codex/install.go`
- Modify: `internal/adapter/claude/install_hooks.go`
- Modify: `internal/adapter/claude/install.go`
- Modify: `internal/adapter/codex/install_test.go`
- Modify: `internal/adapter/claude/install_test.go`

**Interfaces:**
- Produces: `HookTargetDriftMessages(config map[string]any, host, expected string) []string`.
- Consumes: existing host config parsed by the host writer before canonical merge.

- [x] **Step 1: Write failing pure-helper and host adapter tests**

  Cover a completed-worktree target, a relative legacy target, a canonical target, duplicate events, third-party commands, and malformed groups. The two host integration tests must start with a stale target and assert one deterministic message containing `host`, exact `observed`, exact `expected`, and restart guidance in both normal and dry-run modes.

- [x] **Step 2: Run the focused RED tests**

  Run: `go test ./internal/adapter/installutil ./internal/adapter/codex ./internal/adapter/claude -run 'HookTarget|StaleHookTarget' -count=1`

  Expected: FAIL because no shared drift diagnostic exists.

- [x] **Step 3: Implement target extraction and message projection**

  Inspect only groups already identified by `HookGroupContainsAgentHarness`. Extract the command prefix immediately before literal ` hook `, decode the installer-owned single-quoted path form, deduplicate and sort targets, and emit no message for the expected path. Make each host writer return its drift messages alongside its `InstallFile`; adapters append the messages to the existing `Plan` without changing third-party groups.

- [x] **Step 4: Run focused GREEN and adapter contract tests**

  Run: `go test ./internal/adapter/installutil ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter -count=1`

  Expected: PASS; golden output changes only if a fixture intentionally contains a stale target.

### Task 3: Pin strict readback to the stable runtime

**Files:**
- Modify: `internal/adapter/installutil/activation.go`
- Create: `internal/adapter/installutil/activation_test.go`
- Modify: `internal/adapter/codex/activation_test.go`
- Modify: `internal/adapter/claude/activation_test.go`

**Interfaces:**
- Consumes: validated `NativeInstallRequest.Root` and `BinPath` from Task 1.
- Produces: strict semantic rejection of any installed issueops hook group whose executable differs from the stable expected runtime, including `.worktrees` paths.

- [x] **Step 1: Add explicit failing stale-worktree readback cases**

  After canonical install, replace only one host hook command with `/source.worktrees/completed/bin/issueops hook ...`. Assert `VerifyActivation` fails and names the host hook readback surface. Keep formatting-only and third-party co-resident hook cases accepted.

- [x] **Step 2: Run the focused RED tests**

  Run: `go test ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter/installutil -run 'VerifyActivation.*Worktree|VerifyHookActivation' -count=1`

  Expected: existing semantic rejection remains green as characterization, while exact observed/expected diagnostics fail until the shared extraction is wired into readback.

- [x] **Step 3: Add only the missing fail-closed validation**

  Reuse Task 2 target extraction before canonical JSON comparison so an unexpected executable yields an observed/expected diagnostic. Do not introduce host-specific parsing branches.

- [x] **Step 4: Run focused GREEN**

  Run: `go test ./internal/adapter/installutil ./internal/adapter/codex ./internal/adapter/claude -count=1`

  Expected: PASS.

### Task 4: Diagnose a cached worktree runtime from inside native hooks

**Files:**
- Create: `internal/core/install/native_runtime.go`
- Create: `internal/core/install/native_runtime_test.go`
- Modify: `cmd/issueops/hookcli/dependencies.go`
- Modify: `cmd/issueops/hookcli/hook_pre_tool_use.go`
- Modify: `cmd/issueops/hookcli/hook_pre_tool_use_test.go`
- Modify: `cmd/issueops/hookcli/hookcatalog/catalog.go`
- Modify: `cmd/issueops/hookcli/hookcatalog/catalog_test.go`
- Modify: `cmd/issueops/issueopsapp/host_facade.go`

**Interfaces:**
- Produces: `DiagnoseNativeRuntime(executable string) (NativeRuntimeDiagnostic, error)` with exact `Observed`, `Expected`, and `RestartRequired` evidence.
- Consumes: the current executable path, not the already-rewritten host config.
- Projects: the same host-neutral diagnosis through existing Codex and Claude hook response adapters, always with a normal hook status.

- [x] **Step 1: Write the cached-runtime reproduction before production code**

  Construct a source checkout plus completed linked-worktree layout. Keep both persisted host configs canonical at `/source/bin/issueops`, then inject `/source.worktrees/completed/bin/issueops` as the running executable. Assert the diagnostic reports the exact observed and expected executable paths and explicit session-restart guidance. Cover canonical source execution as a no-op and malformed Git metadata as fail-closed.

- [x] **Step 2: Add both-host hook contract tests**

  Invoke PreToolUse and SessionStart through the existing hook adapters with the stale executable dependency. Assert Codex and Claude receive their native block/context shape, the reason contains exact observed/expected paths plus restart guidance, and the command returns status 0 rather than disappearing as `hook exited without a status code`. Keep canonical execution behavior byte-compatible. PostToolUse remains a normal-status smoke assertion and does not acquire mutation policy.

- [x] **Step 3: Run the focused RED tests**

  Run: `go test ./internal/core/install ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookcatalog -run 'NativeRuntime|CachedWorktreeRuntime' -count=1`

  Expected: FAIL because executable-based diagnosis and hook dependency injection do not exist.

- [x] **Step 4: Implement one host-neutral diagnosis and thin projections**

  Resolve symlinks on the observed executable, derive its checkout root only for the installer-owned `bin/issueops` layout, resolve that root through Task 1, and compare against the canonical source `bin/issueops`. Return a typed diagnostic; do not read or rewrite host config here. Inject the executable provider and diagnostic into hook CLI dependencies. PreToolUse emits the existing host-native block response and SessionStart emits host-native context; both exit normally so a cached executable produces actionable evidence instead of a missing status.

- [x] **Step 5: Run focused GREEN and hook contract regression tests**

  Run: `go test ./internal/core/install ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookcatalog ./cmd/issueops/issueopsapp -count=1`

  Expected: PASS for both hosts with no production path duplicated per host.

### Task 5: Activate from any checkout into the stable runtime atomically

**Files:**
- Modify: `scripts/install-native.sh`
- Modify: `internal/adapter/install_contract_matrix_test.go`
- Modify: `.issueops/CAUTIONS.md`

**Interfaces:**
- Consumes: Git's absolute common-dir result for the invoking checkout.
- Produces: separate `BUILD_ROOT` and stable `ROOT`; staged binary file fsync followed by `os.replace` and directory fsync.

- [x] **Step 1: Add failing script contract tests**

  Assert the script derives a stable root from `git rev-parse --path-format=absolute --git-common-dir`, builds from `BUILD_ROOT`, stages under the stable target directory, fsyncs the staged file before `os.replace`, then fsyncs the containing directory. Assert no direct `go build -o "$BIN"` remains.

- [x] **Step 2: Run the focused RED test**

  Run: `go test ./internal/adapter -run 'InstallNativeScript.*Stable|InstallNativeScript.*Atomic' -count=1`

  Expected: FAIL because the script currently conflates build and activation roots and omits staged-file fsync.

- [x] **Step 3: Implement the bounded shell change**

  Keep the invoking checkout as `BUILD_ROOT`. Resolve the common Git directory and use its parent as stable `ROOT` only when it is a physical `.git` directory; otherwise preserve the explicit non-Git root behavior. Build current checkout code into a temp file located beside stable `bin/issueops`, fsync that file, atomically replace the target, fsync the directory, and run the installed binary's version/readback checks.

- [x] **Step 4: Record the operational caution**

  Add one concise CAUTIONS entry: never install hooks from a lifecycle worktree target or overwrite an executing binary inode; build from the checkout, activate at the stable source runtime by rename, and restart sessions that cached an observed old target.

- [x] **Step 5: Run focused GREEN**

  Run: `go test ./internal/adapter ./cmd/issueops/installcli -count=1`

  Expected: PASS.

### Task 6: Verify, review, publish, and dogfood

**Files:**
- Create: `.issueops/issueops/282-verified-execution-report.md`
- Modify: `.issueops/issues/282/plan.md` checkbox state only as evidence is completed.

**Interfaces:**
- Consumes: all prior tasks.
- Produces: review evidence, PR, merged stable binary activation, host config readback, and cleanup receipts.

- [x] **Step 1: Run formatting and focused verification**

  Run: `gofmt -w <changed-go-files>`

  Run: `git diff --check`

  Run: `go test ./internal/core/install ./internal/adapter/installutil ./internal/adapter/codex ./internal/adapter/claude ./internal/adapter ./cmd/issueops/hookcli ./cmd/issueops/hookcli/hookcatalog ./cmd/issueops/installcli -count=1`

- [x] **Step 2: Run full verification sequentially**

  Run: `go test ./... -count=1`

  Run: `go test -race ./... -count=1`

  Run: `go vet ./...`

  Run: `go build -o /tmp/issueops-282 ./cmd/issueops`

  Run: `go test ./cmd/issueops/contractgolden -run Golden -count=1`

  Run: `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1`

- [x] **Step 3: Run fresh code review and resolve every finding**

  Inspect the entire branch diff against `origin/main`, record a pass/revise/stop implementation review in IssueOps, and rerun affected tests after every correction.

- [ ] **Step 4: Publish and merge through IssueOps**

  Commit with the repository Conventional Commit + Lore policy, create a Korean GitHub PR linked to #282, wait for both push and PR CI runs, record completion with the final head and verification report, merge only after all checks pass, and fast-forward source `main`.

- [ ] **Step 5: Dogfood the merged stable activation**

  From the source checkout, record old target inode, run `./scripts/install-native.sh --json`, and prove the target inode changes while SHA matches the merged build. Verify Codex and Claude hook configs contain the stable source binary and zero `.worktrees` targets, exercise PreToolUse/PostToolUse host payloads for exit-status delivery, and confirm stale-target dry-run diagnostics in isolated temporary homes.

- [ ] **Step 6: Clean up only after durable evidence is recorded**

  Use `issueops cleanup remote-branch` and `issueops cleanup finish`; verify the #282 worktree, local branch, remote branch, and runtime record are removed without touching other active lifecycles.

## Self-Review

- Spec coverage: stable root, `.worktrees` rejection, dry-run diagnosis, executable-based cached-target observed/expected guidance, normal hook status, atomic inode activation, two-host parity, native smoke, PR/merge/cleanup each map to an explicit task.
- Placeholder scan: no deferred marker or unspecified test step remains.
- Type consistency: `ResolveStableNativeRoot`, `ValidateStableNativeRuntime`, `DiagnoseNativeRuntime`, and `HookTargetDriftMessages` are defined once and consumed by later tasks with the same signatures.
- Scope boundary: #261 regular-file adoption and #268/#278 upstream signal semantics remain referenced dependencies, not duplicated code.
