# AGENTS.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```text
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

---

`agent-harness`는 Codex와 Claude Code 양쪽에서 같은 방식으로 사용할 수 있는 개인 에이전트 하네스 프로젝트다.
이 문서는 에이전트가 이 저장소에서 작업할 때 먼저 읽어야 하는 루트 규칙이다.

<!-- project rules -->

## 5. 프로젝트 결정 요약

| 항목 | 결정 | 이유 |
|------|------|------|
| 하네스 방식 | **외부 Go 하네스 코어 + 얇은 호스트 어댑터** | Codex plugin 전용 구현은 Claude Code와 공유하기 어렵다. 외부 CLI/MCP/worker 코어를 두면 양쪽에서 같은 동작을 재사용할 수 있다. |
| Plugin의 역할 | 핵심 로직이 아니라 **설치·문서·명령 호출 래퍼** | Codex/Claude별 확장점 차이를 어댑터에 격리한다. |
| 통합 표면 | 1차 CLI, 2차 daemon-backed MCP stdio proxy, 3차 local job worker | 모든 에이전트는 shell/CLI를 다룰 수 있고, Claude Code는 MCP 연동이 자연스럽다. MCP backend daemon은 공통 context/state에 쓰고, 장기 job worker는 필요성이 확인된 뒤 도입한다. |
| 구현 언어 | **Go** | 현재 로컬 toolchain이 Go 1.26.3이고, 단일 바이너리·동시성·CLI/MCP/daemon 구현 생산성이 Rust보다 유리하다. |

상세 근거와 단계별 계획은 `.agent-harness/ADR.md`를 따른다.

## 6. Required Reading / Agent Docs

작업 시작 전 다음 문서를 범위에 맞게 확인한다.

- `.agent-harness/CONSTITUTION.md`: 문서 우선순위, 안전·정확성·아키텍처 원칙
- `.agent-harness/ARCHITECTURE.md`: Codex/Claude 공용 하네스 구조, plugin vs worker 판단, 경계
- `.agent-harness/CONVENTIONS.md`: Go 패키지 구조, CLI/MCP/worker 구현 컨벤션
- `.agent-harness/COMMIT_POLICY.md`: Conventional Commit + Lore body 하이브리드 커밋 규칙
- `.agent-harness/TESTING.md`: 문서/코드 변경 검증 기준
- `.agent-harness/OPEN_API_SPEC.md`: endpoint/DTO/OpenAPI 변경 시 정적+에이전트 문서화 게이트 프롬프트
- `.agent-harness/CAUTIONS.md`: 반복 실수와 운영 주의사항
- `.agent-harness/TECH_STACK.md`: 선택한 기술 스택과 예정 명령어
- `.agent-harness/ADR.md`: 구현 로드맵과 완료 기준
- `.agent-harness/OPERATIONS.md`: Codex/Claude native skill, MCP, CLI 사용법
- `skills/self-verify/SKILL.md`: 자기 검증 루프 실행 계약
- `skills/self-augment/SELF_AUGMENTATION.md`: 자가 증강 루프의 95점 종료 게이트, 테스트/QA/개선 실행 계약

충돌 시 우선순위는 **현재 사용자 지시 → 가장 가까운 `AGENTS.md`/`CLAUDE.md` → 루트 `AGENTS.md` → `.agent-harness/CONSTITUTION.md` → 나머지 project docs → README/과거 계획** 순서다.

## 7. Working Contract

- 이 저장소는 아직 애플리케이션 코드가 없는 초기 하네스 저장소다. 없는 소스 구조를 있다고 가정하지 않는다.
- 핵심 동작은 host-specific plugin에 넣지 말고 Go core/port에 둔다.
- Codex용 plugin/skill, Claude Code용 slash command/hook/MCP 설정은 core 호출을 위한 얇은 어댑터로 둔다.
- 공용 스킬은 `skills/<skill-name>/`을 source of truth로 두고, 기본 설치는 사용자 홈의 Codex/Claude skill 경로만 연결한다. 적용 대상 레포에는 명시적 `--project-local` 없이는 파일을 쓰지 않는다.
- 커밋 메시지는 `.agent-harness/COMMIT_POLICY.md`의 **Conventional Commit subject + Lore body** 형식을 따른다.
- CLI는 사람이 직접 실행해도 이해 가능한 JSON/text 출력을 제공해야 한다.
- MCP tool schema와 CLI JSON 출력은 호스트별로 다르게 만들지 않는다.
- local job worker는 workspace 경계, command policy, secret redaction, audit log가 준비된 뒤 도입한다. 현재 daemon은 MCP proxy backend다.
- 에이전트 state는 repo 소스와 분리한다. 추적해야 할 지식은 `.agent-harness/`에, 런타임 캐시/로그는 user state 또는 ignored workspace state에 둔다.

## 8. Planned Directory Map

| 경로 | 목적 |
|------|------|
| `cmd/harness/` | 단일 Go 바이너리 진입점. 현재 `inspect`, `preflight`, `docs`, `policy`, `state`, `self-verify`, `self-augment`, `mcp` 제공 |
| `internal/core/` | host와 무관한 하네스 usecase. 현재 inspect, preflight, docs index, state store, command policy/fake runner 구현 위치 |
| `internal/port/` | core가 의존하는 interface, 요청/응답 DTO |
| `internal/adapter/cli/` | Cobra/flag 기반 CLI adapter 예정 |
| `internal/adapter/mcp/` | MCP stdio server adapter 예정 |
| `internal/adapter/worker/` | local daemon, job queue, Unix socket/HTTP adapter 예정 |
| `internal/adapter/fs/` | filesystem, git, process runner 구현 예정 |
| `configs/` | Codex/Claude/MCP 설정 템플릿 |
| `.claude/skills/` | 명시적 `--project-local` 때만 생성되는 Claude Code project-native skill 연결. 기본 설치에서는 생성하지 않으며 git 추적 금지 |
| `.mcp.json` | 이 하네스 repo의 dogfood/project-local Claude MCP 설정. 기본 설치는 user-scope MCP를 사용하며 대상 repo에는 쓰지 않음 |
| `bin/agent-harness` | 빌드된 로컬 하네스 CLI/MCP 바이너리 |
| `skills/` | Codex/Claude가 공유하는 스킬 source of truth |
| `.agent-harness/` | 에이전트용 프로젝트 지식 베이스 |

문서·설정·실행 코드가 함께 존재하므로, 작업 전 실제 tree와 설치 상태를 다시 확인한다.

## 9. Essential Commands

현재 기본 검증:

```bash
find . -maxdepth 3 -type f | sort
find .agent-harness -maxdepth 1 -type f -name '*.md' | sort
python3 ${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-creator/scripts/quick_validate.py skills/atomic-commit-push
./scripts/install-native.sh
./bin/agent-harness bootstrap --skip-upstream-tools --dry-run
./bin/agent-harness update --skip-upstream-tools --dry-run
./bin/agent-harness install-native --dry-run --json
go test ./... -count=1
go test ./cmd/harness -run Golden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness daemon status --json
./bin/agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
tmp_state="$(mktemp -d)" && HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state migrate --json && rm -rf "$tmp_state"
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
codex mcp get agent_harness
claude mcp list
```

Go 코드가 추가된 뒤 표준 검증:

```bash
go mod tidy
go test ./... -count=1
go test -race ./... -count=1
go build -o bin/agent-harness ./cmd/harness
```

## 10. Critical Invariants

- Codex와 Claude Code에서 관찰되는 하네스 결과가 같아야 한다.
- 같은 스킬을 두 host에 복사해 중복 관리하지 않는다. `skills/`의 단일 원본을 사용자 홈 skill 경로에서 참조한다. 적용 대상 repo에는 기본 설치가 파일을 남기지 않는다.
- LLM Wiki 기능은 이 하네스가 재구현하지 않는다. 필요하면 upstream `nvk/llm-wiki` Codex/Claude plugin 또는 portable AGENTS.md를 사용한다.
- 하네스 철학은 **바퀴를 재발명하지 않는다**이다. llm-wiki, CodeGraph, claude-mem 같은 전문 도구는 `scripts/install-native.sh --with-upstream-tools`로 upstream installer/plugin을 연결하고, core 동작을 agent-harness에 복제하지 않는다.
- host adapter는 인증·권한·명령 실행 정책을 우회할 수 없다.
- worker/CLI/MCP는 workspace root를 명시적으로 식별하고, root 밖 파일 접근은 정책으로 통제한다.
- shell 실행 기능은 allowlist/denylist, timeout, cwd, env redaction, audit log를 포함해야 한다.
- secret 원문은 문서, 로그, 테스트 fixture, MCP 응답에 남기지 않는다.
- 장기 실행 worker는 stale lock, orphan process, socket permission, 로그 rotation을 고려한다.
- 문서와 구현이 어긋나면 현재 코드·설정 확인 결과를 기준으로 문서를 갱신한다.

## 11. Manual Notes

- 반복 실수나 운영 주의는 `.agent-harness/CAUTIONS.md`에 추가한다.
- 구현 규칙은 `.agent-harness/CONVENTIONS.md`, 테스트 규칙은 `.agent-harness/TESTING.md`, 기술 선택은 `.agent-harness/TECH_STACK.md`에 반영한다.
- 큰 설계 변경은 `.agent-harness/ADR.md`의 결정·로드맵을 함께 갱신한다.

## 12. API Documentation Gate

- Endpoint/controller/DTO/schema/OpenAPI 변경 시 `agent-harness api-doc check --json` 또는 MCP `api_doc_static_check` 후 `api_doc_review`를 실행한다.
- 대상 Node/Nest repo에 `npm run swagger:check`가 있으면 그 wrapper를 우선 사용한다.
- 기본 검사는 git 변경분의 API candidate files로 제한하고, 기존 레거시 전체 Swagger 부채를 이번 변경의 실패 원인으로 삼지 않는다.

- API 문서 검사는 decorator/comment 존재 여부만 보지 말고 변경 endpoint가 호출하는 business logic의 public error contract(404/403/409 등)도 OpenAPI 응답에 반영됐는지 확인한다.
