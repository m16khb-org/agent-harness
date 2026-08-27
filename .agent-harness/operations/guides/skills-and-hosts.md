---
name: skills-and-hosts
description: Native skills, host hook rules, and the harness hook kill-switch.
---

# Skills, Hosts, and Hook Kill-Switch

This guide owns the harness hook kill-switch and routes to host skill and MCP
registration detail. Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Hook Kill-Switch

Harness hooks are registered once at host level (`~/.claude/settings.json`,
`~/.codex/hooks.json`, and `~/.omo/extensions/agent-harness.js`), so they fire
in every repository the agent opens — including repositories the harness does
not own. `HARNESS_DISABLE_HOOKS` turns that registration into a no-op for one
session without editing host settings:

```bash
HARNESS_DISABLE_HOOKS=1 claude    # 이 세션에서만 harness hook 미적용
export HARNESS_DISABLE_HOOKS=1    # 셸 세션 전체에 적용
```

- 값이 `1`, `true`, `yes`, `on` 중 하나면 활성이다 (`hookenv.Bool` 계약).
- context hook(`session-start`, Omo용 `post-compact`)이 아무것도 출력하지 않고 exit 0으로 끝난다. hook은 원래 telemetry나 state를 남기지 않는다.
- 다른 hook subcommand는 없다(2026-08-27에 legacy enforcement/relay/telemetry hook 제거).

## Related references

- [operations/hosts.md](../hosts.md): Codex/Claude/Omo native skills, MCP
  registration, lifecycle hook behavior, and the IssueOps host authority rule.
