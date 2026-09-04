# 2026-07-24 — IssueOps 이원 구조 최적화 (planner/implementer, 이슈 #78)

← [ADR index](../../ADR.md)

- **결정**: 메인 세션은 계획 전용(planner급 모델), 구현은 병렬 격리 워크트리의
  하위 세션(implementer급 모델)이 수행하는 이원 운영 모델을 execution v1 위에
  완성했다. host별 역할 모델 기본값(codex sol/terra, claude fable5/opus4.8)은
  코드가 소유한다(`internal/port/orca.go`).
- **artifact 전달**: 훅 allowlist 개방 대신 CLI 소유 표면(`artifact stage` →
  prepare materialize → orca packet manifest 봉인 → claim 검증). 훅 게이트는 어떤
  것도 완화하지 않는다(design-review 1차 critical 반영).
- **머지 후 정리**: `cleanup finish`가 orca→git 순 멱등 정리 후 레코드를
  삭제한다. cleaned 마킹은 결정적 ID 재사용과 충돌해 기각(design-review 3차). 보존은
  reflect-completion(completion 섹션의 plan/spec 접힌 전문)이 선행 게이트다.
- **구현 리뷰 게이트**: orca 모드 publication은 planner급 design-review 리뷰의 pass +
  변경 집합 fingerprint 일치가 fail-closed 게이트다(ai_slop_clean 선례).
  reviewer 모델 자기신고는 게이트가 아니라 감사 필드다(design-review 1차 critical).
- **기각 대안**: guard skip 술어(released 개방 위험), 머지 자동 감지(훅 워크플로
  금지), 사이클별 span lock 분리(측정 후 후속), 강제 prune 탈출구(list 가시화로
  대체).
- **후속**: deleteIssueOps 2-버킷 원자화, 워크트리 leaf 충돌, done 사이클
  base-branch 게이트 공백, GitLab orca 모드, PrepareWorkspace/LaunchOwner
  커버리지 계량, AC-11b 실제 orca 하위 세션 도그푸드.
