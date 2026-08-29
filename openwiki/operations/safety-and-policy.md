---
type: safety-command-policy
title: Safety & Command Policy
description: The safety model for command execution — the command-policy catalog and evaluation, workspace/cwd fences, the executable shell fence, secret redaction, the read-only executor surfaces across CLI/MCP/daemon/worker, and the fail-closed handling of ambiguous outcomes.
tags: [safety, policy, command-execution, redaction, audit, read-only, worker, shell, fail-closed, workspace-fence]
verified:
  - by: openwiki/0.4.3
    at: 2026-08-29T12:09:25.684Z
sources:
  - id: openwiki-source-e31e8beb2f56c36939086f18
    resource: repo://.agent-harness/architecture/runtime.md
  - id: openwiki-source-2bb2a4727058e4e642ca8676
    resource: repo://.agent-harness/CONSTITUTION.md
  - id: openwiki-source-a0f40a7179bb7b4d80495941
    resource: repo://.agent-harness/testing/unit-and-contract.md
  - id: openwiki-source-cc060a8ebe645afa42f0d4ce
    resource: repo://cmd/harness/gatescli/gates.go
  - id: openwiki-source-f35881ba4ad4a7ca820acdf3
    resource: repo://cmd/harness/harnessapp/policy_preflight_wiring.go
  - id: openwiki-source-d7459c384ef9ebb7eba8f20c
    resource: repo://cmd/harness/mcpcli/mcp_tool_assistant_worker.go
  - id: openwiki-source-87229c1e74ad7e40b120fc63
    resource: repo://cmd/harness/mcpcli/mcp_tool_policy_state.go
  - id: openwiki-source-bd9ed29c5293259c0fd3216a
    resource: repo://cmd/harness/mcpcli/resources/resources.go
  - id: openwiki-source-b12efa293fbf57a82727fee8
    resource: repo://cmd/harness/mcpcli/worker_mcp_test.go
  - id: openwiki-source-c4379071c6b66cf2c7c8b3b2
    resource: repo://cmd/harness/policycli/policy_cli.go
  - id: openwiki-source-0c0e37814f7889a545919b0b
    resource: repo://cmd/harness/policycli/policy_routes_test.go
  - id: openwiki-source-70317f5fb43f574e8096675f
    resource: repo://cmd/harness/statuscli/status_verify_work.go
  - id: openwiki-source-1075fd8ef671a0f7a8ba372d
    resource: repo://cmd/harness/testdata/response_contracts.golden.json
  - id: openwiki-source-6671c10a06ee03a5a73d936e
    resource: repo://cmd/harness/validationcli/commandpolicy/validation_command_policy_checks.go
  - id: openwiki-source-fb71c7f2829eec9a116d84a3
    resource: repo://cmd/harness/validationcli/commandpolicy/validation_command_policy.go
  - id: openwiki-source-f1414b92d365f69eb89d3234
    resource: repo://cmd/harness/validationcli/preflightfuzz/validation_preflight_fuzz.go
  - id: openwiki-source-8abf398006dffb0f4876451c
    resource: repo://cmd/harness/workercli/worker.go
  - id: openwiki-source-9492714ff9cd4f93470555ab
    resource: repo://internal/adapter/audit/audit_test.go
  - id: openwiki-source-db9e1e3f52c922ad5ca00008
    resource: repo://internal/adapter/audit/audit.go
  - id: openwiki-source-c8c3ee6439405b9202c1e140
    resource: repo://internal/adapter/gates/check.go
  - id: openwiki-source-451e00b03be8c40ebcfef4f2
    resource: repo://internal/adapter/gates/init.go
  - id: openwiki-source-a58f0ecb7ff7c8589e56692c
    resource: repo://internal/adapter/issueops/execution_remote.go
  - id: openwiki-source-1941b35adfb0211f4fab3396
    resource: repo://internal/adapter/policy/policy_catalog_test.go
  - id: openwiki-source-c0f20aaa3febc506226dacca
    resource: repo://internal/adapter/policy/policy_catalog.go
  - id: openwiki-source-00753f2c9d6ef93b39a39699
    resource: repo://internal/adapter/policy/policy_command_classification.go
  - id: openwiki-source-f119ca63ff02dd0d59869860
    resource: repo://internal/adapter/policy/policy_evaluate.go
  - id: openwiki-source-ce58b1496f6015362e355352
    resource: repo://internal/adapter/policy/policy_paths.go
  - id: openwiki-source-dac742634c31566e4f0644c5
    resource: repo://internal/adapter/policy/policy_pull_request_target.go
  - id: openwiki-source-e58bc86ddc3e59bc18be4c49
    resource: repo://internal/adapter/policy/policy_run.go
  - id: openwiki-source-38865592a310e8ea469c66a4
    resource: repo://internal/adapter/policy/policy_summary.go
  - id: openwiki-source-f22160a9e24c8ecb5dbd3c0c
    resource: repo://internal/adapter/policy/policy_test.go
  - id: openwiki-source-33c460260b93fb7c68c37061
    resource: repo://internal/adapter/worker/read_only.go
  - id: openwiki-source-6ff6e4ad125ca0ca70428aa6
    resource: repo://internal/adapter/worker/worker_lock.go
  - id: openwiki-source-a5f107c2c9a8f8e55726f1f8
    resource: repo://internal/adapter/worker/worker_test.go
  - id: openwiki-source-0e7055cdc28cfe7d87c16720
    resource: repo://internal/adapter/worker/worker.go
  - id: openwiki-source-897151a482f0ad3d91523502
    resource: repo://internal/application/issueopspublication/reconcile.go
  - id: openwiki-source-11199c8c97ece0cbe1a61e4b
    resource: repo://internal/contract/policy/command_types.go
  - id: openwiki-source-2f5010be48208ed7dc81e552
    resource: repo://internal/domain/auditid/audit_id.go
  - id: openwiki-source-e710fb050b6f0c5438c54919
    resource: repo://internal/domain/commandparse/issueops.go
  - id: openwiki-source-cbdb93341c6f6743b4c0f834
    resource: repo://internal/domain/commandparse/remotepullrequest.go
  - id: openwiki-source-e43857820eb13322ba25339c
    resource: repo://internal/domain/issueopsartifact/artifact.go
  - id: openwiki-source-698e28f909997a641fa3f4bc
    resource: repo://internal/domain/issueopsdecision/decision.go
  - id: openwiki-source-5098bb119460c63daa208f1e
    resource: repo://internal/domain/issueopspublication/decision.go
  - id: openwiki-source-09d61c4d32f4a33a80e14edc
    resource: repo://internal/domain/mcp/command_policy_catalog.go
  - id: openwiki-source-145a87dc4deceba9b272e884
    resource: repo://internal/domain/policy/decision.go
  - id: openwiki-source-d460fa174f7699b071567168
    resource: repo://internal/domain/policy/redaction.go
  - id: openwiki-source-7129cf488988b4954f7d153e
    resource: repo://internal/domain/secretdetection/secret.go
  - id: openwiki-source-856d94e18f54696909381c2b
    resource: repo://internal/domain/shelltoken/tokens_test.go
  - id: openwiki-source-c19013cbe9e28a27b458ea24
    resource: repo://internal/domain/shelltoken/tokens.go
  - id: openwiki-source-e81fffd4e0a92c55dcbba37d
    resource: repo://scripts/verify_skill_shell_test.py
  - id: openwiki-source-644c12cda0e5af240d06dd5e
    resource: repo://scripts/verify-skill-shell.py
generated: { by: "openwiki/0.4.3", at: "2026-08-29T17:13:20.810Z" }
---

# Safety & Command Policy

Command execution is the most dangerous capability agent-harness exposes, so it
is gated by a dedicated, core-owned command policy. Every path that can spawn a
process — the `policy` CLI, the MCP command-policy tools served by the daemon,
`worker run --read-only`, gates CHECK execution, and `status verify-work` —
passes through the same evaluation in `internal/adapter/policy` and the same
pure rules in `internal/domain/policy`, `internal/domain/shelltoken`, and
`internal/domain/secretdetection`. Host adapters (Codex plugin, Claude hook,
Omo extension) never bypass it, because the composition root injects the single
policy implementation into every consumer. This page documents the catalog, the
evaluation, the execution surfaces, the shell and secret fences, and the
fail-closed defaults that apply across CLI, MCP, daemon, and worker.

Related pages: [State, SQLite Store, and Locking](../concepts/state-and-sqlstore.md),
[Verification Gates](../testing/verification-gates.md),
<!-- openwiki: broken internal link [../workflows/execution-lease.md] file "../workflows/execution-lease.md" does not exist. Fix the href or restore the target, then delete this comment. -->
[Execution Lease](../workflows/execution-lease.md),
[Runtime Surfaces](../workflows/runtime-surfaces.md).

## Safety model and invariants

The project constitution (`.agent-harness/CONSTITUTION.md`, chapter 2) ranks
safety above correctness, readability, and performance and states the invariants
this page describes in code:

- Core policy lives in the Go core, never in a host adapter.
- No shell/process execution is added without an explicit `cwd`, timeout, env
  policy, and audit log.
- File access outside the workspace root is denied by default; anything else
  must be made explicit through policy.
- Secret plaintext is never left in logs, documents, test assertions, or
  MCP/CLI responses.
- Host plugins and hooks cannot bypass core policy.

The current implementation adds one deliberate exception to "no shell
execution": the policy layer can *evaluate* a shell-interpreter request only as
an explicitly justified exception tier, and no harness executor will actually
run one (see [Executable shell fence](#executable-shell-fence)).

## The command-policy catalog

`internal/adapter/policy/policy_catalog.go` is the source of truth for what the
policy knows about commands. Classification always uses the resolved basename
of `argv[0]` (case-insensitive) plus, when relevant, the lowercased `argv[1]`
subcommand:

| Category | Built-in commands | Subcommand sets |
| --- | --- | --- |
| Shell interpreters | `sh`, `bash`, `zsh`, `fish`, `dash`, `ksh` | — |
| Network | `curl`, `wget`, `ssh`, `scp`, `sftp`, `rsync`, `gh`, `brew`, `npm`, `pnpm`, `yarn`, `pip`, `pip3` | `git clone/fetch/pull/push/ls-remote/submodule` |
| Write | `touch`, `mkdir`, `rmdir`, `rm`, `mv`, `cp`, `install`, `chmod`, `chown`, `tee`, `python`, `python3`, `node`, `ruby`, `perl` | `git add/commit/reset/clean/checkout/switch/merge/rebase/cherry-pick/revert/push/pull/apply/am/stash`, `go build/test/run/install/mod/work/generate` |
| Read-only | `pwd`, `ls`, `cat`, `grep`, `rg`, `find`, `sed`, `awk`, `head`, `tail`, `wc`, `test`, `stat`, `true`, `false` | `git status/diff/log/show/rev-parse/branch/remote/ls-files/grep/describe/merge-base/config`, `go version/env/list` |

A command classified as network or write is denied unless the request
explicitly grants that capability; a request without `write_allowed` must also
land in the read-only allowlist. `CommandPolicySummary()` exposes the full
catalog (plus the secret-path and secret-assignment patterns) under its
`catalog` field, which the `harness://command-policy` MCP resource serves as
JSON so agents can read the rules without shell access.

## Workspace policy overrides (`policy.json`)

A workspace can widen the built-in catalog by shipping
`.agent-harness/policy.json` at the workspace root. The rules are deliberately
narrow:

- **Additive only.** Every field (`additional_shell_interpreters`,
  `additional_network_commands`, `additional_network_subcommands`,
  `additional_write_commands`, `additional_write_subcommands`,
  `additional_read_only_commands`, `additional_read_only_subcommands`) merges
  into the built-in sets; an override can never remove a built-in entry, so a
  repo file cannot un-deny `sh` or `curl`.
- **Reloaded on every evaluation.** `policyCatalogForWorkspace` rebuilds the
  catalog from the built-in snapshot and re-reads the file for each
  `EvaluateCommandPolicy` call. There is deliberately no global first-root
  cache: edits take effect immediately, and a stale workspace cannot poison
  evaluations of another root (a dedicated test pins that overrides do not leak
  across workspace roots).
- **Fail-soft to warnings, not errors.** A read or JSON parse problem surfaces
  as the warnings `policy_override_read_failed` / `policy_override_parse_failed`
  on the evaluation, and the built-in catalog is used untouched. A missing file
  is normal and warning-free.
- **Scope-local.** Overrides apply only to requests whose `workspace_root` is
  the repo that ships them.

## Evaluation

`EvaluateCommandPolicy` is a pure decision function over the request DTO. The
mandatory policy fields are:

| Field | Meaning |
| --- | --- |
| `workspace_root` | Fence boundary; must exist and be a directory |
| `cwd` | Working directory; must exist and be inside `workspace_root` |
| `argv` | Token array — shell strings are not accepted; argv takes precedence over a shell string |
| `timeout` | Duration string; default `30s`, maximum `15m` |
| `env_allowlist` | Environment variable names the child may inherit |
| `network_allowed` | Whether network-class commands may run |
| `write_allowed` | Whether write-class commands may run |
| `audit_log_id` | Caller-supplied correlation id; generated when empty |

(`shell_allowed` / `shell_reason` extend the request for the shell-exception
tier but the executors force them off.)

The evaluation accumulates **deny reasons** rather than stopping at the first
failure, then sets `Allowed = len(deny_reasons) == 0`. The default-denied
families are: missing/invalid `workspace_root` or `cwd`, `cwd_outside_workspace`,
empty `argv`, `invalid_timeout` / `timeout_exceeds_15m`, invalid env names,
`secret_like_argument`, `path_outside_workspace`,
`shell_interpreter_not_allowed` / `shell_reason_required`,
`network_not_allowed`, `write_not_allowed`,
`command_not_in_read_only_allowlist`, and the IssueOps-aware
`pr_target_branch_required` / `pr_target_branch_mismatch`. A missing or broken
`policy.json` appears in `warnings`, never in `deny_reasons`.

The **default posture** is asymmetric on purpose: read-only inspection is
permissive (anything in the read-only allowlist runs without extra flags),
while write, network, and shell each start narrow and require an explicit
capability flag. Every evaluation also reports a **tier** — a named privilege
envelope synthesized from the capability flags, ordered by ascending privilege:

1. `read_only` — no capability requested; restricted to the read-only allowlist.
2. `workspace_write` — `write_allowed` granted; network and shell remain denied.
3. `network_access` — `network_allowed` granted; shell remains denied.
4. `shell_exception` — `shell_allowed` plus `shell_reason`; audited.

Auto-escalation/"YOLO" tiers are deliberately excluded so a single approval can
never raise a whole session's safety level; the tier classifies what a request
may *attempt* while `DenyReasons` decides the specific command.

<!-- openwiki: mermaid parse failed and this diagram was converted to a text fence so it does not break rendering. Fix the diagram source and restore the mermaid fence. Parser error: Heuristic: an unescaped angle bracket inside a label breaks rendering; rephrase the label. -->
```text
flowchart TD
    REQ["Command request<br/>workspace_root cwd argv timeout<br/>env_allowlist capability flags"] --> EVAL["EvaluateCommandPolicy<br/>catalog reloaded every evaluation<br/>built-ins plus policy.json overrides"]
    EVAL --> CH{"Accumulated deny checks"}
    CH -->|"cwd or path outside workspace"| NO["Deny reasons non-empty"]
    CH -->|"shell interpreter without allowed flag and reason"| NO
    CH -->|"network or write class not granted"| NO
    CH -->|"not in read-only allowlist"| NO
    CH -->|"secret-like argument"| NO
    CH -->|"all checks clear"| YES["Allowed, tier assigned<br/>read_only, workspace_write,<br/>network_access, shell_exception"]
    YES -->|"policy check<br/>command_policy_check"| A1["Evaluation result only"]
    YES -->|"policy fake-run<br/>command_fake_run"| A2["Fake result, Executed false<br/>command never runs"]
    YES -->|"policy audit<br/>command_policy_audit"| A3["Append redacted JSONL<br/>no execution"]
    YES -->|"policy run --read-only<br/>worker_run_read_only, gates CHECK"| A4["Execute argv read-only<br/>no write network shell<br/>redacted, capped output"]
    NO --> B1["Denied, exit code 3<br/>nothing executed"]
```

*Command-policy evaluation and the surfaces that consume its verdict. Denial
reasons accumulate before the verdict, and no surface can execute a denied
command.*

## Evaluation surfaces and the read-only executor

The same evaluation is reachable from every runtime surface, with one verdict
and one DTO shape per result type:

| Surface | CLI | MCP tool (daemon-backed) | Behavior |
| --- | --- | --- | --- |
| Evaluate only | `policy check` | `command_policy_check` | Returns the evaluation; never executes |
| Prove acceptance | `policy fake-run` | `command_fake_run` | Policy result + audit id only; `executed` is always `false` |
| Audit | `policy audit` | `command_policy_audit` | Evaluates and appends a redacted JSONL record; never executes |
| Execute read-only | `policy run --read-only`, `worker run --read-only` | `worker_run_read_only` | Runs the argv for real with write/network/shell forced off |

Key properties of these surfaces:

- **fake-run never executes.** `FakeRunCommand` evaluates the policy and
  returns the policy result plus the audit id; the contract test asserts that
  even an allowed `touch marker` leaves no file behind. An accepted fake-run
  prints `fake-run accepted by policy; command was not executed`; a denied one
  exits with code 3 and the joined deny reasons.
- **`policy run` currently requires `--read-only`** — there is no writable
  command execution surface in the CLI, MCP, or worker plane.
  `RunReadOnlyCommand` forces `write_allowed`, `network_allowed`, and
  `shell_allowed` to `false` on the request *before* evaluation, so callers
  cannot widen it; a `touch` request that lies about write intent is denied
  with `write_not_allowed` and never runs.
- **The worker plane is policy-gated read-only.** `worker_run_read_only`
  (exposed by the daemon via the MCP stdio proxy) creates a worker evidence job
  and runs the command through the same read-only runner; the MCP conformance
  test pins that the worker never widens policy and that write, network, and
  shell requests fail the job with the exact deny reason.
- **The one general executor is gates CHECK.** `RunCommand` can execute with
  the requested privileges (`gates check` defaults to `--write=true`,
  `--network=false`, `env HOME,PATH`, 120s per CHECK), but it always forces
  `shell_allowed=false`, and a CHECK the policy denies is reported as
  `policy_denied` on the gate with a warning instead of executing. `status
  verify-work` uses only the read-only runner for its optional verification
  command.

## Workspace and cwd fences

The fence is enforced on two axes, both resolved against the real filesystem
rather than the string form:

- **cwd containment.** `workspace_root` and `cwd` must exist as directories,
  and the canonicalized `cwd` must be the root or inside it.
- **path-like argv containment.** For every argument after `argv[0]`, the
  policy extracts path candidates: absolute paths, `~`/`~/...` home shorthands
  (expanded with `os.UserHomeDir`), relative paths containing `/` or `./`/`..`,
  and `--flag=path` values. Each candidate is joined against `cwd`, then
  canonicalized with `filepath.EvalSymlinks` — including a walk that resolves
  the existing ancestor chain when the leaf does not exist yet — so a symlink
  inside the workspace pointing outside is denied just like a literal outside
  path. Remote references (`https://...`, `ssh://...`, `git@host:repo`) are
  recognized as non-paths and ignored.

The bundled test matrix covers relative parent escapes, absolute outside paths,
`--file=` flag values, symlink escapes, and `~/note.txt` home shorthands, plus
the negative case that an inside `../inside/local.txt` stays allowed.

## Executable shell fence

The shell fence is layered: the policy layer denies shell interpreters, a pure
lexical layer rejects shell *syntax* that argv execution would silently ignore,
and a documentation fence keeps dangerous shell out of shipped skills.

### Policy layer

If `argv[0]` is a shell interpreter, the evaluation demands
`shell_allowed=true` **and** a non-empty `shell_reason`; otherwise it denies
with `shell_interpreter_not_allowed` or `shell_reason_required`. A granted
exception adds a `shell_interpreter_exception` warning. No executor honors the
flag: `RunReadOnlyCommand`, `RunCommand`, and the worker runner all force
`shell_allowed=false`, so even the exception tier can never actually spawn a
shell through harness surfaces.

### Lexical shell analysis without running the shell

`internal/domain/shelltoken` answers "what would a shell do with this string?"
purely lexically:

- `SplitCommandTokens` tokenizes a command string into argv with POSIX-style
  quote and backslash handling, preserving empty `''`/`""` tokens (losing them
  derails exact flag parsing) and canonicalizing unquoted escapes.
- Detector functions distinguish *active* shell syntax from quoted or escaped
  literal data: `HasUnquotedControlOperator` (`;`, `&`, `|`, newline),
  `HasUnquotedBackgroundOperator` (lone `&`, but not `&&`, `2>&1`, `&>file`),
  `HasActiveCommandSubstitution` (backticks, `$(...)`, unquoted process
  substitution), `HasActiveOutputRedirect` / `HasActiveInputRedirect`,
  `HasActiveParameterOrTildeExpansion` (active even inside double quotes),
  `HasActivePathnameExpansion` (glob and brace expansion),
  `HasActiveShellSpecialQuoting` (`$'...'`, `$"..."` — can synthesize the
  executable name), `HasActiveZshEqualsExpansion`, and `HasActiveShellComment`
  (a word-start `#` that would silently drop the real operands).

Consumers fail closed: gates CHECK rejects any CHECK command with an unquoted
control operator before policy evaluation — because argv execution would turn
`;`/`|`/`&` into ordinary arguments and produce false `met` results — and runs
exactly one tokenized argv command, with the exit code deciding first and an
`EXPECT:` output anchor second. `gates init` refuses to scaffold such a spec.
`commandparse.ParseExactIssueOpsCommand` refuses any generated IssueOps command
carrying active control, substitution, redirect, or expansion syntax.

### Skill shell fence

`scripts/verify-skill-shell.py` (run by CI and documented in
`.agent-harness/testing/unit-and-contract.md`) scans shipped skill fences
(```bash/sh/zsh/shell/console) and checks, **without ever executing the
snippet**:

- **Syntax** — each non-`console` fence is syntax-checked with
  `<shell> -n`; documented `<placeholder>` forms are substituted before the
  check so they do not hide structural errors.
- **Failure swallowing** — `|| true` and `|| :` are rejected
  (`failure-swallow`); shell failures must be classified, not ignored.
- **Fabricated zeros** — `|| echo 0` and `${VAR:-0}` defaults are rejected
  (`fabricated-default`): a measurement failure must not become a plausible
  zero.
- **Destructive commands** — `git reset --hard`, `git clean --force`, `git
  branch -D`, `git rebase`, destructive `git bisect` subcommands, `git push
  --force`, and `rm -rf` require a preceding
  `<!-- skill-shell: destructive recovery="..." -->` annotation whose text
  names verifiable rollback or approval evidence; a `skip` annotation never
  bypasses the destructive policy.
- **Dynamic shell** — `eval`, `source`, `sh -c` in any launcher spelling, and
  command position `$`/backtick expansions are rejected
  (`dynamic-shell`), as is iterating command substitution through unquoted word
  splitting (`word-splitting-loop`, unquoted `git bisect run $VAR`).
- **Symlink bypass** — symlinked skill/reference trees are a
  `symlink-not-allowed` violation so a shipped contract cannot reach outside
  the scanned tree; local links to whole external skills directly under a
  scanned root are explicitly skipped instead.

Missing input paths, zero discovered skill contracts, and symlink violations
all exit `2` (usage-level failure) — a path typo can never pass as a clean scan.

## Secret redaction guarantees

Secret hygiene is enforced at input, output, and storage boundaries:

- **Denial and redaction in evaluation.** `SecretLikeArg` flags secret-looking
  arguments — `token=`/`password=`/`secret=`/`api_key=`/`credential=`/
  `authorization=` assignments, `Authorization: Bearer <value>` headers, and
  secret-like paths (`.env*`, `id_rsa`/`id_dsa`/`id_ecdsa`/`id_ed25519`,
  `*.pem`, `*.key`, `*.p12`, `*.pfx`, `*credentials*`, `*secret*`). Such an
  argument is denied outright (`secret_like_argument`) *and* replaced with
  `<redacted>` in the returned evaluation's `argv` and `shell_reason`, so even
  the denial report cannot leak the value.
- **Executed output.** stdout/stderr of a read-only or privileged run pass
  through `RedactFreeform` and are then capped at 32 KiB per stream with a
  `<truncated>` marker. Anything that re-renders free-form external text into
  responses or traces uses `RedactFreeform`/`RedactDiagnostic`/
  `BoundedDiagnostic` (the latter additionally replaces URLs).
- **Worker records.** `worker enqueue` redacts the job payload before
  persisting, and the read-only job's result is the already-redacted run
  result.
- **Durable docs and state.** `secretdetection.Contains` (assignment-like
  key/value pairs, PEM private-key blocks, `ghp_`/`glpat_`-style tokens, JWT
  shapes) blocks IssueOps artifact staging, decision bodies, and remote
  PR/MR titles/bodies before they are written, so no secret plaintext enters
  project docs or durable state; IssueOps remote diagnostics are stored
  bounded and redacted.
- **Fixtures.** The testing rules require secret-redaction tests to assert that
  token-like fixtures never survive into logs or responses — e.g. the audit
  test writes `token=secret-value` and then asserts the log contains
  `<redacted>` and not the secret.

## Audit trail

`policy audit` / `command_policy_audit` evaluate the request and append one
JSONL record (`kind: command_policy_audit`) containing the evaluation to
`HARNESS_AUDIT_LOG`, or `<state root>/audit/command-policy.jsonl`
(`HARNESS_STATE_DIR` honored), created with mode `0600` in a `0700` directory.
The command is never executed. Denials are audited too — the record's `ok`
mirrors `policy.Allowed`. Every evaluation carries an `audit_log_id`: either
the caller-supplied one or a generated id of the form
`audit-<UTC nanosecond timestamp>-<16-hex sequence>-<8-hex FNV-1a hash of
workspace, cwd, and argv>`, which lets any surface's decision be correlated in
the log.

## Fail-closed handling of ambiguous external outcomes

The same fail-closed instinct governs effects that harness cannot roll back:
**an ambiguous or unverified external outcome is treated as failure to be
reconciled — never retried on a guess.**

- The IssueOps remote PR/MR create path persists an exact operation intent
  *before* calling the provider and never retries an invocation whose outcome
  is ambiguous; a failure is recorded with the code
  `external_operation_ambiguous`, the invocation state (`unknown` vs
  `not_invoked_proven`), the retry count, and the known URL.
- Publication reconcile (`internal/application/issueopspublication`) inspects
  the provider first and only then chooses **adopt** (exactly one candidate,
  verified live), **preserve** (multiple candidates, non-authoritative zero,
  unproven non-invocation, or exhausted retry — intent retained), or **retry**
  (only after non-invocation is proven and the one-shot retry budget is
  unused). A retry whose outcome is again ambiguous is *not* retried again;
  the intent is preserved with `remote_reconcile_retry_ambiguous`.
- At the policy layer the same rule appears as the PR/MR target guard: when an
  active IssueOps cycle pins a base branch, `gh pr create` / `glab mr create`
  commands targeting a different branch (or with no target at all) are denied
  before the remote write happens, with the expected branch surfaced as a
  warning. With no active cycle the guard does not judge ordinary PRs.

## Focused tests and verification

The safety behavior is pinned by tests that stay runnable without network or
credentials:

- `internal/adapter/policy/policy_test.go` — allow/deny per catalog class,
  all path-escape shapes (parent, absolute, flag value, symlink, `~`),
  fake-run non-execution, read-only execution and denial with no marker file,
  and the empty-env-unless-allowlisted rule.
- `internal/adapter/policy/policy_catalog_test.go` — override semantics:
  additive merge, per-evaluation reload, no cross-root leak, invalid JSON warns
  and falls back to built-ins, and the full tier classification matrix.
- `internal/adapter/worker/worker_test.go` and
  `cmd/harness/mcpcli/worker_mcp_test.go` — the read-only worker job lifecycle
  under the span lock, denied commands failing the job without side effects,
  timeout producing exit code `124` with bounded stderr, and the MCP
  `worker_run_read_only` surface never widening policy.
- `cmd/harness/policycli` CLI tests and the response-contract goldens
  (`policy_check`, `policy_audit` shapes, `mcp_tools.golden.json` tool list)
  keep the CLI/MCP/daemon verdicts byte-compatible.
- The self-verify **command policy smoke**
  (`cmd/harness/validationcli/commandpolicy`) replays allow, outside-cwd deny,
  outside-path deny, shell deny, and fake-run non-execution against the built
  binary and fails if fake-run leaves its marker file. The **preflight fuzz**
  seeds a secret-like file (`.env` / `nested.secret`, alternating by seed) and
  asserts `preflight` reports the secret-like paths; the response contract
  additionally records the policy-path fuzz goals: symlink escape, `~/path`,
  remote URL/ref exceptions, and outside-workspace assertions.
- Core policy and adapter transport are tested separately, and command
  execution in tests uses fake runners that keep the same fail-closed
  discipline as the real gate — a fake that returns success for unknown input
  would silently bypass a new check.

```bash
go test ./internal/adapter/policy ./internal/domain/policy ./internal/domain/shelltoken ./internal/adapter/worker ./internal/adapter/audit -count=1
go test ./cmd/harness/policycli ./cmd/harness/mcpcli ./cmd/harness/harnessapp -count=1
python3 scripts/verify-skill-shell.py skills
python3 -m unittest discover -s scripts -p '*_test.py'
```
