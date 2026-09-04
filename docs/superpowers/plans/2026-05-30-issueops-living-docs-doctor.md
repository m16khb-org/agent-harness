# Agent Harness Living Docs Doctor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add project-scoped lifecycle state initialization and a top-level `issueops doctor` that diagnoses harness health and doc-lifecycle issues without storing runtime state in target repos.

**Architecture:** Keep runtime lifecycle state under the existing user state root, namespaced by a hashed repo fingerprint under `projects/<repo-id>`. Extend `project bootstrap` to plan/create the namespace, make `UserPromptSubmit` state-aware, and add a read-only top-level doctor that combines state, install, hook, daemon, project-doc, and repo-local-state diagnostics.

**Tech Stack:** Go standard library, existing `internal/core` DTO style, current CLI/golden tests, existing user-state conventions.

---

## File Structure

- Create `internal/core/lifecycle_state.go`: repo fingerprinting, project namespace resolver, `project.json` initialization/validation, doc-upkeep queue helpers, and state-aware hint conversion.
- Create `internal/core/lifecycle_state_test.go`: resolver, bootstrap initialization, namespace mismatch, upkeep queue, and hook hint tests.
- Create `internal/core/doctor.go`: comprehensive read-only doctor DTO and diagnostics.
- Create `internal/core/doctor_test.go`: doctor reports healthy baseline, repo-local forbidden state, lifecycle namespace mismatch, and state doctor errors.
- Modify `internal/core/project_docs.go`: add lifecycle state planning/initialization to bootstrap result.
- Modify `internal/core/hook_prompt.go`: resolve lifecycle state for `Repo`, include pending upkeep hints when safe, and degrade to keyword routing on state errors.
- Modify `cmd/issueops/main.go`: add top-level `doctor`, extend `project bootstrap` output, and pass repo to hook requests.
- Modify `internal/adapter/cli/usage.go` and `cmd/issueops/testdata/usage.golden.txt`: document `issueops doctor`.
- Modify golden contract snapshots only after tests indicate exact output changes.
- Modify `.issueops/OPERATIONS.md`, `.issueops/ARCHITECTURE.md`, `.issueops/CONVENTIONS.md`, `.issueops/TESTING.md`, and README only where implementation evidence changes user-facing behavior.

---

### Task 1: Lifecycle State Core

**Files:**
- Create: `internal/core/lifecycle_state.go`
- Test: `internal/core/lifecycle_state_test.go`

- [x] **Step 1: Write failing tests**

Create tests for:

```go
func TestResolveProjectLifecycleNamespaceIsProjectScoped(t *testing.T)
func TestInitProjectLifecycleStateWritesProjectJSONOnlyWhenConfirmed(t *testing.T)
func TestValidateProjectLifecycleStateDetectsNamespaceMismatch(t *testing.T)
func TestAppendDocUpkeepEventWritesJSONL(t *testing.T)
```

Assertions:
- two temp repos produce different `RepoID` and `ProjectStateDir`
- dry-run returns planned namespace but creates no files
- confirm writes `project.json`
- altered `project.json` root produces `namespace_mismatch`
- upkeep event appends one JSONL record under that project namespace

Run:

```bash
go test ./internal/core -run 'Test(ResolveProjectLifecycle|InitProjectLifecycle|ValidateProjectLifecycle|AppendDocUpkeep)' -count=1
```

Expected: fail because symbols do not exist.

- [x] **Step 2: Implement lifecycle state DTOs and helpers**

Implement focused structs:

```go
const ProjectLifecycleSchemaVersion = 1

type ProjectLifecycleStatePlan struct {
    OK bool `json:"ok"`
    SchemaVersion int `json:"schema_version"`
    RepoRoot string `json:"repo_root"`
    RepoID string `json:"repo_id"`
    StateRoot string `json:"state_root"`
    ProjectStateDir string `json:"project_state_dir"`
    ProjectJSONPath string `json:"project_json_path"`
    Fingerprint ProjectFingerprint `json:"fingerprint"`
    Exists bool `json:"exists"`
    NamespaceValid bool `json:"namespace_valid"`
    Warnings []string `json:"warnings,omitempty"`
}
```

Functions:

```go
ResolveProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error)
InitProjectLifecycleState(repoRoot string, confirm bool) (ProjectLifecycleStatePlan, error)
ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error)
AppendDocUpkeepEvent(repoRoot string, event DocUpkeepEvent) (DocUpkeepAppendResult, error)
```

Implementation constraints:
- Use `StateDir()/projects/<repo-id>`.
- Use SHA-256 over canonical root and git metadata.
- Store `project.json` only in user-state.
- Use atomic write for `project.json`.
- Use append-only JSONL for `doc-upkeep-queue.jsonl`.

- [x] **Step 3: Run targeted tests**

Run the command from Step 1. Expected: pass.

- [x] **Step 4: Commit**

```bash
git add internal/core/lifecycle_state.go internal/core/lifecycle_state_test.go
git commit -m "feat(state): add project lifecycle namespace"
```

---

### Task 2: Bootstrap Initializes User-State Namespace

**Files:**
- Modify: `internal/core/project_docs.go`
- Modify: `internal/core/project_docs_test.go`
- Modify: `cmd/issueops/main.go`

- [x] **Step 1: Write failing bootstrap tests**

Extend `TestBootstrapProjectDocsDryRunAndWrite`:

```go
if dry.LifecycleState.ProjectStateDir == "" { t.Fatal("missing planned lifecycle namespace") }
if _, err := os.Stat(dry.LifecycleState.ProjectJSONPath); !os.IsNotExist(err) { t.Fatal("dry run wrote project.json") }
if _, err := os.Stat(written.LifecycleState.ProjectJSONPath); err != nil { t.Fatalf("write did not create project.json: %v", err) }
```

Run:

```bash
go test ./internal/core -run TestBootstrapProjectDocsDryRunAndWrite -count=1
```

Expected: fail because bootstrap result lacks lifecycle state.

- [x] **Step 2: Add lifecycle state to bootstrap result**

Add field:

```go
LifecycleState ProjectLifecycleStatePlan `json:"lifecycle_state"`
```

Dry-run calls `InitProjectLifecycleState(root, false)`. Write bootstrap calls `InitProjectLifecycleState(root, true)`.

- [x] **Step 3: Update CLI text output**

After file list, print:

```text
lifecycle state: <project_state_dir> (planned|initialized)
```

- [x] **Step 4: Run targeted tests**

```bash
go test ./internal/core -run TestBootstrapProjectDocsDryRunAndWrite -count=1
```

Expected: pass.

- [x] **Step 5: Commit**

```bash
git add internal/core/project_docs.go internal/core/project_docs_test.go cmd/issueops/main.go
git commit -m "feat(project): initialize lifecycle namespace on bootstrap"
```

---

### Task 3: State-Aware UserPromptSubmit

**Files:**
- Modify: `internal/core/hook_prompt.go`
- Modify: `internal/core/hook_prompt_test.go`
- Modify: `cmd/issueops/hook_user_prompt.go`

- [x] **Step 1: Write failing tests**

Add tests:

```go
func TestBuildUserPromptMCPHintsIncludesPendingUpkeep(t *testing.T)
func TestBuildUserPromptMCPHintsFallsBackWhenLifecycleStateMissing(t *testing.T)
```

Setup a temp repo, initialize lifecycle state, append a `DocUpkeepEvent` targeting `OPERATIONS.md`, then call:

```go
got := BuildUserPromptMCPHints(HookUserPromptRequest{Prompt: "계속", Repo: root})
```

Assert additional context contains `Pending project-doc upkeep:` and `OPERATIONS.md`.

Run:

```bash
go test ./internal/core -run 'TestBuildUserPromptMCPHints(IncludesPendingUpkeep|FallsBack)' -count=1
```

Expected: fail.

- [x] **Step 2: Implement state-aware hinting**

Add lifecycle read path that:
- no-ops when `Repo` is empty
- validates namespace
- reads pending `doc-upkeep-queue.jsonl`
- adds action hints for target docs and `project_docs_read/project_docs_revise`
- never fails the hook because of lifecycle state errors

- [x] **Step 3: Pass repo into hook request**

In `cmd/issueops/hook_user_prompt.go`, derive repo from hook JSON if present or `resolveTarget("")` and pass it into `HookUserPromptRequest`.

- [x] **Step 4: Run targeted tests**

Run command from Step 1. Expected: pass.

- [x] **Step 5: Commit**

```bash
git add internal/core/hook_prompt.go internal/core/hook_prompt_test.go cmd/issueops/hook_user_prompt.go
git commit -m "feat(hook): route docs from lifecycle state"
```

---

### Task 4: Top-Level Doctor

**Files:**
- Create: `internal/core/doctor.go`
- Test: `internal/core/doctor_test.go`
- Modify: `cmd/issueops/main.go`
- Modify: `internal/adapter/cli/usage.go`
- Modify: `cmd/issueops/testdata/usage.golden.txt`

- [x] **Step 1: Write failing doctor tests**

Test functions:

```go
func TestHarnessDoctorHealthyBaseline(t *testing.T)
func TestHarnessDoctorReportsRepoLocalRuntimeState(t *testing.T)
func TestHarnessDoctorReportsLifecycleNamespaceMismatch(t *testing.T)
```

Assert JSON-ish result has:
- `Kind == "harness_doctor"`
- issue codes include `repo_local_state_present` and `lifecycle_namespace_mismatch` in the relevant cases
- fix suggestions include non-destructive command/description strings

Run:

```bash
go test ./internal/core -run 'TestHarnessDoctor' -count=1
```

Expected: fail.

- [x] **Step 2: Implement doctor DTOs**

Structs:

```go
type HarnessDoctorResult struct { OK, Healthy bool; Kind string; Issues []HarnessDoctorIssue; Checks []HarnessDoctorCheck }
type HarnessDoctorIssue struct { Code, Severity, Summary string; Fix *HarnessDoctorFix }
type HarnessDoctorFix struct { Command string; Destructive bool; Description string }
```

Checks:
- `StateDoctor()` summary
- lifecycle namespace validate
- `.issueops/state`, `.issueops/state.schema.json`, runtime-looking `.issueops/STATE.md`
- docs presence
- hook config presence where home is provided
- daemon status can be a warning-only placeholder if unavailable in core without side effects

- [x] **Step 3: Add CLI command**

Dispatch top-level `doctor`:

```bash
issueops doctor [--repo PATH] [--json]
```

Human output:

```text
issueops doctor healthy
```

or issue list.

- [x] **Step 4: Update usage**

Add `doctor` to `Commands()` and usage text.

- [x] **Step 5: Run targeted tests**

```bash
go test ./internal/core -run 'TestHarnessDoctor' -count=1
go test ./internal/adapter/cli ./cmd/issueops -run 'Usage|Golden' -count=1
```

Expected: pass after updating golden files if necessary.

- [x] **Step 6: Commit**

```bash
git add internal/core/doctor.go internal/core/doctor_test.go cmd/issueops/main.go internal/adapter/cli/usage.go cmd/issueops/testdata/usage.golden.txt
git commit -m "feat(cli): add comprehensive issueops doctor"
```

---

### Task 5: Documentation and Verification

**Files:**
- Modify: `.issueops/OPERATIONS.md`
- Modify: `.issueops/ARCHITECTURE.md`
- Modify: `.issueops/CONVENTIONS.md`
- Modify: `.issueops/TESTING.md`
- Modify: `README.md` if CLI examples list doctor/state commands

- [x] **Step 1: Update docs**

Document:
- lifecycle state is project-scoped under user state
- `project bootstrap --write` initializes `project.json`
- `issueops doctor` is the general diagnostic command
- `state doctor` remains narrower state-store integrity check
- runtime state/schema files should not be committed under `.issueops`

- [x] **Step 2: Run full verification**

```bash
go test ./... -count=1
go test ./cmd/issueops -run Golden -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops doctor --json
./bin/issueops project bootstrap --repo . --json
```

Expected: tests pass, build succeeds, doctor returns valid JSON, bootstrap JSON includes lifecycle state plan.

- [x] **Step 3: Commit**

```bash
git add .issueops README.md cmd/issueops/testdata internal cmd/issueops docs/superpowers/plans/2026-05-30-issueops-living-docs-doctor.md
git commit -m "docs(operations): document living docs diagnostics"
```
