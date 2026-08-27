# SQLite Store Maintenance Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `state maintain` checkpoint every existing loop and direct project SQLite store without creating stores for lifecycle-only project namespaces.

**Architecture:** Keep the existing fixed-root catalog and add loop as its fifth root. Add one bounded discovery helper that scans only direct `projects/<repo-id>` directories and returns those containing a regular `harness.db`; `StateMaintain` appends these deterministic candidates before invoking the unchanged `sqlstore.Maintain` path.

**Tech Stack:** Go 1.26.3, `os.ReadDir`, `os.Lstat`, `path/filepath`, existing `internal/core/sqlstore`, Go `testing`.

## Global Constraints

- Do not checkpoint or otherwise mutate the live `~/.local/state/agent-harness` stores during implementation or verification.
- Do not create a SQLite store in a project namespace that lacks `harness.db`.
- Inspect only direct real directories under `$HARNESS_STATE_DIR/projects`; do not recurse or follow symlink directories.
- Preserve `StateMaintainResult`, CLI/MCP schemas, fixed-root order, `HARNESS_WORKER_DIR`, busy-checkpoint semantics, and the 24-hour sentinel gate.
- A fresh sentinel skip remains stat-only: it reports the five fixed roots and does not scan `projects/`.
- Do not delete rows, run `VACUUM`, remove project namespaces, or push commits.

---

## File Map

- `internal/core/state/state_maintain.go`: fixed roots, bounded project-store discovery, maintenance orchestration.
- `internal/core/state/state_maintain_test.go`: loop/project discovery, deterministic ordering, non-materialization, symlink, and discovery-error regression tests.
- `cmd/harness/statecli/state_cli_maintain_test.go`: CLI result counts after loop becomes a fixed root.
- `cmd/harness/mcpcli/mcp_state_maintain_test.go`: MCP result counts after loop becomes a fixed root.
- `.agent-harness/ADR.md`: current maintenance topology decision.
- `.agent-harness/issues/_unnumbered/agent-harness-stability-concurrency-multisession-hardening.md`: R16/T18 evidence and scope alignment.

### Task 1: Discover loop and project SQLite stores

**Files:**
- Modify: `internal/core/state/state_maintain_test.go`
- Modify: `cmd/harness/statecli/state_cli_maintain_test.go`
- Modify: `cmd/harness/mcpcli/mcp_state_maintain_test.go`
- Modify: `internal/core/state/state_maintain.go`

**Interfaces:**
- Consumes: `StateDir() string`, `sqlstore.Open(dir string) (*sqlstore.DB, error)`, `(*sqlstore.DB).Maintain() (sqlstore.MaintainResult, error)`.
- Produces: `projectStoreRoots() ([]string, error)`; `knownStoreRoots() []string` now includes `filepath.Join(StateDir(), "loop")`.

- [ ] **Step 1: Add core failing tests**

Extend the test imports with `runtime`, `strings`, and `agent-harness/internal/core/sqlstore`, then add:

```go
func TestStateMaintainDiscoversLoopAndProjectStores(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")

	loopDir := filepath.Join(stateDir, "loop")
	projectA := filepath.Join(stateDir, "projects", "project-a")
	projectB := filepath.Join(stateDir, "projects", "project-b")
	for _, dir := range []string{loopDir, projectB, projectA} {
		db, err := sqlstore.Open(dir)
		if err != nil {
			t.Fatalf("open %s: %v", dir, err)
		}
		if err := db.Put("maintain", "seed", []byte(strings.Repeat("x", 4096))); err != nil {
			t.Fatalf("seed %s: %v", dir, err)
		}
	}

	result, err := StateMaintain()
	if err != nil {
		t.Fatalf("StateMaintain: %v", err)
	}
	want := []string{loopDir, projectA, projectB}
	if len(result.Roots) != len(want) {
		t.Fatalf("maintained roots=%+v want %v", result.Roots, want)
	}
	for i, root := range result.Roots {
		if root.Dir != want[i] {
			t.Fatalf("root[%d]=%s want %s", i, root.Dir, want[i])
		}
		if !root.Checkpointed || root.WALBytesBefore == 0 || root.WALBytesAfter > 64 {
			t.Fatalf("root[%d] not checkpointed: %+v", i, root)
		}
	}
}

func TestStateMaintainDoesNotMaterializeLifecycleOnlyProjects(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	projectsDir := filepath.Join(stateDir, "projects")
	projectDir := filepath.Join(projectsDir, "profile-only")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "project.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validDir := filepath.Join(projectsDir, "with-db")
	db, err := sqlstore.Open(validDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("maintain", "valid", []byte("value")); err != nil {
		t.Fatal(err)
	}

	result, err := StateMaintain()
	if err != nil {
		t.Fatalf("StateMaintain: %v", err)
	}
	if len(result.Roots) != 1 || result.Roots[0].Dir != validDir {
		t.Fatalf("maintained roots=%+v want only %s", result.Roots, validDir)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "harness.db")); !os.IsNotExist(err) {
		t.Fatalf("maintenance materialized project store: %v", err)
	}
}

func TestStateMaintainIgnoresSymlinkProjectNamespaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	projectsDir := filepath.Join(stateDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	validDir := filepath.Join(projectsDir, "local")
	validDB, err := sqlstore.Open(validDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := validDB.Put("maintain", "local", []byte("value")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	db, err := sqlstore.Open(outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("maintain", "outside", []byte("unchanged")); err != nil {
		t.Fatal(err)
	}
	wal := filepath.Join(outside, "harness.db-wal")
	before, err := os.Stat(wal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(projectsDir, "linked")); err != nil {
		t.Fatal(err)
	}

	result, err := StateMaintain()
	if err != nil {
		t.Fatalf("StateMaintain: %v", err)
	}
	if len(result.Roots) != 1 || result.Roots[0].Dir != validDir {
		t.Fatalf("maintained roots=%+v want only %s", result.Roots, validDir)
	}
	for _, root := range result.Roots {
		if root.Dir == outside {
			t.Fatalf("followed project symlink: %+v", result.Roots)
		}
	}
	after, err := os.Stat(wal)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() != before.Size() {
		t.Fatalf("outside WAL changed: %d -> %d", before.Size(), after.Size())
	}
}

func TestStateMaintainReportsProjectDiscoveryErrors(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateDir)
	t.Setenv("HARNESS_WORKER_DIR", "")
	if err := os.WriteFile(filepath.Join(stateDir, "projects"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := StateMaintain()
	if err == nil || result.OK {
		t.Fatalf("expected project discovery error, result=%+v err=%v", result, err)
	}
}
```

- [ ] **Step 2: Update adapter expectations before implementation**

In `cmd/harness/statecli/state_cli_maintain_test.go`, change the comment to say worker and loop remain absent; change `len(result.Skipped) != 2` to `!= 2` and the failure message to `expected 2 skipped roots (worker, loop)`.

In `cmd/harness/mcpcli/mcp_state_maintain_test.go`, change `len(result.Skipped) != 3` to `!= 3` and the failure message to `expected 1 maintained root and 3 skipped`.

- [ ] **Step 3: Run RED tests and record the expected failures**

Run:

```bash
go test ./internal/core/state ./cmd/harness/statecli ./cmd/harness/mcpcli -run 'Maintain|Discover' -count=1
```

Expected: FAIL because loop/project stores are absent from `Roots`, lifecycle-only discovery has no error surface, and CLI/MCP skipped counts still reflect four fixed roots.

- [ ] **Step 4: Implement bounded discovery**

Add `fmt` to `state_maintain.go` imports. Update `knownStoreRoots` and add `projectStoreRoots`:

```go
// knownStoreRoots returns fixed store directories the harness owns. Project
// stores are discovered separately so lifecycle-only namespaces are not
// reported as skipped or materialized by maintenance.
func knownStoreRoots() []string {
	base := StateDir()
	workerRoot := filepath.Join(base, "worker")
	if dir := os.Getenv("HARNESS_WORKER_DIR"); dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			workerRoot = abs
		}
	}
	return []string{
		base,
		filepath.Join(base, "issueops"),
		workerRoot,
		filepath.Join(base, "loop"),
	}
}

func projectStoreRoots() ([]string, error) {
	projectsDir := filepath.Join(StateDir(), "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("discover project stores %s: %w", projectsDir, err)
	}
	roots := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, entry.Name())
		info, err := os.Lstat(filepath.Join(dir, "harness.db"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("discover project store %s: %w", dir, err)
		}
		if info.Mode().IsRegular() {
			roots = append(roots, dir)
		}
	}
	return roots, nil
}
```

At the start of `StateMaintain`, discover projects before the maintenance loop:

```go
result := StateMaintainResult{Roots: []sqlstore.MaintainResult{}}
roots := knownStoreRoots()
projectRoots, err := projectStoreRoots()
if err != nil {
	return result, err
}
roots = append(roots, projectRoots...)
for _, dir := range roots {
```

Keep the existing `os.Stat` gate and `sqlstore.Open`/`Maintain` body unchanged. `os.ReadDir` supplies lexical project ordering, `entry.IsDir` excludes symlink namespaces, and `os.Lstat` excludes symlink DB files.

- [ ] **Step 5: Run GREEN tests**

Run:

```bash
gofmt -w internal/core/state/state_maintain.go internal/core/state/state_maintain_test.go cmd/harness/statecli/state_cli_maintain_test.go cmd/harness/mcpcli/mcp_state_maintain_test.go
go test ./internal/core/state ./cmd/harness/statecli ./cmd/harness/mcpcli -run 'Maintain|Discover' -count=1
```

Expected: all selected packages PASS with no warnings.

- [ ] **Step 6: Run the broader storage regression suite**

Run:

```bash
go test ./internal/core/sqlstore ./internal/core/state ./cmd/harness/statecli ./cmd/harness/mcpcli -count=1
```

Expected: all four packages PASS.

- [ ] **Step 7: Commit the behavior atomically**

Stage only the four Task 1 files. Verify with `git diff --cached --check` and `git diff --cached`, then commit:

```text
fix(state): discover loop and project sqlite stores

Lore:
- Intent: Extend store maintenance to every existing loop and direct project SQLite store.
- Why: The fixed four-root catalog misses WALs created by loop runs and project-scoped keyed locks.
- Changes:
  - Add bounded, symlink-safe project store discovery and loop fixed-root maintenance.
  - Add RED-to-GREEN core, CLI, and MCP regression coverage.
- Verify: go test ./internal/core/sqlstore ./internal/core/state ./cmd/harness/statecli ./cmd/harness/mcpcli -count=1
- Risk: Low; discovery is bounded to existing regular harness.db files and preserves the existing maintenance path.
```

### Task 2: Align maintenance decision documentation

**Files:**
- Modify: `.agent-harness/ADR.md:585-604`
- Modify: `.agent-harness/issues/_unnumbered/agent-harness-stability-concurrency-multisession-hardening.md:69,375-383`

**Interfaces:**
- Consumes: the Task 1 fixed-root plus bounded-project discovery behavior.
- Produces: documentation that no longer claims maintenance is limited to four roots or loop-only remediation.

- [ ] **Step 1: Update the maintenance ADR**

Replace the four-root decision with:

```markdown
- `state maintain` CLI/MCP covers four fixed roots (state, issueops, worker, loop) plus direct `projects/<repo-id>` directories that already contain a regular `harness.db`. Missing fixed roots are reported as skipped; lifecycle-only project namespaces are neither listed nor materialized.
```

Add to the rationale that direct-only project discovery is bounded and runs only when the 24-hour sentinel allows maintenance; a fresh-sentinel skip remains stat-only.

- [ ] **Step 2: Update R16 and T18 without marking the broader task complete**

Change R16 to state that loop and project-scoped stores were omitted from maintenance. Change T18's `What` and `Acceptance` clauses to include bounded existing-project discovery while retaining the still-open privacy, crash, nested-span, cancellation, and FD probes.

- [ ] **Step 3: Verify documentation and focused tests**

Run:

```bash
rg -n '4 known store roots|loop root가 maintenance에서 빠지고|loop root를 maintenance catalog' .agent-harness/ADR.md .agent-harness/issues/_unnumbered/agent-harness-stability-concurrency-multisession-hardening.md
git diff --check
go test ./internal/core/state ./cmd/harness/statecli ./cmd/harness/mcpcli -run 'Maintain|Discover' -count=1
```

Expected: `rg` returns no matches, `git diff --check` is silent, and focused tests PASS.

- [ ] **Step 4: Commit documentation separately**

Stage only the two Task 2 files. Inspect the staged patch and commit:

```text
docs(state): align sqlite maintenance topology

Lore:
- Intent: Keep the maintenance ADR and hardening plan aligned with implemented store discovery.
- Why: The original four-root decision predates loop and project-scoped SQLite stores.
- Changes:
  - Document five fixed roots and bounded existing-project discovery.
  - Preserve the remaining T18 crash, privacy, and locking work as open scope.
- Verify: git diff --check; focused Maintain and Discover tests pass.
- Risk: Low; documentation-only alignment.
```

### Task 3: Full verification and handoff

**Files:**
- Verify only; do not modify generated goldens unless a test proves the response schema changed.

**Interfaces:**
- Consumes: Task 1 behavior and Task 2 documentation.
- Produces: fresh evidence that the repository, race detector, and binary build remain healthy.

- [ ] **Step 1: Run the complete test suite**

```bash
go test -p 1 -timeout 20m ./... -count=1
```

Expected: exit 0, zero failing packages.

- [ ] **Step 2: Run the complete race suite**

```bash
go test -race -p 1 -timeout 20m ./... -count=1
```

Expected: exit 0, no race reports, zero failing packages.

- [ ] **Step 3: Build outside tracked source output**

```bash
tmp_bin="$(mktemp -d)/agent-harness" && go build -o "$tmp_bin" ./cmd/harness
```

Expected: exit 0. Do not overwrite tracked or installed `bin/agent-harness` during this verification.

- [ ] **Step 4: Review final repository evidence**

```bash
git diff --check
git status --short --branch
git log -3 --oneline --decorate
```

Expected: no unstaged changes, `main` is ahead of `origin/main` by the spec, plan, behavior, and documentation commits, and no push has occurred.
