# 이슈 #118 — claude implementer 기본 모델을 opus 5로 갱신

이슈: https://github.com/m16khb/agent-harness/issues/118

## 변경

- `internal/port/orca.go`의 `IssueOpsImplementerModelClaude`를 `"claude-opus-4-8"` → `"claude-opus-5"` 단일 상수 변경. 주석에 #116 실측 근거(CLI가 4.8을 Opus 5로 폴백 해석 — 봉인값·구동 모델 불일치) 기록.
- `claude-opus-4-8` 참조 전수(rg)를 확인해 테스트·golden 동반 갱신.

## AC

- AC-01: `IssueOpsImplementerDefaults("claude")` == (`claude-opus-5`, `high`).
- AC-02: 전체 테스트 green.

## 비범위

- effort·codex·planner 상수, launch 경로.
