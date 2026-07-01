# Deferred follow-ups (pick up when the trigger is observed)

작은 backlog 노트. 지금 착수할 일이 아니라, 명시된 **트리거가 실제로 관찰될 때만** 여는 항목을 모은다.

## DF-1 — capsule-first tool-error re-surfacing (12-factor #3/#9 H2)

- **상태**: deferred. 정적 넛지 최소구현은 KILL됨(아래 근거).
- **근거/결정**: `.agent-harness/ADR.md` "Defer harness-side tool-error context injection" (+ KILL verdict), 스파이크 `.agent-harness/research/spike-tool-error-nudge.md`, 커밋 c3e68b6 / 3685ee7.
- **왜 KILL됐나**: subagent A/B에서 미해결 에러가 트랜스크립트에 인라인으로 있으면 정적 넛지의 한계효과 0(control 3/3 = treatment 3/3). 호스트 인라인 표시와 중복.
- **남은 가설(H2)**: 넛지의 유일한 비중복 가치는 compaction으로 밀려난 에러의 재부상. 이는 정적 넛지가 아니라 **compaction-survival capsule**(Brooks가 v1에서 잘라낸 것)을 요구 → capsule-first의 다른 기능.
- **착수 트리거**: 실제 장기 dogfood에서 "compaction 이후 에이전트가 이전 미해결 tool 에러를 잊고 후속조치를 누락"하는 사례가 반복 관찰될 때.
- **착수 시 방향**: PostToolUse 관찰-only(§16.1), Claude `systemMessage` 전용(§14), `LifecycleCompactCapsule`에 미해결 에러 보존→재주입, resolved는 결정적(문자열 동등 success)만. 착수 전 별도 ADR + Brooks 리뷰. 검증은 cross-compaction 행동 delta로만(H1 라이브 컨텍스트만으로 GO 금지).
- **원격 이슈 메모**: GitHub 이슈로 트래킹하려 했으나 auto-mode 분류기가 외부 쓰기를 차단. 원하면 사용자가 직접 `gh issue create`(label+assignee 필수)로 등록.
