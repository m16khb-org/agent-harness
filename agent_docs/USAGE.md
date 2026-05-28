# 하네스 사용법

현재 `agent-harness`는 Codex와 Claude Code가 같은 Go binary와 MCP schema를 쓰도록 다음 표면을 제공한다.

1. Codex/Claude native skills: `atomic-commit-push`, `self-augment`
2. MCP stdio proxy: `harness mcp` → shared `agent-harness daemon`
3. CLI: `harness inspect/preflight/docs/policy/state/daemon/self-verify/self-augment/api-doc`

---

## 1. 설치/갱신

저장소 루트에서 실행한다.

```bash
./scripts/install-native.sh
```

이 스크립트는 다음을 수행한다.

- `go build -o bin/harness ./cmd/harness`
- `harness install-native` 실행
- Codex user skill symlink 생성: `~/.codex/skills/* -> <agent-harness>/skills/*`
- Claude user skill symlink 생성: `~/.claude/skills/* -> <agent-harness>/skills/*`
- Codex MCP 설정 추가/갱신: `~/.codex/config.toml`의 `[mcp_servers.agent_harness]`
- Claude user-scope MCP 서버 등록: `claude mcp add-json -s user agent-harness ...`

기본 설치는 적용 대상 repo에 `.claude/skills`, `.claude/settings.json`, `.mcp.json`을 만들지 않는다. 쓰기 전 계획만 확인하려면 `./bin/harness install-native --dry-run --json`을 사용한다. repo-local 파일이 필요할 때만 `./bin/harness install-native --project-local`을 명시적으로 사용한다.

---

## 2. Codex에서 사용

### Native skills

예시:

```text
Use $atomic-commit-push to review my changes, split them into atomic commits, and push safely.
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

`harness mcp`는 user-level daemon을 자동 시작하고 stdio를 daemon socket으로 proxy한다.

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

기본 설치는 user-scope MCP 서버 `agent-harness`가 중앙 `bin/harness mcp`를 등록한다. 이 레포의 `.mcp.json`은 dogfood/project-local 템플릿 역할이다.

확인:

```bash
claude mcp list
```

Claude Code 세션 안에서는 다음으로 상태를 볼 수 있다.

```text
/mcp
```

### SessionStart hook

Claude Code 공식 hook 문서는 `SessionStart`의 `hookSpecificOutput.additionalContext`가 Claude context로 추가된다고 설명한다. `./scripts/install-native.sh`는 user settings `~/.claude/settings.json`에 중앙 hook을 활성화하고, project-local opt-in용 템플릿도 보관한다.

```text
~/.claude/settings.json
```


---

## 4. CLI로 직접 사용

```bash
./bin/harness version
./bin/harness install-native --json
./bin/harness install-native --dry-run --json
./bin/harness inspect --json
./bin/harness preflight --json /path/to/git-repo
./bin/harness docs --json
./bin/harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
./bin/harness state write --key checkpoint-1 --value "작업 메모" --json
./bin/harness state read --key checkpoint-1 --json
./bin/harness state list --json
./bin/harness state prune --max-age 720h --json
./bin/harness state prune --max-age 720h --confirm --json
./bin/harness state doctor --json
./bin/harness state migrate --json
./bin/harness state migrate --confirm --json
./bin/harness daemon start --json
./bin/harness daemon status --json
./bin/harness daemon stop --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/harness self-verify candidates --json
./bin/harness self-verify candidates --save-state --state-key self-verify-candidates-latest --json
./bin/harness self-verify history --prefix self-verify --json
./bin/harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --json
./bin/harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --confirm --json
./bin/harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
./bin/harness self-augment --cycles=1 --target-score=95 --json
./bin/harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/harness self-augment lesson --candidate reflexion-state-memory --lesson "..." --next-action "..." --json
./bin/harness api-doc review --json
./bin/harness mcp
```

---

## 5. MCP smoke test

```bash
tmp_state="$(mktemp -d)"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://commit-policy"}}' \
  | HARNESS_STATE_DIR="$tmp_state" HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/harness mcp
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/harness daemon stop --json
rm -rf "$tmp_state"
```

## 6. 자기 검증 루프 / 자가 증강 루프 요약

`--progress=jsonl`은 stdout의 최종 JSON summary를 유지하면서 stderr에 iteration/step JSON Lines heartbeat를 기록한다. redirect 또는 장기 실행 환경에서 hang으로 오해하지 않도록 진행 상태를 볼 때 사용한다. self-verify에는 golden/docs/skill metadata의 unredacted secret-like 문자열을 막는 redaction audit도 포함된다.

`--save-state`를 함께 쓰면 전체 runs 로그가 아니라 compact summary snapshot만 `state_key`에 저장한다. `history`, `compare`, `promote`는 저장된 summary checkpoint를 조회·비교·승격한다. `compare`는 전체 elapsed, `slow_step:*`, label별 p95 `step_budget:*` regression을 함께 보고한다. `history --retention-limit N`은 newest-first 기준 보존/삭제 후보를 계산하고, `--prune-retention --confirm`이 있을 때만 초과 checkpoint를 삭제한다; `--confirm`이 없으면 dry-run이다.

## 7. API Documentation Pre-commit Review

`harness api-doc review` provides a reusable, framework-agnostic AI gate for API documentation drift. It is not tied to NestJS or one repository. By default it inspects staged API candidate files only, including names such as `*controller*`, `*dto*`, `*handler*`, `*router*`, `openapi.*`, `swagger.*`, and other API/schema files.

```bash
# Review staged API docs/endpoint changes in the current repo
harness api-doc review

# JSON output for hooks/CI
harness api-doc review --json

# Explicit files from lint-staged/pre-commit
harness api-doc review -- src/users/users.controller.ts internal/api/user_handler.go openapi.yaml

# Add project-specific conventions without changing harness core
harness api-doc review --prompt-file docs/api-doc-rules.md
```

The reviewer runs Codex non-interactively with `gpt-5.5` and `model_reasoning_effort=medium` by default, in read-only/no-approval mode. The prompt is framework-agnostic and asks the model to apply the target project's own API documentation conventions, e.g. NestJS Swagger decorators, Go swaggo comments, OpenAPI YAML/JSON, Spring/FastAPI equivalents, or other machine-readable API documentation.

Hook example:

```js
// lint-staged.config.js
module.exports = {
  '*.{ts,js,go,yaml,yml,json}': (files) => [
    'harness api-doc review -- ' + files.join(' '),
  ],
}
```


## 8. LLM Wiki 정책

LLM Wiki 기능은 agent-harness가 직접 제공하지 않는다. 중복 구현을 피하기 위해 upstream `nvk/llm-wiki`의 Codex/Claude plugin 또는 portable AGENTS.md를 사용한다. 하네스 CLI/MCP에 llm-wiki 전용 명령, tool, resource, SessionStart hook을 추가하지 않는다.
