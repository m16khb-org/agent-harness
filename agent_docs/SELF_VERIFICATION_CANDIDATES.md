# 자기 검증 루프 후보 발굴 기록

작성일: 2026-05-27
범위: `harness self-verify` 자체의 신뢰성, 관측성, 보안, 회귀 탐지 개선 후보

이 문서는 자가 증강 루프의 구현 후보가 아니라 **자기 검증 루프를 더 잘 검증하기 위한 다음 후보 backlog**다. 현재 루프가 95점 gate를 통과하더라도, 반복 실행 중 놓칠 수 있는 사각지대를 명시적으로 후보화한다.

## 1. 확인한 현재 상태

인터럽트 후 `/tmp/self-verify-candidate-baseline.json`과 종료된 실행 세션을 확인했다.

- `ok=true`, `termination_eligible=true`
- `elapsed_ms=52078`
- `total_steps=130`, `failed_steps=0`
- `minimum_goal_score=100`, 모든 목표 점수 `100/100`
- step labels: `harness invariants`, `go test`, `contract golden tests`, `risk QA tier`, `go build`, `inspect smoke`, `docs index smoke`, `command policy smoke`, `MCP smoke`, `state roundtrip`, `preflight fuzz`, `native integration`, `QA gate`
- 가장 느린 단계는 모두 `risk QA tier`였고, 상위 duration은 약 2.4~2.6초였다.
- `./bin/harness self-augment --json`의 기존 11개 후보는 모두 `already_satisfied`이며 `selected_candidate`는 없다.

결론: 실행은 멈춘 것이 아니라 약 52초 후 정상 종료했다. 다만 redirect 실행에서는 진행 상황이 보이지 않아 사용자가 hang으로 오해할 수 있다. 따라서 다음 후보는 단순 통과 여부보다 **진행 관측성, 사각지대 탐지, 재현성**을 우선한다.

## 2. 후보 생성 공식

`GENIUS_THINK.md`에서 다음 공식을 혼합했다.

- **문제 재정의 알고리즘**: “self-verify가 통과했는가?”를 “통과 결과가 충분히 관측 가능하고, 반복·병렬·오염·보안 상황에서도 같은 결론을 낼 수 있는가?”로 재정의했다.
- **혁신적 솔루션 생성 공식**: 기존 self-verify 요소를 새 조합으로 묶어 impact, feasibility, novelty, risk, verification cost, user value를 함께 평가했다.
- **사고의 진화 방정식**: 이번 인터럽트처럼 실제 운영 중 발생한 불편을 다음 후보로 반영했다.
- **복잡성 해결 매트릭스**: 큰 “검증 신뢰성” 문제를 관측성, 보안, 상태, 성능, 계약, 재현성 하위 문제로 분해했다.

점수는 우선순위 점수이며 100점 만점이다.

```text
priority = impact*0.25 + feasibility*0.20 + novelty*0.15 + user_value*0.20 + (100-risk)*0.10 + (100-verification_cost)*0.10
```

`risk`와 `verification_cost`는 낮을수록 좋으므로 역점수로 반영한다.

## 3. 후보 목록

2026-05-27 cycle update: `self-verify-progress-heartbeat`는 `self-verify --progress=jsonl`로 구현됐다. stdout의 최종 JSON summary는 유지하고 stderr에 JSON Lines progress event를 기록한다. `self-verify-secret-redaction-audit`도 `redaction audit` self-verify 단계로 구현됐다. `self-verify-coverage-gap-report`는 `summary.coverage`와 `summary.coverage_gaps`로 구현됐다. `self-verify-failure-rerun-recipe`는 실패 summary의 `rerun_commands`로 구현됐다. `self-verify-policy-path-fuzz-plus`는 symlink escape, `~/path`, remote URL/ref 예외 fixture로 보강됐다. `self-verify-json-schema-contract`는 `summary.contract` version/hash/required fields로 구현됐다. `self-verify-flake-classifier`는 `failure_class`와 `failure_clusters`로 구현됐다. `self-verify-output-size-budget`은 bounded stdout/stderr와 truncation metadata로 구현됐다. `self-verify-history-retention-budget`은 history retention-limit 계획/dry-run/confirm으로 구현됐다. `self-verify-parallel-temp-isolation`은 parallel isolation self-verify step으로 구현됐다. `self-verify-duplicate-mcp-warning`은 Claude MCP conflicting scopes fixture 분류로 구현됐다. 다음 self-augment cycle의 기본 1순위는 `self-verify-daemon-restart-resilience`이다.

| 우선순위 | 후보 ID | 분류 | 점수 | 왜 지금 필요한가 | 검증 방법 |
| --- | --- | --- | ---: | --- | --- |
| 1 | `self-verify-progress-heartbeat` | 관측성 | 81 | self-verify가 52초 동안 실행되면 redirect/무출력 환경에서 hang으로 보인다. iteration, current step, elapsed, last-success를 JSONL 또는 stderr heartbeat로 내보내면 불필요한 인터럽트를 줄인다. | `self-verify --progress=jsonl` fixture, timeout 없는 heartbeat test, 기존 `--json` stdout contract 불변 golden |
| 2 | `self-verify-secret-redaction-audit` | 보안 | 79 | command output, state snapshot, golden, MCP 응답에 token-like 문자열이 섞이면 검증 루프가 오히려 secret 유출 경로가 된다. | synthetic secret fixture, stdout/state/golden redaction scan, `go test ./...`, preflight secret-like smoke |
| 3 | `self-verify-coverage-gap-report` | coverage | 78 | 현재 labels는 넓지만 “문서 invariant 또는 CLI 계약 중 어떤 항목이 어느 step으로 보호되는지”가 기계적으로 드러나지 않는다. | docs/invariant-to-step matrix fixture, unowned claim 실패 fixture, `docs --json` smoke |
| 4 | `self-verify-llm-wiki-fixture-guard` | MCP/state | 78 | MCP smoke는 temp wiki를 쓰지만, 회귀 시 사용자 durable wiki를 건드리지 않는다는 guard가 명시 후보로 고정되어야 한다. | `LLM_WIKI_ROOT` temp guard, default env leak negative test, MCP smoke |
| 5 | `self-verify-failure-rerun-recipe` | 재현성 | 78 | 실패 시 현재 summary만으로는 해당 step을 같은 env/seed로 바로 재현하기 어렵다. 실패 step마다 copy-paste 가능한 rerun command를 제공한다. | failing fixture, summary `rerun_commands`, command redaction test |
| 6 | `self-verify-candidate-export` | curriculum | 77 | self-augment 후보가 모두 `already_satisfied`가 된 뒤 다음 self-verify 후보를 별도 명령으로 뽑을 수 있어야 반복 개선이 끊기지 않는다. | `self-verify candidates --json` golden, state save/read, docs smoke |
| 7 | `self-verify-step-budget-baseline` | 성능 | 76 | `slowest_steps` top 5는 있으나 label별 budget/분산을 추적하지 않으면 “항상 조금씩 느려지는” 회귀를 놓친다. | baseline compare fixture, label budget regression, `slow_step:*` 확장 test |
| 8 | `self-verify-install-dry-run-smoke` | native integration | 76 | `install-native --dry-run`은 구현되었지만 self-verify 단계에 독립 evidence label로 노출되면 no-write 설치 계약이 더 분명해진다. | temp HOME smoke, dry-run no-write assertion, adapter matrix golden |
| 9 | `self-verify-policy-path-fuzz-plus` | 정책/보안 | 76 | 현재 workspace 밖 path arg는 막지만 symlink, `--flag=path`, `~/`, URL, git ref, unicode path 조합은 추가 fuzz 가치가 있다. | seeded policy fuzz table, symlink escape negative test, `path_outside_workspace` assertions |
| 10 | `self-verify-json-schema-contract` | 계약 | 76 | `summary.goal_scores`, `slowest_steps`, history/compare/promote 응답이 확장될수록 schema drift를 사람이 golden diff로만 확인하기 어렵다. | JSON schema/hash fixture, backward-compatible field test, response contract golden |
| 11 | `self-verify-flake-classifier` | 신뢰성 | 75 | 10회 반복 중 일부 seed만 실패할 때 deterministic failure와 flaky failure를 분류해야 우선순위가 선명해진다. | synthetic intermittent step, per-seed failure clustering, summary `failure_class` golden |
| 12 | `self-verify-output-size-budget` | 운영성 | 73 | 실패 stdout/stderr가 커지면 JSON 응답과 state 저장이 비대해진다. tailing, truncation, size budget이 필요하다. | oversized output fixture, truncation marker test, state write size check |
| 13 | `self-verify-history-retention-budget` | 상태 운영 | 71 | baseline/history가 계속 쌓이면 상태 저장소가 느려지고 오래된 회귀 기준이 혼재된다. | 구현됨: retention policy fixture, prune dry-run/confirm regression, history ordering golden |
| 14 | `self-verify-parallel-temp-isolation` | 동시성 | 70 | 여러 self-verify가 동시에 실행될 때 temp state, MCP fixture, build artifact가 충돌하지 않는지 별도 stress가 필요하다. | 구현됨: parallel seeded isolation smoke, temp path uniqueness assertion, race tier |
| 15 | `self-verify-duplicate-mcp-warning` | native integration | 70 | Claude/Codex MCP endpoint가 user/project scope에 중복 등록되면 smoke는 통과해도 실제 UX에는 경고가 생긴다. | 구현됨: mocked `claude mcp list` duplicate fixture, warning classification, install docs update |
| 16 | `self-verify-daemon-restart-resilience` | daemon | 68 | daemon-backed MCP proxy가 핵심이 되면 stale lock, socket permission, restart 후 state 재연결을 self-verify가 다뤄야 한다. | daemon temp socket fixture, restart smoke, stale lock recovery test |

## 4. 추천 실행 순서

1. `self-verify-daemon-restart-resilience`: daemon-backed MCP proxy의 stale lock/socket 복구를 다룬다.

## 5. 완료 기준

이 후보 발굴 자체의 완료 기준은 다음이다.

- 후보가 10개 이상이고 모두 self-verify loop 개선과 직접 연결된다.
- 각 후보가 필요 이유와 검증 방법을 가진다.
- 현재 baseline 증거와 이번 인터럽트 원인을 반영한다.
- 다음 cycle에서 바로 선택할 추천 후보가 있다.

다음 자가 증강 cycle에서 실제 구현 후보를 고를 때는 아직 완료되지 않은 후보 중 가장 높은 점수인 `self-verify-daemon-restart-resilience`를 기본 1순위로 본다.
