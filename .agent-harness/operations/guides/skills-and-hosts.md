---
name: skills-and-hosts
description: Native skills, host hook rules, and the harness hook kill-switch.
---

# Skills, Hosts, and Hook Kill-Switch

This guide owns the harness hook kill-switch and routes to host skill and MCP
registration detail. Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Hook Kill-Switch

Harness hooks are registered once at host level (`~/.claude/settings.json` and the Codex equivalent), so they fire in every repository the agent opens — including repositories the harness does not own. `HARNESS_DISABLE_HOOKS` turns that registration into a no-op for one session without editing host settings:

```bash
HARNESS_DISABLE_HOOKS=1 claude    # 이 세션에서만 harness hook 미적용
export HARNESS_DISABLE_HOOKS=1    # 셸 세션 전체에 적용
```

- 값이 `1`, `true`, `yes`, `on` 중 하나면 활성이다 (`hookenv.Bool` 계약).
- 기본 설치 hook 이벤트(`session-start`, `post-compact`)가 검사 없이 exit 0으로 통과하고 telemetry나 state를 남기지 않는다.
- `agent-harness hook failures`와 `hook metrics`는 계속 동작한다. kill-switch가 끄는 것은 강제이지 관측이 아니다.
- 명시적으로 실행하는 legacy diagnostic hook CLI도 같은 kill-switch를 따른다.

## Related references

- [operations/hosts.md](../hosts.md): Codex/Claude native skills, MCP
  registration, lifecycle hook behavior, and the IssueOps host authority rule.
