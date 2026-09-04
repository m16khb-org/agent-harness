## 요약

- IssueOps child prepare의 기본 `auto` 선택을 preview fingerprint와 동일 confirm으로 봉인합니다.
- current schema v1 execution에 requested/resolved mode, Orca probe/readiness/fallback, 선택 시각과 explicit-direct reason 영수증을 원자적으로 보존합니다.
- CLI/MCP/status contract를 통일하고 native host smoke를 `validation_lane=native_host`로 Orca orchestration 증거와 분리합니다.

## 검증

- focused domain/application/inbound/outbound RED→GREEN
- CLI/MCP/issueopsapp response contract golden
- 전체 test, race, vet, build
- lifecycle `io-268bd6ac6e7a` generation 4 Orca owner claim/readback

Closes #248
