# IssueOps Live E2E Defect Recovery Design

## Goal

Fix the four defects confirmed by the 2026-07-23 live IssueOps E2E without
expanding the execution-v1 command surface or mutating the wedged live record.

## Scope

1. Generate an Orca task title that fits Orca's 80-character persisted limit
   while binding the lifecycle, operation marker, and sealed prompt digest.
2. Populate `NativeActorV1.ProcessAncestry` in `issueops remote create-pr`.
3. Run optional glab MCP synchronization before native activation evidence is
   sealed.
4. Make lifecycle-hook recovery guidance point to the actor-free
   `issueops execution status` command.

## Non-goals

- Delete or rewrite `io-c7e2d4e02b59`.
- Add an execution-abandon or unsafe reset command.
- Change Orca, GitHub, GitLab, or MCP schemas.
- Commit, push, publish issues, or alter user-level configuration.

## Design

### Orca task identity

The creation title is:

```text
issueops-v1 lifecycle=<id> intent=<16 hex chars>
```

The intent digest is the first 16 lowercase hex characters of SHA-256 over the
trimmed full marker, a newline delimiter, and the normalized full prompt
SHA-256. The title must be at most 80 characters for a valid execution-v1
lifecycle ID. Creation and current reconciliation use only this compact title.

Legacy Orca-normalized titles are ignored during reconciliation. Only the
compact title derived from the current sealed intent is authoritative.

### Native actor ancestry

`remotecmd` owns one helper that constructs a `NativeActorV1` from CLI flags
and the ancestry observed from the current process. Both remote artifact
verification and confirmed PR creation use the same observation boundary.
Dry-run PR previews do not require process observation. Observation failures
are returned before any confirmed provider mutation instead of being collapsed
into an empty ancestry.

### Activation ordering

`scripts/install-native.sh` keeps glab synchronization best-effort, but performs
it before `issueops install-native`. The Go installer therefore reads and
seals the final Codex and Claude configuration bytes.

### Hook recovery guidance

The lifecycle guard cannot render a valid claim or replacement command because
it does not own the current native process receipt. Claimable, revoking, and
released lease denials therefore return only:

```text
issueops execution status --id <id> --json
```

Status remains actor-free and is the documented entry point for obtaining the
next generation-fenced command.

## Verification

- Orca adapter tests cover the 80-character ceiling, intent sensitivity, and
  legacy-title rejection.
- Remote CLI tests prove the current process ancestry reaches the PR actor.
- Install contract tests prove glab synchronization precedes activation seal.
- Lifecycle matrix tests prove every writerless lease state returns status.
- Run targeted packages, `go test ./... -count=1`,
  `go test -race ./... -count=1`, and `go build ./cmd/issueops`.
