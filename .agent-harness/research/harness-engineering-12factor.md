# Research: 12-Factor Agents & 우아한형제들 하네스 엔지니어링 — agent-harness 적용 관점

## TL;DR
**Conclusion**: 두 소스는 상호보완적이다. 12-factor-agents는 "에이전트 = deterministic 코드 + 전략적 LLM 스텝"이라는 *런타임 아키텍처* 원칙을, 우아한형제들 글은 "팀 규칙/컨텍스트를 어떻게 주입할 것인가"라는 *컨텍스트 엔지니어링* 실천을 제시한다.
**Confidence**: High (둘 다 1차 권위 소스 — 공식 GitHub repo, 공식 techblog)
**Sources**: 4개 문서 (12-factor README + factor-08 + factor-03, woowahan 26177)

## Method
- 검색 각도: (1) 12-factor 공식 README, (2) 하네스에 직결되는 factor-08/03 원문, (3) 우아한형제들 techblog 원문
- Fetch: raw.githubusercontent.com (권위=공식), techblog.woowahan.com (권위=공식 기업 블로그)
- 성격: 특정 문서 요약형 조사(분쟁 사실 검증이 아님) → 각 주장은 원문 1차 소스로 단일 권위 인용

---

## 소스 1: humanlayer/12-factor-agents

### (a) 핵심 원칙 리스트 (12 factors, 원문 제목)
1. **Natural Language to Tool Calls** — LLM은 다음 단계를 구조화된 JSON tool call로 출력한다.
2. **Own your prompts** — 프롬프트를 프레임워크에 위임하지 말고 직접 소유·제어한다.
3. **Own your context window** — 컨텍스트에 무엇이 들어가는지 능동적으로 관리한다(수동 누적 금지).
4. **Tools are just structured outputs** — tool은 특별한 프레임워크 추상이 아니라 구조화된 출력 스키마일 뿐.
5. **Unify execution state and business state** — 에이전트 실행 상태와 애플리케이션 비즈니스 상태를 하나로 동기화.
6. **Launch/Pause/Resume with simple APIs** — 시작·일시정지·재개를 단순 API로 지원.
7. **Contact humans with tool calls** — 사람 개입/승인 요청도 동일한 tool-call 메커니즘으로.
8. **Own your control flow** — 프레임워크 루프에 의존하지 말고 제어 흐름을 명시적으로 작성.
9. **Compact Errors into Context Window** — 에러를 압축해 컨텍스트 공간을 절약, 해결된 에러는 숨김.
10. **Small, Focused Agents** — 모든 것을 하는 모놀리식 대신 좁은 범위의 전문 에이전트.
11. **Trigger from anywhere, meet users where they are** — 다양한 소스에서 트리거, 사용자 채널에 통합.
12. **Make your agent a stateless reducer** — 에이전트를 `(state, event) → state` 순수 변환으로 설계해 replay·복구 가능.

**핵심 테제(원문)**: "Agents, at least the good ones" = "mostly just software" with "LLM steps sprinkled in at just the right points." 순수 agentic 루프가 아니라, deterministic 코드 사이에 전략적 지점에서만 LLM 추론을 끼워넣는다.

### (b) 구체적 실천 방법 (하네스 직결 factor 원문 발췌)
- **Factor 8 (제어 흐름 소유)**: tool 선택 후 *실행 전*에 개입한다 — "interrupt a working agent and resume later, ESPECIALLY between the moment of tool selection and the moment of tool invocation." tool 종류별 3분기:
  - 동기(저위험) tool → 즉시 실행 후 결과 반환
  - 비동기(사람 입력 필요) tool → 루프를 끊고 state 저장, webhook으로 재개
  - 고위험 tool → 루프를 끊고 사람 승인 요청
  - 직접 구현할 것: summarization/caching, LLM-as-judge, context 관리, logging/tracing, rate limiting, durable pause/resume.
- **Factor 3 (컨텍스트 소유)**: 표준 role 기반 message 배열 대신 **typed event 시스템**으로 구성. `event_to_prompt(event) → f"<{event.type}>\n{yaml}\n</{event.type}>"` 형태로 XML/YAML 직렬화. 이점 5가지: Information Density, Error Handling, Safety, Flexibility, Token Efficiency. "hide errors and failed calls from context window once they are resolved."
- **Stateless reducer**: `(input state, event) → new state`, side effect 없음 → deterministic replay·테스트·실패복구 용이.

### (c) 출처
- https://github.com/humanlayer/12-factor-agents (README) — retrieved 2026-07-01, 권위=High(공식 repo)
- https://raw.githubusercontent.com/humanlayer/12-factor-agents/main/content/factor-08-own-your-control-flow.md — 2026-07-01
- https://raw.githubusercontent.com/humanlayer/12-factor-agents/main/content/factor-03-own-your-context-window.md — 2026-07-01

---

## 소스 2: 우아한형제들 — "하네스 엔지니어링으로 팀 맞춤형 AI 코딩 환경 구축하기" (이재홍)

### (a) 핵심 원칙 리스트
- **하네스 엔지니어링 정의(원문)**: "AI가 길을 잃지 않고 안정적으로 일할 수 있도록 외부 통제 환경을 구축하는 것." 개인 프롬프트 기술 < 팀 차원 체계적 구조 설계.
- **Rules vs Skills 역할 분담**:
  - Rules = "코드 작성 규칙 전달" (프로젝트 코딩 컨벤션·아키텍처 패턴 정의)
  - Skills = "작업 자동화" (스크립트/CLI 실행으로 실제 데이터 처리·워크플로 자동화)
- **컨텍스트 비대화 방지 기준**: LLM이 이미 아는 기본 지식(React/TS 문법)은 제외, **프로젝트 특화 규칙만** 포함.

### (b) 구체적 실천 방법
- **폴더 구조**: `.cursor/rules/` + `.cursor/skills/`
- **Rules 작성**: YAML 형식 + `globs` 옵션으로 경로 제한 → AI가 해당 경로 작업 시 자동 적용(alwaysApply/경로 기반 트리거). 예: "상태관리는 React Query 커스텀 훅, API 호출은 API Class 경유, Query Key 중앙 관리".
- **전처리 스크립트 패턴(컨텍스트 최적화)**: AI 자율 탐색 대신 Node.js 스크립트가 메타데이터만 JSON으로 요약 전달 → **데이터량 평균 96.5% 절감**, 환각 방지·응답속도 개선.
- **Before/After**: (Before) 규칙 설명→생성→오류 지적→재요청 반복 / (After) "user 조회 훅 만들어줘" 한 줄로 팀 규칙 자동 준수.

### (c) 출처
- https://techblog.woowahan.com/26177/ — retrieved 2026-07-01, 권위=High(공식 기업 techblog)

---

## Cross-Check / 한계
- 두 소스 모두 **1차 권위 소스**로 자체가 근거(요약형 조사). 분쟁 주장 없음.
- "96.5% 절감", "stateless reducer" 등 수치·정의는 각 원문 표현을 그대로 인용(단일 권위 소스).
- 우아한형제들 글의 원 URL은 26287이 아니라 **26177**로 확인됨(26287은 오탈자/추정).

## Open Questions (적용 분석 단계로 이월)
- agent-harness에 이 원칙들이 이미 구현된 정도는 별도 코드베이스 매핑(진행 중 Explore 에이전트) 결과와 대조 필요.
