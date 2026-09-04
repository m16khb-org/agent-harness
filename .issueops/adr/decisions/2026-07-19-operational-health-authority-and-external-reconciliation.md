# 2026-07-19 — One operational-health authority and external one-time reconciliation

← [ADR index](../../ADR.md)

**결정:** Git, IssueOps, 선택적 Orca, user-state 교차 정합성은 기존 top-level `doctor` 하나가 공개 판정한다. Pure `internal/core/operationalhealth`와 read-only `internal/adapter/operationalhealth`를 두고, stale scan은 cycle authority만 공유하며 destructive release eligibility는 기존 strong-signal 정책에 남긴다. Stability audit는 새 판정기를 만들지 않고 빌드 직후 `doctor`를 호출한다.

- claimed cycle의 자동 live 경계는 complete durable identity와 주입된 현재 시각 기준 15분 이내 heartbeat다. 이 경계는 unhealthy 진단일 뿐 interrupt/delete/release 권한이 아니다.
- `--preserve-cycle`과 `--preserve-terminal`은 exact invocation-only 예외이며 state에 저장되지 않는다. Orca가 없고 Orca-owned durable state도 없으면 optional dependency로 취급하고, owner가 있는데 inventory가 없으면 unknown/unhealthy다.
- 현재 승인된 전체 정리는 제품 command가 아니라 `~/.local/state/issueops-backups/<repo-fingerprint>/<UTC-timestamp>/`의 외부 `0700` recovery bundle, sealed manifest, append-only journal로 수행한다. Git/SQLite backup은 restore-tested이고 Orca snapshot은 archival-only다.
- Orca에는 conditional reset/import/restore가 없어 reset 직전 외부 actor race를 완전히 제거할 수 없다. Pre-reset digest drift는 중단하고, reset 이후 crash/ambiguity는 rollback을 추측하지 않고 동일 journal에서 idempotent forward recovery한다.

**상태:** classifier, collector, doctor projection, stale-scan reuse, stability delegation까지 구현했다. 실제 live artifact 정리는 별도 sealed bundle 생성과 pre-cleanup verification gate 뒤의 operator 단계로 남는다. 새 cleanup CLI/MCP, persistent exemption, scheduler는 추가하지 않는다.

**거절:** binding/resource 일치만으로 live 판정, 별도 reconcile command, background reaper, Orca private storage 편집. 이 대안들은 dead-but-consistent owner를 통과시키거나 판정 source를 늘리고 복구 경계를 약화한다.
