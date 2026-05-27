# 자기 검증 루프와 자가 증강 루프

이 문서는 기존 `self-augment` 검증 루프를 두 개의 서로 다른 계약으로 분리한다.

- **자기 검증 루프**: 서비스/하네스가 의도한 대로 동작하는지 테스트와 QA를 포함해 확인한다.
- **자가 증강 루프**: 레포에 꼭 필요한 기능 추가, 성능 개선, 품질 개선, 문서/운영 개선을 스스로 결정하고 구현한 뒤 자기 검증 루프로 검증한다.

두 루프 모두 구체 목표별 점수를 가진다. 기본 종료 조건은 **각 목표 점수가 95점을 초과**하는 것이다. 95점 이하인 항목이 하나라도 있으면 완료가 아니라 개선/재시도/블로커 보고 상태다.

## 1. 자기 검증 루프

### 목적

하네스가 Codex/Claude 양쪽에서 같은 결과를 내고, CLI/MCP/native integration/state/정책/문서/스킬이 의도대로 동작하는지 검증한다. 이 루프는 **개선을 직접 선택하지 않는다**. 개선 여부를 판정하는 QA 게이트다.

### CLI/MCP 표면

```bash
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --progress=jsonl --json
./bin/harness self-verify --iterations=10 --seed=100 --target-score=95 --save-state --state-key self-verify-latest --json
./bin/harness self-verify history --prefix self-verify --json
./bin/harness self-verify history --prefix self-verify --retention-limit 20 --prune-retention --json
./bin/harness self-verify compare --baseline-key self-verify-baseline --candidate-key self-verify-latest --json
./bin/harness self-verify promote --from-key self-verify-latest --baseline-key self-verify-baseline --confirm --json
```

MCP tools:

- `self_verify`
- `self_verify_history`
- `self_verify_compare`
- `self_verify_promote`

기존 `self_augment_history/compare/promote` 호출은 호환 alias로만 남긴다. 새 문서와 자동화에서는 `self_verify_*`를 사용한다.

### 포함 단계

각 iteration은 최소 다음 단계를 포함한다.

1. core invariant 확인
2. `go test ./... -count=1`
3. contract/golden tests
4. **risk QA tier**: working tree risk를 판정해 민감한 Go 변경에는 `go test -race ./... -count=1`와 `go vet ./...`를 실행하고, 일반 Go 변경에는 `go vet ./...`를 실행한다. 문서/설정만 바뀐 경우에는 skip을 명시적으로 기록한다.
5. `go build`
6. inspect/docs smoke
7. candidate export: `self-verify candidates`가 다음 자기 검증 후보와 open/satisfied 후보를 JSON/state로 내보내는지 확인
8. step budget baseline: label별 `step_duration_stats` p95 budget과 compare regression을 확인
9. command policy smoke
10. MCP smoke
11. llm-wiki fixture guard: MCP/CLI smoke가 사용자 durable wiki 대신 temp fixture와 isolated HOME만 사용하는지 확인
12. state roundtrip, prune, doctor, migrate, compare/promote/history and history retention smoke
13. parallel isolation: 동시 실행 중 temp state, daemon dir, llm-wiki fixture, build artifact path가 충돌하지 않는지 확인
14. daemon resilience: stale lock/socket 상태에서 daemon start/status/stop 복구와 socket permission을 확인
15. git preflight fuzz
16. native Codex/Claude integration smoke, including duplicate MCP warning classification fixture
17. redaction audit: docs, skill metadata, golden response artifacts에 unredacted secret-like 문자열이 없는지 확인
18. **QA gate**: `GENIUS_THINK.md`, 루프 문서, skill frontmatter/openai metadata, self-augment skill 존재 여부 확인

### 점수 목표

| 목표 | 증거 label | 종료 조건 |
| --- | --- | --- |
| 테스트 스위트 | `go test`, `contract golden tests` | 점수 > 95 |
| 위험도 기반 QA | `risk QA tier` | 점수 > 95 |
| 빌드 산출물 | `go build` | 점수 > 95 |
| QA 스모크 | invariants, inspect/docs, candidate export, QA gate | 점수 > 95 |
| 후보 export | candidate export | 점수 > 95 |
| 단계 budget baseline | step budget baseline | 점수 > 95 |
| 정책·보안 | command policy, preflight fuzz, redaction audit | 점수 > 95 |
| MCP·상태 회귀 | MCP smoke, llm-wiki fixture guard, state roundtrip | 점수 > 95 |
| LLM Wiki fixture guard | llm-wiki fixture guard | 점수 > 95 |
| 동시성 격리 | parallel isolation | 점수 > 95 |
| 데몬 복구력 | daemon resilience | 점수 > 95 |
| 네이티브 통합 | native integration | 점수 > 95 |

`self-verify --json`의 `summary.contract`, `summary.goal_scores`, `summary.coverage`, `summary.coverage_gaps`, 실패 시 `summary.failure_class`/`summary.failure_clusters`/`summary.rerun_commands`, `summary.minimum_goal_score`, `summary.termination_eligible`를 판정 기준으로 삼는다.
`--progress=jsonl`은 stdout JSON summary를 깨지 않고 stderr에 `loop_start`, `iteration_start`, `step_start`, `step_end`, `iteration_end`, `loop_end` 이벤트를 JSON Lines로 기록한다.
`self-verify compare`는 전체 `elapsed_ms`뿐 아니라 `summary.slowest_steps`와 `summary.step_duration_stats`의 label별 p95 budget을 비교해 느린 단계 회귀를 `slow_step:*`/`step_budget:*` regression으로 승격한다.
자기 검증 루프 자체의 다음 개선 후보는 `agent_docs/SELF_VERIFICATION_CANDIDATES.md`에 기록한다. progress heartbeat, secret redaction audit, coverage gap report, failure rerun recipe, policy path fuzz plus, JSON schema contract, flake classifier, output size budget, history retention budget, parallel temp isolation, duplicate MCP warning, daemon restart resilience, llm-wiki fixture guard, candidate export, step budget baseline은 구현됐고, 현재 미완료 1순위는 install dry-run smoke이다.

## 2. 자가 증강 루프

### 목적

자가 증강 루프는 레포를 실제로 개선한다. 실행되면 다음 중 하나 이상을 수행해야 한다.

- 필요한 기능 추가
- 성능 개선
- 품질/안전/테스트 강화
- 문서/운영 경험 개선
- 반복 실패를 줄이는 자동화 또는 기억 구조 개선

단순히 테스트를 반복하거나 보고서만 작성하면 자가 증강이 아니다.

### 표면

```bash
./bin/harness self-augment --cycles=1 --target-score=95 --json
./bin/harness self-augment --cycles=1 --target-score=95 --save-state --state-key self-augment-latest --json
./bin/harness self-augment lesson --candidate reflexion-state-memory --lesson "..." --next-action "..." --json
```

이 CLI는 deterministic planner/curriculum 표면이다. 실제 코드 편집과 구현은 native agent skill `skills/self-augment`가 수행한다. `--save-state`를 사용하면 선택 후보, open/satisfied 후보 목록, GENIUS_THINK 공식, 연구 앵커를 `self_augmentation_plan` state snapshot으로 저장해 다음 cycle의 기억으로 재사용한다. `self-augment lesson`은 실패/QA/설계 교훈을 `self_augmentation_lesson` state snapshot과 llm-wiki capture draft로 남긴다. 플래너는 이미 충족된 후보를 `already_satisfied`로 남겨 감사 가능하게 하되, 다음 cycle의 `selected_candidate`에서는 제외한다. 따라서 자가 증강 루프를 반복 실행하면 완료된 일을 다시 고르는 대신 다음으로 필요한 기능·성능·품질·문서 개선을 고른다.

스킬 실행 계약:

```text
$self-augment
```

### 종료 목표

| 목표 | 설명 | 종료 조건 |
| --- | --- | --- |
| 개선 목표 선별 | `GENIUS_THINK.md`, docs index, skill inventory, git evidence로 10개 이상 후보 생성·점수화 | 점수 > 95 |
| 개선 구현 | 선택 후보가 실제 diff로 구현됨 | 점수 > 95 |
| 검증·QA | 타깃 테스트와 자기 검증 루프 통과 | 점수 > 95 |
| 학습 기록 | 실패/성공 교훈과 다음 후보가 state/docs/wiki 중 적절한 곳에 남음. 기본 state artifact는 `self_augmentation_plan`이다. | 점수 > 95 |

### GENIUS_THINK.md 사용

자가 증강 후보 생성에는 `GENIUS_THINK.md`의 공식을 최소 2개 이상 사용한다. 기본 조합은 다음이다.

- 문제 재정의 알고리즘: “이 레포의 진짜 병목은 무엇인가?”를 다시 정의한다.
- 혁신적 솔루션 생성 공식: 가치, 참신성, 실현 가능성, 위험을 함께 점수화한다.
- 사고의 진화 방정식: 이전 실패/학습을 다음 cycle에 반영한다.
- 복잡성 해결 매트릭스: 큰 개선을 작고 검증 가능한 하위 작업으로 쪼갠다.

## 3. 외부 전략에서 채택한 점

사용자 아이디어는 좋은 출발점이지만, 종료 조건과 학습 구조는 기존 agent 연구의 장점을 섞어 강화한다.

| 출처 | 채택한 점 |
| --- | --- |
| [Reflexion: Language Agents with Verbal Reinforcement Learning](https://arxiv.org/abs/2303.11366) | 실패를 scalar 결과로만 버리지 않고 언어 교훈으로 저장해 다음 cycle에 반영 |
| [Self-Refine](https://arxiv.org/abs/2303.17651) | generate → feedback → refine 반복을 후보 설계와 구현 재시도에 적용 |
| [Voyager](https://arxiv.org/abs/2305.16291) | 자동 curriculum과 skill-library 관점으로 “다음에 필요한 개선”을 선택 |
| [SWE-agent](https://arxiv.org/abs/2405.15793) | repo navigation, file edit, test 실행을 명시적 agent-computer interface로 취급 |
| [AgentBench](https://arxiv.org/abs/2308.03688) | 단일 pass/fail 대신 다차원 agent 목표 점수화 |
| [SWE-bench](https://arxiv.org/abs/2310.06770) | 실제 GitHub issue 해결처럼 repo-local, test-backed 개선을 선호 |
| [LangGraph docs](https://docs.langchain.com/oss/python/langgraph/overview) | durable execution/state/recovery와 human oversight를 장기 루프 설계 제약으로 반영 |
| [Microsoft AutoGen docs](https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/human-in-the-loop.html) | termination condition과 max-turn safeguard를 점수 gate와 cycle budget에 반영 |
| [DSPy optimizers docs](https://github.com/stanfordnlp/dspy/blob/main/docs/docs/learn/optimization/optimizers.md) | metric-first optimization 방식으로 candidate scoring과 regression compare 설계 |
| [OpenAI Evals](https://github.com/openai/evals) | 재사용 가능한 eval artifact와 baseline promotion 개념 차용 |

## 4. 운영 규칙

- 자기 검증 루프는 temp dir 기반 쓰기만 수행하고 사용자 repo의 소스 변경을 만들지 않는다.
- 자가 증강 루프는 실제 개선 diff를 만들 수 있지만, 작은 범위와 되돌릴 수 있는 변경을 선호한다.
- 새 CLI/MCP/native capability를 추가하면 자기 검증 루프의 테스트 또는 QA 단계에 증거 label을 추가한다.
- 95점 gate를 낮추지 않는다. 비용 때문에 생략한 검증은 해당 목표 점수를 통과로 계산하지 않는다.
