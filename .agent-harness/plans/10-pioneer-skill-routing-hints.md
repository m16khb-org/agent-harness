# 10: 비-issueops 요청의 pioneer 스킬 라우팅 힌트

이슈: https://github.com/m16khb/agent-harness/issues/10
근거: 진단 보고서의 Activation Fidelity 공백 + 사용자 요청("웹 자료 수집 시 berners-lee가 잘 호출").

## Fix (rules.go 데이터 + 테스트)

- [ ] 1. `hookRoutingRules`에 9개 secondary 룰 추가 — 요청유형→스킬(영/한 키워드):
  berners-lee(웹 리서치/조사/출처), hopper(디버그/원인/flaky), dijkstra(최적화/복잡도/느려),
  codd(스키마/인덱스/쿼리/정규화), torvalds(rebase/bisect/충돌), atomic-commit-push(커밋/푸시),
  von-neumann(계획/플랜/설계해), shannon(품질 측정/슬롭/SNR), karpathy(프롬프트).
  turing은 광범위 키워드('검증') 과활성 위험으로 제외(Non-goal).
- [ ] 2. **TDD**: 테이블 테스트 — 대표 프롬프트("웹에서 자료 조사해줘"→berners-lee, "이 쿼리 인덱스 봐줘"→codd,
  "rebase 하다 충돌"→torvalds, …) 점화 + 무관 프롬프트("안녕") 미점화.

## Acceptance

- `go test ./internal/core/hookprompt/ -count=1` 그린(신규 테이블 테스트 포함).
- `go test ./... -count=1` 무회귀.
- 수동: `hook user-prompt --prompt "웹에서 자료 조사해줘" --json`에 berners-lee 힌트.

## Non-goals

스킬 description 변경, 차단 게이트, turing 키워드.
