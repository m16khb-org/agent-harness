---
name: cautions/lessons/2026-08-29-omo-pr-review-structured-output-tool.md
description: Dated lesson — Omo Parnas verdicts need a schema-constrained submit tool and an enforced final turn.
---

# 2026-08-29 — Omo Parnas verdict는 최종 텍스트가 아니라 schema tool로 받아야 한다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: Omo Parnas degraded run audit and local `zai/glm-5.3-flash` reproduction
- Summary: 18개 후보 중 14개가 필요한 두 verdict를 만들지 못한 실행에서, 검증 작업 자체보다 최종 assistant 텍스트 부재·잘린 비JSON 응답 때문에 결과를 폐기했다. prompt-only JSON 계약과 stdout tail 기반 재시도는 조사 비용을 보존하지 못한다.
- Context: 기존 `omo_driver.py`는 새 세션의 포맷 재시도에 원래 prompt/candidate 대신 이전 stdout tail만 전달했고, parse failure와 confidence 40 이하의 정상 abstention을 모두 `agent_failures`로 합쳤다. Omo custom tool은 JSON Schema와 `constrainedSampling: {type: "json_schema", strict: "require"}`를 지원하지만, `read-only`/`workspace` preset만으로는 custom submit tool 실행이 허용되지 않는다. 실제 smoke에서 `submit_parnas_verdict`가 permission denial로 거부됐고 `--permission submit_parnas_verdict=allow`를 추가한 뒤 schema-valid 인자가 session log에 기록됐다. 프롬프트의 턴 예산 문구만으로는 긴 조사 루프를 끊지 못하므로 확장이 `turn_end`를 세어 마지막 턴 전에 조사 도구를 비활성화해야 한다.
- Resolution: (1) finder/verdict 최종 결과는 strict schema의 `submit_parnas_finder`/`submit_parnas_verdict` tool arguments를 권위 있는 값으로 사용한다. (2) submit tool을 명시적으로 allow하고 역할별 tool allowlist만 노출한다. (3) 마지막 턴 직전에 submit tool만 남긴다. (4) 포맷 재시도에는 원래 prompt/candidate를 다시 주고 submit tool 외 도구를 끈다. (5) raw stdout/stderr, return code, timeout, tool denial, parse/schema error를 `agent_diagnostics`에 보존하고 `parse_failure`, `schema_failure`, `timeout`, `low_confidence_abstain`을 분리한다. (6) confidence가 높아도 reason 어디에든 `미확인:`이 있으면 abstain으로 분류한다. (7) 이전 abstention만 재실행할 때 `--phase verify --retry-degraded-from <workflow-result.json>`을 사용하고 원본 결과는 덮어쓰지 않는다.
- Evidence:
  - `skills/pr-review/extensions/structured_output.js`: strict schema submit tools와 강제 final-turn 전환
  - `skills/pr-review/scripts/omo_driver.py`: session tool payload 회수, 원문 재시도, 진단 및 실패 카운터, degraded-only retry
  - 실제 Omo smoke: tracer submit tool 회수 성공, reproducer가 foreground 회귀 테스트 1건 통과 후 schema-valid verdict 제출
  - `python3 -m unittest skills.pr-review.tests.test_omo_driver -v`: 40 tests passed
- Alternatives / rejected options:
  - `--mode json` 이벤트 스트림만 파싱 — 이벤트 운반 형식일 뿐 verdict schema를 강제하지 않아 거부.
  - Markdown fence 제거와 JSON substring 추출만 강화 — 최종 assistant 텍스트가 0건인 실행을 복구하지 못해 거부.
  - 저신뢰 non-refutation을 agent failure로 유지 — 정상 abstention과 파이프라인 고장을 구분할 수 없어 거부.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
