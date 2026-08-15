# P-3 Doc-Upkeep Pending Retention Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bound pending-only `doc-upkeep-queue.jsonl` growth without silently losing unresolved project-document upkeep intent.

**Architecture:** Keep hook read paths non-destructive: they may canonicalize and deduplicate equivalent pending events, but they must not age-prune unique pending intent. Put destructive pending retention behind an explicit operator command with dry-run by default and `--confirm` for archive/removal.

**Tech Stack:** Go, JSONL state files, existing `corestate.WithKeyLock`, existing `internal/core/lifecycle/docupkeep` store pattern, existing `cmd/harness/statecli` command router.

---

## Context

The v3 branch already fixed the hot hook path enough that measured main-checkout hook reads are no longer a visible bottleneck:

```text
main-user-prompt count=10 min=19.0 p50=20.6 max=53.7
main-post-tool-use count=10 min=9.8 p50=10.3 max=11.0
main-stop count=10 min=19.1 p50=19.3 max=20.8
```

The remaining P-3 gap is file-size policy. The main checkout queue evidence is:

```text
queue: /Users/sample/.local/state/agent-harness/projects/feee6730cfd6453a2cb60ac7/doc-upkeep-queue.jsonl
queue size: 3618 lines, 6196039 bytes
status counts: 3618 pending
```

Current `ReadPending` removes resolved/malformed lines but preserves pending lines. That is correct for intent preservation, but it leaves pending-only queues unbounded.

## File Structure

- Modify `internal/core/lifecycle/model/types.go`
  - Add `DocUpkeepArchiveFile = "doc-upkeep-archive.jsonl"` for confirmed archival writes.
- Create `internal/core/lifecycle/docupkeep/pending_retention.go`
  - Pure helpers for pending canonicalization, duplicate collapse, and explicit archive result shaping.
- Create `internal/core/lifecycle/docupkeep/pending_retention_test.go`
  - TDD coverage for dedupe, evidence merge, dry-run archive, confirmed archive, and lock/rewrite semantics.
- Modify `internal/core/lifecycle/docupkeep/store.go`
  - Wire non-destructive pending compaction into `ReadPending`.
  - Add explicit `ArchiveStalePending` operator-only function.
- Modify `cmd/harness/statecli/state_cli_router.go`
  - Route `agent-harness state doc-upkeep archive`.
- Modify `cmd/harness/statecli/state_cli_maintenance.go`
  - Add CLI flags and text/JSON output for pending archive dry-run/confirm.
- Modify `cmd/harness/statecli/state_cli_test.go`
  - Cover dry-run default and `--confirm` behavior through the CLI.
- Update `.agent-harness/plans/harness-quality-performance-program-v3-measurement-audit.md`
  - Replace the follow-up pointer with implementation evidence after completion.

## Policy

Automatic hook/read behavior:

- Keep every unique pending intent.
- Collapse equivalent pending events by `(kind, normalized target_docs, normalized summary)`.
- Preserve the oldest `CreatedAt`, newest `ID`, unioned `Evidence`, and a deterministic `Source` value.
- Rewrite the queue only when malformed/resolved/duplicate pending records were removed or compacted.

Explicit operator behavior:

- Dry-run by default.
- Require `--confirm` before removing pending records from `doc-upkeep-queue.jsonl`.
- Archive confirmed removals to `doc-upkeep-archive.jsonl` with `status:"archived"` and the original fields preserved.
- Never run destructive archive from hook paths.

## Task 1: Add Pure Pending Dedupe Policy

**Files:**
- Create: `internal/core/lifecycle/docupkeep/pending_retention.go`
- Create: `internal/core/lifecycle/docupkeep/pending_retention_test.go`

- [ ] **Step 1: Write the failing test**

Add this test in `internal/core/lifecycle/docupkeep/pending_retention_test.go`:

```go
package docupkeep

import (
	"reflect"
	"testing"

	"agent-harness/internal/core/lifecycle/model"
)

func TestCompactPendingEventsMergesDuplicateIntent(t *testing.T) {
	events := []model.DocUpkeepEvent{
		{ID: "old", Kind: "code_change", TargetDocs: []string{"OPERATIONS.md"}, Summary: "Hook changed", Evidence: []string{"a.go"}, Source: "hook", Status: "pending", CreatedAt: "2026-07-01T00:00:00Z"},
		{ID: "new", Kind: "code_change", TargetDocs: []string{".agent-harness/OPERATIONS.md"}, Summary: " Hook changed ", Evidence: []string{"b.go", "a.go"}, Source: "hook", Status: "pending", CreatedAt: "2026-07-02T00:00:00Z"},
		{ID: "other", Kind: "code_change", TargetDocs: []string{"TESTING.md"}, Summary: "Test docs changed", Evidence: []string{"c_test.go"}, Source: "hook", Status: "pending", CreatedAt: "2026-07-02T00:00:00Z"},
	}

	result := CompactPendingEvents(events)

	if !result.Changed {
		t.Fatalf("expected duplicate compaction to report Changed")
	}
	if len(result.Events) != 2 {
		t.Fatalf("events=%+v, want 2 compacted pending events", result.Events)
	}
	if result.Events[0].ID != "new" {
		t.Fatalf("duplicate should keep newest ID, got %+v", result.Events[0])
	}
	if result.Events[0].CreatedAt != "2026-07-01T00:00:00Z" {
		t.Fatalf("duplicate should keep oldest CreatedAt, got %+v", result.Events[0])
	}
	if !reflect.DeepEqual(result.Events[0].Evidence, []string{"a.go", "b.go"}) {
		t.Fatalf("evidence=%v, want sorted union", result.Events[0].Evidence)
	}
	if result.Events[0].Status != "pending" {
		t.Fatalf("status=%q, want pending", result.Events[0].Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep -run TestCompactPendingEventsMergesDuplicateIntent -count=1
```

Expected: compile failure because `CompactPendingEvents` is not defined.

- [ ] **Step 3: Add minimal implementation**

Create `internal/core/lifecycle/docupkeep/pending_retention.go`:

```go
package docupkeep

import (
	"sort"
	"strings"

	"agent-harness/internal/core/lifecycle/model"
	"agent-harness/internal/core/state/statepath"
)

type PendingCompactionResult struct {
	Changed bool
	Events  []model.DocUpkeepEvent
}

func CompactPendingEvents(events []model.DocUpkeepEvent) PendingCompactionResult {
	out := []model.DocUpkeepEvent{}
	byKey := map[string]int{}
	changed := false
	for _, event := range events {
		if event.Status == "" {
			event.Status = "pending"
			changed = true
		}
		if event.Status != "pending" {
			changed = true
			continue
		}
		event.TargetDocs = NormalizeTargetDocs(event.TargetDocs)
		key := pendingIntentKey(event)
		if idx, ok := byKey[key]; ok {
			out[idx] = mergePendingEvent(out[idx], event)
			changed = true
			continue
		}
		byKey[key] = len(out)
		out = append(out, event)
	}
	return PendingCompactionResult{Changed: changed, Events: out}
}

func pendingIntentKey(event model.DocUpkeepEvent) string {
	return strings.TrimSpace(event.Kind) + "\x00" + strings.Join(NormalizeTargetDocs(event.TargetDocs), ",") + "\x00" + strings.Join(strings.Fields(event.Summary), " ")
}

func mergePendingEvent(left, right model.DocUpkeepEvent) model.DocUpkeepEvent {
	merged := left
	if eventIsNewer(right, left) {
		merged.ID = right.ID
		merged.Source = right.Source
	}
	if eventIsOlder(right, left) {
		merged.CreatedAt = right.CreatedAt
	}
	merged.Evidence = mergeEvidence(left.Evidence, right.Evidence)
	merged.Status = "pending"
	return merged
}

func eventIsNewer(left, right model.DocUpkeepEvent) bool {
	leftAt, leftErr := statepath.ParseTime(left.CreatedAt)
	rightAt, rightErr := statepath.ParseTime(right.CreatedAt)
	if leftErr == nil && rightErr == nil && !leftAt.Equal(rightAt) {
		return leftAt.After(rightAt)
	}
	return left.ID > right.ID
}

func eventIsOlder(left, right model.DocUpkeepEvent) bool {
	leftAt, leftErr := statepath.ParseTime(left.CreatedAt)
	rightAt, rightErr := statepath.ParseTime(right.CreatedAt)
	if leftErr == nil && rightErr == nil && !leftAt.Equal(rightAt) {
		return leftAt.Before(rightAt)
	}
	return false
}

func mergeEvidence(left, right []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range append(left, right...) {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep -run TestCompactPendingEventsMergesDuplicateIntent -count=1
```

Expected: `ok agent-harness/internal/core/lifecycle/docupkeep`.

## Task 2: Wire Non-Destructive Compaction Into ReadPending

**Files:**
- Modify: `internal/core/lifecycle/docupkeep/store.go:77-121`
- Modify: `internal/core/lifecycle/docupkeep/store_test.go`

- [ ] **Step 1: Write the failing test**

Add this test in `internal/core/lifecycle/docupkeep/store_test.go`:

```go
func TestReadPendingCompactsDuplicatePendingEventsWithoutDroppingIntent(t *testing.T) {
	plan := docUpkeepPlanForTest(t)
	lines := []string{
		`{"id":"old","kind":"code_change","target_docs":["OPERATIONS.md"],"summary":"Hook changed","evidence":["a.go"],"status":"pending","created_at":"2026-07-01T00:00:00Z"}`,
		`{"id":"new","kind":"code_change","target_docs":[".agent-harness/OPERATIONS.md"],"summary":" Hook changed ","evidence":["b.go"],"status":"pending","created_at":"2026-07-02T00:00:00Z"}`,
		`{"id":"other","kind":"code_change","target_docs":["TESTING.md"],"summary":"Test docs changed","status":"pending","created_at":"2026-07-02T00:00:00Z"}`,
	}
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.QueuePath, []byte(joinLines(lines...)), 0o600); err != nil {
		t.Fatal(err)
	}
	store := Store{Validate: func(string) (model.ProjectLifecycleStatePlan, error) { return plan, nil }}

	events, _, err := ReadPending(store, plan.RepoRoot, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events=%+v, want two unique pending intents", events)
	}
	raw, err := os.ReadFile(plan.QueuePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "\n") != 2 {
		t.Fatalf("queue should be rewritten to two compacted pending lines:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"id":"new"`) || strings.Contains(string(raw), `"id":"old"`) {
		t.Fatalf("queue should keep merged newest duplicate and remove old duplicate:\n%s", raw)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep -run TestReadPendingCompactsDuplicatePendingEventsWithoutDroppingIntent -count=1
```

Expected: fail because `ReadPending` keeps both duplicate pending events.

- [ ] **Step 3: Wire compaction**

Change `ReadPending` after scanner processing and before `rewriteDocUpkeepQueue`:

```go
compacted := CompactPendingEvents(events)
events = compacted.Events
if malformedCount > 0 || validCount != len(events) || compacted.Changed {
	if err := rewriteDocUpkeepQueue(plan.QueuePath, events); err != nil {
		return err
	}
}
```

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep -count=1
```

Expected: all docupkeep tests pass.

## Task 3: Add Explicit Archive Function For Stale Pending

**Files:**
- Modify: `internal/core/lifecycle/model/types.go:5-9`
- Modify: `internal/core/lifecycle/docupkeep/pending_retention.go`
- Modify: `internal/core/lifecycle/docupkeep/pending_retention_test.go`

- [ ] **Step 1: Write the failing dry-run/confirm tests**

Add tests in `pending_retention_test.go`:

```go
func TestArchiveStalePendingDryRunDoesNotRewriteQueue(t *testing.T) {
	plan := docUpkeepPlanForRetentionTest(t)
	writeDocUpkeepQueueForRetentionTest(t, plan, []string{
		`{"id":"old","kind":"code_change","target_docs":["OPERATIONS.md"],"summary":"Old pending","status":"pending","created_at":"2026-01-01T00:00:00Z"}`,
		`{"id":"new","kind":"code_change","target_docs":["TESTING.md"],"summary":"New pending","status":"pending","created_at":"2026-07-01T00:00:00Z"}`,
	})
	store := Store{Validate: func(string) (model.ProjectLifecycleStatePlan, error) { return plan, nil }}

	result, err := ArchiveStalePending(store, plan.RepoRoot, PendingArchiveRequest{
		MaxAge:  30 * 24 * time.Hour,
		Now:     mustParseRetentionTime(t, "2026-07-03T00:00:00Z"),
		Confirm: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Confirm || len(result.Archived) != 1 || result.Archived[0].ID != "old" {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	raw, err := os.ReadFile(plan.QueuePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"id":"old"`) {
		t.Fatalf("dry-run should not remove old pending:\n%s", raw)
	}
}

func TestArchiveStalePendingConfirmArchivesAndRemovesOldPending(t *testing.T) {
	plan := docUpkeepPlanForRetentionTest(t)
	writeDocUpkeepQueueForRetentionTest(t, plan, []string{
		`{"id":"old","kind":"code_change","target_docs":["OPERATIONS.md"],"summary":"Old pending","status":"pending","created_at":"2026-01-01T00:00:00Z"}`,
		`{"id":"new","kind":"code_change","target_docs":["TESTING.md"],"summary":"New pending","status":"pending","created_at":"2026-07-01T00:00:00Z"}`,
	})
	store := Store{Validate: func(string) (model.ProjectLifecycleStatePlan, error) { return plan, nil }}

	result, err := ArchiveStalePending(store, plan.RepoRoot, PendingArchiveRequest{
		MaxAge:  30 * 24 * time.Hour,
		Now:     mustParseRetentionTime(t, "2026-07-03T00:00:00Z"),
		Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Confirm || len(result.Archived) != 1 || len(result.Kept) != 1 {
		t.Fatalf("unexpected confirm result: %+v", result)
	}
	raw, err := os.ReadFile(plan.QueuePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"id":"old"`) || !strings.Contains(string(raw), `"id":"new"`) {
		t.Fatalf("confirmed archive should remove only old pending:\n%s", raw)
	}
	archiveRaw, err := os.ReadFile(filepath.Join(plan.ProjectStateDir, model.DocUpkeepArchiveFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archiveRaw), `"id":"old"`) || !strings.Contains(string(archiveRaw), `"status":"archived"`) {
		t.Fatalf("archive should preserve old event as archived:\n%s", archiveRaw)
	}
}
```

- [ ] **Step 2: Add helper test functions**

Add these helpers in the same test file:

```go
func docUpkeepPlanForRetentionTest(t *testing.T) model.ProjectLifecycleStatePlan {
	t.Helper()
	stateDir := t.TempDir()
	return model.ProjectLifecycleStatePlan{
		OK:              true,
		RepoRoot:        t.TempDir(),
		RepoID:          "repo-1",
		ProjectStateDir: stateDir,
		QueuePath:       filepath.Join(stateDir, model.DocUpkeepQueueFile),
		Exists:          true,
		NamespaceValid:  true,
	}
}

func writeDocUpkeepQueueForRetentionTest(t *testing.T, plan model.ProjectLifecycleStatePlan, lines []string) {
	t.Helper()
	if err := os.MkdirAll(plan.ProjectStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.QueuePath, []byte(joinLines(lines...)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustParseRetentionTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep -run 'TestArchiveStalePending' -count=1
```

Expected: compile failure because `ArchiveStalePending`, `PendingArchiveRequest`, and `DocUpkeepArchiveFile` are undefined.

- [ ] **Step 4: Add model constant**

In `internal/core/lifecycle/model/types.go`, add:

```go
const DocUpkeepArchiveFile = "doc-upkeep-archive.jsonl"
```

- [ ] **Step 5: Add archive implementation**

In `pending_retention.go`, add:

```go
type PendingArchiveRequest struct {
	MaxAge  time.Duration
	Now     time.Time
	Confirm bool
}

type PendingArchiveResult struct {
	OK          bool                    `json:"ok"`
	Confirm     bool                    `json:"confirm"`
	DryRun      bool                    `json:"dry_run"`
	RepoRoot    string                  `json:"repo_root"`
	QueuePath   string                  `json:"queue_path"`
	ArchivePath string                  `json:"archive_path"`
	Cutoff      string                  `json:"cutoff"`
	Archived    []model.DocUpkeepEvent  `json:"archived"`
	Kept        []model.DocUpkeepEvent  `json:"kept"`
	Warnings    []string                `json:"warnings,omitempty"`
}
```

Implement `ArchiveStalePending` under the same `corestate.WithKeyLock(plan.ProjectStateDir, "doc-upkeep", ...)` lock used by `ReadPending` and `Append`. It should:

- reject `MaxAge <= 0`;
- default `Now` to `time.Now().UTC()` when zero;
- read queue records;
- keep malformed lines out of the rewritten queue and add `malformed_doc_upkeep_records_skipped` warning;
- split pending records by `CreatedAt < Now-MaxAge`;
- on dry-run, return the split without rewriting files;
- on confirm, rewrite queue with kept pending records and append archived records to `doc-upkeep-archive.jsonl` with `Status = "archived"`.

- [ ] **Step 6: Run focused tests**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep -run 'TestArchiveStalePending|TestCompactPendingEvents' -count=1
```

Expected: all focused retention tests pass.

## Task 4: Add CLI Wrapper

**Files:**
- Modify: `cmd/harness/statecli/state_cli_router.go`
- Modify: `cmd/harness/statecli/state_cli_maintenance.go`
- Modify: `cmd/harness/statecli/state_cli_test.go`

- [ ] **Step 1: Write the failing CLI tests**

Add tests in `cmd/harness/statecli/state_cli_test.go`:

```go
func TestStateDocUpkeepArchiveDryRunRequiresNoConfirm(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if err := runState([]string{"doc-upkeep", "archive", "--repo", repo, "--max-age", "720h", "--json"}); err != nil {
		t.Fatalf("dry-run archive should succeed: %v", err)
	}
}

func TestStateDocUpkeepArchiveRejectsMissingMaxAge(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if err := runState([]string{"doc-upkeep", "archive", "--repo", repo}); err == nil || !strings.Contains(err.Error(), "max-age must be positive") {
		t.Fatalf("expected max-age error, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./cmd/harness/statecli -run 'TestStateDocUpkeepArchive' -count=1
```

Expected: fail with unknown `state` subcommand or missing implementation.

- [ ] **Step 3: Add router branch**

In `state_cli_router.go`, add:

```go
case "doc-upkeep":
	return runStateDocUpkeep(args[1:])
```

- [ ] **Step 4: Add CLI function**

In `state_cli_maintenance.go`, add a `runStateDocUpkeep` dispatcher and `runStateDocUpkeepArchive` with flags:

```go
repo := fs.String("repo", ".", "repository root whose lifecycle queue should be inspected")
maxAge := fs.Duration("max-age", 0, "archive pending doc-upkeep records older than this duration, e.g. 720h")
confirm := fs.Bool("confirm", false, "rewrite queue and append archived records; omitted means dry-run")
jsonOut := fs.Bool("json", false, "print JSON")
```

The command should call the core wrapper for `ArchiveStalePending`, print JSON when requested, and otherwise print:

```text
would archive N pending doc-upkeep records older than 720h
```

or:

```text
archived N pending doc-upkeep records older than 720h
```

- [ ] **Step 5: Run CLI tests**

Run:

```bash
go test ./cmd/harness/statecli -run 'TestStateDocUpkeepArchive' -count=1
```

Expected: all focused CLI tests pass.

## Task 5: Verification And Audit Reconcile

**Files:**
- Modify: `.agent-harness/plans/harness-quality-performance-program-v3-measurement-audit.md`

- [ ] **Step 1: Run package verification**

Run:

```bash
go test ./internal/core/lifecycle/docupkeep ./internal/core/lifecycle ./cmd/harness/statecli ./cmd/harness/hookcli -count=1
```

Expected: all packages pass.

- [ ] **Step 2: Run full repo verification**

Run:

```bash
go test ./... -count=1
go build -o bin/agent-harness ./cmd/harness
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 3: Update measurement audit**

Update `.agent-harness/plans/harness-quality-performance-program-v3-measurement-audit.md`:

```markdown
| P-3 | Done | Resolved/malformed compaction remains automatic; duplicate pending compaction is non-destructive; stale pending archive is explicit dry-run-by-default operator action with `--confirm`. |
```

Include the exact verification commands and any measured queue line-count reduction from a local fixture or dry-run result.

- [ ] **Step 4: Commit**

Stage only P-3 pending retention files:

```bash
git add internal/core/lifecycle/model/types.go \
  internal/core/lifecycle/docupkeep/store.go \
  internal/core/lifecycle/docupkeep/store_test.go \
  internal/core/lifecycle/docupkeep/pending_retention.go \
  internal/core/lifecycle/docupkeep/pending_retention_test.go \
  cmd/harness/statecli/state_cli_router.go \
  cmd/harness/statecli/state_cli_maintenance.go \
  cmd/harness/statecli/state_cli_test.go \
  .agent-harness/plans/harness-quality-performance-program-v3-measurement-audit.md
git commit -m "fix: bound doc-upkeep pending retention"
```

Commit body should follow `.agent-harness/COMMIT_POLICY.md` and include the verification commands above.

## Acceptance Criteria

- Hook read paths never age-prune unique pending doc-upkeep intent.
- Duplicate pending events collapse by kind, normalized target docs, and normalized summary.
- Explicit archive is dry-run by default and requires `--confirm` to rewrite queue files.
- Confirmed archive preserves removed pending entries in `doc-upkeep-archive.jsonl` with `status:"archived"`.
- `go test ./... -count=1`, `go build -o bin/agent-harness ./cmd/harness`, and `git diff --check` pass after implementation.

## Non-Goals

- Do not delete unique pending events from hook read paths.
- Do not change lifecycle compact capsule semantics.
- Do not migrate existing queue files at install/update time.
- Do not add a daemon or background worker for doc-upkeep retention.

## Self-Review

- Spec coverage: the plan covers safe automatic duplicate compaction, explicit destructive archive, CLI access, tests, verification, and audit reconcile.
- Placeholder scan: no task relies on unspecified placeholder work; each code-changing step includes exact file paths and executable test commands.
- Type consistency: `DocUpkeepEvent`, `ProjectLifecycleStatePlan`, `DocUpkeepQueueFile`, and the proposed `DocUpkeepArchiveFile` are all in `internal/core/lifecycle/model/types.go`; the docupkeep store remains the policy owner.
