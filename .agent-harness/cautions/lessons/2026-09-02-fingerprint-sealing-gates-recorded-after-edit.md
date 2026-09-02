---
name: cautions/lessons/2026-09-02-fingerprint-sealing-gates-recorded-after-edit.md
description: Dated lesson — a gate that seals the change fingerprint goes stale the moment a covered file is edited, so every sealing record must be written after the last edit it covers.
---

# 2026-09-02 — fingerprint를 봉인하는 게이트는 수정 뒤에 기록한다

Family index: [CAUTIONS.md](../../CAUTIONS.md).

- Date: 2026-09-02
- Kind: `caution`
- Source: issueops publication gate work (2026-09-02)
- Summary: A gate that seals implementation.ChangeFingerprint goes stale the moment any file in the change set is edited, so a project-doc update recorded before the document is written is immediately rejected.
- Context: project_docs_review seals the change fingerprint the same way implementation_review and ai_slop_clean do. The natural reading of "review the docs, then update them" puts the record first — but updating a document changes the change set, which changes the fingerprint, which makes the just-written record stale. The same trap applies in reverse to implementation_review: running it before the project-doc edit means the brooks review seals a fingerprint the doc edit then invalidates.
- Resolution: Fixed the publication order in the owner prompt and in issueops-implement: ai-slop-clean -> edit project docs -> record project_docs_review -> record schema_evidence -> implementation review -> commit/push. Every fingerprint-sealing record is written after the last edit it covers. Recording gates in this order also means all sealed records share one fingerprint, so strict readiness reports no _stale key. When a later change is unavoidable, both records must be redone against the new fingerprint — this is what the _stale keys are telling you, not a bug to work around.
- Evidence:
  - internal/adapter/issueops/testdata/execution_owner_prompt.txt (publication 단계 3-5)
  - skills/issueops-implement/SKILL.md (Publication evidence gates)
  - internal/adapter/issueops/issueops_readiness_test.go:TestIssueOpsStrictPRReadinessDetectsStaleAISlopCleanAfterImplementationChange
