# IssueOps / Orca 진행 차단 원인 기록 — 2026-07-16

## 범위와 결론

이 기록은 `/Users/m16khb/Workspace/agent-harness`에서 GitHub 이슈를 Orca worker로 배분하려던 중 관찰된 차단을 코드, durable IssueOps record, Codex session log, 설치 파일, GitHub 상태로 다시 검증한 결과다.

핵심 결함은 active supervised cycle이 여러 개일 때 PreToolUse guard가 **읽기 전용 여부보다 record 선택을 먼저 수행**한 것이다. 그래서 owner를 고를 필요가 없는 `pwd`, 안전한 `rg`, read-only Git, 명시적 파일 read까지 `multiple active supervised IssueOps cycles`로 막혔다. 동시에 exact parser가 문서와 실제 작업에서 쓰인 `bin/agent-harness` 표기를 받지 않아 정확한 `--id`가 있는 status/resume도 id-less 요청처럼 취급됐다.

이번 수정은 다중-cycle source checkout에서 기존의 좁은 read-only grammar와 명시적 read tool만 record 선택 전에 처리하고, exact parser에 `bin/agent-harness`를 추가한다. 테스트·patch·Orca terminal write·malformed shell은 계속 fail-closed다.

## 2026-07-16 복구 전 상태 스냅샷

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
- 상태: **2026-07-17 attempt 5에서 실 Orca E2E 완료·수락**. task `task_13fac2099adb`, dispatch `ctx_ed4a3f4d84ef`, worker terminal `term_2fc8f58c-c71c-469f-b0d9-41484c43cba1`가 동일 worktree에 생성됐고 dispatch는 `completed`, terminal 종료 뒤 issue #28 inventory는 0건, durable handoff는 `closed/accepted`다.
- 근거: GitHub issue #28 body, `orca orchestration dispatch-show --task task_13fac2099adb`, `orca terminal list --worktree issue:28`, `issueops status --id io-959e7d74d2ac`.

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

### B-25 — 닫힌 child의 stale global dispatch가 새 #19 worker 배정을 막음

- 증상: #19 handoff bootstrap 직전에 해당 issue와 무관한 #23 task가 전역 `dispatched` inventory에 남아 새 task dispatch가 singleton/sole-writer 검사에서 거부됐다.
- 직접 원인: GitHub issue와 durable child cycle은 닫혔지만 Orca task/dispatch terminality가 같은 시점에 정리되지 않았다. IssueOps의 record 상태와 Orca runtime의 전역 task 상태는 별도 authority라 한쪽만 닫아서는 충분하지 않았다.
- 영향: 독립적인 새 issue도 runtime 전체의 stale writer 하나 때문에 시작되지 않는다. 단순 재시도는 duplicate task를 만들거나 기존 stale lease를 잘못 채택할 수 있다.
- 안전한 탈출 경로: stale task의 원래 cycle과 terminal을 exact ID로 조회하고, 원래 작업이 끝났음을 durable/GitHub evidence로 확인한 뒤 terminal 상태로 정리한다. 그 후 global `task-list --status dispatched`가 0건인지 확인하고 새 handoff를 시작한다.
- 상태: **정리 완료**. #19/#20/#21/#28 실제 Orca wave를 순차 실행해 모두 accepted로 마쳤다.
- 근거: #19 최초 dispatch 차단 당시 global task inventory와 이후 각 accepted record `io-339c2fca0e34`, `io-4f4603393a22`, `io-ff473d80b45b`, `io-959e7d74d2ac`.

### B-26 — coordinator bootstrap이 canonical source cwd를 암묵 추론하지 않음

- 증상: worker worktree 또는 wrapper cwd에서 `handoff start`를 preview하면 source checkout을 확정하지 못해 bootstrap이 중단되거나 다른 root를 기준으로 hook trust를 검사했다.
- 직접 원인: coordinator source와 worker root가 분리된 supervised handoff에서 process cwd 하나만으로 양쪽 identity를 안전하게 추론할 수 없다. CLI는 이 경우 `--source-cwd`를 요구한다.
- 영향: 올바른 record ID와 worktree가 있어도 hook inventory, coordinator session seal, Git base 검증이 서로 다른 root를 보게 된다.
- 안전한 탈출 경로: coordinator가 canonical source의 `pwd -P`와 `git rev-parse --show-toplevel`을 확인한 뒤 모든 preview/confirm/accept에 동일한 `--source-cwd`를 전달한다.
- 상태: **운영 계약 확인·적용 완료**.
- 근거: #19 bootstrap probe 실패와 이후 `/Users/m16khb/Workspace/agent-harness`를 명시해 성공한 #19/#20/#21/#28 handoff records.

### B-27 — coordinator mailbox 재사용과 전역 sole-writer가 독립 issue도 직렬화함

- 증상: 이미 다른 active handoff가 봉인한 coordinator recipient를 새 cycle이 재사용하면 ownership seal 충돌이 발생했다. 별도 mailbox를 만들어도 global dispatched worker가 있으면 다음 issue는 시작할 수 없었다.
- 직접 원인: coordinator mailbox는 cycle-scoped reply authority이고, 현재 Orca adapter는 repository runtime에 한 명의 active writer만 허용한다. Git 파일이 겹치지 않는다는 사실만으로 runtime lease를 병렬화하지 않는다.
- 영향: #19/#20/#21처럼 독립 skill 파일을 다루는 작업도 handoff bootstrap은 병렬 배정할 수 없고 순차 accepted/cleanup이 필요했다.
- 안전한 탈출 경로: attempt마다 고유 coordinator mailbox를 만들고, worker_done을 받은 뒤 worker terminal 종료·dispatch terminality·durable accept까지 마친 다음 다음 cycle을 시작한다.
- 상태: **설계된 안전 경계로 준수**. 성능 한계는 남지만 임의 병렬화로 완화하지 않는다.
- 근거: 각 accepted record의 서로 다른 `coordinator_mailbox_handle`, dispatch/terminal inventory 0건 확인 뒤 다음 cycle을 시작한 실행 원장.

### B-28 — worker task의 검증 문구와 lifecycle guard의 Orca controller 권한이 충돌

- 증상: #28 worker prompt는 terminal/task/dispatch inventory 재조회를 요구했지만 worker가 `orca terminal`, orchestration inbox/escalation/heartbeat/controller를 직접 실행하면 `coordinator-owned`로 PreToolUse 차단됐다.
- 직접 원인: task의 자연어 acceptance criteria는 coordinator/worker 역할을 구분하지 않았고, guard는 worker에게 exact IssueOps `status`, `resume`, `heartbeat`, `handoff finish`와 일반 구현·검증만 허용한다. raw Orca controller는 coordinator 권한이다.
- 영향: worker가 acceptance criteria를 문자 그대로 수행하려 하면 안전 훅에 반복 차단되고, 이를 실패로 오해해 정상 no-change E2E를 `outcome failed`로 제출할 수 있다.
- 안전한 탈출 경로: worker는 sealed IssueOps state·Git root/branch/HEAD/cleanliness만 확인하고, coordinator가 worker_done 뒤 raw Orca terminal/task/dispatch/worktree inventory를 독립 교차 검증한다.
- 상태: **attempt 5에서 역할 분리로 해결**. task prompt 생성 계약 개선은 후속 후보로 기록한다.
- 근거: attempt 4/5 worker terminal의 `wrapped controller commands are coordinator-owned` feedback, coordinator의 `dispatch-show`, `terminal list`, `worktree show` 결과.

### B-29 — hardened shell grammar가 안전한 관찰 명령의 표기 차이까지 controller로 오분류

- 증상: worker에서 `/bin/pwd`는 outside-target command로, `git version --build-options`는 controller 성격 명령으로, 외부 plugin/source tree read와 compound shell·substitution은 wrapped controller로 차단됐다. 같은 목적의 bare `pwd`, `git rev-parse`, `git status`는 허용됐다.
- 직접 원인: allowlist는 executable basename을 정규화하지 않고 정확한 token grammar만 허용하며, shell control/expansion은 의도적으로 fail-closed다. 안전성은 높지만 동등한 read-only 표기의 사용성이 불균일하다.
- 영향: worker가 보편적인 diagnostic 습관을 따르면 검증 자체가 막히고, 반복 차단 output이 실제 제품 결함을 가린다.
- 안전한 탈출 경로: worker prompt에 허용된 standalone grammar를 명시하고 bare `pwd`, `rg`, 제한된 `git status|diff|log|show|rev-parse`, exact IssueOps lifecycle 명령만 사용한다. allowlist 확대는 별도 negative test와 threat review 없이 하지 않는다.
- 상태: **운영 우회 확정, parser 개선 후보 기록**.
- 근거: `internal/core/commandparse/issueops.go`의 `ExactReadOnlyShellCommand`와 #21/#28 worker PreToolUse feedback.

### B-30 — CLI 표면의 실제 flag 이름을 추측해 Orca 전달과 Git commit이 차단됨

- 증상: Orca terminal 전달에 `--payload`를 사용하면 flag parse가 실패했고 실제 flag는 `--text`였다. worker의 multiline commit body 한 덩어리는 shell-special quoting 검사에 막혔지만 분리된 `git commit -m <subject> -m <body>`는 허용됐다.
- 직접 원인: CLI help/source 확인 전에 유사 도구의 flag 이름과 shell command 형태를 추정했다.
- 영향: 작업 결과와 무관한 command-construction 오류가 lifecycle failure처럼 보이고, worker guidance 도착이 지연됐다.
- 안전한 탈출 경로: mutation 전에 설치 CLI `--help`와 exact parser spec을 확인하고, terminal send는 `orca terminal send --terminal <id> --text <message> --json`, commit은 control operator 없는 단일 command와 분리된 `-m` 인자를 사용한다.
- 상태: **운영 명령 교정 완료**.
- 근거: `orca terminal send --help`, worker terminal의 commit hook feedback, 이후 성공한 #19/#20/#21 commits.

### B-31 — 실패 result의 빈 evidence slice가 JSON/SQLite 왕복 뒤 invalid envelope로 변함

- 증상: #21 attempt 1이 `outcome failed`로 닫힌 직후 같은 record를 다시 읽으면 `closed worker_failed handoff requires a failed result`로 거부됐다. 최초 in-memory result는 valid였지만 persistence 왕복 후 invalid였다.
- 직접 원인: `cleanResultList`가 nil 입력을 non-nil empty slice로 바꿨다. `verification`/`cleanup_receipts`는 `omitempty`라 JSON 저장에서 사라지고 재로드 시 nil이 되지만, `ValidateEnvelope`의 canonical equality는 nil과 empty slice를 다르게 비교했다.
- 영향: 정확한 failed result를 기록한 cycle도 status/recovery가 불가능해져 durable 복구 경로 자체가 막힌다.
- 안전한 탈출 경로: empty canonical list는 `cleanChangedFileList`처럼 nil로 정규화한다. 실제 손상 record는 원본 DB를 백업하고 integrity를 확인한 뒤 exact cycle에 최소 cleanup receipt를 보강해 복구했다.
- 상태: **이번 통합에서 TDD 수정**. `TestFailedResultRemainsValidAfterJSONRoundTrip`이 수정 전 RED, nil canonicalization 뒤 GREEN이다.
- 근거: `internal/core/issueops/handoff/state.go`의 `cleanResultList`, `state_test.go` 회귀 테스트, 복구 전 백업 `/tmp/agent-harness-issueops-before-ff473d80b45b-20260717.db`와 `PRAGMA integrity_check=ok`.

### B-32 — failed Orca task와 dispatch terminality가 항상 함께 전이되지 않음

- 증상: #21의 잘못 종료된 attempt에서 task는 failed였지만 dispatch는 한동안 `dispatched`로 남아 다음 sole-writer bootstrap을 막았다. #28 attempt 4에서는 worker terminal을 닫자 dispatch가 `failed`로 terminalize되어 경로별 동작도 달랐다.
- 직접 원인: task outcome 기록, terminal close, dispatch completion은 별도 Orca operation이며 한 operation의 성공이 다른 두 상태의 원자적 전이를 보장하지 않는다.
- 영향: durable handoff가 closed여도 global dispatched inventory가 남을 수 있고, 성급한 retry는 duplicate writer 위험 때문에 fail-closed된다.
- 안전한 탈출 경로: cleanup은 approve-cleanup → task terminality 확인 → exact task/dispatch receipt → worker terminal close → terminal inventory 0건 → terminal_quiescent receipt 순서를 지킨다.
- 상태: **#21/#28 recovery에서 정리 완료, 교차 시스템 atomicity 한계 기록**.
- 근거: `io-ff473d80b45b` prior attempt와 `io-959e7d74d2ac` attempt 4 cleanup receipts, raw `dispatch-show` 결과.

### B-33 — no-change finish 기능의 source·worktree binary·설치 binary 버전이 달랐음

- 증상: #28 attempt 5 worker가 `./bin/agent-harness issueops handoff finish --no-change`를 만들려 했지만 worktree의 추적 binary help에는 `--no-change`가 없었다. 반면 hook parser source와 `/Users/m16khb/.local/bin/agent-harness` help에는 flag가 있었다. absolute main binary 경로를 쓰면 exact executable grammar를 벗어나 hook이 controller로 차단했다.
- 직접 원인: legacy branch에 포함된 `bin/agent-harness` artifact가 최신 main/install-native보다 오래됐고, exact parser는 bare `agent-harness`, `bin/agent-harness`, `./bin/agent-harness`만 받는다.
- 영향: 최신 no-change contract를 구현한 저장소에서도 worker가 가장 가까운 stale binary를 실행하면 unknown flag 또는 잘못된 failed outcome으로 종료한다.
- 안전한 탈출 경로: version-sensitive lifecycle에는 PATH의 설치 binary `agent-harness`를 사용하고 `command -v`/`--help`를 coordinator가 미리 검증한다. worker에는 `--cleanup-receipt`를 직접 넣지 않고 `--no-change`가 sealed plan path와 no-temp receipt를 파생한다는 exact guidance를 claim 직후 전달한다.
- 상태: **attempt 5에서 해결·accepted**. tracked binary 배포 정책은 별도 개선 후보로 남는다.
- 근거: 동일 #28 worktree에서 `./bin/agent-harness ... --help`에는 flag 부재, `agent-harness ... --help`에는 `-no-change` 존재, durable result의 derived plan/cleanup evidence.

### B-34 — no-change worker guidance가 늦으면 정상 검증을 실패로 제출함

- 증상: #28 attempt 4는 source mutation 없이 exact HEAD와 clean status를 모두 증명했지만 worker가 completed result에는 changed file/Turing report가 필수라고 판단해 `outcome failed`를 먼저 제출했다. coordinator의 no-change 안내는 그 뒤 도착했다.
- 직접 원인: generic completed contract와 `--no-change` 파생 계약이 help/prompt에서 충분히 눈에 띄지 않았고, claim 직후 역할별 finish template을 제공하지 않았다.
- 영향: 실제 성공한 verification-only task가 irreversible한 failed result로 닫혀 cleanup/retry 한 사이클을 추가로 소비한다.
- 안전한 탈출 경로: dispatch 전에 no-change finish template을 task body에 포함하고, claim 확인 즉시 coordinator가 exact attempt fence·task·dispatch·final HEAD를 포함한 안내를 보낸다.
- 상태: **attempt 5에서 선제 guidance로 해결**.
- 근거: `io-959e7d74d2ac` attempt 4 `worker_failed` result와 attempt 5 `completed/accepted` result 비교.

### B-35 — Orca worktree comment의 attempt marker가 durable current attempt와 불일치

- 증상: #28 attempt 5가 accepted된 뒤에도 `orca worktree show` comment는 `ownership=b571... attempt=1`을 유지했다. raw worktree HEAD/branch/instance와 durable record는 attempt 5의 `ec5b...`로 정확했다.
- 직접 원인: legacy adoption comment는 최초 adoption metadata로 남고 retry 때 갱신되지 않는다. 현재 acceptance는 durable fence와 raw stable worktree identity를 사용해 comment를 current lease authority로 보지 않는다.
- 영향: 운영자가 comment를 current ownership으로 오해하면 stale attempt를 선택하거나 false mismatch로 E2E를 실패시킬 수 있다.
- 안전한 탈출 경로: current attempt authority는 `issueops status`의 attempt/epoch/context와 exact Orca task/dispatch identity로 판단한다. comment 갱신 여부는 별도 UX/metadata issue로 다룬다.
- 상태: **E2E 비차단 anomaly로 기록**.
- 근거: `orca worktree show --worktree issue:28`의 comment와 `io-959e7d74d2ac` accepted attempt 5 비교.

### B-36 — tool wrapper의 PTY hook 출력이 손상돼 차단 원인을 읽기 어려움

- 증상: `orca terminal read`가 Codex spinner와 PreToolUse 출력이 섞인 긴 tail을 반환해 실제 feedback과 실행 command가 수백 개의 progress token 사이에 묻혔다.
- 직접 원인: interactive PTY rendering stream을 line-oriented JSON tail로 그대로 전달해 carriage-return 기반 UI 갱신이 정규화되지 않았다.
- 영향: 차단 원인이 unknown flag인지 authority fence인지 즉시 분리하기 어렵고, 같은 명령을 반복할 위험이 커진다.
- 안전한 탈출 경로: durable status와 raw command help를 별도로 조회하고 `PreToolUse hook (blocked)` 인접 feedback만 증거로 사용한다. wrapper output만으로 lifecycle 상태를 추정하지 않는다.
- 상태: **관찰성 한계 기록, 작업은 durable/Orca 교차 조회로 완료**.
- 근거: #28 worker terminal cursor 279~305 tail과 같은 시점의 submitted durable record.

### B-37 — 진단 중 ORCA 환경 secret 원문 조회를 시도한 절차 오류

- 증상: runtime identity를 확인하는 과정에서 환경 변수 전체를 조회하는 과도한 진단을 한 번 시도했다.
- 직접 원인: 필요한 값이 runtime/repo/session ID뿐인데 bounded Orca status/API 대신 process environment를 진단 경로로 선택했다.
- 영향: secret이 tool transcript에 노출될 수 있는 위험이 있다. 값은 문서·source·commit·GitHub artifact에 기록하지 않았고 이후 같은 조회를 반복하지 않았다.
- 안전한 탈출 경로: runtime/session/worktree identity는 `orca ... --json`과 redacted IssueOps record에서만 읽고, 환경 변수 dump·token 출력은 금지한다.
- 상태: **절차 위반 기록 및 재발 금지 적용**.
- 근거: 이후 모든 Orca 증거는 runtime ID, repo ID, terminal/task/dispatch ID만 사용하며 secret 원문을 포함하지 않는다.

### B-38 — preview가 생성한 confirmed command에서 sealed delivery option이 누락됨

- 증상: `handoff start` preview가 출력한 `confirmed_command`를 그대로 실행하자 `expected_context_sha256 does not match freshly recomputed sealed context`로 거부됐다. 원래 preview의 delivery option을 모두 다시 넣고 같은 expected hash로 confirm하면 성공했다.
- 직접 원인: sealed context hash는 task/dispatch/terminal/worktree 전달 옵션 전체를 포함하지만, 사람이 복사하도록 렌더된 confirmed command에는 그 옵션 일부가 보존되지 않았다. 기존 contract test도 preview와 confirm 사이의 delivery option 동일성을 요구한다.
- 영향: 안전하게 생성된 명령을 그대로 따른 coordinator가 hash mismatch에 막히고, 임의 재시도나 새 task 생성으로 빠질 위험이 있다.
- 안전한 탈출 경로: preview 입력의 모든 delivery option과 `--expected-context-sha256`를 그대로 보존해 confirm한다. 생성 명령 렌더러 보강은 별도 회귀 테스트와 함께 처리한다.
- 상태: **원 입력 보존으로 운영 복구 완료, CLI 렌더링 개선 후보 기록**.
- 근거: #18 `io-8dab82ade5bf` preview/confirm transcript와 `TestHandoffStartConfirmedCommandPreservesDeliveryOptions` 계약.

### B-39 — child별 focused 검증이 통합된 cross-skill·docs-index 계약 회귀를 놓침

- 증상: #19/#20/#21/#28과 parent #18 handoff가 모두 accepted였지만 parent 전체 `go test ./... -count=1`에서 Turing의 `ORCA-01` 계약 문구 누락과 response-contract golden의 docs count 불일치가 검출됐다.
- 직접 원인: #20의 문구 간소화가 기존 exact contract phrase를 제거했고, 새 `.agent-harness` 문서 네 개가 docs index에 들어갔지만 child 범위 검증은 전역 golden을 실행하지 않았다.
- 영향: 각 작업의 focused test와 skill validator가 통과해도 main 후보 전체 계약은 실패한다. accepted 상태를 곧바로 publish 완료로 해석하면 깨진 main을 만들 수 있다.
- 안전한 탈출 경로: coordinator가 모든 child를 합친 exact HEAD에서 named contract test와 response-contract golden, 전체 Go test를 다시 실행한다. 제거된 ORCA 범위를 최소 복원하고 의도된 docs projection으로 golden을 갱신한다.
- 상태: **parent Turing QA에서 재현 후 TDD 수정 완료**. ORCA 범위 contract test와 갱신된 response-contract golden test가 모두 통과했다.
- 근거: `TestTuringSkillDocumentsSupervisedHandoffEvidenceContract`, `TestResponseContractsGolden`, `origin/main...HEAD` docs diff.

### B-40 — accepted handoff 뒤 Orca terminal이 자동으로 닫히지 않음

- 증상: durable record와 task/dispatch가 accepted/terminal 상태인데 #19/#20/#21 worker·coordinator terminal 일부가 runtime inventory에 계속 열려 있었다.
- 직접 원인: IssueOps accept와 Orca terminal close는 별도 operation이고, acceptance가 이미 열린 PTY lifecycle을 자동 종료하지 않는다.
- 영향: 열린 terminal은 다음 실행의 inventory를 오염시키고 stale writer/ownership 문제로 오인될 수 있으며 리소스도 남긴다.
- 안전한 탈출 경로: accept 뒤 exact task/dispatch terminality를 확인하고 worker와 coordinator terminal을 ID별로 닫은 다음 global terminal/dispatched inventory가 0인지 재확인한다.
- 상태: **#19/#20/#21/#28/#18 종료 terminal 정리 완료**.
- 근거: accepted record 목록과 정리 전후 `orca terminal list`, `orca task-list --status dispatched` 결과.

### B-41 — zsh 예약 변수명이 성공한 QA wrapper의 종료·정리를 깨뜨림

- 증상: self-verify 본체는 `ok=true`, `termination_eligible=true`, 25/25 단계와 최소 점수 100을 반환했지만 후속 `status=$?`에서 `read-only variable: status`가 발생해 wrapper가 exit 1이 됐고 임시 state 삭제도 실행되지 않았다. 이어진 진단 loop에서 `path`를 지역 변수처럼 쓰자 zsh의 `PATH` 연동 배열을 덮어써 loop 내부 기본 명령 탐색도 실패했다.
- 직접 원인: zsh에서 `status`는 읽기 전용 exit-status parameter이고 `path`는 `PATH`와 연결된 특수 배열인데, POSIX shell의 평범한 변수처럼 사용했다.
- 영향: 제품 검증 성공을 외부 wrapper 실패로 오분류하고, cleanup receipt가 거짓이 되며, 뒤따르는 진단 명령까지 `command not found`로 연쇄 실패할 수 있다.
- 안전한 탈출 경로: 종료 코드는 `rc`, 경로 iterator는 `candidate`처럼 예약되지 않은 이름을 사용한다. 본체 JSON의 exact 성공 필드와 wrapper exit를 분리해서 판정하고, 누수된 temp root는 생성 시각·내용을 확인한 exact path만 삭제한다.
- 상태: **원인 확정·누수 디렉터리 정리 후 안전한 변수명으로 재검증**.
- 근거: self-verify JSON의 `ok=true`, `summary.failed_steps=0`, `minimum_goal_score=100`, `termination_eligible=true`; zsh의 두 exact 오류와 `/var/folders/mt/cyw_xzps58768x9tq23r5t200000gn/T/tmp.jtXhIRBXxD` 생성 시각·state 내용.

### B-42 — format validator와 focused test가 실행 가능한 문서 계약 결함 일곱 건을 놓침

- 증상: 전체 테스트·race·self-verify와 10개 `validate-skill.py`가 통과한 뒤 독립 reviewer가 #28 계획의 무효한 `issueops status --json`, Turing의 조건부 reviewer와 무조건 reviewer 지시 충돌, Berners-Lee의 direct access-control 확인 전 Jina 병렬 요청, Shannon의 다중 Go 파일에서 깨지는 line-number loop를 발견해 `REQUEST CHANGES`를 냈다. 1차 수정 재검토에서는 Berners-Lee의 FR-0 비차단 실패 fallback 누락, Turing skip 시 거짓 `APPROVE` JSON, Shannon redundancy 예제의 같은 다중 파일 오류 세 건을 추가 발견했다.
- 직접 원인: Markdown validator는 구조만 검사하고 shell 예제를 실제로 실행하지 않는다. focused contract test는 일부 exact phrase만 고정하며 같은 문서 내 규칙 간 모순과 retrieval 순서까지 모델링하지 않는다.
- 영향: 자동 게이트가 모두 GREEN이어도 사용자가 문서 명령을 그대로 실행하면 `id is required` 또는 `sed` parse 오류가 나고, 안전·위험도 계약도 실행 순서에서 위반될 수 있다.
- 안전한 탈출 경로: 병합 전 fresh reviewer에게 issue 완료 기준과 전체 diff를 제공한다. 명령 finding은 실제 CLI/다중 파일 corpus에서 RED를 재현하고, 안전 finding은 단계 순서를 명시적으로 직렬화한 뒤 같은 reviewer에게 재제출한다.
- 상태: **일곱 건 모두 수정·실행 검증 후 동일 reviewer 최종 재검토 중**.
- 근거: reviewer의 Critical 0/Important 4 및 Critical 0/Important 3 판정, 수정 전 `issueops: id is required`, 수정 전 Shannon `sed ... command i expects` 오류, 수정 후 #28 status exit 0·두 다중 파일 Shannon 예제 실행·skill validator·focused IssueOps test 출력.

### B-43 — macOS BSD sed가 range address의 `=` command를 거부함

- 증상: reviewer의 redundancy finding을 고친 첫 예제에서 `sed -n "${line},/^}/="`가 macOS에서 `command = expects up to 1 address(es), found 2`를 모든 함수마다 출력했다.
- 직접 원인: GNU/BSD 차이를 확인하지 않고 `=` command에 address range를 적용했다. `=`는 현재 line number 출력 command지만 BSD sed에서는 두 address를 받지 않는다.
- 영향: 다중 파일 field parsing은 고쳐졌어도 함수 길이 계산이 비어 음수/잘못된 결과를 만들고 stderr를 대량 발생시킨다.
- 안전한 탈출 경로: range 본문을 portable `p`로 출력하고 `wc -l | tr -d ' '`로 길이를 센다. 예제 전체를 실제 다중 Go 파일 corpus에서 실행해 `file:start:length:name` 출력을 확인한다.
- 상태: **RED 재현 후 portable pipeline으로 수정·실행 검증 완료**.
- 근거: 수정 전 BSD sed exact 오류와 수정 후 `internal/core/issueops/*.go`에서 정상 출력된 `issueops_ai_slop_clean_test.go:10:49:...` 등 file-aware 측정 결과.

### B-44 — final self-verify wrapper가 생성한 temp state를 process 환경에 전달하지 않음

- 증상: final self-verify 재실행을 위해 `state_dir=$(mktemp -d)`를 만들었지만 command 앞에 `HARNESS_STATE_DIR="$state_dir"`를 빠뜨려 기본 user state를 대상으로 시작했다. 시작 1초 안에 발견해 Ctrl-C로 중단했으며 생성한 temp directory는 비어 있었다.
- 직접 원인: temp root 생성과 process environment 주입을 별도 줄로 작성하고 둘의 연결을 검증하지 않았다.
- 영향: 격리 검증이라고 보고하면서 실제 user state DB를 열 수 있다. 이번 실행 시각에 user IssueOps DB/WAL mtime이 관찰돼 내용 변경 여부를 근거 없이 부정할 수 없다.
- 안전한 탈출 경로: self-verify executable 바로 앞에 `HARNESS_STATE_DIR="$state_dir"`를 붙이고, 종료 뒤 exact directory 부재와 reduced JSON 판정 필드를 함께 확인한다. process inventory에서 잘못 시작한 verifier가 남지 않았는지도 확인한다.
- 상태: **즉시 중단·빈 temp root 삭제·잔류 process 없음 확인 후 격리 재실행 완료**. 최종 run은 exit 0과 cleanup을 함께 확인했다.
- 근거: 중단된 session의 `^C`/exit 1, 비어 있던 `/var/folders/mt/cyw_xzps58768x9tq23r5t200000gn/T/tmp.odzsRRNTUI`, user IssueOps DB/WAL mtime, self-verify process inventory 0건; 재실행의 `ok=true`, 25/25 steps, minimum score 100, termination eligible, temp root absent.

### B-45 — publication이 읽기 전용 system Git config에도 `.lock` 생성을 요구함

- 증상: GitHub PR이 병합된 뒤 `issueops handoff publish --confirm`이 active lock이 없는데도 `publication git config lock is unavailable`로 실패했다.
- 직접 원인: `publicationGitConfigOrigins`가 macOS Command Line Tools의 `/Library/Developer/CommandLineTools/usr/share/git-core/gitconfig`를 유효 origin으로 수집하고, `withPublicationGitConfigLocks`가 모든 origin 옆에 `O_EXCL` `.lock`을 생성한다. 일반 사용자에게 `/Library/.../git-core`는 쓰기 불가다.
- 영향: system Git config가 존재하는 표준 macOS 환경에서는 stale lock이나 동시 writer가 없어도 supervised publication receipt를 만들 수 없다.
- 안전한 탈출 경로: 진단에서는 단일 process에 `GIT_CONFIG_NOSYSTEM=1`을 주어 system authority를 읽기와 push 모두에서 제외했다. 제품 수정은 읽기 전용·비사용 config의 안전한 fingerprint 계약을 별도 설계해야 하며, lock 삭제나 권한 상승으로 우회하지 않는다.
- 상태: **원인 확정, 원격 PR/merge는 GitHub에서 완료됐지만 IssueOps publication receipt는 미기록**.
- 근거: system config parent `not writable`, pre-existing `.lock` 없음, 다른 repo/user config parent는 writable, `issueops_handoff_publication.go:272-308,317-433`.

### B-46 — bounded config inventory가 4096바이트 중간에서 잘려 유효 origin을 invalid로 만듦

- 증상: B-45를 process-local `GIT_CONFIG_NOSYSTEM=1`로 격리하자 다음 실패가 `publication git config origin is incomplete`로 바뀌었다.
- 직접 원인: `publicationGitCmd` stdout buffer 한도는 4096바이트인데 현재 `git config --show-origin --includes --list` 출력은 4987바이트다. 4096바이트 prefix의 마지막 줄이 `file:.git/`에서 잘려 tab/value 없는 invalid origin이 됐다.
- 영향: branch/config 항목이 많은 정상 저장소는 config authority를 완전하게 열거할 수 없고 publication이 fail-closed된다. 단순 retry로는 동일 byte boundary가 반복된다.
- 안전한 탈출 경로: config origin/rule inventory에는 완전성 검증 가능한 별도 상한과 explicit truncation error를 사용해야 한다. 진단 한도를 높이거나 출력 일부를 정상 origin으로 해석해서는 안 된다.
- 상태: **exact byte 재현 완료, 제품 개선 후보로 남음**.
- 근거: full output `4987`, `head -c 4096` tail의 `file:.git/`, `publicationDiagnosticLimit=4096`과 bounded buffer 구현.

### B-47 — verification-only accepted parent가 coordinator 후속 구현 변경을 durable phase에 기록하지 못함

- 증상: #18 handoff는 no-change verification worker가 `a21441c`에서 accepted됐지만 coordinator가 그 뒤 통합 회귀와 reviewer finding을 수정해 `c699913`을 만들었다. merge 뒤 `phase --to ai-slop-clean`은 `missing implementation_changes`로 실패했다.
- 직접 원인: accepted result는 의도적으로 worker changed files가 없고, 현재 CLI에는 accepted 뒤 coordinator-owned implementation evidence를 사후 추가하는 명령이 없다. publication도 B-45/B-46에 막혔다.
- 영향: GitHub 원격은 PR #44 merge와 모든 issue close를 정확히 반영하지만 durable parent cycle은 `implement`에 남아 원격 완료와 불일치한다.
- 안전한 탈출 경로: coordinator 후속 fix가 생기면 publication 전에 새 supervised attempt/cycle로 final HEAD를 재봉인하거나, 명시적 coordinator implementation evidence transition을 제품 계약으로 추가한다. 기존 accepted envelope를 직접 편집하지 않는다.
- 상태: **원격 작업 완료, durable phase 불일치 기록**.
- 근거: accepted `a21441c`, final feature `c699913`, merge `30a4aa4`, exact phase error `missing implementation_changes`.

### B-48 — 설치된 agent-harness binary가 병합 소스보다 오래됨

- 증상: 동일 publication 진단에서 PATH의 `/Users/m16khb/.local/bin/agent-harness`와 final source build의 SHA-256이 달랐다.
- 직접 원인: 설치 binary build metadata는 revision `18a8083e3f2a`이고 final 검증 binary는 이번 통합 source에서 빌드됐다. main 병합은 user-level install/update를 자동 수행하지 않는다.
- 영향: source와 CI가 고친 CLI/parser 동작을 로컬 운영 command가 아직 사용하지 않을 수 있어 원인 분리가 어려워진다. 이번 config truncation은 최신 build에서도 재현돼 stale binary만의 문제는 아니다.
- 안전한 탈출 경로: 설치 변경 권한이 있는 별도 update 작업에서 `agent-harness update`와 native integration 검증을 수행한다. 현재 issue merge를 이유로 사용자 설치를 암묵 변경하지 않는다.
- 상태: **version drift 확인·분리 완료, 설치는 변경하지 않음**.
- 근거: installed build `vcs.revision=18a8083e3f2a...`, installed/final binary SHA-256 불일치, 최신 build에서도 B-46 동일 재현.

## 2026-07-17 실행 완료 스냅샷

- #19 `io-339c2fca0e34`: accepted, commit `a8e7dce90500e80a3cb3b68f889710df90ec7374`.
- #20 `io-4f4603393a22`: accepted, commit `73c44725b2693d9276df05af3c509bee5a6b21e9`.
- #21 `io-ff473d80b45b`: attempt 2 accepted, commit `defb73d8f7e6d147f0777b4c3060057f7c78dec4`.
- #28 `io-959e7d74d2ac`: attempt 5 accepted, no-change final HEAD `bafcbbeb2c9ef65be2f8cd7fc770fd5e98dcd08f`.
- #18 `io-8dab82ade5bf`: attempt 2 accepted, integrated final HEAD `a21441c8b4c28a1781ca95df0bd8a5d209bf689f`.
- PR #44: CI push/pull_request 두 run 성공, merge commit `30a4aa4dc98f02bd288e8c59b53b65a5de11efd0`; GitHub open issue 0, open PR 0.
- resumed session에서는 hook을 비활성화하지 않았다. final #28 start 직전 exact cwd `hooks/list`에서 agent-harness/Orca hooks가 모두 enabled/trusted이고 warnings/errors는 비어 있음을 확인했다.

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
