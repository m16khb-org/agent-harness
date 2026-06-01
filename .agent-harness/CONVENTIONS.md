---
name: CONVENTIONS.md
description: Coding conventions, package structure, and layer boundaries.
---

# 구현 컨벤션

현재 `agent-harness`는 초기 문서 단계다. Go 코드가 추가되면 이 문서를 우선 적용한다.

---

## 1. 네이밍 / 구조

- 프로젝트 이름: `agent-harness`
- CLI 바이너리 이름: `agent-harness`
- Go module 이름은 repo 원격이 확정되면 `agent-harness`를 현재 로컬 module 이름으로 사용한다.
- 파일명은 snake_case를 사용한다.
- Go 패키지명은 짧은 소문자 단어를 사용한다.
- 테스트 파일은 대상 파일 가까이에 `*_test.go`로 둔다.

현재/예정 구조:

```text
cmd/harness/main.go
internal/core/docs.go
internal/core/project_docs.go
internal/core/inspect.go
internal/core/policy.go
internal/core/preflight.go
internal/core/state.go
cmd/harness/testdata/*.golden.*
internal/port/
internal/adapter/cli/
internal/adapter/mcp/
internal/adapter/codex/
internal/adapter/claude/
internal/adapter/installutil/
internal/adapter/worker/
internal/adapter/fs/
configs/codex/
configs/claude/
skills/
.agent-harness/
```

---

## 2. 레이어 경계

| 레이어 | 책임 | 의존 가능 | 금지 |
|--------|------|-----------|------|
| `core` | workspace/docs/state/policy/preflight/inspect usecase. 현재 host-neutral core 로직이 여기 있다 | `port`, 표준 라이브러리 | Codex/Claude SDK, CLI flag, MCP transport 직접 의존 |
| `port` | interface, DTO, error contract | 표준 라이브러리 | adapter concrete type 의존 |
| `adapter/cli` | flag/stdout/stderr/exit code | `core`, CLI library | 정책 복제 |
| `adapter/mcp` | MCP tool schema/transport | `core`, MCP library | CLI와 다른 의미의 응답 |
| `adapter/codex` | Codex user skill/MCP 설치 구현 | `core`, `port`, 표준 라이브러리 | 적용 대상 repo 파일 쓰기 |
| `adapter/claude` | Claude user skill/hook/MCP 설치 구현 | `core`, `port`, 표준 라이브러리 | 기본 설치에서 `.claude/skills` 같은 repo-local 파일 쓰기 |
| `adapter/worker` | local daemon, job lifecycle, IPC | `core`, stdlib/net | command policy 우회 |
| `adapter/fs` | filesystem/git/process 구현 | `port`, os/exec 등 | root 밖 접근을 암묵 허용 |

---

## 3. CLI 컨벤션

- subcommand는 동사 중심으로 짧게 둔다: `inspect`, `state`, `run`, `mcp`, `worker`.
- 사람이 읽는 기본 출력과 agent가 파싱하는 `--json` 출력을 구분한다.
- `--json` 출력 field는 snake_case를 사용한다.
- 실패 시 stderr에는 짧은 설명, JSON 모드에는 machine-readable error code를 포함한다.
- exit code는 의미 있게 유지한다.
  - `0`: 성공
  - `1`: 일반 실패
  - `2`: 잘못된 사용법/flag
  - `3`: policy 거부
  - `4`: workspace/config 문제

---

## 4. MCP 컨벤션

- MCP tool 이름은 `harness.<verb>` 형태를 사용한다.
- CLI와 MCP는 같은 core request/response DTO를 공유한다.
- tool response에는 불필요한 대용량 파일 내용을 싣지 않는다. 요약, 경로, hash, line range를 우선한다.
- command 실행 tool은 policy 결과와 audit log id를 함께 반환한다.
- schema 변경은 golden test와 문서 업데이트를 동반한다.
- CLI/MCP contract golden은 `cmd/harness/testdata/`에 둔다. 의도된 schema 변경일 때만 `go test ./cmd/harness -run Golden -update -count=1`로 갱신한다.
- 실제 JSON response golden은 dynamic field(timestamp, temp path, audit id)를 normalize해서 host/session 차이로 인한 drift를 막는다.
- response golden 범위는 state/policy뿐 아니라 docs/inspect/preflight처럼 agent가 자주 의존하는 읽기 표면을 우선 포함한다.

---

## 5. Worker 컨벤션

- worker는 로컬 전용으로 시작한다. 원격 API는 별도 요구가 생기기 전까지 만들지 않는다.
- Unix socket 또는 localhost binding을 사용하고, 권한을 제한한다.
- job은 idempotency key, timeout, cancellation을 갖는다.
- worker 시작/종료는 stale lock과 orphan process를 처리한다.
- 장기 작업 상태와 project lifecycle queue/profile은 user state dir에 저장하고, repo에 secret/state 원문을 쓰지 않는다. lifecycle state는 `projects/<repo-id>/` namespace로 격리해 같은 머신의 여러 repo가 섞이지 않게 한다.
- PostToolUse에서 생긴 draft-wiki 후보는 repo가 아니라 project-scoped user state queue에 bounded/redacted 텍스트로 저장한다. worker가 처리할 때도 shell string을 만들지 말고 `agy -p <prompt>` argv 실행만 사용한다.

---

## 6. Config / env

- env prefix는 `HARNESS_`를 사용한다.
- config 파일 기본 후보는 `~/.config/agent-harness/config.yaml`이다.
- workspace별 설정이 필요하면 `.harness/config.yaml`을 사용하되, secret 원문 저장은 금지한다.
- config 우선순위는 `flag → env → workspace config → user config → default` 순서로 둔다.

---

## 7. Logging / secret hygiene

- 구조화 로그를 사용한다.
- secret으로 보이는 값은 adapter 경계에서 redaction한다.
- command stdout/stderr를 그대로 저장하기 전에 redaction filter를 거친다.
- 로그에는 workspace, command id, duration, exit code를 남기되 token/key 원문은 남기지 않는다.

---

## 8. Dependency 원칙

- 새 dependency는 표준 라이브러리로 명확히 부족할 때만 추가한다.
- CLI/MCP library는 기능보다 안정성과 유지보수성을 우선한다.
- dependency 추가 시 `go.mod`, license, 보안 위험을 확인한다.
- 생성물은 직접 수정하지 않는다. 생성 스크립트와 source를 고친 뒤 재생성한다.

---

## 9. Shared Skill 컨벤션

- 공용 스킬 원본은 `skills/<skill-name>/`에 둔다.
- Codex/Claude별 user skill 경로는 원본으로 향하는 symlink 또는 installer로 연결한다. 적용 대상 repo의 `.claude/skills`는 기본 설치에서 만들지 않는다.
- skill 이름은 lowercase/digit/hyphen만 사용한다.
- 각 skill은 `SKILL.md`를 반드시 포함하고, Codex UI metadata가 필요하면 `agents/openai.yaml`을 둔다.
- host별 설치 대상이 다르면 `install.json`에 `{ "hosts": ["codex"] }` 또는 `{ "hosts": ["claude"] }`처럼 명시한다. 생략하면 모든 host에 설치하는 기존 동작을 유지한다.
- 스킬 안에는 README, 설치 가이드, changelog 같은 보조 문서를 만들지 않는다.
- 검증은 skill-creator의 `quick_validate.py`로 수행한다.

설치 adapter 규칙:

- `internal/core.InstallNative`가 host-neutral 설치 engine이고 `port.HostInstaller`가 SOLID 경계다.
- Codex/Claude adapter는 자기 host의 user/global 설정만 기본으로 쓴다. Codex는 `~/.codex/hooks.json`, Claude는 `~/.claude/settings.json`에 같은 lifecycle hook CLI를 등록한다. repo-local `.mcp.json`, `.claude/settings.json`, `.claude/skills`는 `--project-local` 같은 명시적 opt-in 없이는 만들지 않는다.
- symlink는 사용자 홈의 skill 경로에서 중앙 `skills/<name>`을 참조하기 위해서만 기본 사용한다.
- adapter 설치 계약을 바꾸면 `internal/adapter/install_contract_matrix_test.go`와 `internal/adapter/testdata/native_install_contract_matrix.golden.json`을 함께 갱신해 user/global 기본 설치와 explicit project-local opt-in의 차이를 보존한다.

---

## 10. 커밋 메시지 컨벤션

- 커밋 메시지는 `.agent-harness/COMMIT_POLICY.md`를 따른다.
- subject는 Conventional Commit 형식(`<type>(<scope>)!?: <summary>`)을 사용한다.
- body에는 AI agent가 맥락을 회수할 수 있도록 `Lore:` 블록을 둔다.
- atomic commit 하나에는 Lore `Intent`도 하나만 있어야 한다.
## API Documentation Gate

Reusable API documentation checks belong in `agent-harness api-doc check`, not in a single application repository or framework-specific hook. Keep the core prompt framework-agnostic: it may mention examples such as NestJS Swagger decorators, Go swaggo annotations, OpenAPI specs, Spring/FastAPI annotations, but must instruct reviewers to follow the target project's own convention and staged diff only. Project-specific strictness should be supplied via `--prompt-file` rather than hardcoded into harness core.

## Project Docs Bootstrap 컨벤션

- 적용 대상 repo의 `.agent-harness/` 문서는 명시적 `agent-harness project bootstrap` 또는 `$project-bootstrap` 실행 때만 생성한다.
- 기본 설치나 MCP read는 대상 repo에 파일을 쓰지 않는다.
- `AGENTS.md`는 전체 덮어쓰기 금지다. bootstrap은 behavioral top block이 없으면 prepend하고, 이후에는 `AGENT_HARNESS` marker block만 관리한다.
- 생성 문서에는 추론된 명령과 기술스택의 Evidence/Confidence를 포함한다.
- MCP 라우팅은 문서를 무조건 주입하지 않고 작업별로 읽어야 할 문서 경로와 이유만 제공한다.

## API Documentation Convention

API documentation gates are framework-agnostic and business-logic-aware. A changed endpoint's docs must reflect not only syntax-level params and DTO schemas but also directly reachable domain errors from service/usecase/error-mapping code. Keep Swagger/OpenAPI output clean: concise summaries, consistent sectioned descriptions, complete params, accurate required/optional fields, and explicit success/error responses.


NextCandle-style cleanliness means sectioned Markdown operation descriptions, complete params, accurate auth/error responses, DTO required/optional correctness, and Swagger outputs split/filtered for the intended audience where the framework supports it.

## Single-object response convention

단일 객체 응답은 불필요한 wrapper object로 한 번 더 감싸지 않는다. 공개 계약이 이미 하나의 resource/detail/view model이라면 controller/handler는 `{ data: object }`, `{ result: object }`, `{ item: object }`처럼 포장하지 말고 object fields를 top-level로 spread해 반환한다.

예외는 pagination/list envelope, metadata가 계약의 일부인 경우, backward compatibility 유지, 표준 error envelope처럼 wrapper 자체가 명시적 API 계약인 경우다. 새 endpoint나 DTO 리팩터링 시 OpenAPI schema도 이 규칙과 일치해야 한다.

## SOLID / Design Pattern 적용 지침

SOLID, YAGNI, KISS는 함께 적용한다. SOLID는 인터페이스와 계층을 많이 만들라는 뜻이 아니라, 실제 변경 축이 확인된 곳에서 책임과 의존 방향을 선명하게 하라는 지침이다. Design Pattern은 문제를 설명하고 유지보수를 줄이는 이름일 때만 사용한다.

### 좋은 케이스

- 기존 코드에 이미 있는 Adapter, Strategy, Factory, Repository 같은 패턴을 같은 문제에 일관되게 적용한다.
- 외부 host, SDK, filesystem, process, network처럼 교체 가능성이 실제로 있는 경계에 interface/port를 둔다.
- 두 개 이상의 구현이 있거나 테스트 double이 필요한 경계에서 dependency inversion을 사용한다.
- 책임이 섞인 코드에서 변경 이유가 서로 다른 부분을 작게 분리한다.
- 새 패턴을 도입할 때 ADR에 문제, 선택한 패턴, 기각한 단순 대안, 비용을 기록한다.

### 나쁜 케이스

- 단일 사용처를 위해 interface, factory, registry, plugin layer를 먼저 만든다.
- “미래 확장성”만 근거로 추상화하거나 설정 가능하게 만든다.
- 간단한 함수 호출을 패턴 이름에 맞추려고 class/object graph로 늘린다.
- 기존 repo 스타일과 다른 패턴을 작은 변경에 끼워 넣는다.
- SOLID를 이유로 core 정책을 host adapter에 복제하거나, host별 구현을 중복한다.

### 적용 규칙

- 먼저 가장 단순한 구현을 선택하고, 실제 variation point가 확인될 때 패턴을 도입한다.
- 새 abstraction은 최소 두 사용처, 명확한 테스트 경계, 또는 외부 기술 경계 중 하나가 있을 때만 만든다.
- 패턴 도입이 50줄 해결책을 200줄 구조로 만들면 되돌려 단순화한다.
- 패턴을 쓰는 경우 이름보다 계약을 문서화한다: 책임, 입력/출력, 금지된 의존 방향, 검증 방법.

## Hook 컨벤션

- UserPromptSubmit hook은 사용자의 prompt와 project-scoped lifecycle state를 분석해 MCP 후보 힌트만 주입한다. PreToolUse hook은 tool 실행 직전의 critical path에 있으므로 기본은 host stdout `{}`인 빠른 allow/no-op preflight로 유지하고, raw `--json` 진단만 노출한다. PostToolUse hook은 성공한 mutating tool 실행 이후 lifecycle upkeep queue 기록만 수행하며 read-only Bash/조회 output path로 upkeep을 만들지 않는다. Stop hook은 lifecycle reminder를 계산하되 Stop hook stdout schema 호환을 위해 host에는 빈 JSON 객체만 반환한다. PreCompact/PostCompact hook은 pending upkeep queue를 작은 user-state capsule로 저장하고 한 번만 복원한다. Codex와 Claude Code hook adapter는 같은 `agent-harness hook user-prompt/pre-tool-use/post-tool-use/pre-compact/post-compact/stop` CLI를 호출해야 한다. 어떤 hook도 작업을 대신 실행하거나 shared docs를 직접 수정하거나 긴 파일/네트워크를 읽지 않는다.
- PostToolUse hook의 draft-wiki 연동도 queue append까지만 허용한다. `agy -p` 호출과 `.agent-harness/draft-wiki/draft` 파일 쓰기는 `agent-harness worker draft-wiki`가 hook 밖에서 수행한다.
- Hook 출력은 event별 host schema를 따른다. UserPromptSubmit/PostCompact처럼 additional context를 지원하는 이벤트만 `hookSpecificOutput.additionalContext`를 쓰고, Stop/PreToolUse는 빈 JSON 객체 또는 검증된 host control schema만 사용한다. 실패해도 사용자 작업을 막지 않도록 작고 deterministic하게 유지한다.
- UserPromptSubmit output is host-shaped at the adapter edge: Codex uses `--host codex` where needed, omits `systemMessage`, and avoids noisy route/action/profile/pending-upkeep prose in visible TUI channels; Claude Code may keep the richer `systemMessage` + compact `additionalContext` split for events that support it.
- `.agent-harness/*.md` frontmatter descriptions are canonical bootstrap/sync metadata. Keep them concise English category descriptions, not prose summaries, because every `project bootstrap`/`project bootstrap --sync` target and every project-doc catalog inherits them.
- Codex/Claude별 hook 설정은 adapter/template에서만 다루고, routing/compaction 판단은 공통 `agent-harness hook ...` CLI/core에 둔다.

## Guard 컨벤션

- `agent-harness guard check`는 언어 무관 1차 방어선이다. path, diff, regex, token similarity처럼 deterministic하고 빠른 신호만 core rule로 둔다.
- 확실한 금지만 `block`한다. 예: secret-like path, test sleep, real external service in tests. 기존 코드 재사용 여부, snapshot 품질, production-only 변경처럼 의미 판단이 필요한 항목은 `warn` 또는 `review`로 보고한다.
- 새 symbol/helper가 기존 symbol과 유사하면 `reuse-before-new` review finding을 낸다. 이 finding은 자동 실패가 아니라 기존 코드 탐색 증거 또는 새 구현 근거를 요구하는 신호다.
- 언어별 AST/linter 통합은 optional adapter로 붙이고, core guard가 특정 언어 toolchain에 의존하지 않게 한다.
- `nondeterministic-context-serialization` rule은 DeepSeek-Reasonix의 immutable-prefix 결정성 계약에서 유래한 opt-in 규칙이다. agent가 stable cache prefix로 재사용하는 context를 만드는 파일은 `// harness:immutable-prefix` marker로 opt-in하고, 그 파일에서 `time.Now`/`rand`/`uuid` 같은 비결정 값을 도입하면 `warn`을 낸다. 의도된 volatile 값은 해당 줄에 `volatile-ok`를 달아 면제한다. volatile field 어휘와 stable projection은 `internal/core/context_region.go`(`VolatileContextFields`, `StableProjection`, `Region*` 상수)가 source of truth이며, response-contract golden의 dynamic time key 정규화와 같은 집합을 공유한다.

## Policy tier 컨벤션

- `PolicyTier`는 흩어진 capability 플래그(write/network/shell)를 host-neutral 명명 envelope로 *합성하는 분류*다. tier 계산(`resolvePolicyTier`)에 deny 판정 로직을 넣지 않는다. 명령 허용 여부는 `deny_reasons`가, 권한 envelope 이름은 `tier`가 책임진다.
- tier ladder는 `read_only` → `workspace_write` → `network_access` → `shell_exception` 순이며 most-privileged 차원이 이름을 정한다. 1회 승인이 세션 전체 등급을 올리는 YOLO/AUTO류 자동 승격 tier는 추가하지 않는다.
- tier를 추가/변경하면 `TestPolicyTierClassifiesEveryFlagCombination` table과 `command_policy` contract ResponseFields, response-contract golden을 함께 갱신한다.

## Doctor / lifecycle state conventions

- `agent-harness doctor`는 종합 진단 표면이고 기본 read-only다. 자동 수정은 별도 `--fix` 같은 명시 플래그가 있을 때만 추가한다.
- `agent-harness state doctor`는 checkpoint store 무결성 전용으로 유지한다. 사용자 안내와 troubleshooting 문서는 top-level doctor를 우선한다.
- `project bootstrap`은 target repo 문서와 별도로 user-state의 repo별 lifecycle namespace를 초기화한다. target repo에는 `.agent-harness/state/`나 schema 파일을 생성하지 않는다.
