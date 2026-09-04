---
name: cautions/lessons/2026-08-27-record-delete-bypassed-the-span-gate.md
description: Dated lesson — IssueOps 레코드 삭제가 sqlstore 직렬화 게이트를 지나지 않아 열려 있는 related-update span이 삭제 뒤 related row를 되살렸다.
---

# 2026-08-27 — 레코드 삭제가 span 게이트를 지나지 않아 고아 related state가 남았다

Family index: [CAUTIONS.md](../../CAUTIONS.md).


- Kind: `caution`
- Source: PR #478 CI 실패 조사, Claude Code session 2026-08-27
- Summary: `sqlstore.DB.spanGate`는 `WithSpan`에서만 획득하고 `Apply`/`CompareAndApply`/`CompareAndApplyFunc`는 게이트를 지나지 않는다. 그래서 `issueopsrecord.Store.DeleteIfUnchanged`가 열려 있는 `UpdateRelated` span과 순서를 맺지 못하고, 삭제가 먼저 커밋된 뒤 span의 Put이 related row를 되살렸다.
- Context: `TestDeleteIfUnchangedSerializesNewRelatedState`가 CI에서 간헐적으로 `related state survived serialized delete: found=true`로 실패했다. 격리 실행과 로컬 150회 반복에서는 통과해 리소스 경합 flake로 분류돼 있었다. 테스트 주석은 "게이트가 delete를 잡고 있으므로 순서가 보장된다"고 전제했지만, delete 경로는 게이트를 아예 통과하지 않는다. 실제 위험 경로는 retention sweep(`internal/application/issueopsretention/service.go`)의 `DeleteIfUnchanged`가 다른 작업의 related update와 겹치는 경우다. CAS는 레코드 drift만 막고 related bucket 쓰기와는 순서를 맺지 못한다.
- Resolution: `Store.DeleteIfUnchanged`와 `Store.Delete`를 `database.WithSpan` 안에서 실행해 update/related-update와 같은 직렬화 게이트를 지나게 했다. 게이트는 `Open`이 dir별로 캐시하는 단일 `*DB`에 붙어 있어 in-process 호출자 전체가 공유한다. 게이트를 `Apply` 계열 안으로 내리는 방안은 span 안에서 호출되는 write가 자기 자신과 교착할 수 있어 택하지 않았다. 두 삭제 메서드의 호출자는 span 밖에서만 호출하므로(retention sweep은 read와 delete를 순차 수행한다) 중첩 위험이 없다.
- Evidence:
  - `spanGate`는 `sqlstore.go`의 `WithSpan`에서만 참조된다. `Apply`/`CompareAndApply`/`CompareAndApplyFunc`는 `d.data.BeginTx`로 직행하고 데이터 풀에 `SetMaxOpenConns`도 없다.
  - 기존 테스트에 delete가 먼저 진입하도록 300ms를 유도하면 5/5 실패하며 CI와 같은 메시지가 나온다.
  - RED: 새 회귀 테스트 `TestDeleteIfUnchangedWaitsForOpenRelatedUpdateSpan`가 span이 열려 있는 동안 delete가 즉시 커밋하는 것을 3/3 관측(약 0.01초).
  - GREEN: 수정 후 `go test ./internal/adapter/outbound/issueopsrecord -run TestDeleteIfUnchanged -count=20` ok.
  - `internal/adapter/outbound/issueopsrecord/store.go`, `internal/adapter/outbound/sqlstore/sqlstore.go`
- Rule: 레코드를 삭제하는 경로는 그 레코드의 related bucket을 쓰는 span과 같은 게이트를 지나야 한다. CAS는 대상 레코드의 drift만 막을 뿐 다른 bucket 쓰기와 순서를 맺지 않는다. 격리에서만 통과하는 동시성 테스트는 flake로 분류하기 전에 인터리빙을 강제해 결정적 조건인지 확인한다. Evergreen 규칙: [runtime.md](../runtime.md).

> Incident-time command, field, and state references are historical evidence, not current execution directives.
