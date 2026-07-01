# Spike: tool-error nudge (item ②) — measure before build

- Source: 12-factor #3/#9 적용 조사 + Brooks devil's-advocate 리뷰 (ADR 2026-07-01 "Defer harness-side tool-error context injection")
- Instrument: `cmd/harness/hookcli/hook_user_prompt.go` — `spikeErrorNudgeUserView` (env-gated, Claude `systemMessage` only)
- Status: instrument ready + smoke-verified in the real renderer; decisive measurement is a real multi-session dogfood (below).

## 왜 스파이크인가 (기준선 문제)

현재 하네스는 PostToolUse에서 `tool_response`를 파싱하지 않아 **에러 주입 바이트 = 0** (`hook_lifecycle.go:33-35`). 따라서 에러 digest 기능은 "토큰 절약"이 아니라 오늘 대비 컨텍스트 **순증가**다. 무언가를 짓기 전에, 정적 넛지가 에이전트 행동을 실제로 개선하는지부터 falsify한다. 개선이 없으면 store/queue/capsule/signature 전체가 우발적 복잡도다.

## 계측기 (instrument)

- 환경변수 `HARNESS_SPIKE_ERROR_NUDGE`가 설정되고 `--host claude`일 때만, 그 값을 `systemMessage`로 노출한다.
- persistence·parsing 없음: 순수 정적 넛지. env 미설정 시 hook 출력 계약은 **완전히 불변**(golden 무변화).
- Codex `additionalContext`에는 절대 주입하지 않는다(§14 — 사용자 노출 산문 경계, Codex TUI는 이미 tool 에러 표시).

실측(real renderer) 검증 완료:
- env unset + claude → `systemMessage` 없음 (inert)
- env set + claude → `systemMessage: "[agent-harness spike] ..."` 노출
- env set + codex → `systemMessage` 없음, `additionalContext` 불변

## 두 가지 가설

- **H1 (게이트 테스트, 값싼 falsify)**: 컨텍스트에 정적 에러 넛지가 있으면 에이전트 행동이 바뀐다. → 넛지가 아무 delta도 안 내면 즉시 중단, 아무것도 짓지 않는다.
- **H2 (실제 비중복 가치)**: 호스트(Claude/Codex)는 tool 에러를 이미 인라인 표시하므로, 라이브 컨텍스트에 있는 에러의 재진술은 **중복**이다. 넛지가 비중복 가치를 갖는 유일한 경우는 **compaction으로 컨텍스트에서 밀려난 에러를 재부상**시키는 것이다. H2가 참이면 이 기능의 본질 가치는 "cross-compaction persistence"(brooks가 v1에서 잘라낸 capsule)로 재정의된다 — 즉 정적 넛지 최소구현이 아니라 capsule이 핵심이 된다.

## 측정 프로토콜 (dogfood, 실제 세션)

정적 넛지로는 H1만 게이트한다. 실제 장기 세션에서:

1. **treatment**: 실제 작업 세션에서 tool 에러가 발생한 뒤 `HARNESS_SPIKE_ERROR_NUDGE='errors: N unresolved (tool: summary)'`를 설정(에러 요약을 수기로 넣어 넛지 노출).
2. **control**: 동일 성격 세션에서 env 미설정.
3. **관측 지표**(행동 delta):
   - 에이전트가 미해결 에러를 **자발적으로** 다시 다루는가 (사용자 재지시 없이).
   - compaction 이후에도 그 에러를 기억/재시도하는가 (H2 핵심 조건).
   - 넛지가 **노이즈/혼란**을 유발하지는 않는가 (이미 해결된 에러를 계속 물고 늘어지는 등).
4. **표본**: 최소 5개 실제 세션/arm. 단발 관측은 결론 금지.

## Kill / Go 기준

- **KILL(아무것도 안 짓기)**: H1 delta 없음 — 정적 넛지가 행동을 안 바꾸거나 노이즈만 늘림.
- **재정의(H2로 축소된 build)**: H1은 delta 있으나 **오직 cross-compaction 조건에서만** 가치 → 최소 구현의 핵심은 정적 카운트가 아니라 compaction-survival capsule. 이 경우 별도 ADR로 capsule-first 재설계.
- **최소 GO**: H1 delta가 라이브 컨텍스트에서도 뚜렷 → 세션-한정 카운트 + 문자열 동등 success-resolve만 구현(JSONL store·signature 해시 없음).

## 한계 (정직성)

- 이 계측기는 **정적** 넛지라 H1 게이트만 값싸게 판정한다. H2(cross-compaction)의 결정적 측정은 실제 compaction을 겪는 장기 Claude Code 세션이 필요하며, 단일 자율 패스에서 재현 불가.
- 넛지 텍스트는 현재 수기 주입이다(에러 자동 캡처 없음). 자동 캡처는 GO 판정 이후에만 검토한다.

## 프로브 결과 (2026-07-01, subagent A/B)

H1 게이트를 값싼 subagent A/B로 1차 판정했다. 양쪽 arm 모두 동일 트랜스크립트(미해결 빌드 에러 `undefined: roundHalfUp`가 인라인으로 존재)를 받고, treatment에만 `[agent-harness spike] errors: 1 unresolved (...)` 넛지 라인을 추가. "에러를 고치라"는 지시 없이 중립적으로 "세션 요약 + 다음 할 일 3가지(우선순위 순)"만 요청.

- **control 3/3**: 미해결 빌드 에러를 "다음 할 일 #1"로 최우선 배치.
- **treatment 3/3**: 동일하게 #1 배치.
- **한계효과(marginal delta) = 0**. 인라인 에러가 컨텍스트에 있으면 정적 넛지는 행동을 바꾸지 않는다 → brooks의 "라이브 컨텍스트에서 중복" 가설 확증.

교란요인/한계: (a) 6표본 소규모, (b) subagent는 compaction을 겪지 않아 H2(cross-compaction 재부상)는 **미검증**, (c) 짧은 트랜스크립트라 에러가 눈에 잘 띈다(실제 긴 세션에서 에러가 스크롤로 밀리는 조건은 재현 못 함). 따라서 이 프로브는 H1(라이브 컨텍스트 중복성)만 판정한다.

## Verdict: KILL (정적 넛지 최소구현), H2는 별건

- **KILL**: 라이브 컨텍스트에서 정적 에러 넛지는 호스트 인라인 표시와 중복이라 행동 delta가 없다. 세션-한정 카운트 등 "최소 구현"조차 정당화되지 않는다.
- **H2는 분리된 가설로 남긴다**: 넛지가 비중복 가치를 가지려면 오직 compaction으로 밀려난 에러를 재부상시키는 경우뿐이며, 이는 정적 넛지가 아니라 compaction-survival capsule(brooks가 v1에서 잘라낸 것)을 요구한다. 즉 최소-넛지 build가 아니라 **capsule-first**의 다른 기능이다. 실제 장기 세션에서 이 통증이 관찰될 때 별도 ADR로 재설계한다.

## 정리 (스파이크 종료)

verdict가 KILL이므로 계측기 `spikeErrorNudgeUserView`는 제거 대상이다(이 문서와 ADR이 결정 기록으로 남는다). H2 재설계는 실제 dogfood에서 통증이 관찰될 때만 착수한다.
