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
- Codex와 Claude Code는 `SessionStart`만 등록하고 compact 뒤 다시 실행하므로 `PostCompact`를 등록하지 않는다.
- Omo extension은 `session_start`를 `hook session-start`에, `session_compact`를 `hook post-compact`에 매핑한다.
- `HARNESS_DISABLE_HOOKS`가 켜지면 두 context command 모두 조용히 exit 0으로 끝나며 telemetry나 state를 남기지 않는다.
- 다른 hook subcommand는 없다(2026-08-27에 legacy enforcement/relay/telemetry hook 제거).

## Related references

- [operations/hosts.md](../hosts.md): Codex/Claude/Omo native skills, MCP
  registration, lifecycle hook behavior, and the IssueOps host authority rule.
