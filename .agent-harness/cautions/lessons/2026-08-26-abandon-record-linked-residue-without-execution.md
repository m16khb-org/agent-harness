---
name: cautions/lessons/2026-08-26-abandon-record-linked-residue-without-execution.md
description: Dated lesson — a cycle merged without execution prepare had no typed cleanup exit while its linked worktree/branch remained; cleanup abandon now accepts record-linked residue.
---

# 2026-08-26 — Merged-without-execution cycle had no typed cleanup exit

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Kind: `caution`
- Source: api-servers cycle `io-2fb20438b925` (`2799-image-fixer-spec-source`) 정리 시도
- Summary: `execution prepare` 없이 커밋·MR·머지·이슈 종료까지 끝난 사이클은
  `Execution`도 기록된 `remote_artifact`도 없다. 이 상태에서 link된 워크트리나 로컬
  브랜치가 남아 있으면 세 typed 삭제 경로가 모두 막혔다: `cleanup finish`는 검증된
  artifact를, `cleanup orphan`은 record 부재를, `cleanup abandon`은
  `local_residue_execution`(execution 소유 근거)을 요구했다.
- Context: `phase --to pr`는 strict readiness(`upstream`, 원격 브랜치 삭제됨)와
  `ai_slop_clean`에, `execution prepare`는 워크트리 HEAD가 `branch_prepare.base_sha`를
  벗어나 identity mismatch에 막혀 forward recovery도 닫혀 있었다. 이 게이트군은
  #140/#139(claimable), #342(done), #433(비대칭 잔여물), #437(children)에서 같은
  방식으로 실측 사례마다 완화돼 왔다.
- Resolution: `cleanupAbandonGates`가 record가 link한 워크트리(`record.WorktreePath`)를
  record 소유 잔여물로 인정한다. canonical 경로·브랜치 일치·HEAD·clean 검사와
  fingerprint CAS·`--confirm`·사유는 그대로 적용되고, record가 link하지 않은 잔여물은
  여전히 `local_residue_execution`으로 막힌다(`issueops_cleanup_abandon.go`,
  `TestCleanupAbandonAcceptsRecordLinkedResidueWithoutExecution`,
  `TestCleanupAbandonRejectsLocalResidue`).
- Evidence: `issueops cleanup abandon --preview` → `missing: [local_residue_execution]`;
  `cleanup finish --preview` → `cleanup finish requires a verified remote artifact`;
  `cleanup orphan`은 record 존재로 거부. 워크트리·브랜치를 typed 경로 밖에서 지운 뒤에야
  preview가 ready가 됐다.
- Rule: 사이클을 시작했으면 커밋 전에 `execution prepare`까지 마친다(그래야 phase·PR·
  finish 경로가 열린다). 이미 어긋난 사이클은 잔여물을 손으로 지우지 말고 `cleanup
  abandon --preview`가 제시하는 typed apply 명령으로 정리한다.

> Incident-time command, field, and state references are historical evidence, not current execution directives.
