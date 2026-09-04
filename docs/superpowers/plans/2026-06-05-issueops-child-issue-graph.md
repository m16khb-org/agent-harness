# IssueOps Child Issue Graph Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add provider-neutral durable child issue links to IssueOps state and expose them through CLI and MCP.

**Architecture:** Keep graph state in `internal/core/issueops.go` so Codex, Claude, CLI, and MCP see the same contract. CLI and MCP only parse transport arguments and call the core DTO.

**Tech Stack:** Go standard library, existing `issueops` CLI, existing MCP tool registry in `cmd/issueops/main.go`.

---

### Task 1: Core State Contract

**Files:**
- Modify: `internal/core/issueops.go`
- Test: `internal/core/issueops_test.go`

- [ ] **Step 1: Write the failing test**

Add a test that starts an IssueOps record, links the main issue, calls `LinkIssueOpsChild`, reloads the record, and expects one `issue_links` entry with `type=child`, the child URL, title, inferred provider, and duplicate rejection.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core -run TestIssueOpsChildLinksPersistProviderNeutralGraph -count=1`

Expected: FAIL because `LinkIssueOpsChild` and `IssueOpsIssueLink` do not exist.

- [ ] **Step 3: Implement minimal core support**

Add `IssueOpsIssueLink`, add `IssueLinks []IssueOpsIssueLink` to `IssueOpsRecord`, implement `LinkIssueOpsChild`, validate child URLs with the existing issue URL validator, infer provider from hostname/path, reject duplicate child URLs, and persist with `touchAndWriteIssueOps`.

- [ ] **Step 4: Run targeted core tests**

Run: `go test ./internal/core -run 'TestIssueOps(ChildLinksPersistProviderNeutralGraph|RejectsUnsafeInputs)' -count=1`

Expected: PASS.

### Task 2: CLI Contract

**Files:**
- Modify: `cmd/issueops/issueops.go`
- Modify: `internal/adapter/cli/usage.go`
- Test: `cmd/issueops/issueops_test.go`

- [ ] **Step 1: Write the failing CLI test**

Extend `TestRunIssueOpsLifecycle` to call `issueops link-child --id <id> --child-url <url> --title <title> --json` and assert the JSON includes the child link.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/issueops -run TestRunIssueOpsLifecycle -count=1`

Expected: FAIL because the CLI subcommand is unknown.

- [ ] **Step 3: Implement CLI wiring**

Add a `link-child` case with flags `--id`, `--child-url`, `--title`, and `--json`, then call `core.LinkIssueOpsChild`.

- [ ] **Step 4: Run targeted CLI test**

Run: `go test ./cmd/issueops -run TestRunIssueOpsLifecycle -count=1`

Expected: PASS.

### Task 3: MCP Contract

**Files:**
- Modify: `cmd/issueops/main.go`
- Test: `cmd/issueops/issueops_mcp_test.go`
- Update golden if needed: `cmd/issueops/testdata/mcp_tools.golden.json`, `cmd/issueops/testdata/usage.golden.txt`

- [ ] **Step 1: Write the failing MCP test**

Add an MCP test that starts an IssueOps record, calls `issueops_link_child`, and asserts `issue_links[0].type == "child"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/issueops -run TestMCPIssueOpsLinkChild -count=1`

Expected: FAIL because the MCP tool is not registered.

- [ ] **Step 3: Implement MCP schema and handler**

Add `issueops_link_child` to the tool list and route it in `handleToolCall` to `core.LinkIssueOpsChild`.

- [ ] **Step 4: Run golden and package tests**

Run: `go test ./cmd/issueops -run 'Test(MCPIssueOpsLinkChild|Golden)' -count=1`

Expected: PASS, updating goldens only if the new tool surface changes expected snapshots.

### Task 4: Documentation And Verification

**Files:**
- Modify: `skills/issueops/SKILL.md`
- Modify: `skills/issueops/references/remote-issue.md`
- Modify: `docs/IDD_IMPLEMENTATION_NEEDS.md`

- [ ] **Step 1: Document the local state command**

Add `issueops link-child` to the IssueOps operational examples and clarify that it records provider-native remote child links after creation.

- [ ] **Step 2: Update IDD gap status**

Change durable issue graph status from wholly missing to partial: child links are supported, richer typed links and provider adapters remain future work.

- [ ] **Step 3: Run verification**

Run:

```bash
go test ./internal/core -count=1
go test ./cmd/issueops -count=1
go test ./... -count=1
go build -o bin/issueops ./cmd/issueops
./bin/issueops start --repo "$PWD" --branch issueops-child-smoke --json
```

Expected: all commands exit 0.
