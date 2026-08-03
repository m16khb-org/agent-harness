# Issue #248 Turing Report

## 범위

- lifecycle: `io-268bd6ac6e7a`
- generation: `4`
- execution: `orca`
- branch: `248-orca-ready-issueops-dogfood`
- base: `511457c2e56e73a2c7451ba547a8fd9cfa58ab74`

## RED → GREEN

- selection receipt가 없는 상태에서 contract/domain 테스트는 새 type과 evidence field 부재로 compile RED가 났고, additive schema-v1 receipt와 decision fingerprint 구현 후 GREEN이 됐다.
- confirm fingerprint gate 테스트는 fingerprint 없이 direct mutation까지 도달하는 RED를 재현했고, decision 직후 비교로 mutation 전 GREEN이 됐다.
- direct/Orca repository 테스트는 첫 write에 selection이 nil인 RED를 재현했고, record/holder 또는 record/intent atomic write에 동일 receipt를 포함해 GREEN이 됐다.
- provider/issue drift 테스트는 이전 fingerprint가 intent를 허용하는 RED를 재현했고, normalized provider/IID를 probed projection과 repository 재검증에 포함해 GREEN이 됐다. explicit direct는 provider identity를 읽지 않는다.
- typed error 테스트는 direct reason denial과 fingerprint drift가 CLI/MCP JSON의 `code`를 잃는 RED를 재현했고, shared domain denial fields로 GREEN이 됐다.
- readiness matrix 테스트는 unavailable probe의 `ready=true`가 Orca를 선택하는 RED를 재현했고, 정규화된 `ProbeReady`만 해석해 GREEN이 됐다.
- fresh review는 fingerprint 없는 수동 confirm 예제와 auto→direct fallback code 누락 허용을 지적했다. 문서 단일-preview assertion과 두 current-v1 contract의 exact fallback negative test가 RED를 재현했고, returned `next_command` 및 exact probe/fallback matrix로 GREEN이 됐다.
- 두 번째 fresh review는 private GitLab snapshot path가 `next_command`에서 사라지는 결함을 지적했다. CLI flag부터 shared command까지 path를 보존하고, 반환 command를 실제 CLI adapter로 재실행해 provider fallback 0회와 Orca confirm 성공을 검증했다.
- 최종 fresh review는 생성된 confirm 명령의 `--direct-reason`과 `--expected-readiness-fingerprint`를 exact command parser가 거부하는 RED를 재현했다. 두 value flag를 현재 `execution prepare` spec에 추가하고 생성 명령 회귀 테스트를 GREEN으로 만들었다.
- AC-06 재검증은 SQLite WAL을 포함하는 `mode=ro` 연결에서 generation 4와 현재 task/dispatch를 확인했다. `immutable=1`은 WAL을 읽지 않아 generation 3 본파일만 관찰하므로 현재 durable projection에 사용하지 않는다.
- native-host smoke 테스트는 `validation_lane`이 비어 있는 RED를 재현했고, strict receipt에 `native_host`를 추가해 GREEN이 됐다.

## Acceptance

- AC-01: requested/resolved mode, probe facts/codes, fallback, fingerprint, selected_at, direct reason을 selection receipt로 보존한다.
- AC-02: preview가 fingerprint와 exact confirm command를 반환하고 confirm drift를 첫 mutation 전에 거부한다.
- AC-03: automated child 문서 기본은 auto이며 explicit direct는 bounded reason을 요구한다.
- AC-04: core/inbound/CLI/MCP/status/golden이 같은 selection projection을 노출한다.
- AC-05: native host receipt는 `validation_lane=native_host`다.
- AC-06: governed installed binary의 generation 4 status/claim과 `/Users/m16khb/.local/state/agent-harness/issueops_v1/harness.db` bounded read-only projection에서 schema 1, active lease, Orca runtime/worktree/Run/task/dispatch/native owner를 실측했다. `go run` 임시 바이너리의 다른 contract surface는 production owner authority가 아니다. completion evidence는 publication 후 durable receipt로 기록한다.
- AC-07: unavailable/unready fallback code와 missing/invalid direct reason 회귀를 domain/application 테스트로 고정했다.
- AC-08: nil additive receipt read와 기존 lease/publication/smoke 회귀를 전체 test로 검증한다. 최신 diff의 `go test ./... -count=1`은 `GO_TEST_ALL_EXIT_CODE=0`이었다.

## Verification

focused 두 묶음, 전체 test, 전체 race, vet, build가 모두 실제 종료 코드 0으로 통과했다. 최종 commit/PR/completion receipt는 `.agent-harness/turing/issueops-v1-bfe15e931870f5bc.json`과 durable lifecycle이 함께 소유한다.
