---
name: OPERATIONS.md
description: Install, sync, runtime, and operational procedures.
---

# 하네스 사용법

현재 `agent-harness`는 Codex와 Claude Code가 같은 Go binary와 MCP schema를 쓰도록 다음 표면을 제공한다.

1. Codex/Claude native skills: `atomic-commit-push`, `self-augment`, `project-bootstrap`; Codex-only skills: `workflows`, `ultracode`
2. MCP stdio proxy: `agent-harness mcp` → shared `agent-harness daemon`
3. CLI: `agent-harness inspect/preflight/status/verify-work/doctor/docs/project/policy/guard/state/daemon/worker/self-verify/self-augment/api-doc/hook`

---

## 1. 설치/동기화

공개 UX는 두 가지로 나눈다.

```bash
# 내 머신의 Codex/Claude user-level 통합 설치/갱신
agent-harness bootstrap

# upstream companion 도구 버전까지 함께 최신화
agent-harness bootstrap --sync

# 대상 레포에 AGENTS.md 라우팅 블록과 .agent-harness/*.md 생성
agent-harness project bootstrap --repo /path/to/repo

# 기존 프로젝트 문서까지 최신 템플릿/증거 기준으로 갱신
agent-harness project bootstrap --repo /path/to/repo --sync
```

`bootstrap`은 이 agent-harness checkout을 기준으로 user-level host 통합을 설치/갱신한다. 이 경로는 다음을 수행한다.

- 현재 checkout 기준으로 `go build -o bin/agent-harness ./cmd/harness`를 실행해 binary를 갱신한다. 원격 변경을 가져오는 `git pull`은 자동 실행하지 않는다.
- `~/.local/bin/agent-harness -> <checkout>/bin/agent-harness` command shim 생성/갱신
- `agent-harness install-native` 실행
- Codex user skill symlink 생성: `~/.codex/skills/* -> <agent-harness>/skills/*`
- Claude user skill symlink 생성: `~/.claude/skills/* -> <agent-harness>/skills/*`
- Codex MCP 설정 추가/갱신: `~/.codex/config.toml`의 `[mcp_servers.agent_harness]`
- Codex lifecycle hooks 추가/갱신: `~/.codex/hooks.json`에서 `agent-harness hook user-prompt`, `agent-harness hook pre-tool-use`, `agent-harness hook post-tool-use`, `agent-harness hook pre-compact`, `agent-harness hook post-compact`, `agent-harness hook stop` 실행
- Claude lifecycle hooks 추가/갱신: `~/.claude/settings.json`에서 같은 lifecycle hook 실행
- Claude user-scope MCP 서버 등록: `claude mcp add-json -s user agent_harness ...`

기본 `bootstrap`은 적용 대상 repo에 `.claude/skills`, `.claude/settings.json`, `.mcp.json`을 만들지 않는다. 레포별 파일은 `project bootstrap`이 담당한다. 쓰기 전 계획만 확인하려면 `agent-harness bootstrap --dry-run --json` 또는 low-level `agent-harness install-native --dry-run --json`을 사용한다.

### `--sync`

`--sync`는 “최신화” 의도를 명시하는 단일 옵션이다.

- `agent-harness bootstrap --sync`: user-level 통합을 갱신하면서 llm-wiki, CodeGraph, agentmemory 같은 upstream companion 도구도 설치/갱신한다. 네트워크와 user-level host 설정 변경이 생길 수 있다.
- `agent-harness project bootstrap --sync`: 대상 repo의 `AGENTS.md` 라우팅 블록, `.agent-harness/*.md`, user-state repo profile metadata를 현재 템플릿/프로젝트 증거 기준으로 다시 계산해 갱신한다.

낮은 수준 자동화가 필요할 때만 `scripts/install-native.sh`와 `install-native`를 직접 사용한다. 일반 사용자는 `bootstrap`과 `project bootstrap`만 기억하면 된다.

설치되는 upstream dependency:

| 도구 | Upstream | 동작 |
|------|----------|------|
| LLM Wiki | `nvk/llm-wiki` | Codex marketplace와 Claude marketplace에 `wiki@llm-wiki`를 추가/갱신한다. |
| CodeGraph | `colbymchenry/codegraph` | `npm install -g @colbymchenry/codegraph`, Codex/Claude MCP 등록, 현재 harness repo `.codegraph/` index 초기화를 수행한다. |
| agentmemory | `rohitg00/agentmemory` | Codex/Claude plugin marketplace와 `agentmemory` plugin/CLI/MCP/hooks 배선을 추가/갱신한다. |

agentmemory 전환 이후 full upstream setup은 기존 legacy memory plugin/marketplace 배선을 제거한다. Claude plugin data는 `--keep-data`로 보존한다.

CodeGraph local index 생성을 건너뛰려면 `HARNESS_INIT_CODEGRAPH=0 agent-harness bootstrap --sync`를 사용한다.

---

## 2. Codex에서 사용

### Native skills

예시:

```text
Use $atomic-commit-push to review my changes, split them into atomic commits, and push safely.
Use $ultracode in Codex to automatically apply workflows to substantive tasks.
Use $workflows in Codex to run an explicit dynamic workflow with batched subagents.
```

설치 확인:

```bash
test -f ~/.codex/skills/atomic-commit-push/SKILL.md && echo ok
```

### MCP

Codex MCP 등록 확인:

```bash
codex mcp list
codex mcp get agent_harness
```

`agent-harness mcp`는 user-level daemon을 자동 시작하고 stdio를 daemon socket으로 proxy한다.

### Lifecycle hooks

Codex native hook이 활성화된 환경에서는 `~/.codex/hooks.json`에 lifecycle hook을 등록한다. `UserPromptSubmit`은 `agent-harness hook user-prompt --host codex`로 설치하며 매 prompt의 dynamic routing/profile/upkeep hint만 싣는다. `PreToolUse`의 `agent-harness hook pre-tool-use`는 tool 실행 직전 critical path이므로 기본 host stdout은 `{}`이고 raw `--json`에서만 allow/no-op 진단을 확인한다. `PostToolUse`의 `agent-harness hook post-tool-use`는 hook/state/MCP/test 관련 파일 변경을 repo별 user-state queue에 기록하고, `PreCompact`의 `agent-harness hook pre-compact`는 pending doc-upkeep 상태를 작은 compact capsule로 저장하며, `PostCompact`의 `agent-harness hook post-compact`는 capsule을 한 번만 additional context로 복원한다. `Stop`의 `agent-harness hook stop`은 Codex/Claude Stop hook schema 호환을 위해 host에는 빈 JSON 객체만 반환한다. pending upkeep reminder는 `agent-harness hook stop --json`의 raw result로 확인하며 Stop hook stdout에 `hookSpecificOutput.additionalContext`를 쓰지 않는다. 후속 hook들은 shared docs를 직접 수정하지 않는다.

예:

```bash
printf '{"prompt":"endpoint와 DTO를 추가해줘"}' | ./bin/agent-harness hook user-prompt
```

Raw JSON(`--json`)의 주입 후보 예시는 `project_docs_route`, `api_doc_static_check`, `api_doc_review`, `project_docs_read/project_docs_update`, `project_docs_record`다. 매 prompt마다 실행되므로 네트워크나 긴 파일 읽기를 하지 않고 정적 keyword routing과 짧은 user-state 조회만 수행한다.

---

## 3. Claude Code에서 사용

### Native skills

Claude Code는 기본적으로 user skill 경로에서 중앙 원본을 본다.

- User: `~/.claude/skills/<skill>/SKILL.md`
- Project: `.claude/skills/<skill>/SKILL.md`는 기본 설치에서 만들지 않는다. repo-local attach가 필요한 경우에만 `--project-local`을 사용한다.

직접 호출 예:

```text
/atomic-commit-push
```

### MCP

기본 설치는 user-scope MCP 서버 `agent_harness`가 중앙 `bin/agent-harness mcp`를 등록한다. 이 레포의 `.mcp.json`은 dogfood/project-local 템플릿 역할이며, Claude scope 충돌 경고를 피하려고 project-local 서버 이름은 `agent_harness_project`를 사용한다.

확인:

```bash
claude mcp list
```

Claude Code 세션 안에서는 다음으로 상태를 볼 수 있다.

```text
/mcp
```

### Claude hooks

기본 설치는 `~/.claude/settings.json`에 lifecycle hook을 등록한다. Claude Code의 `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PreCompact`, `PostCompact`, `Stop` hook은 Codex와 같은 공통 CLI/core를 호출하지만 output shape은 host에 맞춘다. Claude Code는 `systemMessage`와 model-facing `hookSpecificOutput.additionalContext`를 분리해 쓸 수 있으므로 지원 이벤트에서 readable view와 compact hint를 함께 유지할 수 있다. `PreToolUse`와 `PostToolUse`는 Claude hook matcher `*`로 모든 tool 이벤트를 받지만, PreToolUse는 기본적으로 차단하지 않고 PostToolUse만 성공 이벤트를 근거로 관련 파일을 queue에 남긴다. Claude project-local hook 설정은 repo에 커밋될 수 있으므로 명시 opt-in 없이 `.claude/settings.json`을 target repo에 만들지 않는다.

---

## 4. CLI로 직접 사용

```bash
./bin/agent-harness version
./bin/agent-harness install-native --json
./bin/agent-harness install-native --dry-run --json
./bin/agent-harness inspect --json
./bin/agent-harness status --json
./bin/agent-harness preflight --json /path/to/git-repo
./bin/agent-harness docs --json
./bin/agent-harness guard check --staged --json
./bin/agent-harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy run --read-only --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
./bin/agent-harness doctor --json
./bin/agent-harness state write --key checkpoint-1 --value "작업 메모" --json
./bin/agent-harness state read --key checkpoint-1 --json
./bin/agent-harness state list --json
./bin/agent-harness state prune --max-age 720h --json
./bin/agent-harness state prune --max-age 720h --confirm --json
./bin/agent-harness state doctor --json
./bin/agent-harness state migrate --json
./bin/agent-harness state migrate --confirm --json
./bin/agent-harness daemon start --json
./bin/agent-harness daemon status --json
./bin/agent-harness daemon stop --json
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/agent-harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/agent-harness self-verify candidates --json
./bin/agent-harness self-verify candidates --save-state --state-key self-verify-candidates-latest --json
./bin/agent-harness self-verify history --prefix self-verify --json
./bin/agent-harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --json
./bin/agent-harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --confirm --json
./bin/agent-harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/agent-harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --json
./bin/agent-harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/agent-harness self-augment lesson --candidate reflexion-state-memory --lesson "..." --next-action "..." --json
./bin/agent-harness worker run --read-only --kind smoke --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/agent-harness verify-work --json -- git status --short
./bin/agent-harness api-doc check --json
printf '{"prompt":"endpoint와 DTO를 추가해줘"}' | ./bin/agent-harness hook user-prompt
./bin/agent-harness project bootstrap --repo . --json
./bin/agent-harness project bootstrap --repo . --sync --json
./bin/agent-harness project route-docs --repo . --task "commit" --json
./bin/agent-harness mcp
```


### Status / read-only runner / verify-work

`status --json` aggregates inspect, doctor, daemon, state, worker, and latest self-verify checkpoint status into one machine-readable health view. `policy run --read-only` is the first real command runner: it only executes argv-only allowlisted read-only commands, applies workspace/cwd policy, timeout, env allowlist, audit metadata, redaction, and bounded stdout/stderr. `worker run --read-only` records the same execution as a worker job with command evidence. `verify-work` combines git preflight, guard checks, and an optional read-only verification command for target-repo change evidence.

These commands intentionally do not open write, network, arbitrary shell, or background execution. Use `policy fake-run` for write-intent planning until a later explicit runner phase exists.

### Guard checks

`agent-harness guard check`는 언어 무관 1차 품질 gate다. 확실한 anti-pattern은 `block`, 오탐 가능성이 있는 품질 냄새는 `warn`, 기존 코드 재사용 여부처럼 의미 판단이 필요한 항목은 `review`로 보고한다. 기본은 staged files만 검사하며, `--all`은 repo 전체의 관련 소스/테스트/설정 파일을 빠르게 훑는다.

현재 portable rule은 secret-like path, test sleep, real external URL in tests, 모호한 테스트명, snapshot/golden review 필요, production-only 변경, CLI/MCP/adapter 계약 변경 시 golden 누락, 기존 symbol과 유사한 새 helper 후보를 잡는다. 언어별 AST 판정은 후속 adapter로 붙이고, core guard는 path/diff/regex/token 기반의 deterministic rule만 유지한다.

---

## 5. MCP smoke test

```bash
tmp_state="$(mktemp -d)"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}' \
  | HARNESS_STATE_DIR="$tmp_state" HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness mcp
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/agent-harness daemon stop --json
rm -rf "$tmp_state"
```

## 6. 자기 검증 루프 / 자가 증강 루프 요약

`--progress=jsonl`은 stdout의 최종 JSON summary를 유지하면서 stderr에 iteration/step JSON Lines heartbeat를 기록한다. redirect 또는 장기 실행 환경에서 hang으로 오해하지 않도록 진행 상태를 볼 때 사용한다. self-verify에는 golden/docs/skill metadata의 unredacted secret-like 문자열을 막는 redaction audit도 포함된다.

`--save-state`를 함께 쓰면 전체 runs 로그가 아니라 compact summary snapshot만 `state_key`에 저장한다. `history`, `compare`, `promote`는 저장된 summary checkpoint를 조회·비교·승격한다. `compare`는 전체 elapsed, `slow_step:*`, label별 p95 `step_budget:*` regression을 함께 보고한다. `history --retention-limit N`은 newest-first 기준 보존/삭제 후보를 계산하고, `--prune-retention --confirm`이 있을 때만 초과 checkpoint를 삭제한다; `--confirm`이 없으면 dry-run이다.

## 7. API Documentation Pre-commit Review

`agent-harness api-doc review` provides a reusable, framework-agnostic AI gate for API documentation drift. It is not tied to NestJS or one repository. By default it inspects staged API candidate files only, including names such as `*controller*`, `*dto*`, `*handler*`, `*router*`, `openapi.*`, `swagger.*`, and other API/schema files.

```bash
# Review staged API docs/endpoint changes in the current repo
agent-harness api-doc review

# JSON output for hooks/CI
agent-harness api-doc check --json

# Explicit files from lint-staged/pre-commit
agent-harness api-doc review -- src/users/users.controller.ts internal/api/user_handler.go openapi.yaml

# Add project-specific conventions without changing harness core
agent-harness api-doc review --prompt-file docs/api-doc-rules.md
```

The reviewer runs Codex non-interactively with `gpt-5.5` and `model_reasoning_effort=medium` by default, in read-only/no-approval mode. The prompt is framework-agnostic and asks the model to apply the target project's own API documentation conventions, e.g. NestJS Swagger decorators, Go swaggo comments, OpenAPI YAML/JSON, Spring/FastAPI equivalents, or other machine-readable API documentation.

Hook example:

```js
// lint-staged.config.js
module.exports = {
  '*.{ts,js,go,yaml,yml,json}': (files) => [
    'agent-harness api-doc check -- ' + files.join(' '),
  ],
}
```


## 8. Project Docs Bootstrap

`agent-harness project bootstrap`은 대상 레포를 분석해 에이전트용 프로젝트 운영 문서 초안과 repo profile metadata를 생성한다. 기본 실행은 누락된 파일과 user-state profile을 쓰며, 계획만 확인하려면 `--dry-run`을 쓴다. 기존 문서를 템플릿/증거 기준으로 다시 최신화하려면 `--sync`를 쓴다. `AGENTS.md`는 전체 덮어쓰지 않고, behavioral top block이 없으면 prepend하며 `AGENT_HARNESS` marker block만 추가/갱신한다. `.agent-harness/*.md` frontmatter `description`은 canonical concise English metadata를 쓴다. 신규 bootstrap과 `--sync` 모두 같은 metadata를 적용하므로, description 변경은 모든 target repo의 project-doc catalog와 hook context 길이에 영향을 준다. 실행은 target repo에 runtime state를 쓰지 않고 user-state의 `projects/<repo-id>/project.json` lifecycle namespace를 생성·검증한다. profile metadata에는 VCS provider/hosting, language, package manager, frontend/backend/fullstack/monorepo/CLI/library classification, framework evidence가 기록되어 이후 hook context injection에서 짧게 참조된다. 정적 bootstrap은 안전한 baseline일 뿐이므로 최초 세팅 후 에이전트가 repo 증거와 lifecycle state를 읽고 `.agent-harness` 문서를 MCP로 보강해야 한다.

생성 대상:

- `AGENTS.md` routing block
- `.agent-harness/ARCHITECTURE.md`
- `.agent-harness/CAUTIONS.md`
- `.agent-harness/COMMIT_POLICY.md`
- `.agent-harness/CONSTITUTION.md`
- `.agent-harness/CONVENTIONS.md`
- `.agent-harness/TECH_STACK.md`
- `.agent-harness/TESTING.md` — 테스트 작성 시 좋은/나쁜 테스트 구성 기준과 검증 명령 후보.
- `.agent-harness/OPEN_API_SPEC.md` — endpoint/DTO/OpenAPI 변경 시 정적+에이전트 문서화 게이트 프롬프트.
- `.agent-harness/ADR.md`
- `.agent-harness/OPERATIONS.md`
- `.agent-harness/AGENT_WORKFLOW.md`

작업별 문서 라우팅은 CLI와 MCP에서 제공한다.

```bash
./bin/agent-harness project bootstrap --repo /path/to/repo --json
./bin/agent-harness project bootstrap --repo /path/to/repo --sync --json
./bin/agent-harness project route-docs --repo /path/to/repo --task "commit" --json
```

MCP tools/resources:

- `project_docs_bootstrap_plan`: 쓰기 없는 bootstrap 계획
- `project_docs_route`: 작업 설명에 따른 `AGENTS.md`/`.agent-harness` 문서 추천
- `harness://project-docs`: 현재 workspace의 기본 project docs routing

## 9. LLM Wiki 정책

LLM Wiki 기능은 agent-harness가 직접 제공하지 않는다. 중복 구현을 피하기 위해 upstream `nvk/llm-wiki`의 Codex/Claude plugin 또는 portable AGENTS.md를 사용한다. 하네스 CLI/MCP에 llm-wiki 전용 명령, tool, resource, SessionStart hook을 추가하지 않는다.

## 10. MCP Usage for Agents

에이전트가 MCP를 효과적으로 쓰는 기본 순서:

1. `project_docs_route`로 현재 작업에 필요한 문서를 고른다.
2. 문서가 코드/사용자 컨센서스와 어긋나면 `project_docs_read`로 현재 SHA를 확인하고 `project_docs_update`로 한 문서씩 갱신한다.
3. 정책·상태·검증처럼 외부 사실이 필요한 경우에만 해당 MCP tool을 호출한다.
4. 해결한 문제는 `project_docs_record(kind=caution)`으로 CAUTIONS에 기록한다.
5. 결정사항·대안 기각은 `project_docs_record(kind=adr)`로 ADR에 기록한다.
5. command 실행 전에는 필요 시 `command_policy_check`로 cwd/workspace/write/network 경계를 확인한다.

쓰기 도구:

- `project_docs_record`는 append-only 기록 도구다.
- `project_docs_update`는 full-document replacement 도구라서 기존 파일 업데이트에는 `project_docs_read`의 `expected_sha256`이 필요하며, `confirm=true` 없이는 dry-run이다.
- `project_docs_bootstrap_plan`은 dry-run 전용이며 파일을 쓰지 않는다.
- `state_prune`, `state_migrate`, `self_verify_promote`처럼 `confirm`이 있는 도구는 기본 dry-run이다.

## 11. API Documentation Gate

`agent-harness api-doc check`(정적+에이전트)와 MCP `api_doc_static_check` 후 `api_doc_review`는 staged API candidate files만 기본 검사한다. 후보는 controller/DTO/handler/router/OpenAPI/Swagger/schema 파일명 기준으로 고른다. `--all`은 명시적으로 전체 tracked API 후보를 에이전트에게 검토시킬 때만 사용하며, 기본 pre-commit 경로는 legacy 전체 부채를 실패시키지 않아야 한다.

권장 package script for target Node/Nest repos:

```json
{
  "scripts": {
    "swagger:check": "agent-harness api-doc check --json"
  }
}
```

lint-staged/pre-commit에서는 staged controller/DTO 파일만 넘긴다. 전체 점검이 필요할 때는 `npm run swagger:check -- --all`처럼 명시적으로 실행한다.

### Business-logic-aware API docs

API docs review must inspect the changed endpoint's directly related business logic, not only decorators/comments. If service/usecase/domain code can return public API errors such as 404 Not Found, 403 Forbidden, 409 Conflict, validation 400, or auth 401, the OpenAPI responses must document them. The MCP resource `harness://api-doc-guidance` exposes this guidance to agents.


## OpenAPI prompt source

Endpoint/controller/DTO/schema/OpenAPI 변경 시 `.agent-harness/OPEN_API_SPEC.md`를 프로젝트별 프롬프트 source로 사용한다. `agent-harness api-doc review`는 별도 `--prompt-file`이 없으면 이 문서를 자동으로 포함한다.


## 12. Contract, audit, worker MVP

```bash
./bin/agent-harness contract schema --json
./bin/agent-harness contract check --json
HARNESS_AUDIT_LOG="$(mktemp)" ./bin/agent-harness policy audit --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
HARNESS_WORKER_DIR="$(mktemp -d)" ./bin/agent-harness worker enqueue --kind smoke --payload "TOKEN=redacted" --json
```

`policy audit`는 redacted JSONL policy decision을 기록하고 command를 실행하지 않는다. `worker`는 lifecycle record만 저장하는 no-shell MVP다. future process execution은 command policy, audit logging, timeout/cancellation, redaction check를 통과해야 한다.

### Comprehensive doctor

`agent-harness doctor`는 일반 사용자가 먼저 실행할 종합 진단 명령이다. 설치, hook, MCP, daemon, user-state, project lifecycle namespace, `.agent-harness` 문서, repo-local runtime state 흔적을 read-only로 점검하고 해결책을 제안한다. `agent-harness state doctor`는 여전히 checkpoint store 무결성만 보는 좁은 명령이다.

```bash
./bin/agent-harness doctor --repo . --json
```

팀 repo에서는 `.agent-harness/state/`, `.agent-harness/state.schema.json` 같은 per-user runtime/schema 데이터를 커밋하지 않는다. lifecycle schema와 validator는 agent-harness core가 소유하고, 실제 queue/profile 상태는 user-state 아래 repo별 namespace에 저장한다.
