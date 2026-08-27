---
name: cli-and-state
description: CLI policy, state maintenance, contract conformance, and smoke checks.
---

# CLI, State, and Command Policy Operations

Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Tool Contract Conformance

```bash
agent-harness contract conformance baseline --json
HARNESS_TOOL_CONFORMANCE_LIVE=1 agent-harness contract conformance live \
  --hosts codex,claude \
  --model codex=default \
  --model claude=default \
  --profile clean \
  --target-completed 1 \
  --max-attempts-per-case 3 \
  --evidence-dir .agent-harness/evidence/tool-conformance \
  --json
agent-harness contract conformance replay --fixture PATH --json
```

`baseline`과 `replay`는 deterministic local gate다. `live`는 opt-in 외부 model
비용 경계이며, opt-in env가 없으면 host process를 시작하지 않는다. Codex는
ephemeral/read-only/ignore-user-config로 실행하고 Claude는 임시 MCP config와
settings-source를 격리한다. 사용자 MCP 등록이나 credential DB는 수정하거나
복사하지 않는다.

Initial `live` report:
- `defer_hardening`: preregistered matrix에서 confirmed drift가 없으므로
  production contract를 변경하지 않는다.
- `needs_reproduction`: report가 지정한 한 host와 fixture만 별도
  10/20-completed batch로 재현한다.
- `authorize_hardening`: 같은 normalized signature를 두 번 이상 관측한 경우에만
  허용한다.

상세 denominator와 fixture promotion 규칙은 `.agent-harness/TESTING.md`에 있다.

## State Store Maintenance

Context hooks never maintain SQLite stores. Run `state maintain` manually when
needed:

```bash
# Checkpoint WAL and repair sidecar permissions on all known store roots
agent-harness state maintain --json
```

`state maintain` is read-only (checkpoint + chmod); it does not delete rows or
recover IssueOps v1 leases.

## Quick Smoke

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

`policy check` computes a predictive evaluation and exits `0` even when its
payload says `allowed: false`; automation must parse `allowed` and
`deny_reasons`. Use `policy fake-run` or the enforcement surface when a denied
command must return nonzero; both print the evaluation before the policy-denied
exit.

For deeper verification, use `.agent-harness/operations/verification.md` and `.agent-harness/TESTING.md`.

## Related references

- [operations/cli-and-mcp.md](../cli-and-mcp.md): direct CLI, command policy,
  guard, state read/write/prune/maintain, loop contracts, daemon, MCP cleanup,
  worker, and contract/audit.
- [operations/verification.md](../verification.md): self-verify, self-augment,
  and the api-doc gate.
