# Cross-host tool contract conformance 및 증거 기반 MCP hardening 구현 계획

## TL;DR

> **Summary**: Codex·Claude Code·GJC가 같은 MCP tool schema를 실제로 어떻게 호출하는지 먼저 capture-only benchmark로 측정하고, 동일한 drift가 재현된 경우에만 agent-harness의 단일 canonical validation 계약을 강화한다. 동시에 기존 `failure_class`와 직교하는 증거 기반 `failure_cause` 축을 self-verify/trace/history에 추가하고, 재현된 drift만 behavioral regression fixture로 승격한다.
>
> **Deliverables**: 3개 대표 schema/4개 payload class의 deterministic baseline, production handler를 절대 호출하지 않는 MCP probe server, 세 host 격리 runner, preregistered 반복·판정 gate, 조건부 all-tools schema closure/validator, self-verify cause axis, behavioral replay, contract/golden/docs 동기화.
>
> **Effort**: Large. P0 benchmark + cause axis 4–6일, P0 live evidence 0.5–1일, P1 canonical enforcement(승인 gate 충족 시) 2–4일, docs/final QA 1–2일.
>
> **Parallel**: 제한적. core contract 이후 probe CLI와 failure-cause 축을 병렬화할 수 있으나, live P0와 production enforcement는 반드시 직렬 gate로 둔다.
>
> **Critical path**: T1 core contract → T2 capture server/CLI → T3 host runners → T5 live P0 gate → T6 conditional production enforcement → T7 contract/self-verify/docs → T8 final verification.

## Context

### Original request

다음 네 자료의 통찰을 agent-harness에 적용할 수 있는지 분석한 선행 결과를, 전부 상세하고 실행 가능한 계획으로 만든다.

1. Armin Ronacher, *Better Models: Worse Tools* — tool schema는 중립 계약이 아니며 host/RL 분포와의 거리에 따라 malformed call 빈도가 달라질 수 있다.
2. Auriel W, *Bad Envs* — stale/false/reward-hacked environment는 model fault와 구분해야 하며, 조용한 repair보다 fail-fast와 failure taxonomy가 중요하다.
3. Murphy-Hill et al., Microsoft CLI agent rollout study — adoption, retention, output proxy, cost를 분리해 해석해야 하며 merged PR 하나로 impact를 단정하면 안 된다.
4. Jarred Sumner, *Rewriting Bun in Rust* — language-independent conformance suite, trial run, adversarial review, 그리고 개별 산출물보다 생성 프로세스 수정이 대규모 agent 작업의 핵심이다.

### Approved design carried into this plan

선행 분석과 Brooks 악마의 변호인 검토에서 다음 범위가 이미 승인되었다.

- **P0**: production 계약을 바꾸기 전에 2–3개 대표 tool과 4개 payload class만 측정하는 최소 cross-host conformance benchmark를 만든다.
- **P1**: P0가 재현 가능한 drift를 보일 때만 모든 host가 공유하는 단일 canonical validation 계약을 강화한다. permissive/strict 이중 production semantics는 두지 않는다.
- **P1**: 기존 deterministic/intermittent `failure_class`를 보존하고 `model`, `harness_environment`, `transport`, `contract_input`, `unknown`의 원인 축을 증거로만 추가한다.
- **P2**: 재현된 실패만 최종 state/output 중심 behavioral regression fixture로 승격한다.
- **Deferred**: 실제 다중 사용자 rollout 전에는 조직 adoption scorecard를 구현하지 않는다.

### Current repo evidence

- `cmd/harness/contractcli/contract.go:35-55`는 현재 `schema`, `check` 두 subcommand만 제공한다. 새 표면은 새 top-level command가 아니라 `contract conformance` 아래에 둔다.
- `cmd/harness/mcpcli/mcp_sdk_server.go:33-44`는 raw arguments를 `map[string]any`로 decode한 뒤 handler로 넘기며 uniform schema validation을 하지 않는다.
- `cmd/harness/mcpcli/mcp_tools.go:23-28,40-70`는 tool-level failure와 JSON-RPC parameter failure를 구분하지만 parse 가능한 schema violation을 공통 판정하는 seam이 없다.
- `cmd/harness/mcpcli/argmap/args.go`는 string→bool/int, CSV→slice 등의 coercion을 조용히 수행한다.
- `internal/adapter/mcp/catalog.go:3-8` 및 catalog 집계는 advertised schema와 dispatch의 source of truth다.
- `cmd/harness/testdata/mcp_tools.golden.json`에는 100개 tool이 있으며, top-level `additionalProperties:false`는 0개다.
- `cmd/harness/selfworkflow/summary/self_verify_summary_failure.go:7-24`의 `failure_class`는 반복 패턴만 분류한다.
- `cmd/harness/selfworkflow/model/self_verify_summary_types.go:38-60`과 `internal/core/trace/self_verify.go:11-52`에는 causal owner 필드가 없다.
- 설치된 GJC 0.9.5는 `$cwd/.gjc/gjc-plugins/registry.json`을 project scope로 읽고, plugin MCP subprocess에는 `noInheritEnv:true`를 적용한다. 따라서 임시 project plugin은 가능하지만 probe child에 parent env를 기대하면 안 된다.
- 설치된 Codex 0.144.3, Claude Code 2.1.209, GJC 0.9.5의 non-interactive/ephemeral/config 관련 flag를 실제 `--help`와 설치 소스로 확인했다.

## Work objectives

### Core objective

“모델이 malformed tool call을 만들었다”는 주장과 “하네스가 불명확한 schema, stale environment, transport failure, silent coercion으로 실패를 만들었다”는 주장을 같은 관측치에서 분리한다. production 동작은 측정 결과가 재현된 뒤에만 변경한다.

### Deliverables

- `internal/core/toolconformance`: fixture manifest, closed-schema projection, deterministic validator, call classifier, repeat/gate logic, replay report.
- `internal/core/failurecause`: 직교 cause enum, typed evidence, 보수적 attribution rule.
- `internal/port/tool_conformance.go`: host runner port.
- `internal/adapter/mcp`: capture-only probe server와 조건부 production validator seam.
- `internal/adapter/hostprobe`: Codex/Claude/GJC runner, exact argv, temp config/plugin lifecycle, output/evidence normalization.
- `agent-harness contract conformance {baseline,live,replay,serve}` CLI.
- self-verify summary/trace/history에 `failure_cause`·reason·evidence 반영.
- 재현된 raw-call drift를 handler non-invocation/final response로 검증하는 regression corpus.
- response contract/golden, ARCHITECTURE/TESTING/OPERATIONS/CAUTIONS/ADR, self-verify/self-augment skill pointer 동기화.

### Definition of done

- deterministic baseline이 preregistered 10개 case를 정확히 분류한다.
- probe server 테스트가 production dispatch를 0회 호출하고 raw argument SHA + redacted canonical arguments만 기록함을 증명한다.
- 세 host runner의 argv/config/temp cleanup을 fake process로 검증하고, live 실행은 명시적 `HARNESS_TOOL_CONFORMANCE_LIVE=1` 없이는 거부한다.
- live P0가 9개 clean-context episode를 완료하거나 `inconclusive`로 fail-closed하며, model denominator에서 environment/transport failure를 제외한다.
- 동일 host/schema/diagnostic signature가 2회 이상 재현되지 않으면 production schema enforcement와 regression promotion을 수행하지 않는다.
- enforcement gate가 열리면 parse 가능한 invalid arguments는 SDK/legacy 두 경로에서 같은 `invalid_tool_arguments` tool result를 반환하고 handler를 호출하지 않는다.
- 실패한 self-verify summary에는 `failure_class`와 별도로 `failure_cause`가 항상 존재하며, evidence가 없으면 `unknown`이다. 성공 summary는 `failure_cause:"none"`이다.
- `go test ./... -count=1`, `go test -race ./... -count=1`, contract golden, build, deterministic conformance, quick self-verify가 마지막 한 번의 all-or-nothing run에서 모두 통과한다.

### Must have

- advertised schema 판정과 canonical-intent 판정을 둘 다 보고한다. advertised schema가 unknown key를 허용했다면 이를 model fault로 분류하지 않는다.
- raw tool arguments는 production handler보다 먼저 capture/validate한다.
- diagnostics는 `code`, JSON Pointer `path`, `expected`, `actual`만 저장하고 raw secret value는 저장하지 않는다.
- live report는 host version, requested/observed model, schema hash, profile, attempt, duration, ambient tool count를 기록한다.
- GJC는 격리된 auth source(API-key env 또는 auth broker)와 임시 `GJC_CODING_AGENT_DIR`을 사용한다. 안전한 auth가 없으면 `isolated_auth_unavailable`로 중단한다.
- 5% environment-failure 수치는 warning 신호로만 사용한다. 보편적 pass/fail threshold로 사용하지 않는다.
- 재현 fixture는 host transcript나 chain-of-thought가 아니라 redacted arguments, schema hash, diagnostic signature, handler call count, final response/state만 담는다.

### Must not have

- Codex용/Claude용/GJC용 semantic schema 복제 금지.
- unknown key 삭제, alias 적용, Unicode 수정, string/CSV coercion 같은 silent repair 금지.
- provider의 비공개/비표준 `strict` flag를 MCP capability처럼 노출하지 않는다.
- live model/API 호출을 CI나 기본 self-verify에 넣지 않는다.
- 사용자 Codex/Claude/GJC MCP/plugin 등록을 수정·disable·rename하지 않는다.
- GJC credential DB를 copy/symlink하지 않는다.
- confirmed reproduction 없이 production validator를 enforce하거나 tracked regression fixture를 만들지 않는다.
- 조직 dashboard, token budget enforcer, scheduler, RL trainer, 전역 2-reviewer fan-out을 만들지 않는다.

## Decision-complete contracts

### 1. Representative schema manifest

tracked manifest는 `internal/core/toolconformance/testdata/fixture_manifest.json` 하나가 소유한다. schema 본문을 복제하지 않고 current adapter catalog에서 source tool을 찾아 canonical JSON SHA-256을 검사한다.

| Fixture ID | Probe tool | Source tool | Why | Current schema SHA-256 |
|---|---|---|---|---|
| `empty_object` | `harness_probe_empty_object` | `contract_schema` | property가 없는 flat object에서 invented key 관측 | `7b1b2936a97f4bbeb5b021fcd767d3d808029152c60d77e75c279d6d51222bfa` |
| `mixed_scalar_array` | `harness_probe_mixed_scalar_array` | `command_policy_check` | string/boolean/array와 required fields 혼합 | `137a93364fc30e8ab066b4bc50cb72231e31da45aea1df2986805476ce9b20ee` |
| `nested_object_array` | `harness_probe_nested_object_array` | `issueops_record_execution_decision` | array-of-object, nested required, enum, Unicode/multiline string | `eac64aae905f8df513c9689de9afeef24bfd3508bf8beeb97a6ef513428abf43` |

manifest의 expected arguments는 다음 의미를 고정한다.

- `empty_object`: `{}`.
- `mixed_scalar_array`: `/workspace` root/cwd, `argv=["git","status","--short"]`, `network_allowed=false`, `shell_allowed=false`, `write_allowed=false`, `env_allowlist=["PATH"]`, `timeout="30s"`.
- `nested_object_array`: `id="issueops-probe"`, 세 gate 배열, `subagent_use="planned"`, 그리고 정확히 한 개의 `devils-advocate-review` plan. `net_positive_rationale`에는 `정확한 스키마 검증\n부작용 없음`을 넣어 Unicode/multiline 보존도 측정한다.

### 2. Four payload classes and deterministic denominator

payload class는 `valid`, `unknown_key`, `coercible_type_drift`, `noncoercible_type_drift` 네 개로 고정한다. empty-object schema에는 type-bearing property가 없으므로 type-drift case를 억지로 만들지 않는다.

- 3 fixtures × `valid` = 3 cases.
- 3 fixtures × `unknown_key`(`requireUnique:true`) = 3 cases.
- mixed + nested × `coercible_type_drift` = 2 cases.
  - mixed: boolean `false`를 string `"false"`로 전달.
  - nested: string array를 comma-separated string으로 전달.
- mixed + nested × `noncoercible_type_drift` = 2 cases.
  - mixed: `argv`를 object로 전달.
  - nested: `subagent_plans`를 string으로 전달.

총 denominator는 **10 cases**다. baseline은 10/10 expected classification이 아니면 hard fail한다.

### 3. Schema semantics and diagnostics

validator는 repo의 실제 schema subset만 지원한다: `type`, `properties`, `required`, `items`, `enum`, `description`, `additionalProperties`. 새 keyword가 나타나면 무시하지 않고 `unsupported_schema_keyword`로 catalog build/baseline을 실패시킨다.

object closure 규칙:

1. top-level object는 `additionalProperties:false`.
2. `properties`를 가진 nested object도 `additionalProperties:false`.
3. 의도적으로 자유형 map인 `fields`와 `score_result` 8개 schema node만 source catalog에 `additionalProperties:true`를 명시한다.
4. implicit open object는 허용하지 않는다.

stable diagnostic code:

- `invalid_json`
- `root_type_mismatch`
- `missing_required`
- `unknown_key`
- `wrong_type`
- `enum_mismatch`
- `unsupported_schema_keyword`

diagnostics는 `(path, code, expected, actual)` 순으로 정렬하고 값 본문은 포함하지 않는다. JSON Pointer escaping은 RFC 6901의 `~0`, `~1`을 사용한다.

### 4. Observed call classification

classification 우선순위는 다음과 같다.

1. raw arguments가 JSON이 아니면 `invalid_json`.
2. call이 없으면 `no_call`, 2회 이상이면 `multiple_calls`.
3. closed schema가 통과하고 expected arguments와 canonical deep-equal이면 `exact_valid`.
4. closed schema가 통과하지만 값이 다르면 `valid_but_semantically_different`.
5. 실패 diagnostics가 `unknown_key`뿐이면 `unknown_key`.
6. type diagnostics가 current legacy argmap으로 expected type에 lossless 변환 가능한 shape이면 `coercible_type_drift`.
7. 그 외 type failure는 `noncoercible_type_drift`.
8. required/enum failure는 각각 `missing_required`, `enum_mismatch`.

`advertised_valid`와 `canonical_valid`를 별도 boolean으로 저장한다. 예를 들어 current advertised schema가 unknown key를 허용하면 `advertised_valid=true`, `canonical_valid=false`, cause=`contract_input`이다.

### 5. Failure-cause axis

`failure_class`는 발생 패턴, `failure_cause`는 책임 가설이다. 둘은 결합하거나 서로 덮어쓰지 않는다.

cause enum:

- `none`: failed step 없음.
- `model`: coherent advertised contract와 clean transport가 확인되었는데 raw pre-repair call이 invalid.
- `harness_environment`: executable/auth/config/tool collision/stale cache/timeout 등 실행 환경이 episode를 오염.
- `transport`: MCP initialize/tools-list/tools-call framing 또는 child pipe가 실패.
- `contract_input`: advertised schema/prompt와 canonical intent가 불일치하거나 schema가 암묵적으로 open.
- `unknown`: typed evidence가 없거나 상충해 causal owner를 확정할 수 없음.

보수적 precedence는 `transport` → `harness_environment` → `contract_input` → `model`이다. model evidence와 다른 cause가 함께 있으면 model attribution을 억제하고 더 앞선 cause를 선택한다. freeform stderr 문자열 추측만으로 cause를 올리지 않는다.

### 6. Live P0 matrix and gate

초기 clean-context matrix는 **3 hosts × 3 fixtures × 1 fresh session = 9 completed episodes**다.

- 한 episode는 probe tool 하나만 광고/허용하고 정확히 한 번 호출하도록 요청한다.
- environment/transport/no-call은 model denominator에 넣지 않는다.
- completed episode를 얻기 위해 case당 최대 3회까지만 재시도한다.
- 9개 completed episode를 얻지 못하면 전체 결과는 `inconclusive`; production contract 변경은 금지한다.
- environment/transport failure rate가 전체 attempt의 5%를 넘으면 warning을 남기지만, 그 수치 자체로 model/harness 판정을 하지 않는다.

invalid raw call이 처음 관측되면 같은 host+fixture를 fresh session 9회 더 실행해 10 completed episodes를 만든다.

- 동일 normalized diagnostic signature가 2회 이상이면 `confirmed_drift`이며 P1 enforcement gate를 연다.
- 10회 중 1회뿐이면 10회를 추가한다.
- 20회 중 동일 signature가 2회 이상이면 `confirmed_drift`.
- 20회 중 1회뿐이면 `unreproduced_observation`; redacted ignored evidence만 남기고 tracked fixture/enforcement를 만들지 않는다.

confirmed drift가 있을 때만 affected host+fixture에 대해 32 KiB deterministic context-pressure profile을 별도 denominator로 실행한다. clean/context-pressure 결과를 합치지 않는다.

### 7. CLI contract

```text
agent-harness contract conformance baseline [--json]
agent-harness contract conformance live --hosts codex,claude,gjc [--model host=value]... [--gjc-auth-env NAME]... [--profile clean|context-pressure] [--only host:fixture] [--resume-report PATH] [--target-completed 1|10|20] [--max-attempts-per-case 1..3] [--evidence-dir PATH] [--json]
agent-harness contract conformance replay --fixture PATH [--json]
agent-harness contract conformance serve --fixture-id ID --result-file PATH --run-token TOKEN
```

- `baseline`: network/auth 없이 10 direct cases + tracked regression corpus를 재생한다.
- `live`: `HARNESS_TOOL_CONFORMANCE_LIVE=1`이 없으면 실행 전 거부한다. baseline을 먼저 내부 실행하고 실패 시 host process를 띄우지 않는다.
- `replay`: 하나의 promoted fixture를 fake handler와 temp state에 재생한다.
- `serve`: host subprocess용 capture-only stdio MCP. production catalog/handler를 등록하지 않는다.
- completed live report는 drift 발견 여부와 무관하게 exit 0이며 `gate.decision`은 `defer_hardening`, `needs_reproduction`, `authorize_hardening`, `unreproduced_observation` 중 하나다. benchmark integrity/environment가 미완료면 exit 1, `gate.decision=inconclusive`다. `valid_but_semantically_different`는 model semantic finding으로 보고하지만 schema hardening gate를 열지 않는다.

### 8. Host isolation contract

| Host | Exact isolation | Tool restriction | Unsafe fallback |
|---|---|---|---|
| Codex | temp cwd, `exec --ignore-user-config --ignore-rules --ephemeral --json --sandbox read-only --ask-for-approval never --skip-git-repo-check`, `-c`로 probe MCP 하나만 설정 | probe MCP만 user-config에 추가되고 built-in은 read-only sandbox | user config 수정 금지 |
| Claude | temp `.mcp.json`, `-p --output-format stream-json --strict-mcp-config --no-session-persistence --permission-mode dontAsk --tools "" --allowedTools "mcp__agent_harness_probe__${probe_tool_name}"` | episode별 한 probe tool만 allow | local/user MCP 등록 수정 금지 |
| GJC | temp cwd + temp `GJC_CODING_AGENT_DIR`, generated project-scope GJC bundle, `-p --mode=json --no-session --no-tools --no-lsp --no-skills --no-rules --model "$GJC_PROBE_MODEL"` | empty user plugin root + project probe MCP만 로드 | real agent dir/registry disable 금지, credential DB copy/symlink 금지 |

GJC generated bundle은 `gajae-plugin.json`, `launcher.ts` 두 파일만 포함한다. adapter가 run-specific absolute harness binary/serve argv를 JSON string literal로 escape해 mode 0600 `launcher.ts`에 내장한다. plugin MCP가 env를 상속하지 않는 현재 GJC 동작을 전제로 하며, bundle/project/agent temp root는 mode 0700이고 process 종료 후 삭제한다.

GJC model auth는 temp agent dir에서도 이용 가능한 provider API-key env 또는 `GJC_AUTH_BROKER_*`만 허용한다. 둘 다 없거나 explicit `--model gjc=...`가 없으면 `harness_environment:isolated_auth_unavailable`로 fail-closed한다.

### 9. Behavioral regression contract

tracked fixture 이름은 `host + "-" + fixture_id + "-" + first12(signature_sha256) + ".json"` 문법으로 만들고 `internal/core/toolconformance/testdata/regressions/` 아래에 둔다. 승격 조건은 동일 signature 2회 이상 재현이다.

각 fixture는 다음만 저장한다.

- fixture schema version, source/probe tool, source schema SHA, host/version/model label.
- redacted canonical arguments와 raw arguments SHA.
- expected classification + sorted diagnostic signature.
- expected handler call count `0`.
- expected final MCP result `{isError:true, error.code:"invalid_tool_arguments"}`.
- expected temp state digest before/after 동일.

host transcript, prompt cache, chain-of-thought, credential, absolute home path는 저장하지 않는다.

### 10. P1 atomic production contract

P1 gate가 열리면 다음 변경을 한 commit/rollback unit으로 적용한다.

1. advertised catalog를 closed projection으로 내보낸다.
2. SDK/legacy 두 call entry에서 동일 validator를 raw decode 직후 호출한다.
3. parse 가능한 schema violation은 tool-level `isError:true` + `invalid_tool_arguments` JSON을 반환한다.
4. validation 실패 시 group handler 호출 횟수는 0이다.
5. `argmap`의 string/CSV coercion과 partial slice filtering을 제거하고 typed accessor로 만든다.
6. valid requests의 final serialized output은 변경하지 않는다.

invalid JSON envelope/unknown tool은 기존 JSON-RPC `-32602`를 유지한다. parse 가능한 arguments schema violation만 normalized tool error로 바꾼다. diagnostic-only production mode나 per-host enforcement flag는 두지 않는다.

### 11. Adoption scorecard deferral

현재 코드에는 조직 dashboard를 추가하지 않는다. ADR에 다음 activation gate만 기록한다.

- 두 번째 실제 human operator가 rollout 참여를 명시적으로 opt in.
- retention/cost/task outcome 데이터 수집 범위와 보존 기간에 대한 승인.
- merged PR 외에 review rework, incident/rollback, completed-task quality를 포함한 outcome proxy 합의.
- host별 비용 source와 baseline 기간 확보.

이 네 조건 전에는 adoption/retention/impact 수치를 수집하거나 agent-harness 성공 지표로 주장하지 않는다.

## File map

### New files

- `internal/core/toolconformance/{types,schema,fixtures,classify,benchmark,replay}.go`
- `internal/core/toolconformance/{schema,fixtures,classify,benchmark,replay}_test.go`
- `internal/core/toolconformance/testdata/fixture_manifest.json`
- `internal/core/toolconformance/testdata/regressions/*.json` — confirmed drift가 있을 때만 생성.
- `internal/core/failurecause/{cause,cause_test}.go`
- `internal/port/tool_conformance.go`
- `internal/adapter/hostprobe/{runner,codex,claude,gjc,gjc_bundle}.go`
- `internal/adapter/hostprobe/{runner,codex,claude,gjc}_test.go`
- `internal/adapter/mcp/conformance_probe.go`
- `internal/adapter/mcp/conformance_probe_test.go`
- `cmd/harness/contractcli/conformance.go`
- `cmd/harness/contractcli/conformance_test.go`

### Existing files to modify unconditionally

- `cmd/harness/contractcli/contract.go`
- `cmd/harness/harnessapp/misc_facade.go`
- `cmd/harness/commandstep/result.go`
- `cmd/harness/selfworkflow/model/self_verify_summary_types.go`
- `cmd/harness/selfworkflow/model/self_augment_compare_types.go`
- `cmd/harness/selfworkflow/summary/{self_verify_summary,self_verify_summary_contract}.go`
- `cmd/harness/selfworkflow/historycompare/self_verify_compare_core.go`
- `cmd/harness/selfworkflow/steps/self_verify_steps.go`
- `internal/core/trace/{analyze,self_verify,fields}.go`
- focused tests beside the files above.
- `cmd/harness/testdata/{usage.golden.txt,mcp_tools.golden.json,response_contracts.golden.json}`
- `.agent-harness/{ARCHITECTURE,TESTING,OPERATIONS,CAUTIONS,ADR}.md`
- `skills/self-verify/SKILL.md`
- `skills/self-augment/SELF_AUGMENTATION.md`

### Existing files modified only when P1 gate is authorized

- `internal/adapter/mcp/{catalog,adapter_owned_catalog,issueops_catalog}.go` 및 explicit open-map schema owner files.
- `cmd/harness/mcpcli/{mcp_sdk_server,mcp_tools}.go`
- `cmd/harness/mcpcli/argmap/args.go`
- their focused tests.
- MCP tool/response goldens generated in T7.

## Verification strategy

- **TDD**: 각 code task는 아래에 명시된 failing test를 먼저 추가하고, 그 test를 통과시키는 최소 구현만 작성한다.
- **No live dependency in deterministic gates**: unit test, contract golden, CI, 기본 self-verify는 host binary 존재나 model auth를 요구하지 않는다.
- **Process doubles**: host runner test는 injected executable/process runner로 argv, env allowlist, stdin, stdout, timeout, cleanup을 검증한다. 실제 Codex/Claude/GJC는 T5에서만 실행한다.
- **Evidence root**: `.agent-harness/evidence/tool-conformance/` 아래 `BenchmarkReport.RunID`과 같은 이름의 child directory. 이 경로는 ignored이며 파일 mode 0600, directory mode 0700을 assert한다.
- **All-or-nothing**: 최종 완료 evidence는 T8의 마지막 전체 run에서만 가져온다. 이전 task의 부분 통과를 조합하지 않는다.
- **Golden discipline**: T1–T6에서는 golden을 갱신하지 않는다. 의도한 public contract가 모두 고정된 뒤 T7에서만 `-update`를 실행하고 diff를 검토한다.
- **Live approval boundary**: initial 9-episode matrix, 10/20 reproduction batch, context-pressure batch, post-enforcement batch는 각각 별도 비용·외부 호출 경계다. 이전 batch 결과가 다음 batch를 필요로 해도 자동진행하지 않는다.

## Execution strategy

### Waves

- **Wave 1**: T1 — core fixture/schema/classification/failure-cause contract.
- **Wave 2**: T2(probe server + deterministic CLI)와 T4(self-verify cause axis)를 병렬 가능. 두 작업 모두 T1 이후 시작한다.
- **Wave 3**: T3 — 세 host runner와 live report/gate. T2의 real serve contract에 의존한다.
- **Wave 4**: T5 — operator-approved live P0. code 변경 없이 evidence를 생성하고 gate를 확정한다.
- **Wave 5**: T6 — `authorize_hardening`일 때만 production enforcement. `defer_hardening`/`inconclusive`면 건너뛴다.
- **Wave 6**: T7 — self-verify deterministic step, public contract/golden, docs/skill 동기화.
- **Final wave**: T8 — focused/full/race/build/self-verify, diff audit, 조건부 post-enforcement live proof.

### Dependency matrix

| Task | Depends on | Blocks | Parallel with |
|---|---|---|---|
| T1 | — | T2, T3, T4 | — |
| T2 | T1 | T3 | T4 |
| T3 | T1, T2 | T5 | — |
| T4 | T1 | T7 | T2 |
| T5 | T3 + explicit live approval | T6, T7 | — |
| T6 | T5=`authorize_hardening` | T7 | — |
| T7 | T4, T5, and T6 when applicable | T8 | — |
| T8 | T7 | — | — |

### Execution result

- T1–T5, T7, T8 completed.
- T6 was intentionally skipped because the completed live matrix returned `gate.decision=defer_hardening`; its `authorize_hardening` entry condition was not met.
- Final live evidence: `.agent-harness/evidence/tool-conformance/20260714T114337.222844000Z/report.json` (`9/9` completed, `0` environment failures, `0` transport failures).
- The final local gate passed `go test`, race tests, `go vet`, build, contract checks, deterministic baseline, install dry-run, quick self-verify, and skill validation.

## Tasks

- [x] **T1. Preregister the core conformance and failure-cause contracts**

  **Owner**: main agent. 이 task는 cross-layer 용어와 validity semantics를 고정하므로 위임하지 않는다.

  **Tests first**:

  1. `internal/core/toolconformance/fixtures_test.go`에 `TestFixtureManifestPinsRepresentativeCatalogSchemas` 작성. 세 source tool의 canonical schema hash가 manifest와 다르면 source tool, want/got hash를 포함해 실패한다.
  2. `schema_test.go`에 10개 baseline table test 작성. 각 case에서 advertised/closed validity, classification, diagnostic code/path를 exact assert한다.
  3. `schema_test.go`에 unsupported keyword, nested unknown key, JSON Pointer escaping, integer-vs-number, enum, mixed-type array, deterministic diagnostic order를 추가한다.
  4. `failurecause/cause_test.go`에 no evidence→unknown, success→none, model-only→model, model+contract→contract_input, model+transport→transport precedence를 작성한다.

  **Implementation**:

  1. `internal/core/failurecause/cause.go`:
     - `type Cause string`과 enum `none`, `model`, `harness_environment`, `transport`, `contract_input`, `unknown`.
     - `Evidence{Cause, Code, Source string}`. `Code`/`Source`는 bounded token으로 sanitize하고 freeform payload를 받지 않는다.
     - `Classify(failed bool, evidence []Evidence) Result{Cause,Reason,Evidence}`. reason은 deterministic code 조합으로 만든다.
  2. `internal/core/toolconformance/types.go`:
     - `FixtureManifestVersion=1`, `ReportSchemaVersion=1`.
     - `Fixture`, `BaselineCase`, `Diagnostic`, `CallObservation`, `CaseResult`, `HostReport`, `GateDecision`, `BenchmarkReport` 정의.
     - status/classification/gate 값은 string constant로 고정하고 unknown enum을 decode 시 거부한다.
  3. `fixtures.go`:
     - `//go:embed testdata/fixture_manifest.json`.
     - injected `[]ToolDescriptor{Name,InputSchema}`에서 source schema를 찾고 canonical `encoding/json` SHA를 검증.
     - expected arguments와 10 baseline cases를 manifest에서 읽고 deep copy해 caller mutation을 차단.
  4. `schema.go`:
     - repo schema subset compiler와 recursive validator 구현.
     - `ClosedProjection`은 input map을 mutate하지 않고 object closure 규칙을 적용.
     - intentional open map은 source의 explicit `additionalProperties:true`만 보존.
  5. `classify.go`:
     - advertised/closed validator 결과와 expected arguments를 사용해 §4 우선순위대로 classification.
     - current legacy coercion 판정은 benchmark 설명용 pure function으로만 두고 실제 값을 repair하지 않는다.
  6. `testdata/fixture_manifest.json`에 §1 hash/expected arguments/10 cases를 그대로 기록한다.

  **Must not**:

  - core가 `cmd/harness` 또는 `internal/adapter`를 import하지 않는다. adapter catalog는 caller가 `ToolDescriptor`로 투영한다.
  - schema hash mismatch를 자동 갱신하지 않는다.
  - validator가 unsupported keyword를 무시하지 않는다.

  **References**:

  - `internal/adapter/mcp/catalog.go:3-8` — adapter tool descriptor.
  - `cmd/harness/testdata/mcp_tools.golden.json` — 현재 100-tool source-backed schema snapshot.
  - `cmd/harness/mcpcli/argmap/args.go:33-123` — 기존 coercion shape. benchmark classification 설명에만 사용.
  - `internal/core/webfetch/{types,benchmark}.go` — deterministic-first/live-opt-in result pattern. import하거나 공용화하지 않고 구조만 따른다.

  **Acceptance criteria**:

  - [ ] `go test ./internal/core/toolconformance ./internal/core/failurecause -count=1` 통과.
  - [ ] baseline count가 exact 10이며 네 payload class가 모두 최소 1회 존재.
  - [ ] source schema map이 validation 전후 byte-equivalent임을 test가 증명.
  - [ ] `rg -n 'oldText2|requireUnique' internal/core/toolconformance/testdata`에서 `requireUnique`는 unknown-key fixture에만 존재하고 silent alias table은 없음.

  **QA evidence**:

  ```bash
  mkdir -p .agent-harness/evidence
  go test ./internal/core/toolconformance ./internal/core/failurecause -count=1 -v \
    | tee .agent-harness/evidence/task-1-core-conformance.txt
  ```

  Expected: 10-case baseline/classifier, schema keyword, cause precedence tests 모두 PASS.

  **Proposed commit**: `feat(contract): preregister tool conformance and failure-cause contracts`

  Lore body에는 왜 3 schemas/10 cases로 제한했는지, production enforcement가 아직 없음을 기록한다.

- [x] **T2. Add a capture-only MCP probe server and deterministic contract CLI**

  **Owner**: main agent. production MCP와 capture server 경계를 직접 확인한다.

  **Tests first**:

  1. `internal/adapter/mcp/conformance_probe_test.go`에서 initialize→tools/list→tools/call stdio round-trip을 수행한다.
  2. tools/list가 requested fixture의 probe tool 정확히 1개만 반환하는지 assert한다.
  3. tools/call 후 injected production dispatch spy가 0회인지 assert한다.
  4. result file이 0600, parent가 0700이며 raw SHA/canonical redacted args/schema hash/run-token hash를 포함하는지 assert한다.
  5. malformed JSON, second call, wrong tool name, stale run token/result collision을 fail-closed test한다.
  6. `contractcli/conformance_test.go`에 baseline exit, replay exit, missing live opt-in, serve flag parsing을 추가한다.

  **Implementation**:

  1. `internal/adapter/mcp/conformance_probe.go`는 official Go MCP SDK로 독립 `mcp.Server`를 만든다. main `registerAllTools`와 `resolveHandlerGroup`를 호출하지 않는다.
  2. `serve --fixture-id`는 catalog source schema를 복사해 probe name으로 바꾸고 **current advertised schema**를 그대로 광고한다. closed projection은 capture 후 classifier에만 사용한다.
  3. raw `req.Params.Arguments` bytes를 64 KiB에서 hard cap하고 SHA-256을 계산한다. parsed canonical arguments는 project redaction helper를 통과시킨 뒤 저장한다.
  4. result는 temp sibling write→fsync→rename으로 atomic 저장한다. 같은 result path의 두 번째 call은 `multiple_calls`를 기록하되 첫 call을 덮어쓰지 않는다.
  5. 성공 response는 fixed body `{"ok":true,"captured":true,"fixture_id":"..."}`만 반환한다. arguments를 model에 echo하지 않는다.
  6. `cmd/harness/contractcli/conformance.go`에 `baseline`, `live`, `replay`, `serve` dispatch를 추가한다. 이 task에서는 `live`가 port dependency를 호출하도록 wiring만 하고 real runner는 T3에서 주입한다.
  7. `contract.go:40-48` switch에 `conformance`, usage에 네 subcommand를 추가한다. `harnessapp/misc_facade.go`의 기존 `configureContractCLI` seam으로 catalog/root/process deps를 주입한다.
  8. `baseline`은 10 synthetic cases와 regression directory를 deterministic order로 실행한다. regression directory가 없으면 0개로 정상 처리한다.
  9. `replay`는 fake handler counter + temp state digest를 이용해 behavioral expectations를 검증한다.

  **Must not**:

  - main MCP tool catalog에 `harness_probe_*`를 추가하지 않는다.
  - probe response/result에 host stdout, prompt, credentials, raw unredacted strings를 저장하지 않는다.
  - `live` opt-in이 없을 때 host executable을 stat/launch하지 않는다.

  **References**:

  - `cmd/harness/mcpcli/mcp_sdk_server.go:16-31,75-84` — official SDK server/register pattern.
  - `cmd/harness/mcpcli/mcp_sdk_server_test.go:12-42` — raw SDK handler test pattern.
  - `cmd/harness/contractcli/contract.go:35-93` — nested subcommand/JSON output style.
  - `cmd/harness/harnessapp/misc_facade.go:34-45` — contract dependency wiring.

  **Acceptance criteria**:

  - [ ] `go test ./internal/adapter/mcp ./cmd/harness/contractcli -run 'Conformance|Probe' -count=1` 통과.
  - [ ] `go build -o bin/agent-harness ./cmd/harness` 성공.
  - [ ] `./bin/agent-harness contract conformance baseline --json`은 `ok:true`, `case_count:10`, `gate.decision:"baseline_passed"`.
  - [ ] `HARNESS_TOOL_CONFORMANCE_LIVE` 없이 `live`는 nonzero, `live_opt_in_required`, process spy 0회.

  **QA evidence**:

  ```bash
  go test ./internal/adapter/mcp ./cmd/harness/contractcli \
    -run 'Conformance|Probe' -count=1 -v \
    | tee .agent-harness/evidence/task-2-probe-cli.txt
  go build -o bin/agent-harness ./cmd/harness
  ./bin/agent-harness contract conformance baseline --json \
    | tee .agent-harness/evidence/task-2-baseline.json
  ```

  **Proposed commit**: `feat(contract): add capture-only MCP conformance probe`

- [x] **T3. Implement isolated Codex, Claude, and GJC host runners plus the live gate**

  **Owner**: main agent. host-specific code는 thin adapter에만 두며 core semantics를 복제하지 않는다.

  **Tests first**:

  1. `internal/adapter/hostprobe/runner_test.go`: missing executable, version timeout, auth failure, MCP startup failure, child timeout, result missing, stale result, output truncation, temp cleanup.
  2. `codex_test.go`: exact argv/config override, read-only sandbox, ignore-user-config/rules, ephemeral, one probe MCP, fresh temp cwd.
  3. `claude_test.go`: exact `--strict-mcp-config`, temp config, no persistence, `--tools ""`, one `--allowedTools`, stream-json.
  4. `gjc_test.go`: temp `GJC_CODING_AGENT_DIR`, explicit model/auth-env requirement, project plugin install path, generated file modes/content, `no-tools/no-skills/no-rules`, cleanup. real agent dir path가 argv/generated bundle에 없음을 assert한다.
  5. benchmark test: 9 completed exact-valid episodes→`defer_hardening`; one env failure after retry exhaustion→`inconclusive`; invalid observation→`needs_reproduction`; resume 10/20 signature gate.

  **Implementation**:

  1. `internal/port/tool_conformance.go`:
     - `HostProbeRunner` interface: `Name()`, `Preflight(ctx, request)`, `Run(ctx, request)`.
     - request/result DTO에는 host-neutral scalar만 두고 core/adapter type import cycle을 피한다.
  2. `internal/adapter/hostprobe/runner.go`:
     - injected `CommandRunner`, `LookPath`, `Now`, `TempDir`.
     - timeout 5분/episode, stdout·stderr 각 64 KiB cap, env key 이름만 report.
     - process exit와 probe result file을 함께 판정. host stdout만으로 tool call success를 추론하지 않는다.
  3. Codex adapter:
     - `codex exec --ignore-user-config --ignore-rules --ephemeral --json --sandbox read-only --ask-for-approval never --skip-git-repo-check -C "$temp_dir"`.
     - `-c`의 `mcp_servers.agent_harness_probe.command`에는 `harness_binary`, `args`에는 `contract conformance serve` argv를 TOML array로 설정.
     - requested model이 `default`가 아니면 `--model` 추가.
  4. Claude adapter:
     - temp `.mcp.json`에 probe server 하나만 작성.
     - `claude -p --output-format stream-json --strict-mcp-config --mcp-config "$mcp_config_file" --no-session-persistence --permission-mode dontAsk --tools "" --allowedTools "mcp__agent_harness_probe__${probe_tool_name}"`.
     - requested model이 `default`가 아니면 `--model` 추가.
  5. GJC adapter:
     - temp project/agent/bundle roots 0700 생성.
     - `gajae-plugin.json`은 MCP surface 하나만 선언.
     - run-specific serve argv를 안전하게 JSON-escape한 mode 0600 `launcher.ts`를 생성하고 `gjc plugin install "$bundle_dir" --project`를 temp cwd에서 실행.
     - child env는 PATH/HOME/TMPDIR/locale + `GJC_CODING_AGENT_DIR` + operator가 `--gjc-auth-env`로 명시한 nonempty keys만 전달. value는 log/report하지 않는다.
     - `gjc -p --mode=json --no-session --no-tools --no-lsp --no-skills --no-rules --model "$GJC_PROBE_MODEL"`.
     - API/broker auth가 없으면 launch 전 `isolated_auth_unavailable`.
  6. `benchmark.go`에 initial/reproduction/resume gate를 구현한다. environment/transport attempt와 completed model episodes를 별도 count한다.
  7. `contract conformance live` flags:
     - `--hosts` default `codex,claude,gjc`.
     - repeatable `--model host=value`.
     - repeatable `--gjc-auth-env NAME`.
     - `--profile clean|context-pressure` default clean.
     - `--only host:fixture` optional.
     - `--resume-report PATH` optional.
     - `--target-completed 1|10|20` default 1.
     - `--max-attempts-per-case 3` fixed upper bound; 1–3 외 값 거부.
     - `--evidence-dir` default `.agent-harness/evidence/tool-conformance`.
  8. context-pressure profile은 fixed SHA를 가진 32 KiB deterministic appendix를 prompt 앞에 붙인다. profile 자체를 별도 fixture hash로 기록한다.

  **Must not**:

  - host adapter가 schema를 변형하지 않는다.
  - GJC real user registry/config/auth DB를 읽기 위해 temp isolation을 해제하지 않는다.
  - live result가 invalid call을 발견했다고 자동으로 10/20 batch를 실행하지 않는다.
  - host stderr text grep만으로 `model` cause를 부여하지 않는다.

  **References**:

  - installed Codex 0.144.3 `exec --help` — `--json`, `--ephemeral`, `--ignore-user-config`, `--ignore-rules`, read-only sandbox/approval flags.
  - installed Claude Code 2.1.209 help — print/stream-json/strict MCP/no-session/tool allow flags.
  - installed GJC 0.9.5 `src/extensibility/gjc-plugins/paths.ts:10-12`, `registry.ts:168-170`, `runtime-adapters.ts:299-345` — project registry와 no-inherit-env behavior.
  - installed GJC `src/main.ts:717-734` — built-in tool disable와 extension discovery behavior.

  **Acceptance criteria**:

  - [ ] `go test ./internal/adapter/hostprobe ./internal/core/toolconformance ./cmd/harness/contractcli -count=1` 통과.
  - [ ] fake-runner matrix에서 host마다 exact 3 fixtures, fresh temp dir, one tool only.
  - [ ] every failure result has exactly one cause evidence source and secret value가 JSON에 없음.
  - [ ] GJC unsafe auth fallback test가 real `~/.gjc/agent` 접근 시도 0회를 증명.

  **QA evidence**:

  ```bash
  go test ./internal/adapter/hostprobe ./internal/core/toolconformance ./cmd/harness/contractcli \
    -run 'HostProbe|LiveGate|Conformance' -count=1 -v \
    | tee .agent-harness/evidence/task-3-host-runners.txt
  ```

  **Proposed commit**: `feat(contract): add isolated cross-host conformance runners`

- [x] **T4. Add the orthogonal failure-cause axis to self-verify, trace, and history**

  **Owner**: main agent. 기존 failure contract와 snapshot backward compatibility를 함께 다룬다.

  **Tests first**:

  1. summary tests: success→`none`+empty evidence, failed generic step→`unknown`, typed conformance evidence→exact cause, existing `failure_class` 결과 불변.
  2. state snapshot tests: schema-v1 legacy snapshot에 cause fields가 없어도 read 후 safe defaults, new snapshot round-trip.
  3. history compare tests: baseline/candidate cause change는 warning이며 단독 regression이 아님.
  4. trace tests: self-verify cause/evidence propagation, dedupe key에 cause 포함, redaction.

  **Implementation**:

  1. `cmd/harness/commandstep/result.go`의 `StepResult`에 `FailureEvidence []failurecause.Evidence \`json:"failure_evidence,omitempty"\`` 추가. command runner는 기본 nil을 유지하고 새 conformance step만 typed evidence를 채운다.
  2. `SelfAugmentSummary`에 다음 required output fields 추가:
     - `FailureCause failurecause.Cause \`json:"failure_cause"\``
     - `FailureCauseReason string \`json:"failure_cause_reason"\``
     - `FailureCauseEvidence []failurecause.Evidence \`json:"failure_cause_evidence"\``
  3. `SummarizeSelfVerification`은 failed steps의 evidence를 모아 `failurecause.Classify`를 호출한다. evidence 없는 실패를 `unknown`으로 유지한다. 기존 `ClassifySelfVerificationFailure`는 수정하지 않는다.
  4. `SelfVerificationContractValue` version을 3→4로 올리고 세 필드를 `RequiredFields`에 추가한다.
  5. state snapshot schema version은 1을 유지한다. additive fields이므로 legacy empty cause를 read 시 `none` 또는 `unknown`으로 normalize한다.
  6. `SelfAugmentCompareResult`에 full summaries는 이미 있으므로 cause field를 중복 추가하지 않는다. 두 failed summary의 cause가 다르면 `"failure_cause_changed:" + oldCause + "->" + newCause`를 `Warnings`에 추가하되 `Regressed`를 바꾸지 않는다.
  7. `TraceAnalysisFinding`에 `FailureCause`와 `FailureCauseEvidence`를 추가하고 self-verify trace parser가 전달한다. generic guard/doc trace는 cause=`unknown`.
  8. trace dedupe/sort key에 cause를 포함한다.

  **Must not**:

  - stderr/error freeform keyword matching으로 model/harness cause를 추정하지 않는다.
  - failure cause change만으로 self-verify compare를 fail하지 않는다.
  - snapshot schema version을 bump하지 않는다.

  **References**:

  - `cmd/harness/selfworkflow/model/self_verify_summary_types.go:38-69` — summary/contract types.
  - `cmd/harness/selfworkflow/summary/self_verify_summary.go:72-77` — current failure classification insertion point.
  - `cmd/harness/selfworkflow/summary/self_verify_summary_failure.go:7-24` — 보존할 occurrence classifier.
  - `cmd/harness/selfworkflow/stateio/self_verify_state_snapshot.go:11-26` — schema-v1 compatibility.
  - `internal/core/trace/analyze.go:25-31`, `self_verify.go:11-52`, `fields.go:116-129` — propagation/dedupe.

  **Acceptance criteria**:

  - [ ] `go test ./cmd/harness/selfworkflow/... ./internal/core/trace -run 'FailureCause|FailureClass|LegacySnapshot' -count=1` 통과.
  - [ ] pre-existing failure-class tests의 expected strings는 바뀌지 않음.
  - [ ] 성공/실패 JSON 모두 세 cause fields를 갖고 nil slice는 `[]`로 normalize.
  - [ ] contract version 4 hash가 deterministic.

  **QA evidence**:

  ```bash
  go test ./cmd/harness/selfworkflow/... ./internal/core/trace \
    -run 'FailureCause|FailureClass|LegacySnapshot' -count=1 -v \
    | tee .agent-harness/evidence/task-4-failure-cause.txt
  ```

  **Proposed commit**: `feat(self-verify): classify failure causes from typed evidence`

- [x] **T5. Run the operator-approved P0 evidence gate and promote only reproduced failures**

  **Owner**: main agent. 이 task는 외부 model 비용과 인증을 사용하므로 각 batch 전에 사용자 승인 경계에서 멈춘다.

  **Preflight — no external model call**:

  ```bash
  git status --short
  go build -o bin/agent-harness ./cmd/harness
  ./bin/agent-harness contract conformance baseline --json \
    | tee .agent-harness/evidence/tool-conformance-baseline.json
  codex --version
  claude --version
  gjc --version
  ```

  Expected: worktree scope가 알려져 있고 baseline `ok:true`, 10/10. host version 명령만 실행하며 model call은 없음.

  **Initial live batch — explicit approval required**:

  GJC를 포함할 때 operator는 isolated model과 그 credential env 이름을 지정한다. 값은 shell/report에 출력하지 않는다.

  ```bash
  : "${GJC_PROBE_MODEL:?set GJC_PROBE_MODEL to an authenticated GJC model id}"
  : "${GJC_PROBE_AUTH_ENV:?set GJC_PROBE_AUTH_ENV to the credential env variable name}"
  test -n "$(printenv "$GJC_PROBE_AUTH_ENV")"
  export HARNESS_TOOL_CONFORMANCE_LIVE=1
  ./bin/agent-harness contract conformance live \
    --hosts codex,claude,gjc \
    --model codex=default \
    --model claude=default \
    --model "gjc=${GJC_PROBE_MODEL}" \
    --gjc-auth-env "${GJC_PROBE_AUTH_ENV}" \
    --profile clean \
    --target-completed 1 \
    --max-attempts-per-case 3 \
    --evidence-dir .agent-harness/evidence/tool-conformance \
    --json
  ```

  Initial batch 상한은 9 cases × 3 attempts = 27 host invocations다. 정상 환경에서는 9회다. report path는 CLI JSON의 `evidence.report_path`에서 읽는다.

  **Decision branch**:

  1. `gate.decision=inconclusive`: environment/transport evidence를 고친 뒤 initial batch만 다시 제안한다. model/P1 판단 금지.
  2. `gate.decision=defer_hardening`: T6를 건너뛰고 T7로 간다. “모델 문제가 없다”가 아니라 “현재 preregistered matrix에서 confirmed drift가 없다”고 기록한다.
  3. `gate.decision=needs_reproduction`: 한 번에 affected host+fixture 하나만 선택해 다음 10-completed batch를 사용자에게 승인 요청한다.

  **Reproduction batch — separate approval**:

  ```bash
  export HARNESS_TOOL_CONFORMANCE_LIVE=1
  ./bin/agent-harness contract conformance live \
    --resume-report .agent-harness/evidence/tool-conformance/initial/report.json \
    --only claude:nested_object_array \
    --target-completed 10 \
    --profile clean \
    --evidence-dir .agent-harness/evidence/tool-conformance \
    --json
  ```

  위 `initial/report.json`과 `claude:nested_object_array`는 CLI가 initial report에 출력한 실제 `report_path`/`next_reproduction_target`을 그대로 사용한다. 임의의 다른 host/fixture로 넓히지 않는다.

  - 10 completed 중 동일 signature 2회 이상: `authorize_hardening`.
  - 정확히 1회: 별도 승인 후 같은 `--resume-report`로 `--target-completed 20`.
  - 20 completed 중 정확히 1회: `unreproduced_observation`, T6 금지.

  **Conditional context-pressure batch — separate approval**:

  confirmed signature가 있을 때만 같은 target에 `--profile context-pressure --target-completed 10`을 실행한다. clean report와 denominator를 합치지 않는다.

  **Fixture promotion**:

  1. CLI가 ignored evidence root에 만든 `candidate-regression.json`을 `jq`로 검토한다.
  2. raw values, home path, prompt/transcript, credential field가 없음을 scan한다.
  3. 동일 signature 2회 evidence id가 들어 있는지 확인한다.
  4. 조건이 모두 맞을 때만 CLI가 제시한 exact `tracked_fixture_path`에 `apply_patch`로 regression JSON을 추가한다.
  5. `contract conformance replay --fixture "$tracked_fixture_path" --json`으로 fixture 자체의 classification/handler-0/state-unchanged expectation을 검증한다. T6는 같은 fixture를 SDK/legacy production entry에 재생하는 failing parity test부터 시작한다.

  **Must not**:

  - batch를 자동 연속 실행하지 않는다.
  - invalid call 한 번만으로 tracked fixture/P1 commit을 만들지 않는다.
  - live evidence를 git add하지 않는다.
  - GJC auth가 막혔다고 real plugin registry를 disable하거나 agent DB를 복사하지 않는다.

  **Acceptance criteria**:

  - [ ] initial report에 9 completed 또는 explicit `inconclusive` cause가 있음.
  - [ ] P1 decision은 §6 denominator/signature rule에서 기계적으로 계산됨.
  - [ ] confirmed drift fixture가 있다면 replay test는 handler call 0/final error/state unchanged를 요구함.
  - [ ] `git status --short`에 `.agent-harness/evidence/`가 나타나지 않음.

  **Evidence commands**:

  ```bash
  jq '{schema_version,run_id,counts,gate,hosts,warnings}' \
    .agent-harness/evidence/tool-conformance/*/report.json
  rg -n --hidden -i \
    '(api[_-]?key|authorization|bearer|refresh[_-]?token|/Users/[^/]+)' \
    .agent-harness/evidence/tool-conformance \
    > .agent-harness/evidence/task-5-secret-scan.txt || true
  git status --short
  ```

  Expected: secret scan output empty. evidence root ignored.

  **Proposed commit**: 없음 if no confirmed drift. Confirmed fixture만 있으면 `test(mcp): capture reproduced tool schema drift`로 fixture + core replay integrity test만 작은 commit으로 만든다.

- [ ] **T6. Conditionally enforce one canonical MCP argument contract across all tools** — skipped because `gate.decision=defer_hardening` did not satisfy the entry condition.

  **Entry condition**: T5 report의 `gate.decision=authorize_hardening`과 confirmed regression fixture가 모두 존재해야 한다. 아니면 이 task 전체를 skip한다.

  **Owner**: main agent. cross-host public contract 변경이므로 한 사람이 SDK/legacy/catalog/argmap을 함께 수정한다.

  **Tests first**:

  1. promoted fixture replay test가 현재 실패함을 확인한다.
  2. adapter catalog test: every object node has explicit boolean `additionalProperties`; expected intentional-open paths만 true.
  3. SDK/legacy parity table: valid, unknown key, coercible string bool, mixed array, missing required, enum failure, invalid JSON, unknown tool.
  4. handler spy/state digest test: parseable invalid args는 both paths에서 call count 0/state unchanged.
  5. valid representative tool calls의 exact prior text/final JSON output regression.
  6. argmap tests: string/CSV coercion과 partial array filtering이 더 이상 일어나지 않음.

  **Implementation**:

  1. catalog source에서 자유형 map 8곳(`fields`, `score_result`)에 `additionalProperties:true`를 명시한다. description/handler가 arbitrary map을 받는 현재 contract가 근거다.
  2. `mcpadapter.AdvertisedTools()`가 모든 implicit object를 cloned closed projection으로 내보내게 한다. source map은 mutate하지 않는다.
  3. tool schema index는 advertised descriptors에서 한 번 만들고 SDK/legacy가 같은 immutable validator를 사용한다. host별 branch가 없다.
  4. `sdkToolHandler`:
     - raw arguments를 validate.
     - invalid면 `CallToolResult{IsError:true}`와 normalized error JSON을 반환.
     - valid일 때만 `map[string]any` decode + group dispatch.
  5. `HandleToolCall`:
     - outer call을 `Name string`, `Arguments json.RawMessage`로 먼저 decode.
     - same validator/normalized error helper 사용.
     - valid일 때만 existing `MCPToolCall`로 dispatch.
  6. normalized payload:

     ```json
     {
       "ok": false,
       "error": {
         "code": "invalid_tool_arguments",
         "tool": "command_policy_check",
         "diagnostics": [
           {
             "code": "unknown_key",
             "path": "/requireUnique",
             "expected": "declared property",
             "actual": "boolean"
           }
         ]
       }
     }
     ```

  7. `argmap` accessors를 strict typed read로 축소한다.
     - `String`: string만.
     - `Bool`: bool만.
     - `Int`/`Int64`: JSON number가 정확한 integer 범위일 때만.
     - `StringSlice`: array의 모든 element가 string일 때만; 하나라도 아니면 nil/default.
     - CSV/string numeric/string boolean parsing 삭제.
  8. `mcpToolErrorPayload` comment를 갱신: parse 가능한 schema violation은 tool-level correction result; invalid envelope/unknown tool만 `-32602`.
  9. promoted regression은 final error/state assertion으로 pass시킨다. raw tool-call syntax나 model retry 횟수는 assert하지 않는다.

  **Atomic rollout/rollback boundary**:

  - schema closure, validator seam, normalized error, argmap strictness는 한 commit이다.
  - golden update는 T7 별도 commit이지만 T6 focused tests가 통과하기 전에는 수행하지 않는다.
  - valid-call output regression, SDK/legacy parity, promoted fixture 중 하나라도 실패하면 전체 T6 diff를 수정하고 부분 enforcement를 남기지 않는다.
  - per-tool flag, per-host bypass, diagnostic-only production branch를 추가하지 않는다.

  **References**:

  - `internal/adapter/mcp/catalog.go`, `catalog_test.go` — catalog projection/shape test.
  - `cmd/harness/mcpcli/mcp_sdk_server.go:33-71` — SDK raw decode/response seam.
  - `cmd/harness/mcpcli/mcp_tools.go:40-70` — legacy raw call/dispatch seam.
  - `cmd/harness/mcpcli/argmap/args.go:33-123` — 제거할 silent coercion.
  - `cmd/harness/mcpcli/mcp_sdk_server_test.go:12-42`, `mcp_tool_project_test.go:135-160` — parity/error tests.

  **Acceptance criteria**:

  - [ ] `go test ./internal/adapter/mcp ./cmd/harness/mcpcli ./internal/core/toolconformance -count=1` 통과.
  - [ ] `go test -race ./internal/adapter/mcp ./cmd/harness/mcpcli ./internal/core/toolconformance -count=1` 통과.
  - [ ] 100 tool top-level object가 `additionalProperties:false`.
  - [ ] nested open object는 explicit true 8곳뿐이며 path golden test로 고정.
  - [ ] promoted fixture replay handler call=0, state digest unchanged, SDK/legacy error body equivalent.
  - [ ] valid-call response snapshots unchanged except intended schema/error contract fields.

  **QA evidence**:

  ```bash
  go test ./internal/adapter/mcp ./cmd/harness/mcpcli ./internal/core/toolconformance \
    -run 'Schema|Arguments|Conformance|Legacy|SDK' -count=1 -v \
    | tee .agent-harness/evidence/task-6-canonical-validator.txt
  go test -race ./internal/adapter/mcp ./cmd/harness/mcpcli ./internal/core/toolconformance \
    -count=1 \
    | tee .agent-harness/evidence/task-6-canonical-validator-race.txt
  ```

  **Proposed commit**: `fix(mcp): enforce canonical tool argument validation`

  Lore body에는 confirmed evidence signature, intentional open-map 8곳, valid-output compatibility, rollback 단위를 기록한다.

- [x] **T7. Make deterministic conformance part of self-verify and synchronize public contracts/docs**

  **Owner**: main agent. code, golden, normative docs의 drift를 한 task에서 닫는다.

  **Tests first**:

  1. `steps/self_verify_steps_test.go`: `tool contract conformance` label이 contract check 다음, worker lifecycle 전의 deterministic step으로 정확히 1회 존재.
  2. step runner test: temp binary의 `contract conformance baseline --json`을 호출하고 report cause evidence를 `StepResult.FailureEvidence`로 변환.
  3. self-verification goal/coverage test: 새 label이 goal/coverage에 포함되고 누락 시 gap.
  4. response contract test: baseline JSON shape와 self-verify cause fields를 pin.

  **Implementation**:

  1. `SelfVerifyStepDeps`에 `ValidateToolConformance` 추가하고 planned steps에 `tool contract conformance`를 넣는다. live host를 호출하지 않는다.
  2. `SelfVerificationGoalDefinitions`의 MCP/state goal에 label을 추가하고 coverage claim `tool-call schema conformance`를 신설한다.
  3. `BuildCompatibilityContract`:
     - compatibility contract version 1→2.
     - `tool_conformance_report` response fields를 추가.
     - self-verification required fields는 v4 cause fields를 포함.
     - verification command에 `agent-harness contract conformance baseline --json` 추가.
  4. harness response snapshot에 `contract conformance baseline --json`을 temp-safe deterministic case로 추가한다.
  5. focused tests가 통과한 뒤 golden을 정확히 한 번 갱신한다.

     ```bash
     go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp \
       -run Golden -update -count=1
     go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp \
       -run Golden -count=1
     ```

  6. golden diff를 검토한다.
     - P1 skipped: usage/response contract/self-verify cause/conformance output만 변해야 하며 MCP schema 100개 closure 변화는 없어야 한다.
     - P1 applied: 위 변화 + every object의 intended `additionalProperties`만 있어야 한다.
     - unrelated command/tool/response 삭제가 있으면 실패로 보고 원인을 수정한다.
  7. 문서 업데이트:
     - `.agent-harness/ARCHITECTURE.md`: capture-only P0, shared canonical validator, host adapter 경계.
     - `.agent-harness/TESTING.md`: 10-case baseline, 9-episode live matrix, reproduction/promotion gate, behavioral assertion, 5% warning의 비규범성.
     - `.agent-harness/OPERATIONS.md`: baseline/live/replay exact commands, Codex/Claude/GJC isolation, live approval/cost boundary.
     - `.agent-harness/CAUTIONS.md`: advertised-vs-intent attribution, GJC auth isolation, implicit open map 금지, no silent repair.
     - `.agent-harness/ADR.md`: evidence-first conditional enforcement, one contract, adoption scorecard activation gate, no scheduler/RL platform.
     - `skills/self-verify/SKILL.md`: default self-verify가 deterministic conformance baseline을 포함한다는 1문장과 TESTING pointer.
     - `skills/self-augment/SELF_AUGMENTATION.md`: reproduced tool failure는 산출물 hand-fix가 아니라 fixture/validator/generator process 수정으로 닫는다는 1문장과 TESTING pointer.
  8. P1가 skip되었다면 docs에 “deferred_no_confirmed_drift”를 현재 영구 사실처럼 쓰지 않고, enforcement activation rule만 기록한다. run-specific 결과는 ignored evidence report에 남긴다.

  **Must not**:

  - live command를 self-verify step/CI example의 기본 실행으로 넣지 않는다.
  - 5%를 gate로 문서화하지 않는다.
  - adoption scorecard 구현/수집을 시작하지 않는다.
  - golden mismatch를 검토 없이 accept하지 않는다.

  **References**:

  - `cmd/harness/selfworkflow/steps/self_verify_steps.go:43-84` — planned step order.
  - `cmd/harness/selfworkflow/summary/self_verify_summary_contract.go:44-83` — goals/coverage.
  - `cmd/harness/contractcli/contract.go:95-157` — compatibility contract/hash.
  - `.agent-harness/TESTING.md:100-110` — golden update procedure.
  - `skills/self-verify/SKILL.md` — quick/full gate contract.

  **Acceptance criteria**:

  - [ ] focused selfworkflow/contract/golden tests pass.
  - [ ] `python3 scripts/validate-skill.py skills/self-verify` 통과.
  - [ ] `skills/self-augment/SELF_AUGMENTATION.md` 관련 기존 validation command가 있으면 통과; 없으면 markdown/reference scan으로 검증.
  - [ ] `./bin/agent-harness contract check --json`에서 version 2, ok true.
  - [ ] `./bin/agent-harness contract conformance baseline --json`에서 regression corpus 포함 pass.

  **QA evidence**:

  ```bash
  go test ./cmd/harness/selfworkflow/... ./cmd/harness/contractcli \
    ./cmd/harness/contractgolden ./cmd/harness/harnessapp -count=1 \
    | tee .agent-harness/evidence/task-7-contract-integration.txt
  python3 scripts/validate-skill.py skills/self-verify \
    | tee .agent-harness/evidence/task-7-self-verify-skill.txt
  ./bin/agent-harness contract check --json \
    | tee .agent-harness/evidence/task-7-contract-check.json
  ```

  **Proposed commits**:

  1. `feat(self-verify): gate deterministic tool conformance`
  2. `test(contract): update conformance response goldens`
  3. `docs(contract): document evidence-first tool hardening`

  각 commit은 `.agent-harness/COMMIT_POLICY.md`의 Conventional subject + Lore body를 따른다.

- [x] **T8. Final all-or-nothing verification, adversarial review, and handoff**

  **Owner**: main agent. 독립 review는 read-only net-positive `devils-advocate-review` pattern으로만 허용하며 수정/판정 책임은 main agent가 가진다.

  **Pre-final diff audit**:

  ```bash
  git status --short
  git diff --stat
  git diff --check
  git diff -- cmd/harness/testdata/usage.golden.txt \
    cmd/harness/testdata/mcp_tools.golden.json \
    cmd/harness/testdata/response_contracts.golden.json
  ```

  Verify every changed line maps to benchmark, cause axis, conditional validator, regression, contract, or docs. Unrelated cleanup은 되돌리지 말고 이 작업 diff에서 제거한다.

  **Focused verification**:

  ```bash
  rg --files \
    internal/core/toolconformance \
    internal/core/failurecause \
    internal/adapter/hostprobe \
    cmd/harness/contractcli \
    cmd/harness/selfworkflow \
    internal/core/trace \
    -g '*.go' | xargs gofmt -w
  gofmt -w \
    internal/port/tool_conformance.go \
    internal/adapter/mcp/conformance_probe.go \
    internal/adapter/mcp/conformance_probe_test.go \
    cmd/harness/commandstep/result.go \
    cmd/harness/harnessapp/misc_facade.go
  go mod tidy
  git diff --check
  go test ./internal/core/toolconformance ./internal/core/failurecause \
    ./internal/adapter/hostprobe ./internal/adapter/mcp \
    ./cmd/harness/contractcli ./cmd/harness/mcpcli \
    ./cmd/harness/selfworkflow/... ./internal/core/trace -count=1
  go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp \
    -run Golden -count=1
  ```

  T6를 적용한 run에서는 `go mod tidy` 전에 production MCP owner files도 format한다.

  ```bash
  rg --files internal/adapter/mcp cmd/harness/mcpcli -g '*.go' | xargs gofmt -w
  ```

  `go mod tidy`가 의도하지 않은 dependency change를 만들면 새 dependency를 추가하지 말고 원인을 제거한다. 이 계획의 validator는 standard library만으로 구현 가능하다.

  **Full final gate — one uninterrupted run**:

  ```bash
  set -e
  go test ./... -count=1
  go test -race ./... -count=1
  go vet ./...
  go build -o bin/agent-harness ./cmd/harness
  ./bin/agent-harness contract check --json
  ./bin/agent-harness contract conformance baseline --json
  ./bin/agent-harness install-native --dry-run --json
  tmp_state="$(mktemp -d)"
  HARNESS_STATE_DIR="$tmp_state" \
    ./bin/agent-harness self-verify \
      --seed=100 --target-score=95 --llm-eval=false --json
  rm -rf "$tmp_state"
  python3 scripts/validate-skill.py skills/self-verify
  ```

  Expected: 모든 command exit 0; self-verify summary contract v4, failure_cause=`none`, tool conformance coverage covered, termination eligible.

  **Adversarial review checklist**:

  - P0가 advertised schema gap을 model fault로 오분류하는가?
  - probe server가 어떤 경로로든 production handler/state를 건드릴 수 있는가?
  - GJC isolation이 auth 편의를 위해 real registry/DB로 새는가?
  - one-off observation이 fixture/enforcement로 승격될 수 있는가?
  - SDK/legacy가 다른 error semantics를 노출하는가?
  - intentional open maps가 allowlist drift 또는 silent implicit open으로 남는가?
  - failure cause가 stderr heuristic이나 confidence 없는 model blame으로 올라가는가?
  - golden에서 unrelated tool/command/field가 사라지는가?

  발견 사항은 main agent가 source/test로 재현한 뒤 수정하고, full final gate를 처음부터 다시 실행한다.

  **Conditional post-enforcement live proof — separate approval**:

  T6가 적용되었을 때만 affected host+fixture에 clean 10-completed batch를 다시 제안한다. 이 batch는 외부 비용 경계이므로 full local gate가 통과해도 자동 실행하지 않는다. 승인되면 expected outcomes는:

  - advertised schema hash가 new closed schema와 일치.
  - valid prompt call은 exact-valid.
  - promoted malformed replay는 normalized error/handler 0/state unchanged.
  - environment/transport unresolved 0.

  live proof가 없으면 완료 보고는 local/deterministic contract 완료와 live revalidation 미실행을 분리해 말한다. live를 실행했다면 report path와 host/version/model label을 제시한다.

  **Final acceptance criteria**:

  - [ ] full final gate 한 run 통과.
  - [ ] `git diff --check` clean, unexpected go.mod/go.sum/lockfile drift 없음.
  - [ ] evidence directory ignored, tracked regression은 confirmed cases만.
  - [ ] P1 entry condition과 실제 T6 수행 여부가 일치.
  - [ ] docs, CLI usage, response contract, golden, self-verify contract가 동일 semantics를 기술.
  - [ ] commit/push/PR은 사용자 요청 없이는 수행하지 않음.

  **Proposed final commit**: 검증 자체로 새 commit을 만들지 않는다. final fixes가 있으면 해당 owning task commit에 작은 follow-up Lore commit을 만들고 다시 검증한다.

## Rollback plan

- T1–T4/T7의 deterministic benchmark와 cause axis는 production tool-call semantics를 바꾸지 않으므로 독립 유지 가능하다.
- T6에서 host/tool compatibility regression이 나오면 T6 atomic commit만 revert한다. benchmark, confirmed fixture, cause taxonomy는 남겨 원인을 계속 재현한다.
- fixture는 재현 evidence가 잘못되었음이 확인될 때만 별도 commit으로 삭제하고 ADR/CAUTIONS에 false-positive 원인을 기록한다.
- user MCP/plugin registries에는 이 계획이 어떤 write도 하지 않으므로 host 설정 rollback 절차가 없어야 한다. 그런 diff가 생기면 구현이 계약을 위반한 것이다.

## Explicit assumptions and defaults

- 세 host executable은 구현 시점에도 PATH에서 발견되며, version drift는 report metadata로 기록한다. hard-coded version gate는 두지 않는다.
- Codex/Claude는 기존 login store를 CLI가 정상적으로 읽을 수 있다. 읽지 못하면 environment failure이며 credential 우회를 만들지 않는다.
- GJC는 isolated temp agent dir에서 사용할 explicit model + env/broker auth가 operator에게 있다. 없으면 GJC episode는 blocked이며 P1 gate는 inconclusive다.
- MCP catalog schema subset은 현재 확인한 7 keyword로 제한된다. 새 keyword는 baseline failure로 드러나며 validator가 조용히 무시하지 않는다.
- live model default는 host의 현재 실제 사용자 경험을 측정하기 위해 Codex/Claude `default`; GJC isolation은 config가 비어 있으므로 explicit model 필수다.
- 이 계획은 implementation authorization이 아니다. 코드 변경, live 외부 호출, commit/push는 각각 사용자 지시에 따른다.

## Unresolved questions

없음. 실행 시 필요한 GJC model id/auth env 이름은 secret이나 설계 결정이 아니라 operator runtime input이며, 없으면 이미 정의한 `isolated_auth_unavailable` 경로로 종료한다.
