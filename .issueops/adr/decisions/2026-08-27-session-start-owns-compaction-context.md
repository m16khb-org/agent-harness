# 2026-08-27 — SessionStart owns compaction context; the legacy hook surface is removed

← [ADR index](../../ADR.md)

**결정:** Codex와 Claude의 기본 설치는 `SessionStart` 하나만 등록하고, 그 hook이 `startup`·`resume`·`clear`·`compact` 모든 source에서 project-doc catalog를 주입한다. `PostCompact`는 기본 등록에서 제거한다. `issueops hook`은 `session-start`와 `post-compact` 두 context subcommand만 남기고, `user-prompt`·`pre-tool-use`·`post-tool-use`·`pre-compact`·`stop`·`failures`·`metrics`와 그 전용 구현(hookprompt 라우팅, hookfailure/hookmetrics 원장, lifecycle 가드 체인, commandguard, nextaction, read-only 명령 문법, native runtime 진단, hook 트리거 유지보수)은 삭제한다.

이 기록은 [2026-08-10 default hooks are thin static context only](2026-08-10-default-hooks-thin-static-context.md)의 "`SessionStart`와 `PostCompact`만 등록한다"와 "legacy hook CLI는 compatibility/diagnostic surface로 보존한다" 두 조항을 대체한다. [2026-06-23 IssueOps hook and state-machine boundary](2026-06-23-issueops-hook-state-machine-boundary.md)가 hook에 남겨 둔 fast deterministic guard 역할도 사라진다. IssueOps authority가 `issueops ...` CLI/MCP에만 있다는 결론은 그대로다.

**근거 (설치된 host 바이너리와 전사로 확인, 2026-08-27):**

- Claude Code 2.1.247: `PostCompact` hook 결과는 `userDisplayMessage`(사용자 표시 문자열)로만 소비되고, `hookSpecificOutput` 해석 switch에 `PostCompact` case가 없다. 압축 뒤에는 `SessionStart`가 `source:"compact"`로 다시 실행된다(전사에 `SessionStart:compact` hook attachment 84건, `PostCompact` attachment 0건). 기존 하네스는 `SessionStart(compact)`에 빈 컨텍스트를 내고 `PostCompact`에 catalog를 실었으므로, 압축마다 모델은 catalog를 잃고 사용자 화면에는 raw JSON 한 줄이 남았다.
- Codex 0.150.1: `post-compact.command.output` JSON schema는 `continue`·`stopReason`·`suppressOutput`·`systemMessage`뿐이고, `session-start.command.input.source` enum에 `compact`가 있다. 공식 문서: "After Codex compacts a root session, SessionStart hooks that match source: 'compact' run before the next model request."
- Omo는 `session_compact`(accepted) 이벤트가 `session_start`와 별개이므로 확장이 `hook post-compact --json`을 계속 사용한다. `--json` 출력은 snake_case(`should_inject`, `compact`)로 맞춘다.
- legacy hook subcommand는 2026-08-10 이후 어떤 host에도 등록되지 않았고, 삭제 대상 구현은 `deadcode`로 main에서 도달 불가능함을 확인했다. 미등록 `pre-tool-use`가 외부 호출로 최근 3일간 957회 실행되며 호출당 약 70ms를 소모한 관측은 그 표면을 남겨 둘 이유가 아니라 지울 이유였다.

**결과:**

- 기본 hook 표면: Claude `~/.claude/settings.json`과 Codex `~/.codex/hooks.json`의 `SessionStart` 하나. upgrade는 known lifecycle event에서 issueops group만 제거하고 co-resident group 순서를 보존한다.
- 주입할 문서가 없으면 두 context hook 모두 `{}`를 낸다. `post-compact`의 host shape는 `systemMessage`만 싣는다.
- `hook --help`를 포함한 모든 subcommand의 `--help`는 exit 0이며 `flag: help requested` 줄을 남기지 않는다.
- `ISSUEOPS_DISABLE_HOOKS`는 남은 두 context hook을 무출력 no-op으로 만든다.

**거절:** `SessionStart`와 `PostCompact`를 모두 유지하는 방식. 두 host 모두 `PostCompact`에 모델 컨텍스트를 실을 수 없고, `SessionStart(compact)`가 이미 사용자 표시 채널(`systemMessage`, Codex TUI `hook context:`)까지 담당하므로 중복 표시만 남는다. legacy hook 코드를 진단 표면으로 계속 보존하는 방식도 거절했다. 등록되지 않은 enforcement 경로는 동작 증거를 만들지 못하면서 host 스키마 변화에 맞춰 유지비만 발생시켰다.
