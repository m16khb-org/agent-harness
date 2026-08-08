# Clean-break 헥사고날 아키텍처 전환 설계

- 날짜: 2026-08-02
- IssueOps: `io-c26802f00c2b`
- 상위 이슈: [#228](https://github.com/m16khb/agent-harness/issues/228)
- 작업 브랜치: `228-clean-break-hexagonal-architecture`
- 기준 SHA: `4b86dd46a454d241cdb348194754b1e1e452bc00`
- 상태: 사용자 승인 완료, production 구현 전
- 설계 종류: breaking clean break

## 1. 문제와 확인된 기준선

기존 capability vertical과 dependency ratchet은 방향을 검증했지만 완료 상태는 아니다. 현재 기준선은 다음과 같다.

- `internal/core`: production package 78개, production Go 파일 308개.
- `internal/core`를 직접 import하는 외부 production package: 58개.
- 외부에서 직접 참조하는 core target: 17개.
- `internal/architecture/testdata/legacy_imports.txt`: 100개 edge.
  - `cmd → adapter`: 18개.
  - adapter coupling: 20개.
  - `core → infrastructure`: 62개.
- uncapped `golangci-lint unused`: production 147개, test 12개.
- production legacy surface:
  - IssueOps `reset-legacy`.
  - state migration과 schema 0 승격.
  - version별 unsupported-schema 타입·문구·mapping.
  - hand-rolled MCP fallback.
  - daemon legacy PID.
  - deprecated `install-native` alias.
  - IssueOps·Orca compatibility marker/oracle.
  - 사용되지 않는 facade와 alias.
- 로컬 state doctor는 현재 schema v1 레코드만 정상으로 보고하고 legacy IssueOps preview는 0건이다.

따라서 목표는 “의존 일부 정리”가 아니라 production `internal/core`와 모든 legacy 실행 경로의 완전 제거다.

## 2. 승인된 결정

### 2.1 Clean break

보존 대상은 현재 schema v1과 현재 정상 CLI/MCP 동작뿐이다. 구형 state, deprecated alias, fallback transport, compatibility oracle은 migration·dual read·feature flag 없이 삭제한다.

### 2.2 Strict core zero

`internal/core/**`의 production package와 Go 파일을 전부 역할별 소유 경계로 이동한다. 작업 완료 시 `internal/core` 디렉터리 자체가 없어야 한다.

`core`, `common`, `service`, `utils`처럼 기존 결합을 이름만 바꿔 숨기는 catch-all package는 만들지 않는다.

### 2.3 Generic invalid state

존재하는 state 또는 IssueOps record는 다음 두 경우만 허용한다.

- exact schema v1이고 invariant가 유효함: 정상 처리.
- 그 밖의 기존 record: 단일 generic `invalid state`.

schema missing/0, future schema, malformed JSON, legacy field, key mismatch, byte mismatch는 같은 공개 오류로 수렴한다. schema version, legacy field 이름, JSON parser 세부는 CLI·MCP·일반 출력에 노출하지 않는다.

존재하지 않는 record는 기존 `not found` 계약을 유지한다.

production에는 `UnsupportedSchemaError`, `unsupported_schema`, `unsupported state schema`, `unsupported issueops schema`, schema 0→v1 승격과 state migration이 남지 않는다.

### 2.4 Child별 실제 host smoke

10개 하위 이슈는 각각 exact child HEAD에서 Codex와 Claude Code 실동작 smoke를 통과해야 부모 브랜치에 병합할 수 있다. mock-only, 기존 세션 재사용, 최종 child 한 번만의 검증은 인정하지 않는다.

## 3. 목표와 비목표

### 목표

- legacy dependency 100→0 및 baseline 파일 삭제.
- production unused 147→0, test unused 12→0.
- `internal/core` package 78→0, production Go 파일 308→0, production import 0.
- current-v1 정상 동작과 not-found 구분 유지.
- invalid existing record의 generic `invalid state` 통합.
- 각 child exact HEAD의 Codex·Claude fresh-session smoke.
- IssueOps issue→branch→worktree→TDD→review→PR→merge→completion→cleanup 전 과정 dogfood.
- ARCHITECTURE, ADR, CONVENTIONS, OPERATIONS, TESTING, release note를 최종 계약으로 갱신.

### 비목표

- 새 사용자 기능 추가.
- 외부 도구 기능을 agent-harness에 복제.
- #226의 revoking/shared-PID 결함 동시 수정.
- 구형 입력을 새 compatibility layer로 다시 지원.
- absent record를 `invalid state`로 합치기.
- historical ADR의 과거 사실 또는 negative rejection 의미 삭제.
- OpenWiki 자동 갱신 또는 직접 편집.

## 4. 목표 아키텍처

### 4.1 소유 경계

| 역할 | 목표 위치 | 허용 책임 | 금지 |
| --- | --- | --- | --- |
| Domain | `internal/domain/<capability>` | invariant, pure decision, typed domain error | filesystem, process, SQL, network, JSON transport |
| Application | `internal/application/<capability>` | usecase orchestration, transaction ordering | concrete adapter, host별 정책 |
| Contract | `internal/contract/<capability>` | 안정 DTO, wire mapping, generic error identity | I/O 실행, legacy decoder fallback |
| Port | `internal/port/<capability>` | capability가 실제 사용하는 최소 interface | mega filesystem/process repository |
| Inbound adapter | `internal/adapter/inbound/<capability>` | CLI/MCP/worker request projection | domain 정책 복제 |
| Outbound adapter | `internal/adapter/outbound/<capability>` | fs/git/process/SQL/network/provider 구현 | usecase 결정 |
| Composition root | `cmd/harness/harnessapp` | concrete wiring과 lifecycle | domain/application 로직 |
| CLI entry | `cmd/harness/<cli>` | flag, stdout/stderr, exit code | concrete adapter 조립과 정책 중복 |

새 abstraction은 실제 외부 기술 경계, 최소 두 사용처 또는 필요한 test double 중 하나가 있을 때만 만든다. 한 호출을 감싸는 facade는 공개 계약상 필요하지 않으면 삭제한다.

### 4.2 의존 방향

허용 방향은 다음과 같다.

```text
cmd/harness/harnessapp
  -> inbound adapter
  -> application
  -> domain / contract / capability port
  -> outbound adapter는 composition root에서 주입
```

Fitness test는 다음을 baseline 없이 거부한다.

- `internal/domain/** -> internal/adapter/**|cmd/**`
- `internal/application/** -> internal/adapter/**|cmd/**`
- `internal/contract/** -> internal/adapter/**|cmd/**`
- `internal/port/** -> internal/adapter/**|cmd/**`
- `internal/adapter/** -> cmd/**`
- composition root 밖 concrete outbound adapter wiring.
- 모든 `agent-harness/internal/core` package와 import.

최종 gate는 `test ! -d internal/core`도 실행한다.

## 5. Package 이동 전략

기계적 rename 대신 capability 단위로 이동한다.

| Child | 소유 범위 | 완료 조건 |
| --- | --- | --- |
| #230 | reset-legacy, pre-v1 state, version별 오류 | migration/승격/unsupported type·문구 0, generic invalid-state matrix |
| #231 | MCP·daemon·CLI compatibility | fallback transport, legacy PID, deprecated alias 0 |
| #232 | IssueOps·Orca oracle·marker | compatibility oracle·legacy marker 0 |
| #235 | tooling·project 38개 core package | domain/application/contract/port/outbound 소유 경계로 전량 이동 |
| #236 | IssueOps·lifecycle·worker 35개 core package | 전량 이동, lock·atomicity·generation fence 유지 |
| #237 | state·SQL·network 4개 core package | 전량 이동, invalid-state/not-found 경계 유지 |
| #233 | cmd→adapter 18개 edge | `harnessapp` composition root로 이동 |
| #234 | adapter coupling 20개 edge | capability별 최소 port로 역전 |
| #229 | root facade와 unused | 모든 대체 소유 경계 이후 facade/unused 0 |
| #238 | 최종 통합 | core 디렉터리·baseline·legacy 0과 전체 검증 |

Wave 1의 child는 구현을 독립적으로 시작할 수 있다. 부모 브랜치 병합은 child별 exact-head 검증을 직렬화해 사용자 host 설정의 임시 활성화가 겹치지 않도록 한다.

## 6. 상태와 오류 흐름

### 6.1 Read

```text
inbound request
  -> application usecase
  -> repository port
  -> outbound state/SQL adapter
  -> exact v1 decode + invariant validation
  -> success | invalid state | not found
  -> 동일 contract를 CLI/MCP/worker에 투영
```

### 6.2 Write

새 write는 항상 schema v1만 기록한다. legacy field, version negotiation, dual-write는 없다. 기존 atomic rename/SQLite transaction/IssueOps generation CAS 순서는 capability 이동 전후 동일해야 한다.

### 6.3 Error matrix

| 입력 | Domain 결과 | CLI/MCP 공개 결과 |
| --- | --- | --- |
| valid v1 | success | 기존 정상 응답 |
| schema missing/0 | invalid state | generic `invalid state` |
| future schema | invalid state | generic `invalid state` |
| malformed JSON | invalid state | generic `invalid state` |
| legacy field | invalid state | generic `invalid state` |
| key mismatch | invalid state | generic `invalid state` |
| byte mismatch | invalid state | generic `invalid state` |
| record absent | not found | 기존 not-found 계약 |

테스트 이름과 내부 fixture는 원인을 구분할 수 있지만 public message는 구분하지 않는다.

## 7. 테스트 설계

### 7.1 Child별 TDD

각 child는 다음 순서를 따른다.

1. RED: current-v1 정상 동작과 제거 대상 legacy 경로를 재현한다.
2. GREEN: 해당 child 범위의 최소 이동·삭제로 통과시킨다.
3. SURFACE: CLI/MCP/worker projection과 package importer를 갱신한다.
4. CLEAN: orphan import, facade, marker, fixture를 제거한다.
5. VERIFY: focused unit/race, architecture fitness, exact grep을 실행한다.
6. HOST SMOKE: source coordinator lane이 exact child HEAD를 두 native host에서 검증한다.
7. MERGE GATE: 모든 증거가 같을 때만 부모 브랜치 병합을 허용한다.

### 7.2 Capability differential

다음 동작은 이동 전후 결과를 비교한다.

- IssueOps state lock과 동일-root/cross-root 직렬화.
- atomic write와 persisted bytes.
- generation fence와 actor/cwd 검증.
- provider publication create/reconcile.
- daemon process identity와 socket lifecycle.
- current-v1 CLI/MCP response golden.
- not-found와 generic invalid-state 분리.

byte-stable 계약은 byte comparison을 사용하고, timestamp/path처럼 동적인 값만 기존 normalization 규칙을 따른다.

### 7.3 Architecture/unused gate

```bash
test ! -d internal/core
go test ./internal/architecture -count=1
golangci-lint run --enable-only unused --max-issues-per-linter 0 --max-same-issues 0 ./...
```

Architecture test는 legacy baseline 없이 0 edge를 요구하고, `internal/core` prefix package/import 발견 시 exact importer→imported 진단으로 실패한다.

### 7.4 전체 repository gate

```bash
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build -o bin/agent-harness ./cmd/harness
go test ./cmd/harness/contractgolden -run Golden -count=1
go test ./cmd/harness/harnessapp -run TestResponseContractsGolden -count=1
./bin/agent-harness self-verify --full --iterations=10 --seed=100 --target-score=95 --llm-eval=false --progress=jsonl --json
```

다단계 gate는 한 단계 실패 시 첫 단계부터 다시 실행한다. 서로 다른 run의 부분 통과를 합쳐 완료 증거로 사용하지 않는다.

## 8. Child별 Codex·Claude 실동작 smoke

### 8.1 실행 소유권

- child owner는 canonical worktree에서 code/test/commit까지만 수행한다.
- 실제 사용자 host integration 변경은 source checkout의 coordinator lane만 수행한다.
- 동시에 두 child의 host smoke를 실행하지 않는다.
- coordinator는 child issue 번호와 full HEAD를 입력으로 받고, 그 SHA가 child remote head와 같은지 먼저 확인한다.
- smoke 중 source code는 수정하지 않는다.

### 8.2 임시 활성화와 복원

각 child에서 다음 sequence를 사용한다.

1. 현재 설치의 root, binary SHA-256, Codex/Claude hook·MCP semantic/raw receipt, host version을 read-only로 봉인한다.
2. exact child HEAD를 disposable checkout에서 빌드한다.
3. 임시 `HOME`/`CODEX_HOME`에서 `install --dry-run --project-local --json`을 실행해 두 host와 no-write contract를 검증한다.
4. coordinator source lane에서 exact child build를 staged activation한다. co-resident hook 순서와 제3자 설정을 보존하고 activation receipt를 마지막에 쓴다.
5. child build가 활성화된 상태에서 fresh Codex와 fresh Claude Code session을 각각 시작한다.
6. 성공·실패와 관계없이 `defer/finally` cleanup으로 1단계의 기존 설치를 재활성화한다.
7. 기존 binary/config/hook/MCP receipt와 새 fresh-session readback이 1단계와 일치한 뒤 disposable checkout을 제거한다.
8. 복원 실패 시 다음 child와 부모 merge를 모두 중단한다.

사용자 credential 원문은 복사·출력·hash artifact에 포함하지 않는다. native host는 기존 인증 경계를 사용하고, evidence에는 redacted version/exit/digest만 남긴다.

### 8.3 Fresh Codex smoke

- installed `codex --version`을 기록한다.
- 새 non-persistent session을 사용한다. 기존 thread resume는 금지한다.
- read-only sandbox와 approval `never`를 사용한다.
- SessionStart가 exact child binary의 project-doc/IssueOps context를 반환하는지 확인한다.
- 하나의 allowlisted read-only tool을 호출해 PreToolUse가 exact host/session/cwd payload를 수신하고 allow projection을 반환하는지 확인한다.
- exact child MCP server에 최소 한 번 tool call을 보내고 response schema를 확인한다.
- `codex mcp get agent_harness` readback이 활성 child root/binary를 가리키는지 확인한다.
- output은 bounded JSONL로 수집하고 token, transcript, private reasoning, absolute home path를 저장하지 않는다.

### 8.4 Fresh Claude Code smoke

- installed `claude --version`을 기록한다.
- `--print --no-session-persistence`의 새 session을 사용하고 resume를 금지한다.
- read-only/plan 권한과 bounded allowlisted tool만 사용한다.
- `--include-hook-events --output-format stream-json`으로 SessionStart와 PreToolUse event를 확인한다.
- exact child MCP server에 최소 한 번 tool call을 보내고 response schema를 확인한다.
- `claude mcp list` readback이 활성 child root/binary를 가리키는지 확인한다.
- settings-source와 MCP scope 충돌을 검사한다.
- output은 bounded/redacted event digest만 남긴다.

### 8.5 공통 판정

한 child의 결과는 다음 필드를 모두 가진다.

- child issue URL.
- full child HEAD와 remote head.
- Codex/Claude version.
- activation receipt digest.
- host별 SessionStart observed.
- host별 PreToolUse observed.
- host별 MCP call count와 response-contract digest.
- restore receipt digest.
- 각 command exit code와 duration.
- 최종 verdict: `pass` 또는 `fail`.

`inconclusive`, timeout, truncated output, missing hook event, zero/multiple unexpected MCP call, version drift, restore mismatch는 모두 `fail`로 취급한다. 실패를 재시도하려면 원인을 분류하고 같은 child HEAD에서 전체 sequence를 처음부터 다시 실행한다.

## 9. Rollout과 rollback

### 9.1 Rollout

- migration 없음.
- dual read/write 없음.
- feature flag 없음.
- compatibility fallback 없음.
- 각 child는 부모 브랜치 exact HEAD에 병합한 뒤 다음 wave의 base가 된다.
- child merge마다 architecture baseline과 unused count는 증가할 수 없고 목표 방향으로만 감소해야 한다.
- #238이 전체 gate, 10개 child smoke evidence inventory, docs/release note를 최종 확인한다.
- release note에 구형 state·alias·fallback 제거를 breaking change로 명시한다.

### 9.2 Preflight

최종 merge 전 local/state preflight는 legacy/pre-v1 record 0건을 증명한다. 0이 아니면 자동 migration하지 않고 merge를 중단해 사용자가 데이터 폐기 또는 별도 보존을 결정한다.

### 9.3 Rollback

코드 rollback은 merge PR revert만 사용한다. 자동 state rewrite는 하지 않는다.

child host smoke의 임시 활성화 rollback은 매 run의 기존 activation receipt로 즉시 복원한다. 복원이 검증되기 전에는 다음 run, push, merge 또는 cleanup을 진행하지 않는다.

breaking release 이후 생성된 current-v1 state는 revert된 구버전 binary와의 양방향 호환을 약속하지 않는다. rollback 필요 시 current-v1 bytes를 먼저 백업하고 운영 결정을 별도로 기록한다.

## 10. IssueOps 실행과 검토

- parent #228은 umbrella coordination issue로 유지한다.
- provider-native child 10개는 각자 범위·검증·병합 gate를 가진다.
- parent 작업은 `io-c26802f00c2b`의 canonical worktree만 수정한다.
- production 구현 전 design review, compatibility review, Brooks devil's-advocate verdict, plan link가 모두 필요하다.
- 구현은 RED→GREEN→SURFACE→CLEAN 순서다.
- AI slop cleanup 전후 Shannon SNR·entropy·redundancy를 측정한다.
- exact-head CI와 remote body/label/assignee readback 이후에만 draft PR을 만든다.
- merge는 사용자 승인 경계다.
- `execution complete`는 merge나 cleanup을 수행하지 않는다.
- merge 후 completion reflection, child close, parent close, cleanup preview를 거쳐 별도 승인된 resource만 삭제한다.

## 11. 문서 migration

구현과 함께 다음 문서를 갱신한다.

- `.agent-harness/ARCHITECTURE.md`: `internal/core` 중심 설명을 domain/application/contract/port/adapter 구조로 교체.
- `.agent-harness/CONVENTIONS.md`: package map, dependency rules, facade 금지를 교체.
- `.agent-harness/OPERATIONS.md`: legacy reset/migrate/alias/fallback 명령 제거와 child host smoke 운영 추가.
- `.agent-harness/TESTING.md`: strict-core-zero, generic invalid-state, child live smoke gate 추가.
- `.agent-harness/ADR.md`: 이번 사용자 결정이 과거 retained-facade/legacy-transport 결정을 supersede함을 append-only로 기록.
- release note: breaking surface와 no-migration 정책.
- `AGENTS.md`: 필수 명령·planned map이 최종 구현과 달라지는 경우에만 갱신.

OpenWiki는 이 작업에서 실행하거나 직접 수정하지 않는다.

## 12. 대안과 기각 사유

### 단계적 compatibility 유지

구형 decoder와 alias를 새 위치에 남기는 방식은 위험을 줄이지만 사용자의 clean-break 요구와 legacy 0 기준을 위반하므로 기각한다.

### 기계적 package rename

가장 빠르지만 기존 결합과 facade를 다른 이름으로 보존하고 순환 의존을 만들 가능성이 높아 기각한다.

### version별 unsupported error 유지

운영 진단은 자세하지만 pre-v1 세부를 공개 계약으로 다시 고착하므로 기각한다. 내부 fixture 이름으로만 원인을 구분한다.

### 최종 #238에서만 live host smoke

비용은 낮지만 어느 child가 host drift를 만든 것인지 격리하지 못한다. 사용자가 각 child별 검증을 선택했으므로 기각한다.

### Child worktree에서 직접 install

경로는 단순하지만 사용자 범위 통합을 worker checkout에 연결하고 복원 provenance를 잃을 수 있어 기각한다. source coordinator lane과 activation receipt를 사용한다.

## 13. 위험과 완화

| 위험 | 완화 |
| --- | --- |
| 78 package 이동 중 순환 의존 | capability별 vertical, 최소 port, architecture fitness |
| facade를 새 catch-all로 재생성 | forbidden prefix/directory gate, unused gate |
| 일반 invalid-state로 진단력 감소 | test/fixture 내부 원인 분류, public detail 비노출 |
| IssueOps atomicity·lease 회귀 | differential, race, generation/actor/cwd test |
| 반복 host activation이 사용자 환경을 흔듦 | coordinator single-flight, before receipt, staged activation, finally restore, fresh readback |
| host CLI version drift | 매 run version 기록, unknown/inconclusive fail-closed |
| exact child HEAD와 smoke 대상 불일치 | local/remote full SHA와 activation receipt를 함께 봉인 |
| 긴 검증 시간 | child별 focused gate와 merge 직전 single live run; 실패 시 전체 재실행 |
| historical docs와 현재 결정 충돌 | 과거 기록은 보존하고 superseding ADR을 append |

## 14. 완료 정의

다음이 모두 참일 때만 parent scope가 완료다.

- `internal/core` directory/package/import 0.
- legacy dependency edge 0, baseline 파일 없음.
- production/test unused 각각 0.
- production legacy runtime/compatibility symbol 0.
- generic invalid-state matrix와 not-found distinction 통과.
- current-v1 unit/race/golden/build/self-verify 통과.
- 10개 child 각각 Codex·Claude fresh-session smoke `pass`와 restore receipt 보유.
- 문서와 release note가 최종 구조와 일치.
- Brooks unconditional pass와 AI-slop cleanup evidence.
- exact-head draft PR, CI, merge, completion reflection 완료.
- 사용자 승인에 따른 child/worktree/branch cleanup receipt 완료.
