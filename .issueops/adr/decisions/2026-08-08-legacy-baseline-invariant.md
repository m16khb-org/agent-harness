# 2026-08-08 — legacy baseline을 없애고 래칫을 불변식으로 바꾼다

← [ADR index](../../ADR.md)

- Source: GitHub #234, #238
- Decision: `internal/architecture/testdata/legacy_imports.txt`를 삭제하고,
  래칫을 "legacy adapter edge는 0"이라는 불변식으로 대체한다. 새 edge는
  baseline에 등록하는 것이 아니라 애초에 들어올 수 없다.
- Rationale: baseline은 전환 중에만 의미가 있다. 263개에서 시작해 0이 된 지금
  파일을 남겨두면 "여기 한 줄 추가하면 통과한다"는 우회로가 그대로 남는다.
  마지막까지 남았던 `outbound/state -> outbound/sqlstore`와
  `issueops -> outbound/sqlstore`는 빚이 아니라 의도된 설계였다 — sqlstore는
  특정 capability의 어댑터가 아니라 저장 엔진이고, 포트로 감싸 주입할 수는
  있으나 그 대가로 저장소의 거의 모든 테스트 패키지가 배선을 짊어진다. 엔진
  교체는 실제 요구가 아니므로 `isSharedStorageEngineEdge`로 명시했다.
- Consequence: 예외는 outbound -> sqlstore 한 방향뿐이며
  `TestSharedStorageEngineExceptionIsOneDirectionOnly`가 cmd·inbound·domain에서
  들어오는 edge와 sqlstore 밖으로의 확장을 함께 막는다.
