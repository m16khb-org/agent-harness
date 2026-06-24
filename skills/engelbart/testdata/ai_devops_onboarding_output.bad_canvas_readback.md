# 2026-06-24 [온보딩] AI DevOps R and R 및 추천 시스템 온보딩

::: {.callout}
회의일 2026-06-24 · 대상 #dev-team-backend · Source pasted transcript · Status Follow-up 필요
:::

## 메타데이터
| Field | Value |
|---|---|
| Date | 2026-06-24 |
| Status | Follow-up 필요 |
| Owner | 김현호 팀리더 |
| Last updated | 2026-06-24 |

## 회의록 인덱스 항목
| 이름 | Date | Topic | Status | Counts | Meeting Canvas |
|---|---|---|---|---|---|
| AI DevOps R&R 및 추천 시스템 온보딩 | 2026-06-24 | 온보딩 | Follow-up 필요 | 결정 9 / 액션 10 / 질문 6 | 생성 후 인덱스 참조 |

## TL;DR
- 팀 R&R과 추천 시스템 온보딩을 다뤘다.
- 후속 확인이 필요하다.

## 결정사항
- 팀 R&R을 백엔드 아키텍처 설계 및 구현, 인텔리전트 인프라 운영, AI 에이전트 개발 및 서비스 융합 세 축으로 정리한다. 근거: 00:00-02:28. 영향: 백엔드 팀 미션과 업무 배분. 결정자/동의자: 김현호 팀리더. 상태: 확정.

## 액션 보드
- [ ] 이푸름 님: Weave Unlimited 대응을 1차 진행한다. 기한: 다음 주 출시 전. Tracking: GitLab Issue.

## 주제별 논의
### Recommendation System
- 추천 시스템 구조를 논의했다.

## 후속 확인
- Recommendation target architecture 확인. 담당: 참석자 1. 확인 위치: Recommendation 문서/GitLab issue. 기한: 미정.

## 리스크/열린 질문
- **Recommendation target architecture**: trainer/serving 분리, NCP Object Storage 저장, cache 도입 시점을 확정해야 한다. 확인 담당: 참석자 1. 확인 방법: Recommendation 문서/GitLab issue. 상태: 확인 필요.
- **Agent 반영 강제화**: reward/affiliate 같은 큰 정책 변경 시 agent 문서/지식 반영을 강제하는 프로세스가 필요할 수 있다. 확인 담당: 참석자 1. 확인 방법: 개발 프로세스 논의. 상태: 검토 필요.

---

## 보정 및 원문 부록

### 원문 전사본 전문
```text
참석자 1 00:00
그래서 그거에 대한 이제 배경 설명을 좀 하고...
```
