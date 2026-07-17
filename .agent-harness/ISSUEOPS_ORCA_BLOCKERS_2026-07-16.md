# IssueOps / Orca 진행 차단 원인 기록 — 2026-07-16

## 범위와 결론

이 기록은 `/Users/m16khb/Workspace/agent-harness`에서 GitHub 이슈를 Orca worker로 배분하려던 중 관찰된 차단을 코드, durable IssueOps record, Codex session log, 설치 파일, GitHub 상태로 다시 검증한 결과다.

핵심 결함은 active supervised cycle이 여러 개일 때 PreToolUse guard가 **읽기 전용 여부보다 record 선택을 먼저 수행**한 것이다. 그래서 owner를 고를 필요가 없는 `pwd`, 안전한 `rg`, read-only Git, 명시적 파일 read까지 `multiple active supervised IssueOps cycles`로 막혔다. 동시에 exact parser가 문서와 실제 작업에서 쓰인 `bin/agent-harness` 표기를 받지 않아 정확한 `--id`가 있는 status/resume도 id-less 요청처럼 취급됐다.

이번 수정은 다중-cycle source checkout에서 기존의 좁은 read-only grammar와 명시적 read tool만 record 선택 전에 처리하고, exact parser에 `bin/agent-harness`를 추가한다. 테스트·patch·Orca terminal write·malformed shell은 계속 fail-closed다.

## 현재 상태 스냅샷

2026-07-16 재조회에서 같은 source checkout에 다음 다섯 durable handoff가 `coordinator_preparing`으로 남아 있었다.

| IssueOps ID | branch | phase | attempt base |
|---|---|---|---|
| `io-339c2fca0e34` | `19-review-strategy-research-pioneers` | `compatibility-review` | `18a8083e3f2ad1b9185edd7e4a0c8a2738e5d1a5` |
| `io-4f4603393a22` | `20-review-execution-quality-pioneers` | `compatibility-review` | `18a8083e3f2ad1b9185edd7e4a0c8a2738e5d1a5` |
| `io-5ee972c2604d` | `25-verify-coordinator-cancel-authority` | `implement` | `5a146eaf068f0f6170fe7a05815a47e501487189` |
| `io-c85c9bca45ac` | `33-fix-legacy-terminal-inventory-dispatch` | `compatibility-review` | `3e816373bb6abf6a0a2c6090c35c93a0abbdd6e0` |
| `io-ff473d80b45b` | `21-review-engineering-workflow-pioneers` | `compatibility-review` | `18a8083e3f2ad1b9185edd7e4a0c8a2738e5d1a5` |

이 ambiguity 자체는 정상이다. guard가 임의 cycle을 고르면 안 된다. 결함은 ambiguity를 mutation fence로 유지하면서도 안전하게 관찰·복구할 통로가 막혔다는 점이다.

GitHub 재조회 결과 open issue는 `#18`, `#19`, `#20`, `#21`, `#28`이고, `#22`와 그 복구 child `#23/#24/#25/#26/#27/#29/#31/#33/#35/#37/#39/#41`은 닫혀 있다. PR `#43`은 merge commit `18a8083e3f2ad1b9185edd7e4a0c8a2738e5d1a5`로 병합됐다. 따라서 남아 있는 open issue는 hook을 끄거나 durable record를 지워서 숨길 대상이 아니라, hook 복구 뒤 exact cycle로 재개해야 할 실제 작업이다.

## 차단 원인별 기록

### B-01 — 여러 active supervised cycle의 source checkout ambiguity

- 증상: source checkout에서 id-less mutation 또는 owner가 필요한 명령을 실행하면 `multiple active supervised IssueOps cycles share this source checkout`으로 차단된다.
- 직접 원인: `selectSupervisedHandoffRecord`가 같은 `record.Repo`에 매칭되는 supervised record를 둘 이상 발견한다.
- 영향: 임의 cycle 선택을 방지하므로 writer 차단 자체는 올바르다. 그러나 B-02와 결합해 관찰 명령까지 막으면서 복구 진입점이 사라졌다.
- 안전한 탈출 경로: `issueops status --id` 또는 `issueops resume --id`로 exact cycle을 선택한다. 상태 전이는 그 cycle의 canonical lifecycle command만 사용한다.
- 상태: ambiguity는 **의도된 안전 경계로 유지**. 관찰 deadlock만 이번 변경에서 수정.
- 근거: 위 다섯 `issueops status --id ... --json` 결과와 `internal/core/lifecycle/lifecycle_handoff_authority.go`의 source match 분기.

### B-02 — read-only 분류보다 supervised record 선택이 먼저 실행됨

- 증상: `pwd`, 안전한 `rg`, `git status --short`, read-only Git, `mcp__filesystem__read_text_file` 같은 요청이 B-01 메시지로 차단됐다.
- 직접 원인: `handoffOwnershipBlockReason`이 `selectSupervisedHandoffRecord`를 먼저 호출하고, 기존 `ExactReadOnlyShellCommand` / `explicitHandoffReadOnlyTool` 검사는 그 뒤에 있었다.
- 영향: 코드를 읽고 상태를 확인할 수 없어 원인을 진단하거나 exact recovery command를 만들 수 없는 전역 self-lock이 발생했다. 이전 세션에서는 `git status`, `pwd`, `apply_patch`, collaboration/code-mode 내부 tool까지 같은 pre-selection ambiguity의 영향을 받았다.
- 안전한 탈출 경로: 기존 hardened read-only grammar만 multi-cycle source ambiguity 전에 평가한다. 일반 shell, `cat`, test/build/format/install/generate는 read-only로 승격하지 않는다.
- 상태: **이번 변경에서 수정**. `TestHandoffMultiCycleAllowsExactLifecycleAndReadOnlyObservations`와 writer/negative companion이 허용·차단 경계를 함께 고정한다.
- 근거: `internal/core/lifecycle/lifecycle_handoff_guard.go`, `.agent-harness/turing/evidence/G1-C2-observations.txt`, `.agent-harness/turing/evidence/G1-C3-negative-fence.txt`.

### B-03 — exact parser가 `bin/agent-harness` 실행 표기를 누락

- 증상: `bin/agent-harness issueops status --id <id> --json`이 exact lifecycle 명령으로 인식되지 않아 `--id`가 선택자에 전달되지 않았다.
- 직접 원인: parser가 `agent-harness`와 `./bin/agent-harness`만 허용했다. 저장소 운영 문서와 실제 명령에는 세 번째 동등 표기인 `bin/agent-harness`도 사용된다.
- 영향: 정확한 cycle ID가 있어도 id-less source command처럼 fallback되어 B-01 ambiguity에 걸렸다.
- 안전한 탈출 경로: 세 표기만 exact parser에서 허용하고, shell control/expansion과 unknown flag validation은 그대로 유지한다.
- 상태: **이번 변경에서 수정**.
- 근거: `internal/core/commandparse/issueops.go`, `TestParseExactIssueOpsCommandAcceptsRepoLocalBinSpelling`의 RED→GREEN 증거.

### B-04 — 이전 복구 명령에 존재하지 않는 `handoff recover` flags가 추가됨

- 증상: 과거 세션이 다음 형태의 명령을 만들었다.

  `agent-harness issueops handoff recover --id 'io-56533379bff3' --action reconcile --coordinator-recipient ... --coordinator-host codex --coordinator-session-id ... --source-cwd ... --confirm --json`

- 직접 원인: `handoff start` 계열 identity flags를 `handoff recover`에도 쓸 수 있다고 추정했다. 실제 recover handler에는 `--id`, `--action`, `--cleanup-disposition`, `--cleanup-step`, `--confirm`, `--force`, `--reason`, `--json`만 있다.
- 영향: exact parser의 `IssueOpsCommandSpec`가 unknown flag 때문에 명령 전체를 거부했고, hook selection에서는 다시 id-less ambiguity로 보였다. CLI에 도달해도 flag parse 단계에서 실패한다.
- 안전한 탈출 경로: 아래 canonical recover 문법만 사용하고 `--action`별 confirm 요건은 CLI help와 durable state를 따른다.
- 상태: **운영 원인 기록 완료**. parser를 느슨하게 만들어 잘못된 flags를 허용하지 않는다.
- 근거: Codex rollout `2026/07/15/rollout-2026-07-15T20-57-15-019f65a3-65ef-7f90-aeec-76a3bf582597.jsonl`의 exact command와 `./bin/agent-harness issueops handoff recover --help`.

### B-05 — hook 백업 파일명과 실제 복구 대상 불일치

- 증상: 예상했던 `~/.codex/hooks.json.issueops-dispatch.bak`과 현재 `~/.codex/hooks.json`은 없고, 실제 보존본은 `~/.codex/hooks.json.issueops-recovery.bak`이다.
- 직접 원인: 복구 과정에서 설명한 backup suffix와 실제 실행된 suffix가 달랐다.
- 영향: 존재하지 않는 파일을 복구 대상으로 삼으면 unrelated Orca hook group을 잃거나 빈 config에 agent-harness hook만 설치할 수 있다.
- 안전한 탈출 경로: `issueops-recovery.bak`을 authoritative pre-disable snapshot으로 복원한 뒤 installer의 merge 경로를 실행하고, unrelated hook group 보존을 diff로 검증한다.
- 상태: **복원·검증 완료**. authoritative recovery backup을 mode 보존 복원한 뒤 fixed installer가 merge했으며, 최종 파일은 mode `0600`, agent-harness/Orca 그룹 모두 보존됐다.
- 근거: 2026-07-16 `ls/stat`: recovery backup은 mode `0600`, 4,651 bytes, `hooks.json`/dispatch backup은 absent.

### B-06 — config 파일 교체와 현재 Codex runtime reload는 동일하지 않음

- 증상: `~/.codex/hooks.json`을 바꿔도 이미 실행 중인 Codex session이 이전 hook definition을 계속 사용할 수 있다. 반대로 trust review 때문에 hook이 조용히 skip되면 “실패 없음”을 성공으로 오인할 수 있다.
- 직접 원인: hook runtime은 session setup/refresh 시 definition과 trust fingerprint를 적재한다. 파일 readback만으로 active runtime 적용을 증명할 수 없다.
- 영향: 수정 binary를 설치해도 현재 session에서 옛 self-lock이 계속 보이거나, hook 자체가 실행되지 않은 false-green이 생길 수 있다.
- 안전한 탈출 경로: `hooks/list`를 exact cwd로 조회하고 warnings/errors/enabled command를 검증한 뒤 fresh hooks-enabled Codex process에서 allow와 deny를 각각 관찰한다.
- 상태: **native E2E 검증 완료**. fresh app-server `hooks/list`와 ephemeral `codex exec`에서 enabled/trusted inventory, read-only allow, mutation pre-execution block을 모두 관찰했다.
- 근거: `.agent-harness/CAUTIONS.md`의 Codex hook runtime/trust 항목과 설치 후 G2-C2/G2-C3 증거.

### B-07 — source mutation 차단은 장애가 아니라 supervised ownership 규칙

- 증상: active handoff가 있는 source checkout에서 `apply_patch`, `go test`, build/format/install/generate, raw Orca terminal send가 차단된다.
- 직접 원인: source coordinator는 observation/lifecycle authority만 갖고 implementation writer는 claimed worker root로 제한된다.
- 영향: 이 경계를 일반 read-only 허용과 함께 풀면 두 writer가 같은 cycle을 수정하거나 다른 cycle의 terminal을 조작할 수 있다.
- 안전한 탈출 경로: exact cycle을 resume/start/claim한 뒤 worker worktree에서 실행한다. recovery가 필요하면 exact ID로 reconcile/cancel/abandon/retry한다.
- 상태: **의도된 fail-closed 경계로 유지**.
- 근거: `TestHandoffMultiCycleBaselineBlocksAmbiguousSourceWriters`, `TestHandoffMultiCycleKeepsMalformedAndMutatingRequestsBlocked`.

### B-08 — legacy Git worktree와 Orca worker identity/terminal은 같은 것이 아님

- 증상: Git worktree가 존재하고 Orca `worktree show` identity가 있어도 worker terminal/task/dispatch가 0개여서 supervised worker가 시작되지 않았다.
- 직접 원인: 기존 Git checkout의 path/branch/HEAD가 맞는 것과 Orca가 그 checkout을 exact worktree instance로 adopt하고 terminal/task/dispatch를 봉인하는 것은 별도 단계다.
- 영향: raw legacy worktree를 이미 worker-capable하다고 간주하면 dispatch가 없거나 foreign/stale terminal을 채택할 수 있다.
- 안전한 탈출 경로: #28의 exact adoption 조건(path·branch·HEAD·repo·instance·marker·issue metadata)과 singleton terminal bootstrap을 통과한 뒤 durable identity와 raw Orca inventory를 교차 확인한다.
- 상태: **GitHub #28 open backlog**. hook deadlock 수정만으로 자동 완료되지 않는다.
- 근거: GitHub issue #28 body와 현재 `28-legacy-worktree-handoff-e2e` worktree.

### B-09 — branch prepare의 provider base SHA가 로컬에서 확인되지 않던 readiness 실패

- 증상: 준비된 provider base SHA가 로컬 commit/tracking ref로 확인되지 않으면 handoff prepare가 `prepared base SHA is missing or stale in the local repository`로 멈춘다.
- 직접 원인: source/remote ref 동기화 없이 branch metadata만 먼저 기록되었거나, 오래된 parent base를 가리킨 historical cycle이 있었다.
- 영향: attempt lineage를 증명할 수 없으므로 dispatch를 생성할 수 없다.
- 안전한 탈출 경로: provider ref를 read-only로 확인하고 exact full SHA를 `branch prepare --base-sha`에 기록한다. 현재 #19/#20/#21 records는 모두 `18a8083...`를 보유한다.
- 상태: **신규 pioneer cycles에서는 해소**, 오래된 #25/#33 records는 각자의 historical base를 유지하므로 임의 rewrite하지 않는다.
- 근거: `internal/core/issueops/issueops_handoff_prepare.go`의 fail-closed check와 위 durable snapshot.

### B-10 — Git publication lock이 command-line origin 설정을 mutable config로 오인

- 증상: Git credential/config가 command-line origin에서 주입된 환경에서 publication lock/readiness가 실패했다.
- 직접 원인: immutable command-line origin을 workspace에서 수정 가능한 config와 동일하게 다뤘다.
- 영향: 검증이 끝난 handoff의 coordinator publication이 막혔다.
- 안전한 탈출 경로: command-line origin은 immutable input으로 분류하되 exact remote/branch/HEAD 검증은 유지한다.
- 상태: **PR #43에서 수정·main 병합 완료** (`18a8083...`).
- 근거: PR #43 summary와 main/origin/main HEAD.

### B-11 — response-contract golden이 project-local GJC/Reasonix 설치 상태에 흔들림

- 증상: `TestResponseContractsGolden`이 코드 계약 변경 없이 `project_gjc_skill`, `project_reasonix_skill`, `project_reasonix_settings` boolean mismatch로 실패했다.
- 직접 원인: normalizer는 gitignored project-local Claude/Codex skill 존재 여부만 placeholder로 바꿨고, 같은 성격의 GJC/Reasonix 설치 artifact는 실제 머신 boolean을 golden에 남겼다.
- 영향: 프로젝트에 `.gjc/skills` 또는 `.reasonix`가 설치된 개발자는 원본 golden을 통과하지 못하고, update 모드를 쓰면 환경 상태를 계약 변경처럼 커밋하게 된다.
- 안전한 탈출 경로: 네 host의 project-local skill presence를 같은 placeholder로 정규화하고 Reasonix project settings는 별도 settings-presence placeholder로 정규화한다. user artifact를 삭제하거나 golden을 현재 boolean에 맞추지 않는다.
- 상태: **이번 변경에서 수정**. 단위 테스트 RED→GREEN 뒤 golden diff가 여섯 placeholder 변경으로만 제한됨을 확인했다.
- 근거: `cmd/harness/harnessapp/responsecontract/normalize.go`, `normalize_test.go`, `cmd/harness/testdata/response_contracts.golden.json`.

### B-12 — daemon admission test가 overflow classifier 준비 전에 assertion을 시작

- 증상: 전체 `go test ./...`에서 `TestServeDaemonConnectionBoundsUnclassifiedConnections`가 `connection beyond bounded raw reserve was not rejected: read pipe: i/o timeout`으로 간헐 실패했다.
- 직접 원인: 테스트는 silent connection 두 개를 시작한 뒤 active slot이 1인지만 기다렸다. 두 번째 goroutine이 overflow classifier를 reserve하기 전에 third connection이 먼저 실행되면 third가 classifier를 차지해 read에서 대기하므로, 테스트가 가정한 “beyond reserve” 상태가 아직 아니었다.
- 영향: hook 수정과 무관한 full gate가 flaky하게 실패하고, 실제 daemon capacity regression과 불완전한 test setup을 구분하기 어려웠다.
- 안전한 탈출 경로: production admission을 완화하지 않고 test setup이 active slot 1개와 overflow classifier 1개를 모두 관찰할 때까지 기다린 뒤 third rejection을 검증한다.
- 상태: **이번 변경에서 test synchronization 수정**. 수정 전 isolated `-count=100`에서 4회 실패, 수정 후 일반 `-count=200`과 race `-count=100` 모두 통과했다.
- 근거: `cmd/harness/daemoncli/daemon_admission_test.go`와 위 반복 실행 결과.

### B-13 — self-verify 성공 JSON을 존재하지 않는 `.passed` 필드로 판정

- 증상: full test/race/vet/golden/build 뒤 `self-verify`가 결과 JSON을 끝까지 생성했지만, 외부 QA wrapper의 `jq`가 `false`를 출력하며 검증 파동을 실패로 종료했다.
- 직접 원인: wrapper가 top-level `.passed` boolean이 있다고 추측했다. 실제 `SelfAugmentResult`의 종료 계약은 `.ok`, `.termination_eligible`, `.summary.termination_eligible`이며 top-level `.passed`는 존재하지 않는다. `jq`의 missing field는 `null`이라 boolean 비교가 오류 대신 false가 되어 제품 실패처럼 보였다.
- 영향: 통과한 self-verify를 오탐 실패로 분류하고, 약 6분짜리 full test/race 파동을 처음부터 다시 실행하게 됐다.
- 안전한 탈출 경로: 소비 전에 source/contract의 JSON tag를 확인하고 `.ok == true and .termination_eligible == true and .summary.termination_eligible == true`를 사용한다. 결과 스키마가 바뀌면 wrapper도 contract test로 함께 갱신한다.
- 상태: **검증 wrapper 수정**. 제품 코드 결함과 분리해 튜링 원장에 실패 파동을 남기고 올바른 predicate로 final wave를 다시 실행한다.
- 근거: `cmd/harness/selfworkflow/model/self_verify_summary_types.go:8-23`, `:41-66`의 JSON tags와 `turing-qa-g2-c1-r2` pane의 마지막 `STEP=self-verify` / `false` / exit 1 출력.

### B-14 — 긴 tmux `send-keys` 입력 유실로 서로 다른 검증 명령이 결합

- 증상: 실제로 존재하는 `TestResponseContractsGolden` selector가 pane에서는 `TestReecho STEP=stability`로 변형되어 `[no tests to run]`을 출력했고, build/self-verify 구간 일부가 실행 command에서 사라졌다.
- 직접 원인: 수천 byte짜리 전체 검증 파동을 한 interactive ZLE 입력으로 빠르게 전달했다. tmux/PTY 입력 경로가 중간 suffix를 유실했고, 뒤 chunk의 `echo STEP=stability`가 잘린 `TestRe` 뒤에 결합됐다.
- 영향: command exit는 0인 채 실제 golden이 실행되지 않고 필수 단계가 건너뛰어져, 단계 marker를 확인하지 않으면 false-green으로 완료될 수 있었다.
- 안전한 탈출 경로: 긴 파동은 `apply_patch`로 만든 mode-0700 `/tmp` 스크립트에 고정하고 tmux에는 짧은 script path만 전달한다. 각 필수 단계 marker와 최종 exit를 모두 확인하며 `[no tests to run]`을 실패로 취급한다.
- 상태: **QA 전달 방식을 수정**. 손상된 파동은 무효 처리하고 temp script 기반으로 처음부터 재실행한다.
- 근거: `turing-qa-g2-c1-r3` pane의 literal `TestReecho STEP=stability`, `[no tests to run]`, 누락된 단계 marker와 `cmd/harness/harnessapp/response_contract_golden_test.go:13`의 실제 테스트 선언.

### B-15 — stability audit가 제거된 top-level `bootstrap --sync`를 계속 호출

- 증상: full-install stability audit의 `bootstrap_sync_dry_json` 단계가 `flag provided but not defined: -sync`로 즉시 실패했다.
- 직접 원인: sync는 현재 `project bootstrap --sync` 문서 동기화 표면에만 남아 있는데, audit가 과거 top-level install CLI의 `bootstrap --sync --dry-run --json` 명령을 고정해 두었다.
- 영향: 실제 install-native와 hook smoke가 정상이어도 audit 전체 `ok`가 false가 되어 hooks-enabled 완료 게이트를 통과할 수 없었다.
- 안전한 탈출 경로: install audit는 현재 공개 계약인 `bootstrap --dry-run --json`, `install-native --dry-run --json`, full-install의 `install-native --json`만 실행한다. project-doc sync는 별도 project bootstrap 검증에서 다룬다.
- 상태: **이번 변경에서 수정**. audit unit test가 step 이름과 argv에서 obsolete `--sync` 부재를 고정한다.
- 근거: `skills/stability-audit/scripts/e2e_stability_audit.py`의 `install_checks`, 실제 `bootstrap --help`/`update --help`, `test_install_checks_use_only_current_bootstrap_flags` RED→GREEN.

### B-16 — stability audit timeout이 현재 full gate의 실제 실행시간보다 짧음

- 증상: audit regression의 `go test -race ./...`가 180초 timeout으로 종료되고, 내부 10회 full self-verify가 900초에 강제 종료됐다.
- 직접 원인: 고정 timeout이 현재 저장소 규모와 측정치를 따라가지 못했다. 같은 final wave에서 일반 test는 약 169초, race는 180초를 넘겨 정상 완료했고, 10회 full self-verify는 3712초가 걸렸다.
- 영향: 정상 전진 중인 프로세스를 hang으로 오판해 regression과 audit 전체를 false로 만들고, 1시간 이상 실행한 검증 파동을 무효화했다.
- 안전한 탈출 경로: 단일 full regression command timeout은 300초, 10회 full self-verify timeout은 5400초로 명명된 상수에 둔다. timeout 상한을 줄이려면 먼저 반복 가능한 새 측정치와 회귀 테스트를 갱신한다.
- 상태: **이번 변경에서 수정**. unit test가 두 timeout 하한과 실제 `run` 전달값을 고정한다.
- 근거: `/tmp/turing-g2-c1-stability-r4.json`의 180초/900초 timeout failure, 외부 self-verify `elapsed_ms=3712048`, `test_regression_timeouts_cover_observed_full_gate_durations` RED→GREEN.

### B-17 — stability audit가 nonzero self-verify의 원인 증거를 폐기

- 증상: 10회 내부 self-verify가 약 64분 실행된 뒤 regression이 실패했지만 report에는 `summary: null`과 `ok: false`만 남고 exit code, timeout 여부, parse 오류, stdout/stderr가 없었다.
- 직접 원인: audit는 self-verify가 exit 0일 때만 JSON을 parse했고, 결과 detail에는 성공 summary와 duration만 투영했다. nonzero 또는 parse 실패 경로의 진단 필드는 모두 버렸다.
- 영향: 실제 test flake, 제품 self-verify 실패, JSON parse 문제를 구분할 수 없어 동일한 장시간 검증을 근거 없이 다시 실행해야 했다.
- 안전한 탈출 경로: self-verify detail에 `returncode`, `timed_out`, parsed `ok`, `termination_eligible`, `parse_error`, bounded stdout/stderr tail을 항상 기록하고 raw 장기 산출물은 판정 전 보존한다.
- 상태: **이번 변경에서 수정**. 실패 진단 보존을 요구하는 audit unit test를 RED→GREEN으로 추가했고 attempt 6에서 B-18의 실제 원인을 확정했다.
- 근거: attempt 5 report의 `cmd=self-verify`, `ok=false`, `summary=null`, `duration_ms=3853826`; `test_regression_preserves_self_verify_failure_diagnostics` RED의 `KeyError: returncode`; attempt 6 report에 보존된 exit/stderr/progress tail.

### B-18 — stability audit 내부 self-verify가 비결정적 LLM 평가 게이트를 암묵적으로 활성화

- 증상: 내부 self-verify의 10회·250단계와 `loop_end ok=true`가 모두 성공했는데도 command가 `LLM evaluation gate failed: llm_eval_not_ok; score 0.00 below target 95.00`로 exit 1을 반환했다.
- 직접 원인: audit 명령은 deterministic final wave와 달리 `--llm-eval=false`를 생략했다. 현재 CLI 기본값이 LLM 평가를 켜므로 외부 평가 비가용/실패가 코드 검증 성공을 뒤집었다.
- 영향: 동일 소스의 명시적 deterministic self-verify는 통과하지만 audit 내부 재실행은 마지막 외부 평가에서 실패해 약 64분의 정상 검증을 두 번 무효화했다.
- 안전한 탈출 경로: repository stability gate는 `--llm-eval=false`를 명시해 코드·계약 검증만 판정한다. LLM 평가는 별도 advisory/gate 작업에서 자격 증명·응답을 독립 검증한다.
- 상태: **이번 변경에서 수정**. audit command에 `--llm-eval=false`를 추가했고 unit test가 progress, timeout, deterministic LLM 설정을 함께 고정한다.
- 근거: attempt 6 stability report의 `returncode=1`, `timed_out=false`, 마지막 `loop_end ok=true`와 직후 LLM evaluation error; `test_regression_timeouts_cover_observed_full_gate_durations`의 flag assertion RED→GREEN.

### B-19 — dropped child를 Stop relay가 계속 incomplete로 분류

- 증상: 병합된 #22 parent에 대해 child #31이 `validation_verdict=dropped`로 정리됐는데도 Stop hook은 `bound_cycle:io-0348be41f391; missing: child_incomplete:io-b9abb4d4f2e2`를 계속 붙여 다음 행동 판단을 재진입시켰다.
- 직접 원인: PR readiness의 `issueOpsChildPRGateKey`는 dropped child를 이미 제외하지만, 별도 hot path인 `hookprompt.orchestrationChildMissingKeys`와 `orchestrationChildrenReminder`는 verdict를 확인하지 않고 child record의 `phase != done`만 보았다. 또한 이미 `phase=done`인 bound parent도 orchestration 대상으로 다시 읽었다.
- 영향: parent가 child 변경을 직접 포함해 검증한 뒤 child를 정당하게 drop해도 Stop relay가 영구 incomplete로 남는다. 사용자는 같은 3개 선택지와 승인 판단을 반복해서 보게 되고 coordinator 진행이 교착된다.
- 안전한 탈출 경로: hookprompt에서도 `validation_verdict=dropped` child를 active total·missing key에서 제외하고, `phase=done`인 bound cycle은 orchestration reminder/relay 대상에서 제외한다. rejected나 verdict 없는 child는 기존대로 차단한다.
- 상태: **이번 변경에서 수정**. 실제 #22/#31 형태를 재현한 named RED 두 건을 확인한 뒤 최소 구현과 기존 relay companion을 GREEN으로 만들었다.
- 근거: `internal/core/hookprompt/orchestration_reminder.go`, `TestOrchestrationReminderIgnoresDroppedChild`, `TestOrchestrationReminderIgnoresDoneBoundCycle`, 2026-07-17 `issueops status/child list`의 parent `io-0348be41f391`·dropped child `io-b9abb4d4f2e2`.

### B-20 — cross-process PR-create 테스트가 subprocess 실패를 삼켜 원인 없는 flake를 만듦

- 증상: 7차 full self-verify의 seed 106에서 `TestCreateRemotePullRequestTwoProcessesInvokeProviderOnce`가 `provider-count: no such file or directory`로 실패했다. 앞선 6개 seed와 standalone 50회는 통과했다.
- 직접 원인: 테스트가 두 `exec.Cmd.Run`을 goroutine에서 동시에 시작하고 반환 error를 버렸으며, helper도 초기 durable record read error를 조용히 return했다. 두 helper가 provider callback 전에 종료되면 count 파일만 없고 실제 start/read/create 원인은 모두 사라졌다.
- 영향: cross-process single-invocation production invariant의 결함과 OS/process-test setup 실패를 구분할 수 없고, 약 40분 진행된 self-verify가 진단 불가능한 한 줄로 종료됐다.
- 안전한 탈출 경로: 두 helper를 parent에서 순서대로 `Start`해 둘 다 launch됐음을 먼저 확인한 뒤 동시에 실행시키고, 각 `Wait` error와 bounded output을 보존한다. helper는 record read 실패를 test failure로 내고 create 승자/차단자를 각각 `created`/`blocked`로 표시한다.
- 상태: **이번 변경에서 테스트 하네스를 수정**. 새 harness로 named test 50회 연속 통과했으며 최종 full wave에서 다시 검증한다.
- 근거: `/tmp/turing-g2-c1-selfverify-r7.json`의 failed seed 106/step `go test`, `internal/core/issueops/issueops_remote_create_claim_test.go`, 수정 전후 named `-count=50` 실행 기록.

### B-21 — dropped prefix가 bounded child scan budget을 소모해 뒤의 active child를 숨김

- 증상: B-19의 단순 skip 구현에서 parent child 목록 앞 16개가 모두 dropped이고 17번째가 active이면 Stop relay facts가 빈 문자열을 반환했다.
- 직접 원인: `orchestrationChildMissingKeys`가 raw `ChildCycles`를 먼저 16개로 자른 뒤 dropped verdict를 건너뛰었다. 반면 reminder total은 전체 non-dropped 목록을 기준으로 해 두 hot path의 bounded semantics도 달라졌다.
- 영향: scope에서 제거된 child가 read budget을 모두 소비해 실제 incomplete child가 Stop gate에서 누락되는 fail-open이 생길 수 있었다.
- 안전한 탈출 경로: 전체 child refs를 한 번 순회하면서 dropped를 제외한 total을 세고, 처음 16개 non-dropped ref만 공통 bounded slice로 만든다. reminder와 missing-key 계산이 그 동일 projection을 사용한다.
- 상태: **self-review에서 최종 파동을 중단하고 수정**. 16 dropped + 1 active fixture가 수정 전 빈 facts로 RED, 수정 후 exact `child_incomplete:<id>`로 GREEN이다.
- 근거: `boundedOrchestrationChildren`, `TestOrchestrationRelayBoundCountsNonDroppedChildren`, 중단된 Turing attempt 8의 self-review 기록.

### B-22 — self-verify JSON parse 자체를 returncode 0 분기 안에 둠

- 증상: B-17 보강 뒤에도 self-verify가 exit 1과 유효한 JSON summary를 함께 반환하면 audit detail의 `parsed_ok`, `termination_eligible`, `summary`가 모두 null이었다.
- 직접 원인: diagnostic 필드를 추가했지만 `parse_json_output` 호출이 `if step_ok` 안에 남았다. 따라서 가장 진단이 필요한 nonzero 결과는 stdout tail만 보존되고 구조화된 종료 필드는 다시 폐기됐다.
- 영향: 250단계 이후 평가 실패처럼 “실행 결과는 유효하지만 최종 exit만 nonzero”인 사례를 자동 분류할 수 없고, 사람이 긴 JSON tail을 다시 해석해야 했다.
- 안전한 탈출 경로: returncode와 무관하게 bounded stdout을 parse하고 parse error를 별도로 기록한다. 최종 성공 판정만 `returncode == 0 && parsed.ok && parsed.termination_eligible`로 결합한다.
- 상태: **diff self-review에서 수정**. exit 1 + valid JSON fixture가 수정 전 `summary=None`으로 RED, 수정 후 exact failed-step summary 보존으로 GREEN이다.
- 근거: `test_regression_preserves_self_verify_failure_diagnostics`, `skills/stability-audit/scripts/e2e_stability_audit.py`의 unconditional `parse_json_output` 경로.

### B-23 — Codex installer가 co-resident hook 배열 인덱스를 바꿔 trust hash를 서로 뒤바꿈

- 증상: real `install-native` 뒤 fresh `hooks/list`에서 agent-harness와 Orca의 `PreToolUse`, `PostToolUse`, `SessionStart`, `UserPromptSubmit`, `Stop` 항목이 모두 `enabled=true`이지만 `trustStatus=modified`로 나타났다. 단독 event인 agent-harness `PreCompact`/`PostCompact`만 `trusted`였다.
- 직접 원인: Codex는 user hook trust를 `source-path:event:matcher-index:hook-index` key와 `trusted_hash`로 저장한다. 기존 설정은 agent-harness가 matcher index 0, Orca가 index 1이었지만 `mergeHookConfig`가 agent-harness 그룹을 제거하고 모든 제3자 그룹 뒤에 새 그룹을 append했다. 설치 후 Orca가 index 0, agent-harness가 index 1이 되어 각 key에 저장된 hash가 반대 command의 current hash와 비교됐다.
- 영향: 설정 파일과 direct shell smoke는 정상이어도 새 Codex 세션은 두 hook 집합을 변경된 미신뢰 코드로 판단한다. trust review를 건너뛰지 않는 정상 실행에서는 enforcement가 실행되지 않거나 추가 승인에 막힐 수 있어 hooks-enabled 완료를 거짓으로 보고하게 된다.
- 안전한 탈출 경로: 첫 기존 agent-harness 그룹을 발견한 바로 그 배열 위치에서 새 generated group으로 교체하고, 유효한 co-resident 그룹의 상대 위치를 보존한다. agent-harness가 없을 때만 append하며 duplicate agent-harness 그룹은 제거한다. 설치 후 `hooks/list`의 exact key/currentHash/trustStatus를 다시 검증한다.
- 상태: **fresh runtime 검증 중 발견해 attempt 9를 중단하고 이번 변경에서 수정·설치 검증 완료**. named test는 수정 전 Orca가 index 0으로 이동하는 정확한 RED를 출력했고 수정 후 GREEN이다. authoritative backup 복원과 fixed reinstall 뒤 13개 hook 모두 enabled/trusted이고 warnings/errors는 비어 있었다.
- 근거: Codex 0.144.5 `hooks/list` 결과, 설치 binary strings의 `hooks.state`/`trusted_hash`, `/Users/m16khb/.codex/config.toml`의 stored key/hash와 `internal/adapter/codex/install_hooks.go`, `TestMergeHookConfigPreservesCoResidentHookPositions`.

### B-24 — cross-process remote create가 SQLite 최초 초기화 경합을 그대로 노출

- 증상: 독립 반대 검토 뒤 helper error를 더 이상 `blocked`로 뭉개지 않자, 동시 PR-create stress에서 `sqlstore open/init data/span db: database is locked (5) (SQLITE_BUSY)`와 예상 외 phase error가 실제 subprocess exit로 드러났다.
- 직접 원인: `sqlstore.Open`의 process-local cache mutex는 다른 OS process를 직렬화하지 못한다. 두 process가 새 state root의 data/span SQLite 파일과 schema를 동시에 처음 만들면 `newDB`의 open/schema pragma가 일시적인 BUSY를 반환했지만, `Open`은 span transaction과 달리 이 typed lock contention을 재시도하지 않았다. 동시에 테스트 helper는 모든 create error를 같은 `blocked` marker로 바꿔 이 원인을 숨겼다.
- 영향: 실제 동시 coordinator publication에서 provider가 한 번만 호출되는 안전 불변식과 무관한 초기화 경쟁이 요청 하나를 실패시킬 수 있었다. 기존 회귀는 한 승자·한 차단자와 count byte만 세어 이 실패를 정상 차단으로 허위 통과시킬 수 있었다.
- 안전한 탈출 경로: `sqlstore.Open`은 오직 typed `SQLITE_BUSY`/`SQLITE_LOCKED` 초기화 오류만 10초 한도로 재시도하고 다른 오류는 즉시 반환한다. process test는 두 helper가 start barrier에 모두 도달한 뒤 동시에 진입시키며, exact claim/finalized exclusion만 `blocked`로 인정하고 그 밖의 오류는 helper별 stderr와 nonzero exit로 보존한다.
- 상태: **독립 검토 후 이번 변경에서 수정**. 오류 분류 unit test는 helper classifier 부재로 RED였고, 강화된 process test가 수정 전 BUSY/예상 외 오류를 반복 재현했다. 최소 재시도와 barrier/진단 보강 후 claim/create process tests 20회, `sqlstore`/`issueops` 일반·race package가 통과했다.
- 근거: `internal/core/sqlstore/sqlstore.go`의 `newDBWithRetry`, `internal/core/issueops/issueops_remote_create_claim_test.go`의 exact outcome classifier/start barrier/full Wait, 반대 검토 finding과 named `-count=20`/package race 출력.

## 금지된 우회

- routine 운영에서 hook 전체를 비활성화하지 않는다. 이번 비활성화는 self-lock 복구를 위한 일회성 조치였으며 설치·live E2E 뒤 원복한다.
- 같은 Orca create/dispatch를 ambiguity 상태에서 재시도하지 않는다. timeout/error 뒤에는 durable `pending_operation`/inventory를 reconcile한다.
- 여러 record 중 임의 하나를 선택하거나, 진행을 위해 durable record/worktree를 삭제하지 않는다.
- 잘못된 recovery flags를 parser allowlist에 추가하지 않는다.
- `cat`, test/build/format/install/generate, arbitrary shell을 read-only로 승격하지 않는다.

## Canonical 관찰·복구 명령

```bash
./bin/agent-harness issueops status --id <io-id> --json
./bin/agent-harness issueops resume --repo /Users/m16khb/Workspace/agent-harness --id <io-id> --json
./bin/agent-harness issueops handoff recover --id <io-id> --action <reconcile|abandon|cancel|finalize-cancel|retry|approve-cleanup|record-cleanup> [--confirm] [--force] [--reason TEXT] --json
```

`--confirm`과 `--force`는 action/state별 CLI 계약을 따른다. 명령을 만들기 전에 exact `status --id`를 읽고, 다른 cycle의 terminal/worktree identity를 재사용하지 않는다.

## 완료 검증 요구

1. parser 및 multi-cycle allow/deny 회귀가 통과한다.
2. 전체 Go test/race/vet/build, contract golden, self-verify, stability audit가 통과한다.
3. recovery backup을 복원한 뒤 native installer가 unrelated hook group을 보존한다.
4. 설치된 모든 agent-harness hook event가 schema-valid JSON을 출력한다.
5. Codex `hooks/list`와 fresh process에서 read-only allow 및 ambiguous mutation deny가 모두 실제로 관찰된다.
6. 모든 QA tmux/session/process와 임시 artifact가 정리된다.
