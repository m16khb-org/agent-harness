---
name: CAUTIONS.md
description: 반복되는 실수와 운영상 주의점을 담는다. 과거에 무엇이 잘못됐고 어떻게 피하는지 알 수 있다.
---

# 주의사항 모음

`agent-harness`에서 반복적으로 실수하기 쉬운 설계·운영 주의사항을 모은다.

---

## 1. Host-specific lock-in

Codex plugin 또는 Claude Code hook에 핵심 로직을 넣으면 다른 host에서 같은 동작을 재사용할 수 없다.

주의:
- core behavior는 Go core에 둔다.
- plugin/skill/slash command/hook은 CLI/MCP 호출 wrapper로 제한한다.
- host adapter가 늘어날수록 contract test로 결과 동일성을 확인한다.

---

## 2. Plugin-only 착각

plugin 방식은 설치 UX에는 좋지만, Codex와 Claude Code가 같은 plugin runtime을 공유하지 않는다.

주의:
- plugin은 배포/발견/문서화 layer로 본다.
- 장기 상태, command policy, audit log는 외부 core/worker가 담당한다.

---

## 3. 위험한 shell 실행

에이전트 하네스에서 shell runner는 가장 위험한 기능이다.

주의:
- argv 실행을 기본으로 하고 shell string 실행은 예외로 둔다.
- cwd, timeout, env, write/network 허용 여부를 명시한다.
- stdout/stderr는 redaction 후 저장/반환한다.
- workspace root 밖 파일 접근을 기본 거부한다. `cwd`뿐 아니라 path-like argv(`../`, `/abs/path`, `--flag=/abs/path`, `~/path`, symlink escape)도 경계 검사를 통과해야 한다.

---

## 4. Secret leakage

agent prompt, logs, MCP responses, test failures에 secret이 쉽게 섞일 수 있다.

주의:
- token/key/password-like pattern은 adapter 경계에서 마스킹한다.
- fixture secret은 실제 값을 쓰지 않는다.
- command echo와 verbose log를 기본 비활성화한다.

---

## 5. Worker lifecycle 문제

persistent worker는 편하지만 stale lock, orphan process, socket 권한, 오래된 binary 문제가 생긴다.

주의:
- MVP에서는 CLI/MCP one-shot을 먼저 안정화한다.
- worker 도입 시 health check, version handshake, graceful shutdown, stale lock cleanup을 구현한다.
- socket path와 permission을 문서화하고 테스트한다.

---

## 6. State 위치 혼동

프로젝트 지식과 런타임 state가 섞이면 repo가 오염되고 secret이 커밋될 수 있다.

주의:
- 추적할 지식은 `.agent-harness/`에 둔다.
- cache/log/runtime state는 user state dir 또는 ignored `.harness/`에 둔다.
- `.harness/`를 도입하면 `.gitignore`에 추가한다.

---

## 7. MCP schema drift

CLI와 MCP가 서로 다른 응답 의미를 갖기 시작하면 host별 동작이 갈라진다.

주의:
- CLI JSON과 MCP response는 같은 core DTO를 공유한다.
- schema 변경은 golden test와 migration note를 남긴다.
- tool 이름과 field 이름은 안정적으로 유지한다.

---

## 8. Shared skill drift

Codex용 skill과 Claude용 skill을 복사본으로 따로 두면 금방 내용이 갈라진다.

주의:
- `skills/<name>`을 원본으로 둔다.
- 기본 설치는 `~/.codex/skills/<name>`과 `~/.claude/skills/<name>`만 중앙 원본으로 연결한다.
- `.claude/skills/<name>` 같은 repo-local 연결은 적용 대상 repo에 커밋될 수 있으므로 명시적 project-local 모드에서만 만든다.
- 스킬 수정 후 user-level host 경로가 같은 원본을 가리키는지 확인한다.

---

## 9. 자기 검증/자가 증강 drift

자기 검증 루프가 실제 native integration과 QA gate를 검증하지 않으면 문서만 통과하는 가짜 안정성이 생긴다. 자가 증강 루프가 실제 diff를 만들지 않으면 단순 분석 루프로 퇴화한다.

주의:
- 새 CLI/MCP/native skill 기능은 `agent-harness self-verify`의 테스트 또는 QA 단계에 smoke/fuzz evidence label로 승격한다.
- 반복 횟수 10회 하한을 임의로 낮추지 않는다.
- temp git repo 외 실제 사용자 repo에서 commit/push를 수행하지 않는다.

---

## 10. 과도한 초기 추상화

처음부터 remote server, distributed queue, plugin marketplace packaging을 만들면 개인 하네스 MVP가 늦어진다.

주의:
- 1단계는 `agent-harness inspect`와 state/checkpoint 같은 작은 기능으로 시작한다.
- 반복 사용으로 필요가 확인된 기능만 worker/plugin layer로 승격한다.

---

## 11. LLM Wiki 재구현 금지

`agent-harness`는 llm-wiki vault, 검색, capture, SessionStart 주입을 직접 구현하지 않는다. LLM Wiki 기능이 필요하면 upstream `nvk/llm-wiki` plugin/portable AGENTS.md를 설치해 사용한다. 하네스 MCP/CLI에는 llm-wiki 전용 tool/resource를 다시 추가하지 않는다.

같은 원칙으로 CodeGraph와 claude-mem도 하네스 core에 복제하지 않는다. 이 프로젝트의 철학은 **바퀴를 재발명하지 않는다**이며, `scripts/install-native.sh --with-upstream-tools`는 upstream installer/plugin을 호출하는 opt-in convenience path일 뿐이다. companion tool이 실패해도 하네스 core contract를 약화하거나 adapter에 임시 구현을 넣지 말고 upstream 설치/문서 경로를 고친다.

예외: Codex native hook validator가 upstream companion plugin의 오래된/Claude 전용 출력 필드만 거부하는 경우에는, 설치/업데이트 단계에서 **기능 재구현 없이** 호환성 shim을 적용할 수 있다. 예를 들어 `suppressOutput`처럼 Codex 0.135.0에서 unsupported top-level field로 실패하는 값은 백업 후 제거하되, `hookSpecificOutput`, MCP 등록, worker 시작, context 주입 동작은 유지한다.

## 12. Daemon lifecycle drift

`agent-harness mcp`가 daemon을 자동 시작하므로 오래된 binary가 이미 떠 있으면 새 코드 검증과 실제 MCP 동작이 갈라질 수 있다.

주의:
- 설치/빌드 후 MCP smoke 전에는 필요하면 `agent-harness daemon stop --json`으로 기존 daemon을 내린다.
- 테스트는 `HARNESS_DAEMON_DIR=$(mktemp -d)/daemon`으로 실제 user daemon과 분리한다.
- daemon socket/pid/log는 user state dir에 두고 repo나 wiki vault에 쓰지 않는다.

## MCP tool-use risks

- Broad tool descriptions make agents over-call tools or pass wrong arguments.
- Always injecting all project documents at session start wastes context and can hide task-specific evidence.
- Writable tools need explicit write semantics; prefer dry-run or append-only behavior.
- Tool output is evidence, not proof: verify file existence, warnings, and command/test results before claiming completion.
