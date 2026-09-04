---
name: 2026-09-02-issueops-publication-evidence-gates-project-doc-reflection-a
description: Accepted decision record with rationale, alternatives, and consequences.
---

# IssueOps publication evidence gates: project-doc reflection and conditional schema measurement

- Date: 2026-09-02
- Kind: `adr`
- Source: issueops publication gate work (2026-09-02)
- Summary: Publication now fails closed until the cycle records whether the change left anything for the operating docs, and — only when the change set touches schema files — until it records observed database measurements with their source.
- Context: IssueOps read .issueops/ADR.md at plan-prep (--decisions-evidence) and shipped CAUTIONS/ADR into the execution owner packet as required reading, but nothing in the cycle ever asked whether the finished change should be written back. The project-docs-update skill existed with exactly that contract and was referenced by no IssueOps skill or gate — evidence that documentation-only wiring does not execute. Separately, schema and migration work was reaching PR with index and row-count assumptions that were never measured against a real database.
- Decision: (1) Add a first-class IssueOpsProjectDocsReview record (verdict updated|no-change, docs, evidence, sealed ReviewedFingerprint) following the implementation_review pattern, surfaced as missing key project_docs_review / project_docs_review_stale in both IssueOpsPRReadiness and strict readiness. Unlike implementation_review it applies to direct and orca modes alike, because direct cycles also produce durable decisions. (2) verdict updated requires each --doc path to be present in the current change set, so a document that was not actually edited cannot pass the gate. (3) Add IssueOpsSchemaEvidence as a conditional gate that activates only when the change set contains migration/entity/.sql/schema.prisma paths; it requires paired measurements and sources, or an explicit waiver with a rationale. (4) Extract implementation.ChangedPaths from ChangeFingerprint so both gates can ask which files a fingerprint covers. (5) Order the owner prompt as ai-slop-clean -> project-docs review -> schema evidence -> implementation review, because editing a document changes the fingerprint and a record written before the edit is immediately stale.
- Consequences: Two CLI subcommands (issueops project-docs-review record, issueops schema-evidence record) and two record fields become public contract; the command catalog, parser path list, owner command catalog, and owner prompt template all carry them. Cycles already in flight will report project_docs_review as missing until they record the verdict once. The schema gate is silent in repositories without schema files, so issueops itself never sees it. Detection is deliberately conservative (path segments migrations/migration/entities, *.entity.{ts,js,go}, *.sql, schema.prisma) — a false positive would activate the gate in a database-free cycle.
- Evidence:
  - internal/adapter/issueops/issueops_project_docs_review.go
  - internal/adapter/issueops/issueops_schema_evidence.go
  - internal/adapter/issueops/implementation/evidence.go (ChangedPaths)
  - internal/adapter/issueops/issueops_pr_readiness.go
  - internal/adapter/issueops/issueops_pr_readiness_strict.go
  - internal/adapter/issueops/testdata/execution_owner_prompt.txt
  - go test ./internal/adapter/issueops/ -run 'ProjectDocs|SchemaEvidence|SchemaChange'
- Alternatives / rejected options:
  - Add project-doc fields to IssueOpsImplementationReview instead of a separate record — rejected: implementation_review is scoped to orca mode, so direct cycles would skip documentation reflection entirely.
  - Wire project-docs-update into the IssueOps skill prose only — rejected: the skill already existed unreferenced, which is the exact failure mode being fixed.
  - Make the schema gate unconditional — rejected: it would fire in every database-free cycle, and a gate that is routinely waived stops being read.
  - Verify schema measurements by querying the database from the harness — rejected: the harness does not hold database credentials and must not become a database client.
