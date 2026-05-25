# 하네스 사용법

현재 `agent-harness`는 문서 전용 상태가 아니라, 다음 세 가지 네이티브/외부 실행 표면을 제공한다.

1. Codex native skill: `atomic-commit-push`
2. Claude Code native skill: `atomic-commit-push`
3. MCP stdio server: `harness mcp`

---

## 1. 설치/갱신

저장소 루트에서 실행한다.

```bash
./scripts/install-native.sh
```

이 스크립트는 다음을 수행한다.

- `go build -o bin/harness ./cmd/harness`
- Codex skill symlink 생성: `~/.codex/skills/atomic-commit-push`
- Claude user skill symlink 생성: `~/.claude/skills/atomic-commit-push`
- Claude project skill symlink 생성: `.claude/skills/atomic-commit-push`
- Claude project MCP 설정 생성: `.mcp.json`
- Codex MCP 설정 추가/갱신: `~/.codex/config.toml`의 `[mcp_servers.agent_harness]`

---

## 2. Codex에서 사용

### Native skill

새 Codex 세션에서 다음처럼 호출한다.

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

새 Codex 세션에서 MCP tool search/discovery를 통해 `agent_harness` 서버의 tools를 사용할 수 있다.

현재 제공 tools:

- `harness_inspect`
- `atomic_commit_preflight`
- `commit_policy`
- `skill_manifest`
- `docs_index`
- `command_policy_check`
- `command_fake_run`
- `state_write`
- `state_read`
- `state_list`
- `state_prune`
- `state_doctor`
- `state_migrate`
- `self_augment`
- `self_augment_history`
- `self_augment_compare`
- `self_augment_promote`

---

## 3. Claude Code에서 사용

### Native skill

Claude Code는 user skill과 project skill 양쪽에서 같은 원본을 본다.

- User: `~/.claude/skills/atomic-commit-push/SKILL.md`
- Project: `.claude/skills/atomic-commit-push/SKILL.md`

Claude Code에서 직접 호출:

```text
/atomic-commit-push
```

또는 자연어로 요청하면 description에 따라 자동 호출될 수 있다.

```text
현재 변경사항을 atomic commit으로 나누고 Conventional + Lore 형식으로 커밋해줘.
```

### MCP

프로젝트 루트의 `.mcp.json`이 `bin/harness mcp`를 등록한다.

확인:

```bash
claude mcp list
```

Claude Code 세션 안에서는 다음으로 상태를 볼 수 있다.

```text
/mcp
```

---

## 4. CLI로 직접 사용

```bash
./bin/harness version
./bin/harness inspect --json
./bin/harness preflight --json /path/to/git-repo
./bin/harness docs --json
./bin/harness policy check --workspace-root "$PWD" --cwd "$PWD" --json -- git status --short
./bin/harness policy fake-run --workspace-root "$PWD" --cwd "$PWD" --write --json -- touch marker
./bin/harness state write --key checkpoint-1 --value "작업 메모" --json
./bin/harness state read --key checkpoint-1
./bin/harness state list --json
./bin/harness state prune --max-age 720h --json          # dry-run
./bin/harness state prune --max-age 720h --confirm --json
./bin/harness state doctor --json
./bin/harness state migrate --json                       # dry-run
./bin/harness state migrate --confirm --json
./bin/harness self-augment --iterations=10 --seed=100 --save-state --state-key self-augment-latest --json
./bin/harness self-augment history --prefix self-augment --json
./bin/harness self-augment compare --baseline-key self-augment-baseline --candidate-key self-augment-latest --json
./bin/harness self-augment promote --from-key self-augment-latest --baseline-key self-augment-baseline --confirm --json
./bin/harness mcp
./bin/harness self-augment --iterations=10 --seed=100
```

`self-augment --json`은 전체 run/step count, 실패 위치, step label coverage, 가장 느린 step 상위 5개를 `summary`에 포함한다. 실패 triage는 먼저 `summary.failed_*`와 `summary.slowest_steps`를 확인한다.
`--save-state`를 함께 쓰면 전체 runs 로그가 아니라 compact summary snapshot만 `state_key`에 저장한다.
`self-augment history`는 저장된 summary checkpoint를 `generated_at` 최신순으로 조회해 baseline/candidate key를 빠르게 찾게 한다. 기본 prefix는 `self-augment`이고 `--limit 0`은 전체 조회다.
`self-augment compare`는 저장된 summary checkpoint 2개를 비교하며, `--max-elapsed-regression-pct`로 elapsed time 회귀 threshold를 조정하고 `--fail-on-regression`으로 CI gate처럼 사용할 수 있다.
`self-augment promote`는 compare 통과 후 candidate summary를 baseline key로 복사한다. 기본은 dry-run이고 실제 승격은 `--confirm`이 필요하다.

`preflight`는 read-only로 git 상태를 확인한다.

- branch/upstream
- staged/unstaged/untracked files
- secret-like path
- 최근 Conventional/Lore commit style hint

`docs`는 하네스의 agent-facing markdown을 lightweight index로 노출한다.

- 대상: `AGENTS.md`, `CLAUDE.md`, `agent_docs/*.md`
- 출력: relative path, absolute path, title, headings, byte size
- MCP 대응: `docs_index` tool, `harness://docs` resource

`policy`는 실제 명령 실행 전 command boundary를 검증한다.

- `policy check`: argv 요청이 허용되는지 평가만 수행
- `policy fake-run`: policy 평가 후 **명령을 실행하지 않고** fake runner 결과와 audit id를 반환
- 기본 거부: workspace 밖 cwd, shell interpreter, network/write 미허용 명령, read-only allowlist 밖 명령, secret-like argument
- catalog 확인: `harness://command-policy` resource 또는 `command_policy_check`/`command_fake_run` 응답의 policy metadata
- shell은 `--shell --shell-reason TEXT`가 있어야 예외 검토 대상이 된다.
- 거부된 fake-run은 exit code `3`을 사용한다.

`state`는 에이전트 간/턴 간 작은 체크포인트를 repo 소스와 분리해 저장한다.

- 기본 위치: `~/.local/state/agent-harness/`
- 테스트/임시 위치 override: `HARNESS_STATE_DIR=/tmp/agent-harness-state`
- key 규칙: `[A-Za-z0-9._-]`, 최대 128자, `/`, `\`, `..` 금지
- content 입력: `--value TEXT`, `--input FILE`, `--stdin` 중 하나
- cleanup: `prune`은 기본 dry-run이며, 실제 삭제는 `--confirm`을 명시해야 한다.
- compatibility: 새 record는 `schema_version=1`로 저장한다. version이 없는 legacy record는 `read/list/prune`과 호환되며, `doctor`는 `legacy_schema` warning을 보고하고 `migrate --confirm`은 내용을 보존한 채 schema만 승격한다.

---

## 5. MCP smoke test

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
  | ./bin/harness mcp
./bin/harness self-augment --iterations=10 --seed=100
./bin/harness self-augment --iterations=10 --seed=100 --save-state --state-key self-augment-latest --json
./bin/harness self-augment history --prefix self-augment --json
./bin/harness self-augment compare --baseline-key self-augment-baseline --candidate-key self-augment-latest --json
./bin/harness self-augment promote --from-key self-augment-latest --baseline-key self-augment-baseline --confirm --json
```

응답에 `harness_inspect`, `atomic_commit_preflight`, `commit_policy`, `skill_manifest`, `docs_index`, `command_policy_check`, `command_fake_run`, `state_write`, `state_read`, `state_list`, `state_prune`, `state_doctor`, `state_migrate`, `self_augment`, `self_augment_history`, `self_augment_compare`, `self_augment_promote`가 포함되어야 한다.
