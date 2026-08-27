---
name: OPTIMIZATION_PLAN.md
description: 중복 제거와 아키텍처 경계 정리를 위한 단계별 최적화 계획.
---

# agent-harness 최적화 계획

> 작성일: 2026-06-16
> 범위: 중복 코드 축소, 아키텍처 경계 복원, 명령/도구 registry drift 방지

## 1. 배경

현재 `agent-harness`는 테스트와 계약 검증이 강해 동작 안정성은 높다. 반면 다음 구조적 비용이 누적되고 있다.

- MCP tool catalog와 dispatch 정보가 여러 곳에 중복되어 registry drift 위험이 있다.
- Codex/Claude native installer와 hook 조립 로직이 평행 복제되어 정책 변경 시 host 간 불일치가 생기기 쉽다.
- `internal/core/*_facade.go`가 boundary보다 barrel 역할을 더 많이 수행해 API 표면만 넓어지고 있다.
- `core`가 concrete adapter/provider를 직접 알아 hexagonal boundary가 흐려졌다.
- CLI 의존성 주입이 package-global `init()` 기반이라 테스트 격리성과 유지보수성이 떨어진다.

## 2. 근거

### 검증 명령

- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o bin/agent-harness ./cmd/harness`
- `./bin/agent-harness quality inspect --json`

### 주요 hotspot / 증거 파일

- MCP registry 중복
  - `internal/adapter/mcp/catalog.go`
  - `cmd/harness/mcpcli/catalog/core_tools.go`
  - `cmd/harness/mcpcli/catalog/tools.go`
- installer / hook 중복
  - `internal/adapter/codex/install.go`
  - `internal/adapter/claude/install.go`
  - `internal/adapter/codex/install_hooks.go`
  - `internal/adapter/claude/install_hooks.go`
- broad facade / boundary leak
  - `internal/core/issueops_facade.go`
  - `internal/core/workflow_facade.go`
  - `internal/core/utility_facade.go`
- global wiring
  - `cmd/harness/harnessapp/cli_facade.go`
  - `cmd/harness/harnessapp/root_command_facade.go`
  - `cmd/harness/qualitycli/quality_inspect.go`
- 고분기 router
  - `cmd/harness/issueopscli/issueops.go`
  - `cmd/harness/issueopscli/benchmarkcmd/benchmark.go`
  - `cmd/harness/mcpcli/mcp_tool_issueops.go`

## 3. 목표

1. 같은 개념을 여러 파일에서 수동으로 유지하는 구조를 줄인다.
2. `internal/core`가 adapter 구체 구현을 직접 아는 부분을 제거한다.
3. CLI/MCP/installer 변경 시 수정 지점을 줄이고 drift 가능성을 낮춘다.
4. 기존 golden/contract/test 체계와 충돌하지 않는 작은 단계로 정리한다.

## 4. 비목표

- 기능 추가
- worker/daemon 신규 아키텍처 도입
- 대규모 패키지 rename
- 성능 수치가 확인되지 않은 speculative micro-optimization

## 4.1 2026-06-16 현재 상태

이 계획은 2026-06-16 리팩터링 세션에서 대부분 완료됐다. 현재 `quality inspect --json` 기준으로 self-augment/self-verify/audit P1/P2 후보는 모두 `already_satisfied`이며, 남은 신호는 repo-wide branch complexity 관찰값이다.

| 항목 | 상태 | 근거 |
|------|------|------|
| P0. MCP registry 단일화 | 완료 | `4e3e299 refactor(mcp): unify tool catalog into single source of truth` |
| P1. Codex/Claude installer 공통 pipeline | 완료 | `e2a0a5e refactor(install): share host installer pipeline via installutil` |
| P2. IssueOps facade / provider 경계 | 완료 | `26cd38f refactor(core): restore provider boundary via adapter-resolved IssueProvider` |
| P3. CLI global dependency wiring | 완료 | `b5e850d refactor(cli): replace init() global wiring with explicit deps injection` |
| P4. 고분기 router 분해 | 완료 | `662adb7`, `f1bba5e`, `6be7eaa` router registry commits |
| P5. broad facade 축소 | 결정 변경 | `8046737 docs(core): codify facade boundary rules; keep facades as public surface` |

## 5. 우선순위별 실행 계획

### P0. MCP registry 단일화

**상태:** 완료.

**대상 파일**
- `internal/adapter/mcp/catalog.go`
- `cmd/harness/mcpcli/catalog/core_tools.go`
- `cmd/harness/mcpcli/catalog/tools.go`

**변경 방향**
- tool schema 정의를 한 곳만 authoritative source로 남긴다.
- dispatch group 매핑을 별도 수동 map이 아니라 tool 정의에서 파생 생성한다.
- `daemon_status`, `coreProjectTools`, `selfLoopTools`의 수동 재삽입을 제거한다.

**완료 기준**
- 새 tool 추가 시 수정 지점이 1곳 또는 명시적 파생 코드 1경로로 제한된다.
- MCP golden/contract test가 추가 수정 없이 통과한다.

**검증**
- `go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1`
- `go test ./... -count=1`
- `./bin/agent-harness contract check --json`

### P1. Codex/Claude installer 공통 pipeline 추출

**상태:** 완료.

**대상 파일**
- `internal/adapter/codex/install.go`
- `internal/adapter/claude/install.go`
- `internal/adapter/codex/install_hooks.go`
- `internal/adapter/claude/install_hooks.go`
- `internal/adapter/codex/install_helpers.go`

**변경 방향**
- 공통 install step accumulator를 도입한다.
- dry-run message, error fan-in, template write 흐름을 공통화한다.
- pre-tool-use / stop enforcement flag bundle은 공통 helper로 만들고 host 차이는 matcher, `--host` 부여 규칙, project-local extra만 남긴다.
- existing user hook preservation rule도 host 간 동일 기준으로 맞춘다.

**완료 기준**
- 공통 정책 플래그 변경이 host별 두 파일 이상 동시 수정 없이 가능하다.
- Codex/Claude 결과 차이는 의도된 host 차이만 남는다.

**검증**
- `go test ./internal/adapter -count=1`
- `go test ./... -count=1`
- `./bin/agent-harness install-native --dry-run --json`

### P2. `internal/core/issueops_facade.go` 분리 및 provider 경계 복원

**상태:** 완료.

**대상 파일**
- `internal/core/issueops_facade.go`
- 필요 시 `internal/core/issueops/` 하위 패키지
- provider 생성 책임을 받는 registry/helper 신규 파일

**변경 방향**
- facade를 cycle, remote, benchmark, session 정도의 관심사로 나눈다.
- `CreateRemoteIssue`, `CreateRemotePullRequest`는 concrete provider import 대신 `port.IssueProvider` 또는 작은 registry interface를 받도록 바꾼다.
- `core`에서 `internal/adapter/provider/github`, `gitlab` 직접 import를 제거한다.

**완료 기준**
- `internal/core`가 concrete provider adapter를 직접 import하지 않는다.
- IssueOps 변경이 facade 전체를 흔들지 않고 관심사 단위로 국소화된다.

**검증**
- `go test ./internal/core -count=1`
- `go test ./cmd/harness -count=1`
- `go test ./... -count=1`

### P3. CLI global dependency wiring 제거

**상태:** 완료.

**대상 파일**
- `cmd/harness/harnessapp/cli_facade.go`
- `cmd/harness/harnessapp/root_command_facade.go`
- `cmd/harness/basiccli/dependencies.go`
- `cmd/harness/statuscli/dependencies.go`
- `cmd/harness/qualitycli/dependencies.go`
- `cmd/harness/selfworkflow/dependencies.go`
- `cmd/harness/qualitycli/quality_inspect.go`

**변경 방향**
- package-global 함수 변수 주입을 constructor/deps struct 방식으로 바꾼다.
- root command composition point는 유지하되, 의존성 wiring을 명시적으로 수행한다.
- 테스트를 위해 global 값을 덮어쓰고 복구하는 패턴을 제거한다.

**완료 기준**
- `init()` 기반 wiring이 사라진다.
- race-friendly하고 import-order에 덜 민감한 CLI 테스트 구조가 된다.

**검증**
- `go test ./cmd/harness/... -count=1`
- `go test -race ./... -count=1`

### P4. 고분기 router 분해

**상태:** 완료.

**대상 파일**
- `cmd/harness/issueopscli/issueops.go`
- `cmd/harness/issueopscli/benchmarkcmd/benchmark.go`
- `cmd/harness/mcpcli/mcp_tool_issueops.go`

**변경 방향**
- 큰 switch 문을 subcommand/tool handler registry로 분해한다.
- 공통 parsing, printing, error shaping helper를 추출한다.
- router는 dispatch만 담당하고 세부 동작은 작게 나눈 handler가 담당하게 한다.

**완료 기준**
- 신규 subcommand/tool 추가 시 giant switch 수정량이 줄어든다.
- `quality inspect`의 high-branch hotspot 수가 줄어든다.

**검증**
- `./bin/agent-harness quality inspect --json`
- `go test ./... -count=1`

### P5. broad facade 축소

**상태:** 결정 변경. facade 제거/직접 import 전환 대신 `internal/core/*_facade.go`를 의도된 공개 표면으로 유지하고, 허용 범위를 `internal/core/doc.go`와 ADR에 문서화했다.

**대상 파일**
- `internal/core/workflow_facade.go`
- `internal/core/utility_facade.go`

**변경 방향**
- facade에 남길 규칙을 문서화한다: 조합, 타입 변환, boundary enforcement만 허용.
- pure passthrough alias/one-line delegate 제거는 보류한다. 전수 조사에서 facade export 133개가 모두 사용 중이었고, cmd가 core 내부 subpackage에 결합되는 비용이 표면 축소 이득보다 컸다.

**완료 기준**
- facade에 새 도메인 로직을 추가하지 않는 규칙이 문서화되어 있다.
- 조합, 타입 변환, boundary enforcement를 넘는 로직은 owning subpackage로 이동한다.
- 하위 패키지 변경이 facade churn으로 번지지 않도록 `internal/core/doc.go` 규칙을 따른다.

**검증**
- `go test ./internal/core -count=1`
- `go test ./... -count=1`

## 6. 실행 순서 제안

1. P0 MCP registry 단일화
2. P1 installer 공통 pipeline
3. P2 IssueOps facade / provider 경계 정리
4. P3 CLI global wiring 제거
5. P4 router 분해
6. P5 broad facade 축소

이 순서는 **drift 위험이 큰 것부터**, 그리고 **검증 가능한 작은 단계부터** 처리하기 위한 것이다.

## 7. 리스크와 완화책

- **리스크:** contract/golden 변경이 예상보다 넓게 퍼질 수 있다.
  - **완화:** P0/P1은 먼저 dry-run/contract 중심으로 고정한다.
- **리스크:** facade 축소 중 caller import churn이 커질 수 있다.
  - **완화:** P2/P5는 관심사별로 쪼개고, 한 번에 한 facade만 정리한다.
- **리스크:** DI 전환 중 test fixture가 깨질 수 있다.
  - **완화:** P3는 race test와 harnessapp/CLI targeted test를 함께 돌린다.

## 8. 성공 기준

- 동일 개념의 registry/installer policy가 여러 파일에 수동 복제되지 않는다.
- `internal/core`가 concrete adapter를 직접 아는 경로가 줄어든다.
- 신규 command/tool/installer policy 추가 시 수정 지점 수가 줄어든다.
- 기존 `go test`, race, build, golden, contract 검증이 유지된다.
