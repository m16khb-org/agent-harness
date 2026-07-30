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

`agent-harness`는 Codex와 Claude Code 두 호스트에서 같은 방식으로 사용할 수 있는 개인 에이전트 하네스 프로젝트다.
이 문서는 에이전트가 이 저장소에서 작업할 때 먼저 읽어야 하는 루트 규칙이다.

<!-- project rules -->

## 0. 검증 우선 — 추측·회피 금지 (Verify, don't hedge)

**확인할 수단이 있으면 반드시 확인한 뒤 답한다. "미검증/확실치 않음/문서에 없어서 모름/사용자만 볼 수 있어 모름" 같은 회피성 단서를 다는 것은 금지다.**

- 주장이나 caveat이 도구로 검증 가능하면(소스·바이너리 grep, 명령 실행, 코드 실행, 로그·전사 확인) **답변·완료 선언 전에 반드시 검증한다.** 검증 안 한 추측을 결론처럼 말하지 않는다.
- 호스트/외부 도구의 동작(예: Claude Code·Codex의 hook 출력 렌더링, CLI 플래그 의미)이 공식 문서에 없으면 **설치된 CLI 바이너리/번들을 직접 grep**해서 실제 구현으로 확인한다. (`~/.local/share/claude/versions/*`, Codex pnpm vendor 바이너리 등)
- "사용자 화면에만 보여서 나는 모른다"는 핑계 금지: 렌더링 코드·스키마를 까보면 동작을 확정할 수 있다.
- 검증이 **물리적으로 불가능할 때만**(외부 secret 필요, 실행 불가 환경 등) 한계를 남기고, 그때도 **무엇을·왜 못 했는지와 시도한 검증 경로**를 함께 적는다.
- 결론에는 항상 **근거(파일:라인, 명령 출력, 바이너리 grep 결과)**를 함께 제시한다. 근거 없는 단정·근거 없는 회피 둘 다 금지.

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
- `.agent-harness/AGENT_WORKFLOW.md`: 에이전트 시작·작업·검증·완료 흐름과 MCP/문서 사용 규칙
- `skills/self-verify/SKILL.md`: 자기 검증 루프 실행 계약
- `skills/self-augment/SELF_AUGMENTATION.md`: 자가 증강 루프의 95점 종료 게이트, 테스트/QA/개선 실행 계약

충돌 시 우선순위는 **현재 사용자 지시 → 가장 가까운 `AGENTS.md`/`CLAUDE.md` → 루트 `AGENTS.md` → `.agent-harness/CONSTITUTION.md` → 나머지 project docs → README/과거 계획** 순서다.

## 7. Working Contract

- 핵심 동작은 host-specific plugin에 넣지 말고 Go core/port에 둔다.
- **Sub-agent 사용 원칙:** 메인 에이전트가 직접 작업을 수행한다. Sub-agent는 12가지 검증된 net-positive 패턴(악마의 변호인, 대량 탐색, 병렬 독립 연구, 격리 작업 등 — `.agent-harness/SUB_AGENT_PATTERNS.md` 참조)에만 예외적으로 사용한다. 단일 파일 편집, 전체 컨텍스트가 필요한 작업, 교차 아키텍처 판단은 sub-agent로 위임하지 않는다.
- Codex용 plugin/skill, Claude Code용 slash command/hook/MCP 설정은 core 호출을 위한 얇은 어댑터로 둔다.
- 공용 스킬은 `skills/<skill-name>/`을 source of truth로 두고, 기본 설치는 사용자 홈의 Codex/Claude skill 경로만 연결한다. 적용 대상 레포에는 명시적 `--project-local` 없이는 파일을 쓰지 않는다.
- 커밋 메시지는 `.agent-harness/COMMIT_POLICY.md`의 **Conventional Commit subject + Lore body** 형식을 따른다.
- CLI는 사람이 직접 실행해도 이해 가능한 JSON/text 출력을 제공해야 한다.
- MCP tool schema와 CLI JSON 출력은 호스트별로 다르게 만들지 않는다.
- command policy는 built-in catalog를 기본으로 하되 workspace별 `.agent-harness/policy.json` override를 매 평가마다 로드한다. load/parse 문제는 기존 `warnings` 필드로 노출하고, 전역 first-root cache를 만들지 않는다.
- IssueOps record JSON에는 `schema_version`이 포함된다. missing/zero legacy record는 현재 버전으로 읽고, future/unsupported schema는 fail-safe로 거부한다.
- local job worker는 workspace 경계, command policy, secret redaction, audit log가 준비된 뒤 도입한다. 현재 daemon은 MCP proxy backend다.
- 에이전트 state는 repo 소스와 분리한다. 추적해야 할 지식은 `.agent-harness/`에, 런타임 캐시/로그는 user state 또는 ignored workspace state에 둔다.

## 8. Planned Directory Map

| 경로 | 목적 |
|------|------|
| `cmd/harness/` | 단일 Go 바이너리 진입점. 현재 `inspect`, `preflight`, `docs`, `policy`, `state`, `issueops`, `loop`, `self-verify`, `self-augment`, `mcp` 제공 |
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
python3 scripts/validate-skill.py skills/atomic-commit-push
./scripts/install-native.sh
./bin/agent-harness bootstrap --dry-run
./bin/agent-harness install-native --dry-run --json
go test ./... -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness inspect --json
./bin/agent-harness docs --json
./bin/agent-harness daemon status --json
./bin/agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
tmp_state="$(mktemp -d)" && HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness state migrate --json && rm -rf "$tmp_state"
tmp_state="$(mktemp -d)" && HARNESS_STATE_DIR="$tmp_state" ./bin/agent-harness loop start --repo "$PWD" --name smoke --goal "smoke loop contract" --json && rm -rf "$tmp_state"
./bin/agent-harness self-verify --seed=100 --target-score=95 --llm-eval=false --json
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
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

- Codex와 Claude Code에서 관찰되는 하네스 결과가 같아야 하며 `agent-harness update`가 두 호스트를 함께 갱신한다.
- 같은 스킬을 두 host에 복사해 중복 관리하지 않는다. `skills/`의 단일 원본을 사용자 홈 skill 경로에서 참조한다. 적용 대상 repo에는 기본 설치가 파일을 남기지 않는다.
- 하네스 설치·업데이트·검증 경로는 독립 실행 가능해야 한다. 외부 도구가 필요하면 사용자가 해당 도구의 공식 경로로 별도 설치하고, agent-harness는 그 설치를 대행하거나 readiness gate로 요구하지 않는다.
- 외부 도구의 전문 기능은 agent-harness core에 복제하지 않는다. 필요한 통합은 파일/프로세스/문서처럼 검증 가능한 일반 경계로만 다룬다.
- host adapter는 인증·권한·명령 실행 정책을 우회할 수 없다.
- response-contract golden은 CLI command list, MCP tool list, 필수 response fields를 고정한다. docs-index는 문서 본문/목록 churn에 과민하지 않도록 required-doc projection과 count/schema만 검증한다.
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

- Endpoint/controller/DTO/schema/OpenAPI 변경 시 `agent-harness api-doc static-check --json` 또는 MCP `api_doc_static_check` 후 `api_doc_review`로 host-agent prompt/schema를 렌더하고, 리뷰 결과 JSON을 `--result`/`result_file`로 기록한다.
- 대상 Node/Nest repo에 `npm run swagger:check`가 있으면 그 wrapper를 우선 사용한다.
- 기본 검사는 git 변경분의 API candidate files로 제한하고, 기존 레거시 전체 Swagger 부채를 이번 변경의 실패 원인으로 삼지 않는다.

- API 문서 검사는 decorator/comment 존재 여부만 보지 말고 변경 endpoint가 호출하는 business logic의 public error contract(404/403/409 등)도 OpenAPI 응답에 반영됐는지 확인한다.

<!-- OPENWIKI:START -->

## OpenWiki

This repository uses OpenWiki for code documentation. Start with `openwiki/quickstart.md`, then follow its links to architecture, workflows, domain concepts, operations, integrations, testing guidance, and source maps.

OpenWiki 갱신은 자동으로 실행하지 않는다. 필요할 때 사용자가 `openwiki code --update --print`를 수동으로 실행하고 변경 diff를 검토한다. 명시적 요청 없이 생성된 OpenWiki 페이지를 직접 수정하지 않는다.

<!-- OPENWIKI:END -->
