# Completion Evidence Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `issueops verify-work --json` produce structured completion evidence that a host adapter can inspect, then mark the existing `completion-evidence-audit` self-verification candidate as satisfied only after tests prove the behavior.

**Architecture:** Keep the logic in the Go CLI/core surface already used by both Codex and Claude Code. Do not create host-specific hook behavior. Extend the existing `VerifyWorkResult` DTO in `cmd/issueops/status_verify.go`; keep legacy `evidence` strings for compatibility; add machine-readable `evidence_matrix` and `suggested_commands` fields. Update the candidate catalog and export validation in the CLI after the matrix is covered by tests.

**Tech Stack:** Go standard library tests, existing `go test` golden flow, existing `issueops` CLI.

---

## File Structure

```
cmd/issueops/status_verify.go                         # Extend verify-work JSON DTO and builder
cmd/issueops/status_verify_test.go                    # New TDD coverage for evidence matrix and suggestions
cmd/issueops/self_verify_candidates.go                # Mark completion-evidence-audit satisfied
cmd/issueops/self_augment_summary_test.go             # Update candidate export expectations
cmd/issueops/main.go                                  # Update candidate export self-verify validation
cmd/issueops/response_contract_golden_test.go           # Add verify_work JSON response snapshot
cmd/issueops/testdata/response_contracts.golden.json  # Regenerated response contract fixture
skills/self-verify/CANDIDATES.md                     # Document satisfied candidate state
```

## Success Criteria

- `verify-work --json` includes `evidence_matrix` entries for `git_preflight`, `guard_check`, and `read_only_command`.
- `verify-work --json` keeps the legacy `evidence` string list intact for consumers that already parse it.
- A repository containing `go.mod` gets suggested verification commands from existing project signals, including `go test ./...`, `go build ./...`, and `go vet ./...`.
- The `completion-evidence-audit` self-verification candidate is listed as satisfied, not open.
- Golden response contracts are updated from the tested CLI behavior.
- Verification commands pass:
  - `go test ./cmd/issueops -run TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions -count=1`
  - `go test ./cmd/issueops -run 'TestExportSelfVerificationCandidates|TestSelfVerificationCandidateExport' -count=1`
  - `go test ./cmd/issueops -run TestResponseContractsGolden -count=1`
  - `go test ./... -count=1`
  - `go build -o bin/issueops ./cmd/issueops`
  - `./bin/issueops verify-work --json -- git status --short`

---

## Task 1: Add structured `verify-work` completion evidence

**Files:**
- `cmd/issueops/status_verify.go`
- `cmd/issueops/status_verify_test.go`

### TDD Steps

- [x] Add this failing test in a new `cmd/issueops/status_verify_test.go`:

```go
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions(t *testing.T) {
	repo := t.TempDir()
	runTestCommand(t, repo, "git", "init")
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/verifywork\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	result := buildVerifyWork(repo, false, []string{"git", "status", "--short"})
	if !result.OK {
		t.Fatalf("expected verify-work result to be ok, warnings=%v", result.Warnings)
	}

	assertEvidenceItem(t, result.EvidenceMatrix, "git_preflight", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "guard_check", "passed")
	assertEvidenceItem(t, result.EvidenceMatrix, "read_only_command", "passed")

	if len(result.Evidence) == 0 {
		t.Fatalf("expected legacy evidence strings to remain populated")
	}
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "test", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "build", "./..."})
	assertSuggestedCommand(t, result.SuggestedCommands, []string{"go", "vet", "./..."})
}

func runTestCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(output))
	}
}

func assertEvidenceItem(t *testing.T, items []VerifyWorkEvidenceItem, name string, status string) {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			if item.Status != status {
				t.Fatalf("evidence item %s status=%s, want %s", name, item.Status, status)
			}
			if item.Summary == "" {
				t.Fatalf("evidence item %s has empty summary", name)
			}
			return
		}
	}
	t.Fatalf("missing evidence item %s in %#v", name, items)
}

func assertSuggestedCommand(t *testing.T, commands []VerifyWorkSuggestedCommand, want []string) {
	t.Helper()
	for _, command := range commands {
		if equalStringSlices(command.Command, want) {
			if command.Name == "" || command.Reason == "" {
				t.Fatalf("suggested command %#v must include name and reason", command)
			}
			return
		}
	}
	t.Fatalf("missing suggested command %v in %#v", want, commands)
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [x] Run the focused test and confirm RED before production edits:

```bash
go test ./cmd/issueops -run TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions -count=1
```

- [x] Extend `VerifyWorkResult` and add two small DTOs in `cmd/issueops/status_verify.go`:

```go
type VerifyWorkEvidenceItem struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Command string `json:"command,omitempty"`
}

type VerifyWorkSuggestedCommand struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Reason  string   `json:"reason"`
}
```

- [x] Add these fields to `VerifyWorkResult`:

```go
EvidenceMatrix    []VerifyWorkEvidenceItem      `json:"evidence_matrix"`
SuggestedCommands []VerifyWorkSuggestedCommand  `json:"suggested_commands,omitempty"`
```

- [x] Populate `EvidenceMatrix` in `buildVerifyWork` with deterministic entries:
  - `git_preflight`: `passed` when `preflight.OK`, otherwise `failed`.
  - `guard_check`: `passed` when `guard.OK`, otherwise `failed`.
  - `read_only_command`: `passed` or `failed` when an argv is provided; `skipped` when no argv is provided.
- [x] Populate `SuggestedCommands` through a small helper that calls `core.AnalyzeProjectSignals(root)` and converts `TestCommands`, `BuildCommands`, and `LintCommands` into deterministic command suggestions. Use `strings.Fields` for the existing simple command strings, include the signal confidence/evidence in the reason, and do not suggest issueops-specific build commands for arbitrary target repositories.

- [x] Run the focused test and confirm GREEN:

```bash
go test ./cmd/issueops -run TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions -count=1
```

## Task 2: Mark `completion-evidence-audit` satisfied after evidence exists

**Files:**
- `cmd/issueops/self_verify_candidates.go`
- `cmd/issueops/self_augment_summary_test.go`
- `cmd/issueops/main.go`
- `skills/self-verify/CANDIDATES.md`

### TDD Steps

- [x] Update `TestExportSelfVerificationCandidatesSelectsNextOpenCandidate` in `cmd/issueops/self_augment_summary_test.go` so it proves the candidate is satisfied and no longer selected:

```go
if result.SelectedCandidate != nil {
	t.Fatalf("expected no selected candidate after completion-evidence-audit is satisfied, got %s", result.SelectedCandidate.ID)
}
if len(result.OpenCandidateIDs) != 0 {
	t.Fatalf("expected no open candidates, got %v", result.OpenCandidateIDs)
}
if !containsString(result.SatisfiedCandidateIDs, "completion-evidence-audit") {
	t.Fatalf("expected completion-evidence-audit to be satisfied, got %v", result.SatisfiedCandidateIDs)
}
```

- [x] Run the focused test and confirm RED:

```bash
go test ./cmd/issueops -run TestExportSelfVerificationCandidatesSelectsNextOpenCandidate -count=1
```

- [x] Change the `completion-evidence-audit` candidate status in `selfVerificationCandidateCatalog()` from `selfAugmentCandidateStatusOpen` to `selfAugmentCandidateStatusSatisfied`.
- [x] Add satisfaction evidence to the candidate, including exact behavior names: `verify-work evidence_matrix`, `verify-work suggested_commands`, and `verify_work response contract golden`.
- [x] Update `validateSelfVerifyCandidateExport` in `cmd/issueops/main.go` so the built-in self-verification expects:
  - `completion-evidence-audit` is in `SatisfiedCandidateIDs`.
  - `OpenCandidateIDs` is empty.
  - `SelectedCandidate` is `nil`.
  - The snapshot also has `SelectedCandidate == nil`.
- [x] Update `skills/self-verify/CANDIDATES.md` so it records that all current candidates are satisfied and row 4 is satisfied by the new `verify-work` evidence matrix plus response contract golden. Replace the previous workflow-doc check wording with the implemented evidence.
- [x] Run the focused tests and confirm GREEN:

```bash
go test ./cmd/issueops -run 'TestExportSelfVerificationCandidates|TestSelfVerificationCandidateExport' -count=1
```

## Task 3: Regenerate golden contracts and run final verification

**Files:**
- `cmd/issueops/response_contract_golden_test.go`, `cmd/issueops/testdata/response_contracts.golden.json`
- `bin/issueops`

### Steps

- [x] Add a `verify_work` CLI snapshot to `TestResponseContractsGolden` before regenerating the golden. The snapshot must run `runVerifyWork([]string{"--repo", gitRepoDir, "--json", "--", "git", "status", "--short"})` so the golden directly proves the `verify-work --json` output shape.

- [x] Regenerate response contract golden output:

```bash
go test ./cmd/issueops -run TestResponseContractsGolden -update -count=1
```

- [x] Confirm the regenerated fixture is stable:

```bash
go test ./cmd/issueops -run TestResponseContractsGolden -count=1
```

- [x] Run focused and full verification:

```bash
go test ./cmd/issueops -run TestBuildVerifyWorkIncludesEvidenceMatrixAndSuggestions -count=1
go test ./cmd/issueops -run 'TestExportSelfVerificationCandidates|TestSelfVerificationCandidateExport' -count=1
go test ./... -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops verify-work --json -- git status --short
```

- [x] Inspect the final diff for unrelated changes:

```bash
git diff --stat
git diff -- docs/superpowers/plans/2026-05-31-completion-evidence-audit.md cmd/issueops/status_verify.go cmd/issueops/status_verify_test.go cmd/issueops/self_verify_candidates.go cmd/issueops/self_augment_summary_test.go cmd/issueops/main.go skills/self-verify/CANDIDATES.md cmd/issueops/response_contract_golden_test.go cmd/issueops/testdata/response_contracts.golden.json
```

## Stop Condition

Stop after the final verification commands pass and the diff is limited to the files listed above. Do not implement status summaries, context-local commands, loop detection, hook routers, or worker/job orchestration in this task.
