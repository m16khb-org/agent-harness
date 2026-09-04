# 6: lifecycle state 초기화 read-while-write 레이스 수정

이슈: https://github.com/m16khb-org/issueops/issues/6
근거: `.issueops/issues/_unnumbered/harness-quality-improvement-program.md` Q6 (P1), debugging 진단(ISOLATE 완료).

## Root Cause (확정)

`createJSONAtomic`(internal/core/lifecycle/lifecycle_project_state_store.go:172-190)이 `O_CREATE|O_EXCL`로 연
**최종 경로에 직접 Write** → 파일이 내용 기록 전부터 다른 세션에 노출. O_EXCL 패자 경로(92-101행)가 즉시
read → 승자의 Write 완료 전이면 빈/부분 JSON → `unexpected end of JSON input`.

재현(확보): `go test ./internal/core/lifecycle ./cmd/issueops/... -count=3` 병렬 부하에서
`TestInitProjectLifecycleStateConcurrentNoDuplicates` 간헐 실패(lifecycle_state_test.go:189).

## Fix (단일 task, <20줄)

- [ ] 1. `createJSONAtomic`을 temp-완전기록 후 `os.Link(tmp, path)` 노출로 변경
  - temp 생성·기록·chmod·close는 `writeJSONAtomic`(138-170행) 패턴 재사용.
  - `os.Link`의 EEXIST가 O_EXCL의 단일-승자 의미를 보존(`os.IsExist`는 `*LinkError`를 unwrap — 패자 경로 92행 무변경 호환).
  - temp 파일은 성공/실패 모두 제거.
  - **TDD**: RED = 기존 재현 명령(부하 의존이므로 `-count=3` 병렬, 간헐) + race 모드. GREEN 후 반복 검증으로 비결정성 소멸 확인.

## Acceptance (이슈와 동일)

- 재현 명령 10회 연속 그린; `go test -race ./internal/core/lifecycle -count=3` 그린.
- `go test ./... -count=1` 무회귀; self-verify go-test 스텝 그린.
- O_EXCL 단일-승자 의미 회귀 없음(기존 Concurrent 테스트가 RepoID 일치로 검증).

## Non-goals

다른 쓰기 경로 개편, flock 도입.
