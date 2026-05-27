# 하네스 사용법

현재 `agent-harness`는 Codex와 Claude Code가 같은 Go binary와 MCP schema를 쓰도록 다음 표면을 제공한다.

1. Codex/Claude native skills: `atomic-commit-push`, `llm-wiki`
2. MCP stdio proxy: `harness mcp` → shared `agent-harness daemon`
3. CLI: `harness inspect/preflight/docs/policy/state/llm-wiki/daemon/self-verify/self-augment`

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
- Claude user SessionStart hook 활성화: `~/.claude/settings.json`
- LLM Wiki SessionStart hook helper/템플릿 생성: `scripts/session-start-llm-wiki.sh`, `configs/claude/hooks/session-start-llm-wiki.settings.json`

기본 설치는 적용 대상 repo에 `.claude/skills`, `.claude/settings.json`, `.mcp.json`을 만들지 않는다. 쓰기 전 계획만 확인하려면 `./bin/harness install-native --dry-run --json`을 사용한다. repo-local 파일이 필요할 때만 `./bin/harness install-native --project-local`을 명시적으로 사용한다.

---

## 2. Codex에서 사용

### Native skills

예시:

```text
Use $llm-wiki to search durable local knowledge before answering.
Use $atomic-commit-push to review my changes, split them into atomic commits, and push safely.
```

설치 확인:

```bash
test -f ~/.codex/skills/llm-wiki/SKILL.md && echo ok
test -f ~/.codex/skills/atomic-commit-push/SKILL.md && echo ok
```

### MCP

Codex MCP 등록 확인:

```bash
codex mcp list
codex mcp get agent_harness
```

`harness mcp`는 user-level daemon을 자동 시작하고 stdio를 daemon socket으로 proxy한다. 새 Codex 세션은 MCP initialize instructions와 tools/resources를 통해 `llm-wiki` session context를 발견할 수 있다.

---

## 3. Claude Code에서 사용

### Native skills

Claude Code는 기본적으로 user skill 경로에서 중앙 원본을 본다.

- User: `~/.claude/skills/<skill>/SKILL.md`
- Project: `.claude/skills/<skill>/SKILL.md`는 기본 설치에서 만들지 않는다. repo-local attach가 필요한 경우에만 `--project-local`을 사용한다.

직접 호출 예:

```text
/llm-wiki
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
scripts/session-start-llm-wiki.sh
configs/claude/hooks/session-start-llm-wiki.settings.json
```

Hook은 core logic을 갖지 않고 `harness llm-wiki session-context`만 호출한다. 출력은 Claude Code hook 형식의 JSON이며, `HARNESS_SESSION_CONTEXT_MODE=plain`을 주면 generic/Codex-compatible hook runner용 plain text로 출력한다.

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
./bin/harness llm-wiki inventory --json
./bin/harness llm-wiki session-context --json
./bin/harness llm-wiki search --query "llm wiki" --limit 5 --json
./bin/harness llm-wiki read --page llm-wiki-pattern --json
./bin/harness llm-wiki capture --title "Reusable finding" --content "..." --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/harness self-verify history --prefix self-verify --json
./bin/harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --json
./bin/harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --confirm --json
./bin/harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
./bin/harness self-augment --cycles=1 --target-score=95 --json
./bin/harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/harness self-augment lesson --candidate reflexion-state-memory --lesson "..." --next-action "..." --json
./bin/harness mcp
```

---

## 5. LLM Wiki 운영

Canonical root:

```text
~/workspace/knowledge-base/llm-wiki
```

Override:

```bash
LLM_WIKI_ROOT=/path/to/llm-wiki ./bin/harness llm-wiki inventory --json
```

세션 시작 context는 다음을 포함한다.

- current vault inventory
- `00-meta/index.md` bounded excerpt
- 언제 검색해야 하는지 / 하지 말아야 하는지
- `10-sources/`, `20-wiki/`, `30-sessions/`, `.obsidian/` write boundary
- 사용 가능한 MCP tools/resources

MCP tools:

- `llm_wiki_inventory`
- `llm_wiki_session_context`
- `llm_wiki_search`
- `llm_wiki_read`
- `llm_wiki_capture`

MCP resources:

- `harness://llm-wiki/session-context`
- `harness://llm-wiki/inventory`
- `harness://llm-wiki/index`
- `harness://llm-wiki/schema`

자세한 설계와 리서치 근거는 `agent_docs/LLM_WIKI_INTEGRATION.md`를 따른다.

---

## 6. MCP smoke test

```bash
tmp_state="$(mktemp -d)"
tmp_wiki="$(mktemp -d)"
mkdir -p "$tmp_wiki/00-meta" "$tmp_wiki/20-wiki/concepts"
printf '# Schema\n' > "$tmp_wiki/00-meta/AGENTS.md"
printf '# Wiki Index\n\n- [[llm-wiki-pattern]]\n' > "$tmp_wiki/00-meta/index.md"
printf '# Log\n' > "$tmp_wiki/00-meta/log.md"
printf '%s\n' '---' 'title: LLM Wiki Pattern' 'type: concept' 'status: active' 'tags: [llm-wiki]' '---' '' 'Durable wiki memory.' > "$tmp_wiki/20-wiki/concepts/llm-wiki-pattern.md"
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  '{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"harness://llm-wiki/session-context"}}' \
  | HARNESS_STATE_DIR="$tmp_state" HARNESS_DAEMON_DIR="$tmp_state/daemon" LLM_WIKI_ROOT="$tmp_wiki" ./bin/harness mcp
HARNESS_DAEMON_DIR="$tmp_state/daemon" ./bin/harness daemon stop --json
rm -rf "$tmp_state" "$tmp_wiki"
```

응답에 `llm_wiki_session_context`, `llm_wiki_search`, `harness://llm-wiki/session-context`가 포함되어야 한다.

---

## 7. 자기 검증 루프 / 자가 증강 루프 요약

`self-verify --json`은 전체 run/step count, 실패 위치, step label coverage, contract version/hash, coverage claim matrix/`coverage_gaps`, 실패 시 `failure_class`/`failure_clusters`/`rerun_commands`, bounded stdout/stderr metadata, parallel isolation evidence, 가장 느린 step 상위 5개, 목표별 `goal_scores`를 `summary`에 포함한다. 실패 triage는 먼저 `summary.failed_*`, `summary.goal_scores`, `summary.slowest_steps`를 확인한다.
`--progress=jsonl`은 stdout의 최종 JSON summary를 유지하면서 stderr에 iteration/step JSON Lines heartbeat를 기록한다. redirect 또는 장기 실행 환경에서 hang으로 오해하지 않도록 진행 상태를 볼 때 사용한다. self-verify에는 golden/docs/skill metadata의 unredacted secret-like 문자열을 막는 redaction audit도 포함된다.

`--save-state`를 함께 쓰면 전체 runs 로그가 아니라 compact summary snapshot만 `state_key`에 저장한다. `history`, `compare`, `promote`는 저장된 summary checkpoint를 조회·비교·승격한다. `history --retention-limit N`은 newest-first 기준 보존/삭제 후보를 계산하고, `--prune-retention --confirm`이 있을 때만 초과 checkpoint를 삭제한다; `--confirm`이 없으면 dry-run이다.

`self-augment --json`은 `GENIUS_THINK.md`와 repo signal을 바탕으로 자가 증강 후보 curriculum과 95점 종료 계약을 반환한다. `--save-state`는 선택 후보와 open/satisfied 후보 목록을 `self_augmentation_plan` state snapshot으로 저장해 다음 cycle에서 같은 일을 반복하지 않도록 한다. `self-augment lesson`은 Reflexion식 교훈을 `self_augmentation_lesson` state snapshot으로 저장하고 llm-wiki capture draft를 함께 반환한다. 실제 구현 실행은 native skill `$self-augment`가 담당한다.
