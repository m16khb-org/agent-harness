# #256 verification report

## Acceptance matrix

| Acceptance | Test / evidence | Result |
| --- | --- | --- |
| GitHub unverified resume launch is actionable | `TestOrcaIntentIssueIdentityAllowsOnlyUnverifiedGitHubForResumeLaunch`, `TestBeginOrcaExecutionResumeIntentAllowsUnverifiedGitHubLaunch` | GREEN |
| GitLab and non-launch authority remain verified-only | `TestOrcaLaunchIssueIdentityAllowsOnlyUnverifiedGitHub`, `TestAuthoritativeOrcaIssueIdentityRequiresVerifiedMatchingRecord` | GREEN |
| Pre-link owner cannot mutate production or enter implement | `TestUnverifiedOrcaHolderMayRecordBranchLinkButCannotMutateProduction` | GREEN |
| Typed shell/MCP controls cannot bypass the pre-link fence | shell and MCP `release` subtests in `TestUnverifiedOrcaHolderMayRecordBranchLinkButCannotMutateProduction` | GREEN |
| Exact branch recorder remains available | same lifecycle test, exact sealed topology command | GREEN |
| Claimable recovery and read-only status remain available | `TestUnverifiedClaimableOrcaMayResumeOwnerLaunch` | GREEN |
| Status-to-resume parity | `TestExecutionWriterAbsentRecoveryRoutesUnversionedOrcaThroughReseed` plus begin-resume integration fixture | GREEN |
| Live Orca recovery | #248 lifecycle `io-268bd6ac6e7a`, generation 2 | pending exact-head dogfood |

## RED evidence

- Resume identity test failed with `intent_identity_mismatch: Orca intent requires verified branch issue identity`.
- Begin-resume integration fixture failed with the same contract error.
- Hook test observed `apply_patch` decision `allow` for the active unverified Orca holder.
- Independent review found that typed execution controls skipped the mutation
  decision; RED tests observed `allow` for both shell and MCP `release`.

## GREEN implementation

- Added launch-scoped identity selection for GitHub prepare/resume intent sealing and persistence validation.
- Preserved `authoritativeOrcaIssueIdentity` for non-launch authority and required verified links for GitLab launches.
- Added a lifecycle pre-link fence that permits only the exact branch-link recorder among mutations until the durable record is verified.
- Applied the same fence before the typed control-plane skip, keyed to the exact
  lifecycle ID, while preserving claimable `resume` and read-only `status`.

## Verification log

- `go test ./internal/core/issueops ./internal/core/lifecycle -count=1` — pass.
- `go test ./... -count=1` — pass.
- `go test -race ./... -count=1` — pass.
- `go vet ./...` — pass.
- `go build -o /tmp/issueops-256 ./cmd/issueops` — pass.
- `go test ./cmd/issueops/contractgolden -run Golden -count=1` — pass.
- `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1` — pass.
- `go test ./internal/architecture -count=1` — pass.
- Exact-head host smoke, live #248 Orca recovery, CI, and merge evidence will be appended before completion.
