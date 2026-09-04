---
name: 2026-09-02-issueops-binds-artifacts-to-the-code-project-not-the-issue-p
description: Accepted decision record with rationale, alternatives, and consequences.
---

# IssueOps binds artifacts to the code project, not the issue project

- Date: 2026-09-02
- Kind: `adr`
- Source: issueops cross-project support (2026-09-02)
- Summary: A cycle whose issue lives in one provider project and whose code lives in another now seals a code_project_key at branch prepare, and PR creation plus every artifact validation binds to that project instead of the issue's.
- Context: Three independent places assumed the issue's project also owned the code: remote create-pr derived its target project from the issue URL, artifact verification called ValidateArtifactMatchesIssue, and execution completion compared the artifact against the issue project. A cycle filed in a planning project with code in a service project could therefore never seal remote_artifact — verify-artifact rejected the real MR — and cleanup then stalled on the unfillable remote_artifact, leaving manual worktree/branch/issue teardown as the only exit.
- Decision: (1) Seal the owning project on IssueOpsBranchPrepare as code_project_key. branch prepare observes the checkout's origin and seals the value only when it differs from the issue's project; --code-project-key pins it explicitly when origin is absent or points elsewhere. (2) Add remote.EffectiveProjectKey (sealed key, else the issue's project) and remote.ValidateArtifactMatchesProject, and route create-pr, reconcile, verify-artifact, and completion through them. (3) Keep the validation strength identical — an artifact outside the bound project is still rejected; only the binding target changed. (4) An unobservable origin falls back to the issue project rather than blocking branch preparation, so no cycle can be wedged by a missing remote.
- Consequences: code_project_key becomes public contract on the record, the lease stable v1 shape, the completion snapshot, and the branch prepare CLI. Same-project cycles seal nothing and their records stay byte-identical, so no in-flight cycle changes behavior. Declaring the wrong key rejects the artifact at three gates at once; the sealed value is visible in issueops status. Cleanup's remote-branch identity check already compared origin against the artifact rather than the issue, so it needed no change and was in fact the pattern this decision generalizes.
- Alternatives / rejected options:
  - Relax ValidateArtifactMatchesIssue to allow any project — rejected: it removes the guard that keeps an unrelated PR from being attached to a cycle.
  - Observe the code project at each validation site instead of sealing it — rejected: validation would depend on live git state, so a moved remote would silently retarget an already-published cycle.
  - Require an explicit --code-project-key for every cross-project cycle with no observation — rejected: origin already carries the answer, and a mandatory flag would be forgotten exactly once per cycle, at branch prepare, where the failure only surfaces much later at verify.
- Evidence:
  - internal/domain/issueopsremote/issueops_remote_url.go (EffectiveProjectKey, ValidateArtifactMatchesProject, ValidProjectKey)
  - internal/adapter/issueops/branchprepare/branch_prepare.go (resolveCodeProjectKey)
  - internal/adapter/issueops/artifactverify/cross_project_test.go
  - internal/adapter/issueops/branchprepare/code_project_key_test.go
  - internal/domain/issueopscompletion/artifact_test.go (TestValidateArtifactBindsToSealedCodeProject)
  - go test ./... -count=1
