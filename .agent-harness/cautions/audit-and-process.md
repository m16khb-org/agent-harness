---
name: cautions/audit-and-process.md
description: Cautions for self-verify/augment loops, stability-audit CLI contracts, QA process discipline, and cross-process test helpers.
---

# Stability-audit, self-verify, and QA-process cautions

Family index: [CAUTIONS.md](../CAUTIONS.md). Evergreen hazards for self-verify
and self-augment loops, stability-audit CLI contracts, JSON/QA process
discipline, and cross-process test helpers. Removed self-verify CLI modes are
preserved as dated history under [lessons/](lessons/).

## 10. 자기 검증/자가 증강 drift

자기 검증 루프가 실제 native integration과 QA gate를 검증하지 않으면 문서만 통과하는 가짜 안정성이 생긴다. 자가 증강 루프가 실제 diff를 만들지 않으면 단순 분석 루프로 퇴화한다.

주의:
- 새 CLI/MCP/native skill 기능은 `agent-harness self-verify`의 테스트 또는 QA 단계에 smoke/fuzz evidence label로 승격한다.
- 반복 횟수 10회 하한을 임의로 낮추지 않는다.
- temp git repo 외 실제 사용자 repo에서 commit/push를 수행하지 않는다.
- 교정 후보의 `VerifyWith`는 모델 자기비판이 아니라 외부 검증 메커니즘을 명시해야 한다(`VerificationKind`로 분류, `qualitycatalog.VerifyWithGrounded`가 강제). 불변식 전문은 `CONVENTIONS.md` §9 "self-augment/self-verify 교정 가드레일". intrinsic self-correction은 외부 신호 없이 추론을 악화시킨다(Huang/Kamoi).

## 18. Audit harness flags must match the CLI contract

A stability-audit failure is not automatically a harness defect; the audit framework itself can call the CLI with invalid flags.

주의:
- `self-verify --iterations=N requires --full` 및 `self-verify --full --iterations=10`(10개 seeded deterministic iteration, >=180s/~3712s budget): **해당 CLI mode는 2026-08-11에 제거됐다.** 현재 operational command로 쓰지 않는다. 역사적 기록과 관측치는 [2026-08-11 — self-verify `--full`/`--iterations` modes removed](lessons/2026-08-11-self-verify-iterations-full-modes-removed.md)에, 현재 `self-verify` 동작은 testing family의 self-verification module(`testing/self-verification.md`)을 본다.
- When an audit step fails suspiciously fast, reproduce the exact invocation directly and compare against the documented commands in `.agent-harness/OPERATIONS.md` / root `AGENTS.md` before concluding the harness is unstable.
- `HARNESS_SELF_VERIFY_LLM_EVAL=gate` is a valid ambient runtime configuration, but the current self-verify implementation only renders the read-only evaluator prompt. It sends no Z.AI request and ingests no external verdict, so `gate` intentionally returns a non-passing `llm_eval` result. Do not diagnose that result as environment drift or claim an external judgment occurred. Repository completion gates must use explicit `--llm-eval=false`, record the override, and restart from the first gate after any interrupted or prompt-only run.
- Handoff focused tests must use `./cmd/harness/hookcli/hookinput`; the plausible-looking `./internal/core/hookinput` path does not exist and causes a command-spec failure after other packages have already started. Pin the full focused command in `.agent-harness/TESTING.md` and restart the sequence rather than reusing partial results.
- `orca orchestration send --type` rejects values outside `status|dispatch|worker_done|merge_ready|escalation|handoff|decision_gate|heartbeat`. Verify the installed CLI when this enum changes; do not improvise `progress`, `blocked`, or `completed` message types.

## 22. Stability audit smoke tests must track current host/MCP contracts

The stability audit can false-fail when its smoke assumptions lag the harness contract.

주의:
- `agent-harness mcp` defaults to the daemon-backed proxy. Newline-delimited JSON-RPC smoke tests that expect direct stdout responses must set `HARNESS_MCP_DIRECT=1`, matching `validationli/mcpsmoke`. Otherwise stdout can be empty while the proxy path exits successfully, producing a false `mcp_ids=[]` failure.
- UserPromptSubmit currently injects compact per-turn `[agent-harness]` bullet context. The audit must reject old/noisy catalog injection markers such as `Required project docs` or `필수 프롬프트 주입중`, but must not fail merely because compact context contains newlines.
- `self-verify promote --confirm` refuses failed snapshots by default. Validation fixtures that intentionally promote a non-termination-eligible snapshot for state-roundtrip coverage must pass `--allow-failed-source`; production baseline promotion should not use that override.
- Pin these assumptions in `skills/stability-audit/scripts/e2e_stability_audit_test.py` and `cmd/harness/validationcli/stateroundtrip` tests before changing the audit script.

## Stability audit 명령과 timeout을 현재 공개 계약·측정치에 맞출 것

- top-level install audit에 과거 `bootstrap --sync`를 남기지 않는다. 현재 install 표면은 `bootstrap`/`install-native`; docs sync는 `project bootstrap --sync`다.
- live 정합성 gate인 `operational_doctor`는 상위 live harness 환경을 그대로 사용해야 한다. 반대로 audit 내부 ordinary/race `go test`는 `HARNESS_ROOT`를 exact audited source checkout으로 고정하고 `HARNESS_STATE_DIR`, `HARNESS_DAEMON_DIR`, `HARNESS_WORKER_DIR`를 audit 전용 임시 루트로 격리한다. live 환경으로 회귀 테스트를 실행하면 성공한 테스트가 IssueOps session row를 다시 만들어 최종 정리가 영구히 종료되지 않으며, `HARNESS_ROOT`를 빈 임시 경로로 바꾸면 source identity를 잃어 정상 회귀 검사가 실패한다.
- full repository test timeout은 가장 느린 정상 package와 race의 관측 상한보다 커야 한다. 현재 regression timeout은 300초다.
- `self-verify --full --iterations=10` 매 seed test/race 실행 및 3712초/5400초 audit timeout: **해당 CLI mode는 2026-08-11에 제거됐다.** 현재 operational command로 쓰지 않는다. 역사적 기록은 [2026-08-11 — self-verify `--full`/`--iterations` modes removed](lessons/2026-08-11-self-verify-iterations-full-modes-removed.md), 현재 동작은 testing family의 `testing/self-verification.md`를 본다.
- timeout 실패는 마지막 성공 package, elapsed time, 살아 있는 child command를 확인해 hang과 짧은 wrapper 상한을 구분한다.
- 장기 self-verify가 nonzero 또는 JSON parse 실패하면 audit report에 exit code, timeout 여부, parse error, parsed 종료 필드, bounded stdout/stderr tail을 남긴다. `summary: null`만 남기면 제품 실패와 audit 해석 실패를 구분할 수 없다.
- JSON parse를 `returncode == 0` 분기 안에 두지 않는다. nonzero가 바로 구조화된 실패 summary를 보존해야 하는 경우이며, parse와 성공 판정은 별도 단계다.
- deterministic stability gate에서 `self-verify`를 호출할 때는 `--llm-eval=false`를 명시한다. 코드 250단계가 모두 성공해도 암묵적 LLM gate의 외부 실패가 command exit를 뒤집을 수 있다.

## 리뷰 finding의 production fix를 named RED보다 먼저 적용하지 말 것

리뷰가 구체적인 재현을 제공해도 그것은 코드 변경 전 named failing-test transcript를 대신하지 않는다. 먼저 regression test만 추가하고 exact test name이 `RUN` 뒤 의도한 assertion으로 FAIL하는 terminal exit를 기록한 다음 production code를 수정한다. 순서를 어겼다면 RED를 소급 합성하거나 RED→GREEN이라 부르지 말고 `RED skipped` process defect로 기록하며, reviewer repro는 별도 pre-fix evidence로만 남긴다.

## JSON 검증 wrapper에서 추측한 필드를 성공 조건으로 쓰지 말 것

`jq`는 존재하지 않는 필드를 `null`로 평가하므로 `.passed == true` 같은 잘못된 predicate가 schema error가 아니라 정상적인 `false`와 exit 1로 나타난다. 이 때문에 성공한 장기 self-verify를 제품 실패로 오인하고 전체 검증 파동을 반복한 사고가 있었다.

- 장기 명령의 JSON 성공 조건은 실행 전에 DTO의 실제 JSON tag 또는 response contract golden에서 확인한다.
- `self-verify`의 종료 조건은 top-level `.ok`, `.termination_eligible`, `.summary.termination_eligible`이며 top-level `.passed`가 아니다.
- wrapper 실패는 제품 명령의 raw exit와 JSON artifact를 보존해 product failure와 orchestration/predicate failure로 분리한다.

## 긴 QA 파동을 interactive `tmux send-keys` 한 줄로 주입하지 말 것

긴 ZLE 입력을 여러 `send-keys` chunk로 빠르게 붙이면 중간 suffix가 유실되어 테스트 selector와 다음 명령이 결합될 수 있다. 실행은 계속되고 일부 명령은 exit 0을 반환하므로 단계 marker가 없으면 false-green이 된다.

- 긴 검증 파동은 reviewable temp script로 고정하고 tmux에는 script path 한 줄만 전달한다.
- script는 `set -euo pipefail`을 사용하고 각 필수 단계 전후 marker를 남긴다.
- named test 출력의 `[no tests to run]`, 누락된 marker, 예상하지 않은 결합 token은 파동 실패다.
- temp script와 JSON artifact는 terminal exit를 회수한 뒤 삭제하고 process/session 부재를 확인한다.

## cross-process 테스트에서 helper 시작·종료 오류를 버리지 말 것

두 helper의 `exec.Cmd.Run` error와 helper 내부 초기 read error를 버린 결과, provider 호출 횟수 파일이 없다는 2차 증상만 남고 실제 실패 원인이 사라진 적이 있다.

- 여러 helper를 동시에 검증할 때는 각 process를 순서대로 `Start`해 launch를 확인한 뒤 모두 `Wait`하여 동시 실행과 진단을 함께 보장한다.
- stdout/stderr와 exit error를 helper별로 보존하고, 승자·차단자처럼 기대하는 서로 다른 결과를 명시적 marker로 검증한다.
- count/result artifact 부재만 보고 production concurrency defect로 분류하지 않는다. helper launch/read/create 경계를 먼저 확인한다.
