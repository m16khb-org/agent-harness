---
title: "Dogfood the real write path before trusting an observation loop"
source: "session"
target_wiki: "dev-fundamentals"
target_type: "notes"
summary: "Two defects invisible to unit tests — a severity-convention mismatch and same-second state-key collisions — were caught only by driving the real CLI path; observation/feedback loops must be dogfooded end-to-end against the conventions of their actual producers."
suggester: "main-agent"
model: "claude-fable-5"
---

# 관측 루프는 실제 기록 경로로 dogfood해야 한다

2026-07-02 agent-harness에 Reflexion lesson 소비(self-augment planner의 후보 강등)를
추가할 때, 단위 테스트는 모두 green이었지만 실제 CLI 경로 dogfood가 결함 2건을 잡았다.

## 결함 1 — 소비자와 생산자의 어휘 불일치

- 소비 측(planner)은 severe severity를 `major/critical/blocker`로 정의했다.
- 생산 측(CLI `self-augment lesson`)의 문서화된 severity 관례는 `info|warning|error`였다.
- 결과: 실사용에서 severe lesson은 전부 `error`로 기록되므로 페널티가 **한 번도 발화하지
  않는** 기능이 될 뻔했다. 단위 테스트는 소비 측 어휘로만 fixture를 만들었기 때문에 통과했다.
- 교훈: 관측/피드백 루프를 붙일 때는 **생산자의 실제 값 관례**(enum, 기본값, 도움말 문서)를
  먼저 확인하고, 소비자 테스트 fixture에 그 관례 값을 포함하라.

## 결함 2 — 초 단위 키 granularity로 인한 조용한 기록 유실

- lesson state key가 `...-20060102T150405Z`(초 단위)여서 한 루프에서 연속 기록된
  두 lesson이 같은 키로 덮어써졌다.
- 소비 로직(threshold 2회)은 정확했지만 저장소가 조용히 1건을 삼켜 threshold 미달이 됐다.
- 교훈: append-only 의도의 레코드가 timestamp 키를 쓰면 **생성 빈도보다 세밀한
  granularity**(나노초 suffix 등)를 보장하라. 덮어쓰기는 에러 없이 성공하므로 테스트로만
  잡기 어렵다 — 연속 2회 기록 후 키가 서로 다른지 확인하는 회귀 테스트를 남겨라.

## 일반화

- 유닛 green ≠ 통합 정상. 관측 루프(기록→집계→소비)는 **실제 바이너리와 실제 기록
  경로**로 최소 1회 end-to-end를 관찰해야 한다(격리 `HARNESS_ROOT`/`HARNESS_STATE_DIR`로
  실환경 오염 없이 가능).
- 이 dogfood 패턴은 verification-grounding("실제 렌더러에서 관찰")의 CLI 판이다.

## 근거

- 수정 커밋: `cce5cdd`(severity 관례), `9f6b866`(키 충돌)
- 관찰: 격리 dogfood에서 96.66→66.66 강등 + advisory 경고 + 커리큘럼 회전 확인
