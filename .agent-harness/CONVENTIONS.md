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

현재 구조(대표 경로):

```text
cmd/harness/main.go
cmd/harness/<cli>/                 # harnessapp, issueopscli, mcpcli, workercli, daemoncli, hookcli, installcli, ...
cmd/harness/testdata/*.golden.*
internal/core/<domain>_facade.go   # 의도된 공개 표면: issueops, issueops_remote, workflow, policy, state_trace, utility, draft_wiki, project_doc
internal/core/doc.go               # facade 경계 규칙 codify (ADR 2026-06-16)
internal/core/<subpackage>/        # 분할 도메인: issueops, lifecycle, state, policy, worker, docs, inspect, preflight, ...
internal/port/
internal/adapter/cli/
internal/adapter/mcp/
internal/adapter/codex/
internal/adapter/claude/
internal/adapter/hook/
internal/adapter/installutil/
internal/adapter/provider/         # github/gitlab issue provider
configs/codex/
configs/claude/
skills/
.agent-harness/
```

---

## 2. 레이어 경계

| 레이어 | 책임 | 의존 가능 | 금지 |
|--------|------|-----------|------|
| `core` | workspace/docs/state/policy/preflight/inspect/worker/issueops/lifecycle usecase. host-neutral 도메인은 `internal/core/<subpackage>/`로 분할되고 `internal/core/*_facade.go`가 의도된 공개 표면이다(경계 규칙은 `internal/core/doc.go`) | `port`, 표준 라이브러리 | Codex/Claude SDK, CLI flag, MCP transport 직접 의존 |
| `port` | interface, DTO, error contract | 표준 라이브러리 | adapter concrete type 의존 |
| `adapter/cli` | flag/stdout/stderr/exit code | `core`, CLI library | 정책 복제 |
| `adapter/mcp` | MCP tool schema/transport | `core`, MCP library | CLI와 다른 의미의 응답 |
| `adapter/codex` | Codex user skill/MCP 설치 구현 | `core`, `port`, 표준 라이브러리 | 적용 대상 repo 파일 쓰기 |
| `adapter/claude` | Claude user skill/hook/MCP 설치 구현 | `core`, `port`, 표준 라이브러리 | 기본 설치에서 `.claude/skills` 같은 repo-local 파일 쓰기 |
| `adapter/hook` | host별 hook 출력 schema formatter | `core`, `port` | host schema와 다른 응답 |
| `adapter/provider` | github/gitlab issue·PR/MR·child 생성/검증(gh·glab CLI) | `core`, `port`, os/exec | 정책 복제, root 밖 접근 |

> Worker daemon/job lifecycle는 별도 adapter가 아니라 `internal/core/worker`(+`cmd/harness/daemoncli`)에 있다. filesystem/git/process는 전용 `adapter/fs` 없이 각 usecase가 `os/exec`로 직접 다룬다.
> `cmd/harness`는 기본적으로 `internal/core` facade를 import한다. 예외는 cmd-local 품질/진단/fixture 도구가 subpackage 전용 메커니즘을 직접 검사하는 경우로 제한한다. `state`와 `issueops` lifecycle record 접근은 여러 command의 계약이므로 facade 경유를 유지하고, 내부 검사 helper를 숨기기 위한 새 one-line facade wrapper는 만들지 않는다.

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
- CLI/MCP contract golden은 `cmd/harness/testdata/`에 둔다. 의도된 schema 변경일 때만 `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -update -count=1`로 갱신한다.
- 실제 JSON response golden은 dynamic field(timestamp, temp path, audit id)를 normalize해서 host/session 차이로 인한 drift를 막는다.
- response golden 범위는 state/policy뿐 아니라 docs/inspect/preflight처럼 agent가 자주 의존하는 읽기 표면을 우선 포함한다.

---

## 5. Worker 컨벤션

- worker는 로컬 전용으로 시작한다. 원격 API는 별도 요구가 생기기 전까지 만들지 않는다.
- Unix socket 또는 localhost binding을 사용하고, 권한을 제한한다.
- job은 idempotency key, timeout, cancellation을 갖는다.
- worker 시작/종료는 stale lock과 orphan process를 처리한다.
- 장기 작업 상태와 project lifecycle queue/profile은 user state dir에 저장하고, repo에 secret/state 원문을 쓰지 않는다. lifecycle state는 `projects/<repo-id>/` namespace로 격리해 같은 머신의 여러 repo가 섞이지 않게 한다.
- 메인 에이전트가 명시 판단한 draft-wiki 후보는 repo가 아니라 project-scoped user state queue에 bounded/redacted 텍스트로 저장한다. worker가 처리할 때도 shell string을 만들지 말고 `agy -p <prompt>` argv 실행만 사용한다.

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

### Dependency fitness ratchet

- `internal/architecture/dependency_test.go`는 direct production import만 검사한다. edge 표기는 항상 `importer -> imported`이며 정렬 순서를 바꾸지 않는다.
- unconditional layer rule 위반은 baseline에 추가하지 않는다. 기존 legacy edge는 `internal/architecture/testdata/legacy_imports.txt`에만 기록하고, 제거 시 baseline도 함께 줄인다.
- composition root 예외는 `cmd/harness/harnessapp` 하나로 제한한다. 새 concrete-adapter import가 그 밖에 필요하다면 먼저 boundary를 재검토한다.

---

## 9. Shared Skill 컨벤션

- 공용 스킬 원본은 `skills/<skill-name>/`에 둔다.
- Codex/Claude별 user skill 경로는 원본으로 향하는 symlink 또는 installer로 연결한다. 적용 대상 repo의 `.claude/skills`는 기본 설치에서 만들지 않는다.
- skill 이름은 lowercase/digit/hyphen만 사용한다.
- 각 skill은 `SKILL.md`를 반드시 포함하고, Codex UI metadata가 필요하면 `agents/openai.yaml`을 둔다.
- host별 설치 대상이 다르면 `install.json`에 `{ "hosts": ["codex"] }` 또는 `{ "hosts": ["claude"] }`처럼 명시한다. 생략하면 모든 host에 설치하는 기존 동작을 유지한다.
- 스킬 안에는 README, 설치 가이드, changelog 같은 보조 문서를 만들지 않는다.
- 검증은 repo-owned `python3 scripts/validate-skill.py skills/<skill-name>`로 수행한다. Codex/Claude host-managed system skill 사본의 `quick_validate.py`는 upstream 상태와 로컬 Python 패키지 설치에 따라 달라질 수 있으므로 필수 검증 경로로 쓰지 않는다.

설치 adapter 규칙:

- `internal/core.InstallNative`가 host-neutral 설치 engine이고 `port.HostInstaller`가 SOLID 경계다.
- Codex/Claude adapter는 자기 host의 user/global 설정만 기본으로 쓴다. Codex는 `~/.codex/hooks.json`, Claude는 `~/.claude/settings.json`에 같은 lifecycle hook CLI를 등록한다. repo-local `.mcp.json`, `.claude/settings.json`, `.claude/skills`는 `--project-local` 같은 명시적 opt-in 없이는 만들지 않는다.
- 기본 symlink는 사용자 홈의 skill 경로에서 중앙 `skills/<name>`을 참조하거나 installer-owned command shim(`~/.local/bin/agent-harness`, `~/.local/bin/ah`)을 연결할 때만 사용한다.
- adapter 설치 계약을 바꾸면 `internal/adapter/install_contract_matrix_test.go`와 `internal/adapter/testdata/native_install_contract_matrix.golden.json`을 함께 갱신해 user/global 기본 설치와 explicit project-local opt-in의 차이를 보존한다.

self-augment/self-verify 교정 가드레일 (v1 S5/S6 승계):

- 모든 self-augment/self-verify 교정 후보의 `VerifyWith`는 **외부 검증 메커니즘을 최소 1개 명시**해야 한다 — 실행 가능한 도구 신호(`go test`/`go build`/lint/golden/contract/smoke/coverage 또는 CLI 명령), 또는 문서·거버넌스 후보(`doc_artifact`)의 경우 구체적 산출물(ADR 엔트리·README 섹션·checklist·matrix·transcript). 모델 자기비판("inspection으로 확인했다", "읽어보니 맞다" 등)은 주관 축(문서 가독성)에 한해 advisory이며 **correctness 게이트로 절대 사용 금지**다.
- 후보는 `VerificationKind`(`tool_signal`/`doc_artifact`)로 **명시 분류**하고, `qualitycatalog.VerifyWithGrounded`가 종류에 맞는 외부 메커니즘 명시를 강제한다(`internal/core/qualitycatalog`·`cmd/harness/selfworkflow/augmentcatalog` 테스트).
- 본 규약은 **카탈로그 위생**(후보가 메커니즘을 *명명*하는지)을 강제한다. 메커니즘이 실제 존재·통과하는지의 *실행* 게이팅은 `agent-harness self-verify`/CI가 담당한다.
- 근거: intrinsic self-correction은 외부 신호 없이 추론을 악화시킨다(Huang/Kamoi, CRITIC). v1 S5("measured gap or no EDIT")·S6(정직성 단서)를 문서 규약에서 Go-test 강제 불변식으로 격상한 것이다.

---

## 10. 커밋 메시지 컨벤션

- 커밋 메시지는 `.agent-harness/COMMIT_POLICY.md`를 따른다.
- subject는 Conventional Commit 형식(`<type>(<scope>)!?: <summary>`)을 사용한다.
- body에는 AI agent가 맥락을 회수할 수 있도록 `Lore:` 블록을 둔다.
- atomic commit 하나에는 Lore `Intent`도 하나만 있어야 한다.
## API Documentation Gate

Reusable API documentation checks belong in `agent-harness api-doc check`, not in a single application repository or framework-specific hook. Keep the core prompt framework-agnostic: it may mention examples such as NestJS Swagger decorators, Go swaggo annotations, OpenAPI specs, Spring/FastAPI annotations, but must instruct reviewers to follow the target project's own convention and staged diff only. Project-specific strictness should be supplied via `--prompt-file` rather than hardcoded into harness core.

## Prompt Structure Convention

Reusable harness prompts follow the strong prompt shape from the user-provided example. Do not copy example-specific domain content into harness prompts; reuse the structure:

- Identity: name the role and expertise the model should inhabit.
- Objective: state the concrete outcome.
- Operating phases: break the work into explicit ordered stages.
- Inputs: name the data the model may use and how missing data is represented.
- Rules: state hard constraints, safety boundaries, and decision rules.
- Output contract: define the exact response format and forbidden wrapper text.
- Verification checklist: require a final self-check against the prompt contract.

New reusable prompts should use `internal/core.BuildStructuredPrompt` where possible. JSON packet prompts may use equivalent structured keys instead of Markdown headings, but they still need the same identity/objective/phases/inputs/rules/output/checklist shape. Existing prompt-specific strictness, such as JSON-only output or no Markdown fences, must remain stronger than the generic structure.

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

- UserPromptSubmit hook은 사용자의 prompt와 project-scoped lifecycle state를 분석해 MCP 후보 힌트를 주입한다. 예외적으로 exact `승인 AH-XXXXXX`는 Codex kubectl live-access pending record를 10분짜리 one-shot grant로 전환하며, raw command 없이 session/request fingerprint에만 결합한다. PreToolUse hook은 tool 실행 직전의 critical path에 있으므로 기본은 host stdout `{}`인 빠른 allow/no-op preflight로 유지한다. 명시적으로 설치된 deterministic gate만 차단할 수 있고, Codex live-access gate는 project-scoped user state의 keyed lock 안에서 token 발급 또는 grant 한 번 소비만 수행한다. PostToolUse hook은 성공한 mutating tool 실행 이후 lifecycle upkeep queue 기록만 수행하며 read-only Bash/조회 output path로 upkeep을 만들지 않는다. Stop hook은 lifecycle reminder를 계산하되 Stop hook stdout schema 호환을 위해 host에는 빈 JSON 객체만 반환한다. PreCompact/PostCompact hook은 pending upkeep queue를 작은 user-state capsule로 저장하고 한 번만 복원한다. Codex와 Claude Code hook adapter는 같은 `agent-harness hook user-prompt/pre-tool-use/post-tool-use/pre-compact/post-compact/stop` CLI를 호출해야 한다. 어떤 hook도 작업을 대신 실행하거나 shared docs를 직접 수정하거나 긴 파일/네트워크를 읽지 않는다.
- draft-wiki queue append는 hook 휴리스틱이 아니라 메인 에이전트의 명시 판단 뒤에만 수행한다. 장기 재사용 가치가 있다고 판단한 경우 `agent-harness project draft-wiki queue --stdin`을 heredoc과 함께 쓰거나 `--input` 파일을 넘긴다. `agy -p` 호출과 `.agent-harness/draft-wiki/draft` 파일 쓰기는 `agent-harness worker draft-wiki`가 hook 밖에서 수행한다.
- Hook 출력은 event별 host schema를 따른다. UserPromptSubmit/PostCompact처럼 additional context를 지원하는 이벤트만 `hookSpecificOutput.additionalContext`를 쓰고, Stop/PreToolUse는 빈 JSON 객체 또는 검증된 host control schema만 사용한다. Codex PreToolUse는 native `ask`를 내보내지 않고 token 안내를 포함한 flat block을 사용하며, 승인된 동일 요청을 한 번 소비하면 `{}` allow를 반환한다. 실패해도 사용자 작업을 불필요하게 막지 않도록 작고 deterministic하게 유지한다.
- UserPromptSubmit output is host-shaped at the adapter edge: Codex uses `--host codex` where needed, omits `systemMessage`, and avoids noisy route/action/profile/pending-upkeep prose in visible TUI channels; Claude Code may keep the richer `systemMessage` + compact `additionalContext` split for events that support it.
- `.agent-harness/*.md` frontmatter descriptions are canonical bootstrap/sync metadata. Keep them concise English category descriptions, not prose summaries, because every `project bootstrap`/`project bootstrap --sync` target and every project-doc catalog inherits them.
- Codex/Claude별 hook 설정은 adapter/template에서만 다루고, routing/compaction 판단은 공통 `agent-harness hook ...` CLI/core에 둔다.

## Guard 컨벤션

- `agent-harness guard check`는 언어 무관 1차 방어선이다. path, diff, regex, token similarity처럼 deterministic하고 빠른 신호만 core rule로 둔다.
- 확실한 금지만 `block`한다. 예: secret-like path, test sleep, real external service in tests. 기존 코드 재사용 여부, snapshot 품질, production-only 변경처럼 의미 판단이 필요한 항목은 `warn` 또는 `review`로 보고한다.
- 새 symbol/helper가 기존 symbol과 유사하면 `reuse-before-new` review finding을 낸다. 이 finding은 자동 실패가 아니라 기존 코드 탐색 증거 또는 새 구현 근거를 요구하는 신호다.
- 언어별 AST/linter 통합은 optional adapter로 붙이고, core guard가 특정 언어 toolchain에 의존하지 않게 한다.
- 차단은 **왜 막혔는지**를 함께 낸다. 게이트가 판정 과정에서 이미 관측한 것을 버리지 않는다 — 슬러그나 코드만 받은 owner는 명령을 조금씩 바꿔가며 추측 재시도를 반복한다(이슈 #90 발견 4, #154). 구조화된 deny가 사람이 읽을 사유를 대체하는 출력 경로라면 그 사유를 deny 안에 실어야 한다. 담는 것은 분류 결과와 이미 추출된 경로이며 명령 원문은 담지 않는다 — 인자에 토큰이 있을 수 있다.
- 해소 경로가 **하나로 정해지는** 차단만 그 명령을 안내한다. 상황에 따라 갈리는 항목에는 붙이지 않는다. 틀린 안내는 안내가 없는 것보다 나쁘다.
- **관측 불가와 조건 위반을 다른 슬러그로 구분한다.** 둘 다 fail-closed지만 다음 행동이 다르다 — 전자는 관측 도구를 고치고 후자는 상태를 고친다(#154의 `workspace_processes_observable` vs `workspace_processes_quiescent`).
- **`missing` 안의 슬러그는 요구형 극성으로 통일한다**(#185). `missing`은 *충족되지 않은 요구*의 목록이므로 상태 차단도 요구형으로 뒤집어 적는다 — 원격 브랜치가 남아 막혔으면 `remote_branch_absent`이고, 워크트리가 더러워 막혔으면 `worktree_clean`이다. 차단 사실을 그대로 적으면(`remote_branch_present`, `worktree_dirty`) "그 상태라는 요구가 미충족" = **반대 상태**로 읽힌다. `#181` 정리에서 `cleanup status`와 `cleanup finish`가 같은 상태를 반대 극성으로 보고해 운영자가 실제 상태를 따로 확인해야 했다. 이 축은 위의 관측/조건 축과 직교한다 — 관측 실패 슬러그(`remote_branch_check_failed`)는 그대로 둔다. 같은 이름이 다른 표면에서 진짜 요구일 수 있으므로(`execution sync-base`의 `remote_branch_present`는 브랜치가 **있어야** 한다는 요구다) 표면별로 그 조건이 요구인지 차단인지 먼저 정한다.
- preview는 **자기 근거의 강도를 밝힌다.** 외부 자원을 조회하지 않고 낸 결과가 관측 증거로 읽히면 오진단이 생긴다(#99의 잘못된 의혹이 그렇게 나왔다, #154). 아울러 실행 가능성 판정은 preview에서도 수행한다 — 실행할 수 없는 계획을 preview가 성공으로 보여주면 운영자는 confirm에서 처음 막히고, 모드 자동 선택에서는 preview가 보여준 모드와 실제 모드가 달라진다(#152).
- **lifecycle execution guard의 allowlist는 세 층으로 분류한다**(#170·#177). 층을 잘못 고르면 진단이 막히거나 mutation이 새어 나간다.
  - **읽기 허용**(`executionObservation`): 상태를 바꾸지 않는 조회. `--preview`/`--status`처럼 같은 명령의 읽기 변종도 여기 속한다 — 진단 표면을 mutation으로 분류하면 갇힌 상태를 관측할 방법이 사라진다(#170의 `reset-legacy --preview`).
  - **typed control plane**(`executionTypedControlPlane`): harness 자신의 typed 명령. 실행 주체가 owner가 아니어도 되는 lifecycle 조작이 여기 온다(#177의 `cleanup orphan`).
  - **owner mutation**(`exactIssueOpsOwnerMutation`): 활성 홀더만 할 수 있는 provider/워크트리 변경. 형태를 exact matcher로 고정한다.
- **allowlist matcher는 형태를 고정하되 의미를 확인한다.** provider API를 여는 matcher는 플래그 위치·개수와 함께 본문의 특징 문자열까지 검사해 임의 호출이 통과하지 않게 한다(#176의 `exactProviderBranchLink`가 GraphQL 본문의 `createLinkedBranch`를 요구한다).
- **정적 분류를 깨는 명령 형태를 안내에 넣지 않는다.** 파이프·`&&`·리다이렉트·`$(...)`는 가드가 분류할 수 없어 거부된다. 값 전달이 필요하면 단계를 나눠 출력을 옮겨 적게 안내한다. GraphQL 변수처럼 `$`가 불가피한 경우는 단일 인용으로 표기한다 — 파라미터 확장 검사는 단일 인용 안을 건너뛰므로(`internal/core/commandparse/tokens.go`) 가드를 약화하지 않고 통과한다.
- `nondeterministic-context-serialization` rule은 immutable-prefix 결정성 계약에서 유래한 opt-in 규칙이다. agent가 stable cache prefix로 재사용하는 context를 만드는 파일은 `// harness:immutable-prefix` marker로 opt-in하고, 그 파일에서 `time.Now`/`rand`/`uuid` 같은 비결정 값을 도입하면 `warn`을 낸다. 의도된 volatile 값은 해당 줄에 `volatile-ok`를 달아 면제한다. volatile field 어휘와 stable projection은 `internal/core/contextregion/context_region.go`(`VolatileContextFields`, `StableProjection`, `Region*` 상수)가 source of truth이며, response-contract golden의 dynamic time key 정규화와 같은 집합을 공유한다.

## Policy tier 컨벤션

- `PolicyTier`는 흩어진 capability 플래그(write/network/shell)를 host-neutral 명명 envelope로 *합성하는 분류*다. tier 계산(`resolvePolicyTier`)에 deny 판정 로직을 넣지 않는다. 명령 허용 여부는 `deny_reasons`가, 권한 envelope 이름은 `tier`가 책임진다.
- tier ladder는 `read_only` → `workspace_write` → `network_access` → `shell_exception` 순이며 most-privileged 차원이 이름을 정한다. 1회 승인이 세션 전체 등급을 올리는 YOLO/AUTO류 자동 승격 tier는 추가하지 않는다.
- tier를 추가/변경하면 `TestPolicyTierClassifiesEveryFlagCombination` table과 `command_policy` contract ResponseFields, response-contract golden을 함께 갱신한다.

## Doctor / lifecycle state conventions

- `agent-harness doctor`는 종합 진단 표면이고 기본 read-only다. 자동 수정은 별도 `--fix` 같은 명시 플래그가 있을 때만 추가한다.
- `agent-harness state doctor`는 checkpoint store 무결성 전용으로 유지한다. 사용자 안내와 troubleshooting 문서는 top-level doctor를 우선한다.
- `project bootstrap`은 target repo 문서와 별도로 user-state의 repo별 lifecycle namespace를 초기화한다. target repo에는 `.agent-harness/state/`나 schema 파일을 생성하지 않는다.

## State machine reducer contract

12-factor #12(stateless reducer)를 IssueOps 상태머신에 명문화한 계약이다. 코드에 이미 성립하는 불변식을 규율로 고정하는 것이지, 새 추상을 도입하는 것이 아니다.

- IssueOps phase 전이의 **판정(validation)은 균일하게 순수하지 않다**. readiness 게이트는 실측상 세 층으로 갈린다 — 이 분류가 계약이고, "전부 순수"라고 적지 않는다(#107, Codex 외부 검토로 확정).
  - **record-only 순수**: `IssueOpsProblemReadiness`(`internal/core/issueops/issueops_phase_ledger.go:31`)/`IssueOpsGrillReadiness`(`issueops_phase_ledger.go:39`)/`IssueOpsPlanReadiness`(`internal/core/issueops/issueops_readiness.go:14`)만 `IssueOpsRecord` 필드만 읽어 `Ready/Missing`을 돌려주며 clock·git·FS·network를 건드리지 않는다.
  - **FS 존재검사 수행**: `IssueOpsCompatibilityReviewReadiness`(`issueops_readiness.go:86`)/`IssueOpsImplementationReadiness`(`issueops_readiness.go:110`)/`IssueOpsAISlopCleanReadiness`(`issueops_readiness.go:74`)와 비-strict `IssueOpsPRReadiness`(`internal/core/issueops/issueops_pr_readiness.go:9`)는 `issueOpsWorktreePathValid`/`issueOpsPlanPathExists`/`issueOpsPlanInLinkedWorktree`(`issueops_readiness.go:194-204`)를 거쳐 `os.Stat`(`internal/core/issueops/readinesspaths/paths.go:24`, `paths.go:36`)과 `filepath.EvalSymlinks`(`paths.go:60`, `paths.go:64`)를 실행한다. 같은 record라도 디스크 상태가 바뀌면 결과가 바뀐다.
  - **git·network 수행**: `IssueOpsStrictPRReadiness`(`internal/core/issueops/issueops_pr_readiness_strict.go:12`)는 `rev-parse --is-inside-work-tree`(`:23`), `issueOpsCurrentHead`의 `rev-parse HEAD`(`issueops_readiness.go:262-270`), `branch --show-current`(`:28`), `status --porcelain=v1`(`:33`), upstream `rev-parse @{u}`(`:36`), 네트워크를 타는 `git fetch --quiet`(`:40`), `rev-list --left-right --count`(`:46`)를 직접 실행한다.
- **비결정·side-effect의 소유 경계는 게이트마다 다르다**(전부 wrapper 소유가 아니다). 전이 적용 함수 `applyIssueOpsPhaseTransition`(`internal/core/issueops/issueops_phase.go:140`) 밖에서 wrapper가 소유하는 것: wall-clock(`time.Now()`, `issueops_phase.go:142`), git read(`issueOpsCurrentHead`/`implementation.ChangeFingerprint`, `issueops_phase.go:148-149`), 디스크 write(`touchAndWriteIssueOps` 호출 `issueops_phase.go:72`, 정의 `internal/core/issueops/issueops_state.go:182`). 그러나 **판정 함수 `validateIssueOpsPhaseTransition`(`issueops_phase.go:75`) 자체가 IO를 실행한다**: PR 진입에서 strict 게이트를 직접 호출하고(`issueOpsStrictPRReadinessWithState`, `issueops_phase.go:122` → `issueops_pr_readiness_strict.go:97`) 그 안에서 git·network가 돈다. compatibility-review/implement/ai-slop-clean 진입 판정(`issueops_phase.go:104`, `:109`, `:114`)도 같은 이유로 FS를 읽는다. 규율은 "판정은 순수하다"가 아니라 **이 경계 안으로 새 IO를 더 밀어 넣지 않는다**이다.
- ledger stamp(`stampIssueOpsForwardTransition(ledger, prev, new, now)`, `issueops_phase_ledger.go`)는 `now`를 **주입받으면 순수**하다. 같은 `(record, to, now)`는 항상 같은 record를 낳아 replay/derive가 결정적이며, 이는 `DeriveIssueOpsPhaseLedger`의 결정성 테스트로 보장된다(`issueops_phase_ledger_test.go`).
- **신규 상태머신은 이 경계를 따른다**: 판정 로직에 clock/rand/uuid/IO를 섞지 않고, 비결정 입력은 값으로 주입한다(`nondeterministic-context-serialization` guard와 같은 정신). 결정성 검증이 필요해지고 두 번째 사용처가 생기면 그때 순수 함수를 *전체* 판정 블록 단위로 추출한다. 동작 무변화를 위해 `AdvanceIssueOpsPhase`를 선제적으로 리팩터하지 않는다(§28 게이트 파급이 큰 최고-민감 함수).

## Optional external orchestration adapter convention

- External orchestrators use one concrete adapter per verified boundary. The Orca adapter owns safe argv, bounded timeout/output, envelope decoding, and narrow DTO projection; it does not own IssueOps transitions and does not justify a registry/factory.
- Every external mutation follows `lock + persist pending -> unlock -> external call -> lock + compare-and-set result`. Never hold the cycle lock during an Orca/network call or mutating subprocess and never persist an observed identity against a stale attempt/epoch/context fence. A fixed read-only local Git checkpoint may run under the lock only when branch/HEAD/clean filesystem evidence must be sealed immediately before the same write.
- Completion projection is the narrow terminal-message variant: completed finish persists `submitted` result evidence plus a deterministic projection intent (or no-call diagnostic) in one cycle-lock write, releases the lock, and makes at most one argv-only `worker_done` call. Any persisted intent is a no-retry tombstone; post-call success/failure only annotates it and never rolls back submitted authority.
- Sole-writer attestation inventories every exact-worktree terminal plus server-filtered dispatched tasks immediately before each Orca create/dispatch boundary. Every connected or writable terminal, including baseline terminals, is a possible writer; only the designated active worker must be both connected and writable. Truncated, unparsable, incomplete, or duplicate identities persist `recovery_required` and never authorize replacement.
- Accepted publication is fenced to the full submitted `FinalHead`. Push requires the exact local branch ref at that SHA; PR/MR creation requires a durable provider-neutral receipt plus fresh local/remote ref verification for the same provider, remote, branch, and SHA. Provider adapters receive literal rendered body argv, never arbitrary local body-file paths.
- Failed/cancelled cleanup persists disposition before mutation and exact task/dispatch, terminal/PTY/worktree, then worktree-instance receipts in order afterward. Accepted handoffs cannot approve cleanup, and duplicate receipt writes are idempotent only for the same exact identity.
- `WorkerMailboxHandle` is the immutable dispatch assignee and completion sender. `WorkerTerminalHandle` is refreshable live control only. Runtime reconciliation may update the latter and runtime/PTY/tab/leaf evidence but never either sealed mailbox recipient.
- Timeout or transport error after invocation is ambiguous. Persist `recovery_required`; do not automatically repeat create/dispatch or switch to inline execution.
- Native worker identity is `(host, session_id, agent_id)` plus exact canonical worktree root. Host adapters forward that identity; common core decides ownership.
- CLI and MCP must use the same request/result DTOs. Keep the handoff MCP surface as one action-discriminated tool instead of multiplying near-identical lifecycle tools.
- **외부 시스템의 어휘는 그 시스템의 정의에서 인용하고 출처를 코드에 남긴다**(#171·#147·#180). CLI 출력에서 관측한 값의 집합은 어휘가 아니라 표본이다 — 그것으로 열거를 채우면 관측되지 않은 값이 "미지"로 오분류된다. Orca는 공개 저장소이므로 `DispatchStatus`/`GateStatus` 같은 union 정의와 `CHECK` 제약을 직접 읽고, provider API는 공식 문서의 필드 설명을 근거로 쓴다(GitLab `ref`는 "Branch name or commit SHA"). 인용 문구를 주석에 그대로 남겨 다음 독자가 다시 추측하지 않게 한다.
- **도구가 로컬에 없는 것은 계약을 확인할 수 없다는 뜻이 아니다**(#180). 설치 여부는 실측에만 영향을 준다 — 소스 읽기와 공식 문서 확인은 그대로 가능하다. `glab`이 없다는 이유로 GitLab 경로를 비범위로 둔 판단이 틀렸고, 상류 소스를 읽자 `ref` 문제와 **독립된 결함**이 나왔다(MCP 도구 인자가 실제 스키마와 달라 검증에서 실패하는 형태였다). 안내하는 MCP 도구의 인자 형태는 그 도구의 스키마를 만드는 코드에서 확정한다 — 이름만 맞고 형태가 틀린 안내는 따라 실행하면 실패한다.
- **분류 축을 섞지 않는다.** dispatch 수명주기와 task 수명주기는 다른 열거이고 종료 조건이 다르다 — dispatch가 `failed`여도 task는 `ready`로 남아 재시도를 기다린다(`db.ts` failDispatch). 한 축의 상태로 다른 축의 생존을 판정하면 재시도 가능한 사이클이 죽은 것으로 보인다(#147).
