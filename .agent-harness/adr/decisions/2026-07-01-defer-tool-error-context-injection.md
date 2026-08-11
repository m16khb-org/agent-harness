# 2026-07-01 — Defer harness-side tool-error context injection; spike a nudge before building persistence

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: 12-factor-agents #3/#9 (own your context window / compact errors) 적용 조사, Brooks devil's-advocate 리뷰
- Summary: PostToolUse가 tool 에러를 구조화 이벤트로 압축·주입하는 기능(신규 `ToolFailureEvent` store/queue/signature)은 **빌드하지 않는다**. 먼저 Claude `systemMessage`에 수기 에러 넛지를 넣어 에이전트 행동 개선을 측정하는 스파이크를 거친 뒤 최소 구현 여부를 결정한다.
- Context: 현재 PostToolUse는 `tool_response`를 파싱하지 않아 하네스가 주입하는 에러 바이트는 0이다(`cmd/harness/hookcli/hook_lifecycle.go:33-35`; 유일한 feedback은 B3 lint-gate `:60-72`). "에러 압축으로 토큰 절약"의 기준선이 존재하지 않으므로, 기능은 오늘 대비 컨텍스트를 순증가만 시킨다. 또한 12-factor #3은 메시지 배열을 소유하는 커스텀 루프용 원칙인데, 하네스는 컨텍스트를 소유하지 않는 hook guest이고 호스트(Claude/Codex)가 이미 tool 에러를 인라인 표시한다. 계획이 "재사용할 기존 인프라"로 지목한 docupkeep `Resolve()`는 실제로 존재하지 않아(write-side resolve/dedup은 전부 신규) 비용이 과소평가돼 있었다.
- Decision: (1) v1에서 `ToolFailureEvent` store/JSONL queue/cross-turn capsule/signature 해시를 만들지 않는다. (2) 검증 순서는 스파이크-먼저 — Claude `systemMessage`에만(Codex `additionalContext` 금지, §14) `- errors: N unresolved (tool: summary)` 넛지를 수기 주입해 행동 delta를 측정한다. (3) 행동 개선이 확인되면 최소 구현(세션 한정 카운트, 문자열 동등 기반 success-resolve, persistent store 없음)만 검토한다. 확인 안 되면 폐기한다.
- Consequences: 지금은 코드 변경이 없다. 성공 지표는 "기능 내부 fold 바이트 비교"가 아니라 "에이전트 행동 개선"으로 정의한다. `hook_lifecycle.go` PostToolUse boundary(§16.1 관찰-only, 대행 금지)는 유지된다.
- Verdict (2026-07-01 spike): **KILL** — 정적 넛지 계측기(`spikeErrorNudgeUserView`, env `HARNESS_SPIKE_ERROR_NUDGE`, Claude systemMessage 전용)를 붙여 subagent A/B로 H1을 판정. 양쪽 arm 모두 인라인 미해결 빌드 에러를 "다음 할 일 #1"로 배치해 **한계효과 0**(control 3/3 = treatment 3/3). 라이브 컨텍스트에서 넛지는 호스트 인라인 표시와 중복이라 최소 구현조차 정당화 안 됨. 유일한 비중복 가치인 cross-compaction 재부상(H2)은 정적 넛지가 아니라 capsule-first를 요구하는 별건으로, 실제 장기 세션에서 통증이 관찰될 때만 착수. 상세: `.agent-harness/research/spike-tool-error-nudge.md`.
- Alternatives / rejected options:
  - 계획대로 store/queue/signature/capsule 빌드 — rejected: 존재하지 않는 기준선 대비 토큰 절약을 주장했고, resolve 인프라 신규 비용을 과소평가했으며, 아키텍처적으로 "own the context"를 소유하지 않는 hook에 이식하는 우발적 복잡도.
  - Codex `additionalContext`에도 에러 digest 주입 — rejected: §14(사용자 노출 산문 경계) + Codex TUI가 이미 tool 에러 표시 → 중복 노이즈.
  - resolved 판정에 "N턴 미재발" 규칙 추가 — rejected: wall-clock/turn-count 도입으로 결정성 훼손.
