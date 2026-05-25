# 자가증강 루프

`agent-harness`의 자가증강 루프는 `/Users/habin/workspace/eye-tracking-scroll/scripts/self-augment.js`의 운영 방식을 하네스에 맞게 옮긴 것이다.

참고한 원칙:

- 최소 10회 반복을 강제한다.
- 각 반복은 `base_seed + iteration - 1` seed를 사용한다.
- 매 반복마다 정적 invariant, smoke, fuzz, 외부 경계 검증을 모두 수행한다.
- 첫 실패에서 즉시 중단하고 실패 단계의 stdout/stderr/error를 남긴다.
- 통과 시 반복 횟수와 elapsed time을 요약한다.
- JSON 결과의 `summary`는 전체 run/step 수, 실패 위치, step label coverage, 가장 느린 step 상위 5개를 포함한다.

---

## 현재 구현

CLI:

```bash
./bin/harness self-augment --iterations=10 --seed=100
./bin/harness self-augment --iterations=10 --seed=100 --json
./bin/harness self-augment --iterations=10 --seed=100 --save-state --state-key self-augment-latest --json
./bin/harness self-augment history --prefix self-augment --json
./bin/harness self-augment compare --baseline-key self-augment-baseline --candidate-key self-augment-latest --json
./bin/harness self-augment promote --from-key self-augment-latest --baseline-key self-augment-baseline --confirm --json
```

MCP tool:

- `self_augment`
  - `iterations`: 10 이상
  - `seed`: deterministic fuzz fixture base seed
  - `save_state`: true이면 compact summary checkpoint를 harness state에 저장
  - `state_key`: 저장 key, 기본 `self-augment-latest`
- `self_augment_history`
  - `prefix`: 조회할 state key prefix, 기본 `self-augment`
  - `limit`: 반환 개수 제한, 기본 20, 0은 전체
- `self_augment_compare`
- `self_augment_promote`

`--json` 출력은 다음 triage 필드를 포함한다.

- `summary.total_runs`, `summary.total_steps`, `summary.passed_steps`, `summary.failed_steps`
- `summary.failed_iteration`, `summary.failed_seed`, `summary.failed_step` (실패 시)
- `summary.step_labels`
- `summary.slowest_steps`

`--save-state`는 전체 `runs` 원문 대신 compact summary snapshot만 state에 저장한다. 저장된 state record의 content는 `kind: self_augment_summary`, `schema_version: 1`, `summary`를 포함한다.
`self-augment history`는 저장된 summary snapshot들을 `generated_at` 최신순으로 정렬해 운영 중 baseline/candidate key를 찾는 데 쓴다. self-augment summary가 아닌 같은 prefix record는 실패하지 않고 `skipped`로 보고한다.
`self-augment compare`는 저장된 summary checkpoint 2개를 비교해 elapsed time 증가, failed step 증가, step label 누락을 regression으로 보고한다.
`self-augment promote`는 regression이 없는 candidate summary를 baseline key로 승격한다. 기본은 dry-run이며 `--confirm`을 명시해야 쓴다.

자가증강 루프는 새 capability가 추가될 때마다 smoke/fuzz 단계를 늘리는 것을 원칙으로 한다. 현재 `docs` capability는 agent docs index smoke로, `command policy` capability는 allow/deny/fake-run smoke로, `state` capability는 temp `HARNESS_STATE_DIR`에서 write/read/list/prune/doctor/migrate 라운드트립으로 검증한다.

---

## 반복 단계

각 iteration은 다음 순서로 실행된다.

1. `harness invariants`
   - 필수 문서, skill, MCP 설정, native skill 연결 파일 존재 확인
   - `internal/core/inspect.go`, `internal/core/preflight.go`, `internal/core/docs.go`, `internal/core/state.go`, `internal/core/policy.go` 존재 확인
   - `atomic-commit-push` skill frontmatter 확인
   - legacy 개인 식별자 계열 이름이 repo/config/skill에 남아 있지 않은지 확인
2. `go test`
   - `go test ./... -count=1`
3. `contract golden tests`
   - `go test ./cmd/harness -run Golden -count=1`
   - CLI usage text, MCP tools list, MCP resources list가 `cmd/harness/testdata/*.golden.*`와 일치하는지 확인
   - normalized CLI/MCP 실제 JSON response snapshot(`response_contracts.golden.json`)이 drift 없이 유지되는지 확인
   - self-augment summary helper가 성공/실패 fixture를 올바르게 요약하고, summary checkpoint를 state에 저장할 수 있는지 확인
   - 현재 snapshot은 `inspect`, `docs_index`, `preflight`, `policy_check`, `state_*`, `self_augment_history`, `self_augment_compare`, `self_augment_promote`의 CLI/MCP 응답을 포함한다.
4. `go build`
   - temp dir에 `harness` binary build
5. `inspect smoke`
   - temp binary로 `inspect --json` 실행
   - skill/native/MCP 상태와 legacy name 미노출 확인
6. `docs index smoke`
   - temp binary로 `docs --json` 실행
   - `AGENTS.md`, `CLAUDE.md`, `agent_docs/COMMIT_POLICY.md`, `agent_docs/USAGE.md`가 index에 포함되는지 확인
   - 각 문서 title과 legacy name 미노출 확인
7. `command policy smoke`
   - read-only `git status --short` 요청이 workspace 안에서 허용되는지 확인
   - workspace 밖 `cwd`와 shell interpreter가 거부되는지 확인
   - `policy fake-run`이 write 허용 요청을 받아도 실제 파일을 만들지 않는지 확인
8. `MCP smoke`
   - temp `HARNESS_STATE_DIR`에서 stdio JSON-RPC로 `initialize`, `tools/list`, `resources/read`, `tools/call state_prune`, `tools/call state_doctor`, `tools/call state_migrate` 실행
   - `atomic_commit_preflight`, `docs_index`, `command_policy_check`, `state_write`, `state_prune`, `state_doctor`, `state_migrate`, `self_augment_history`, `Lore:` 리소스 노출 확인
   - `state_prune` MCP tool이 기본 dry-run으로 응답하고 `state_doctor` MCP tool이 healthy 결과를, `state_migrate` MCP tool이 target schema를 반환하는지 확인
9. `state roundtrip`
   - temp `HARNESS_STATE_DIR` 생성
   - `state write --json`, `state read --json`, `state list --json` 실행
   - seed 기반 key/content가 손실 없이 저장·조회·목록화되는지 확인
   - 오래된 checkpoint fixture를 만들고 `state prune --max-age 1h --json` dry-run이 old/fresh key를 올바르게 분류하는지 확인
   - `state prune --max-age 1h --confirm --json`이 old key만 삭제하고 fresh key를 보존하는지 확인
   - legacy schema fixture를 만들고 `state migrate --json` dry-run과 `state migrate --confirm --json`이 schema를 승격하면서 content/updated_at을 보존하는지 확인
   - saved self-augment summary fixture 2개를 만들고 `self-augment compare --json`이 정상/회귀 threshold를 구분하는지 확인
   - `self-augment promote --json` dry-run/confirm이 baseline 승격을 안전하게 처리하고, 승격 후 compare가 깨끗한지 확인
   - `self-augment history --json`이 저장된 baseline/candidate/promoted summary를 최신순 조회 결과에 포함하는지 확인
   - corrupt JSON fixture를 만들고 `state doctor --json`이 `invalid_json` issue와 valid key 보존을 보고하는지 확인
10. `preflight fuzz`
   - seed 기반 temp git repo 생성
   - Conventional + Lore commit 작성
   - secret-like untracked path 생성
   - `preflight --json`이 commit style과 secret-like path를 감지하는지 확인
11. `native integration`
   - `~/.codex/skills`, `~/.claude/skills`, `.claude/skills`, `.mcp.json`, config template 존재 확인
   - Codex MCP config에 `agent_harness` 서버가 있는지 확인

---

## 승격 규칙

- 새 CLI/MCP/native capability를 추가하면 self-augment 반복 단계에 smoke 또는 fuzz를 추가한다.
- 반복적으로 놓친 회귀는 `agent_docs/CAUTIONS.md`에 기록하고 invariant로 승격한다.
- 10회 루프가 너무 느려지면 단계를 삭제하지 말고 targeted quick mode를 별도 command로 추가한다.
- self-augment는 쓰기 작업을 temp dir로 제한해야 한다. 실제 사용자 repo에는 commit/push를 수행하지 않는다.
