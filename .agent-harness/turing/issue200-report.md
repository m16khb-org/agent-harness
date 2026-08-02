# Issue #200 Turing 검증 보고서

- 이슈: https://github.com/m16khb/agent-harness/issues/200
- lifecycle: `io-3d9b5af8721b`, direct generation 1
- 대상 브랜치: `117-hexagonal-architecture-migration`
- source 브랜치: `200-final-architecture-gate`
- sealed base: `9883f534f50fb628ac2fa754a3bf8f80d4734989`
- local acceptance HEAD: `f1d347df9c5a1c8bd6a5766b11f34aa32e139772`
- 설계: `docs/superpowers/specs/2026-08-02-final-architecture-gate-design.md`
- 계획: `docs/superpowers/plans/2026-08-02-final-architecture-gate.md`

## 결과

최종 production import graph에는 네 compatibility package 모두 package-level
caller가 남아 있었다. 따라서 package와 legacy allowlist entry의 제거 목록은
모두 비어 있다. 삭제량을 성공 기준으로 삼지 않고 active caller와 contract
ownership이 있는 경계를 보존했다.

| compatibility package | production importer 수 | 판정 |
|---|---:|---|
| `internal/core` | 41 | 보존 |
| `internal/core/issueops` | 15 | 보존 |
| `internal/port` | 28 | 보존 |
| `cmd/harness/issueopscli` | 1 | 보존 |

두 번 독립 캡처한 `go list -json ./...` production-only graph의 정규화 SHA-256은
모두 `bf3e95dded1a56286d43cb2b39aaf0607f0fd959053a3fb3c380e53cbe9fa2a1`이다.
각 package의 전체 reverse-import 경로는 위 설계 문서와 원격 issue body에
동일하게 기록했다.

`internal/architecture/testdata/legacy_imports.txt`의 base blob과 acceptance HEAD
blob은 모두 `68419916362da135256036e820cc4b6478d763c8`이다. Production Go package
삭제는 0건이고 public DTO, CLI/MCP JSON, error code, hook deny semantics와
persisted schema를 변경하지 않았다.

## Acceptance evidence

| acceptance | 결과 | 핵심 증거 |
|---|---|---|
| AC-200-01 | PASS | 두 normalized graph digest가 동일하고 importer 수 41/15/28/1 및 전체 경로를 설계와 issue에 기록했다. |
| AC-200-02 | PASS | 네 package 모두 production caller가 남아 있어 삭제하지 않았다. Symbol-level unused 여부를 package 삭제 근거로 확대하지 않았다. |
| AC-200-03 | PASS | 제거 대상이 없으므로 legacy allowlist를 수정하지 않았고 base/current blob SHA가 동일하다. |
| AC-200-04 | PASS | `go test ./internal/architecture -count=1`이 `TestProductionGraphMatchesBaseline`과 forbidden/new/stale edge gate를 통과했다. |
| AC-200-05 | PASS | release, claim, reseed, resume, reconcile, publication, completion, preparation을 포함한 architecture ratchet과 focused unit/race가 통과했다. |
| AC-200-06 | PASS | 현재 hook-disabled 부모 세션을 제외했다. Host별 full native payload matrix와 별도 fresh Codex/Claude configured runtime이 native block/deny 및 sentinel 미생성을 증명했다. Exact holder의 source-CWD compatibility도 유지했다. |
| AC-200-07 | PASS | Production/runtime/schema 변경은 0건이다. Contract golden과 response golden이 통과했고 baseline bytes도 동일하다. |
| AC-200-08 | LOCAL PASS / REMOTE PENDING | focused unit/race, architecture, golden, full unit/race, vet, build와 deterministic self-verify 250/250이 통과했다. PR final-head GitHub CI는 publication 뒤 이 문서에 추가한다. |
| AC-200-09 | PENDING | child PR을 `117-hexagonal-architecture-migration`에 만든 뒤 CI/merge/close/cleanup receipt를 추가한다. Parent #117은 유지한다. |

## 활성형 훅 환경 검증

현재 메인 Codex 프로세스는 `--disable hooks`로 실행됐으므로 어떤 합격 근거에도
사용하지 않았다. 검증은 빌드된 child binary와 repository 제공 설정을 사용한
두 독립 proof layer로 수행했다.

### Full native payload direct matrix

Codex와 Claude 각각 별도 SQLite fixture에 같은 generation holder를 host별로
투영하고 transcript/agent-transcript metadata, session ID, cwd, hook event,
permission mode와 tool input을 포함한 전체 payload를 전달했다.

| host / case | 결과 |
|---|---|
| Codex exact holder, canonical CWD | `{}` allow |
| Codex exact holder, source CWD + explicit canonical target | `{}` allow |
| Codex foreign session | native `decision=block`, `holder_identity_mismatch`, axis `session_id` |
| Codex mismatched process receipt | native block, axis `session_process_ancestry` |
| Codex unrelated CWD, exact holder identity 없음 | native block |
| Claude exact holder, canonical CWD | `{}` allow |
| Claude exact holder, source CWD + explicit canonical target | `{}` allow |
| Claude foreign session | `hookSpecificOutput.permissionDecision=deny`, axis `session_id` |
| Claude mismatched process receipt | native deny, axis `session_process_ancestry` |
| Claude unrelated CWD, exact holder identity 없음 | native deny |

모든 mutation sentinel은 생성되지 않았다. 이 matrix는 unrelated CWD를 독립
권한축으로 만들지 않으며, exact holder가 source checkout CWD에서 explicit
canonical target을 지정하는 기존 contract를 보존한다.

### Fresh configured Codex runtime

- 버전: `codex-cli 0.146.0`
- isolated `CODEX_HOME`의 `hooks.json`은 `configs/codex/hooks.json`과 `cmp`로
  byte-identical했다.
- `codex app-server --enable hooks --stdio`의 `hooks/list`가
  `./bin/agent-harness hook pre-tool-use ...`를 enabled hook으로 반환했다.
- fresh session `019fbfcb-d4ef-7291-a422-f2fde1fd4ad8`을
  `--enable hooks --dangerously-bypass-hook-trust --ephemeral`로 실행했다.
- 정확히 한 번 요청한 sentinel `touch`는 PreToolUse에서
  `holder_identity_mismatch`/axis `session_id`로 block됐다.
- sentinel은 process 종료 뒤에도 없었다.
- `--disable hooks`는 사용하지 않았다.

### Fresh configured Claude runtime

- 버전: `2.1.220 (Claude Code)`
- `--settings configs/claude/hooks.settings.json --setting-sources local
  --include-hook-events --permission-mode bypassPermissions`로 fresh session
  `9b083014-aec4-49da-b645-022f089fcc72`을 실행했다.
- stream event에서 repository 설정의 `SessionStart`, `UserPromptSubmit`,
  `PreToolUse:Bash`, `Stop` command가 실제로 시작·응답한 것을 확인했다.
- 정확히 한 번 요청한 sentinel `touch`는
  `hookSpecificOutput.permissionDecision=deny`,
  `holder_identity_mismatch`/axis `host`로 거부됐다.
- process 결과의 `permission_denials`에 해당 Bash tool call이 포함됐고
  sentinel은 없었다.
- `--safe-mode`와 `--bare`는 사용하지 않았다.

## 최종 gate 중 발견한 테스트 시간폭탄

첫 full suite는 `internal/core/hookmetrics`의 line/byte prune 테스트에서
`Kept:0`으로 실패했다. 두 fixture가 `2026-07-03`에 고정돼 있었고 실행 시각이
720시간 경계를 넘어 모든 event가 정상적으로 age-prune된 것이 원인이었다.

`f1d347df`는 production 코드를 건드리지 않고 두 fixture의 base를 실행 시각
1시간 전으로 옮겼다. 수정 전 focused test는 10/10 실패했고 수정 뒤 unit/race
각 10/10 통과했다. 동시에 full unit/race를 돌린 최초 시도에서 발생한
`webfetch` comparator 5초 kill은 isolated 10회가 10/10 통과했고, 이후 전체
unit과 race를 순차 실행해 둘 다 통과함으로써 자원 경쟁으로 분리했다.

## Shannon 품질 gate와 AI slop clean

- Diff inventory: committed 설계·계획 2개, test fixture 1개와 untracked Turing
  report 1개를 모두 측정 범위에 포함했다.
- SNR before/after: production source delta가 0이고 전체 diff가 docs-dominant라
  수치 SNR은 `N/A`다. 유일한 code delta는 test timestamp 2줄을 동일 역할의
  상대 timestamp 2줄로 교체한 것이다.
- Secondary metrics: changed production function 0, 새 production duplicate block
  0, production channel overhead `N/A`다.
- Heuristic caveat: production AST 입력이 없어 line-delta/scope audit를 사용했다.
- No-input guard: test delta 2 additions/2 deletions은 명시하고, production 입력 0을
  0으로 나누거나 임의 SNR로 만들지 않았다.
- Claim/minimality audit: 각 변경 path는 #200 또는 final gate에서 발견한 test
  시간폭탄에 직접 연결되고, unsupported completion claim과 cleanup 대상
  scaffolding/주석/추상화는 없었다. Cleanup 중 추가 파일 변경은 하지 않았다.

## 로컬 최종 검증

다음 명령은 canonical #200 worktree의 `f1d347df`에서 실행했고 모두 exit 0이다.

```text
go test ./internal/architecture -count=1
go test ./internal/core/issueops ./internal/application/issueopslease ./internal/application/issueopspublication ./internal/application/issueopscompletion ./internal/application/issueopspreparation -count=1
go test -race ./internal/core/issueops ./internal/application/issueopslease ./internal/application/issueopspublication ./internal/application/issueopscompletion ./internal/application/issueopspreparation -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
go test ./internal/core/hookmetrics -count=10 -run 'TestPruneHookMetricsLogKeepsNewestEntriesWithin(Line|Byte)Limit'
go test -race ./internal/core/hookmetrics -count=10 -run 'TestPruneHookMetricsLogKeepsNewestEntriesWithin(Line|Byte)Limit'
go test ./internal/core/webfetch -run TestRunBenchmarkLiveRunsBaselineComparatorAsBlackBox -count=10
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
git diff --check 9883f534f50fb628ac2fa754a3bf8f80d4734989...HEAD
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
  result: ok=true, 10/10 iterations, 250/250 steps, failed_steps=0,
  minimum_goal_score=100, termination_eligible=true, elapsed_ms=1334011
```

## 변경 범위와 커밋

Acceptance 동작에 영향을 주는 base-to-HEAD 변경은 설계·계획 2개 문서와 test
fixture 1개뿐이며, 이 보고서는 evidence artifact로 추가된다. Production Go와
legacy baseline 변경은 없다.

```text
5b3e6919 docs(architecture): define final migration gate
5280ed88 docs(architecture): harden final gate evidence
f45ee479 docs(architecture): preserve source-cwd hook contract
f1d347df test(hookmetrics): avoid time-bound prune fixtures
```

OpenWiki, public API/schema, runtime behavior, compatibility package와 unrelated
legacy entry는 수정하지 않았다.

## Turing evidence block

Success criteria: AC-200-01부터 AC-200-07은 PASS, AC-200-08은 local PASS,
AC-200-09는 publication/CI/merge receipt 전까지 PENDING이다.

Evidence artifact: `.agent-harness/turing/issue200-report.md`, final architecture
design/plan, normalized import digest, full native payload matrix, fresh configured
Codex/Claude hook event receipts와 deterministic self-verify summary.

Cleanup receipt: child의 ignored `.codex/hooks.json`과 evidence directory는
`/Users/m16khb/.Trash/agent-harness-issue200-proof.OrZ4QA`로 이동했다. Isolated
host homes, state fixtures, schema와 import captures는
`/Users/m16khb/.Trash/agent-harness-issue200-temp.nnFDY7`로 이동했다. 모두
recoverable하며 원래 path 부재와 tracked worktree clean 상태를 확인했다.

Verification mode: high-risk full loop. Focused/race/architecture/golden/vet/build,
Codex/Claude direct payload와 fresh configured runtime, local deterministic
self-verify 10회를 실행했다.

Skipped checks: local 필수 check는 없다. `llm-eval=false`는 issue가 요구한
결정적 self-verify mode다. Final-head GitHub CI와 child cleanup은 PR publication
뒤 수행한다.
