---
name: cli-and-state
description: CLI discovery, command policy, guard, state maintenance, contract conformance, kubectl approval, and quick smoke.
---

# CLI, State, and Command Policy Operations

This guide owns CLI-driven operational procedures for the operations family.
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

`baseline`과 `replay`는 deterministic local gates다. `live`는 외부 model 비용 경계이며 opt-in env가 없으면 host process를 시작하지 않는다. Codex는 ephemeral/read-only/ignore-user-config 실행, Claude는 strict temp MCP config와 settings-source isolation을 사용한다. 사용자 MCP 등록이나 credential DB는 수정하거나 복사하지 않는다.

Initial live report가 `defer_hardening`이면 현재 preregistered matrix에서 confirmed drift가 없다는 뜻이며 production contract를 변경하지 않는다. `needs_reproduction`이면 report가 지정한 한 host+fixture만 별도 10/20-completed batch로 재현한다. `authorize_hardening`은 같은 normalized signature가 두 번 이상 관측된 경우에만 가능하다. 상세 denominator와 fixture promotion 규칙은 `.agent-harness/TESTING.md`를 따른다.

## State Store Maintenance

The sqlite-backed state stores accumulate WAL frames and sidecar files that need periodic checkpointing. Context hooks never maintain them; use the manual CLI when needed:

```bash
# Checkpoint WAL and repair sidecar permissions on all known store roots
agent-harness state maintain --json

```

`state maintain` is read-only (checkpoint + chmod); it does not delete rows.
IssueOps v1 lease recovery is not part of store maintenance and never happens
from a time threshold.

## Kubectl Live-Access Approval

With `--enforce-gitops-kubectl`, live access requires explicit confirmation. Claude uses its native `ask`. Codex cannot emit native PreToolUse `ask`, so the first eligible request blocks with a short instruction such as `승인 AH-XXXXXX`.

Codex can reuse approval only for exact-allowlisted read-only exec diagnostics that state both kube context and namespace. For example:

```bash
kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user
kubectl --context bc-stgdev -n stg exec -c linkerd-proxy deploy/rest-api-gateway -- curl -fsS http://localhost:4191/metrics
```

Enter the exact token in the same session. The approval must be activated by an allowlisted diagnostic within 10 minutes. The first allowed command and each later allowed command refresh a 30-minute idle TTL for the same session, canonical repo, context, and namespace; workload target and container may change. Changing context or namespace, allowing the TTL to expire, or losing state requires a new token. Runtime state uses mode `0600` and stores only request/scope fingerprints, never raw commands or cluster identifiers.

Codex `kubectl port-forward` remains exact-command one-shot: the next identical request consumes its 10-minute grant. Unsafe or unclassified Codex exec, including generic shells, interactive flags, arbitrary file/env reads, redirects, and non-allowlisted curl/dig options, blocks without an approval token. Do not remove `--enforce-gitops-kubectl` or use a generic shell as routine recovery. Direct mutating kubectl commands remain blocked and must go through GitOps.

## Quick Smoke

```bash
agent-harness inspect --json
agent-harness docs --json
agent-harness daemon status --json
agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

`policy check` is a predictive inspection command. A successfully computed
evaluation exits `0` even when its payload says `allowed: false`; automation
must parse `allowed` and `deny_reasons`. Use `policy fake-run` or the
enforcement surface when the process exit code must reject a denied command;
those return a nonzero policy-denied exit after printing the evaluation.

For deeper verification, use `.agent-harness/operations/verification.md` and `.agent-harness/TESTING.md`.

## Related references

- [operations/cli-and-mcp.md](../cli-and-mcp.md): direct CLI, command policy,
  guard, state read/write/prune/migrate, loop contracts, daemon, MCP cleanup,
  worker, and contract/audit.
- [operations/verification.md](../verification.md): self-verify, self-augment,
  and the api-doc gate.
