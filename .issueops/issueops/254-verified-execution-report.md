# #254 verification report

## Goal

Bind Orca resume to the sealed artifact identity stored for its lease generation, so owner prompt template upgrades cannot invalidate intact in-flight work and legacy executions have an explicit fail-closed reseed recovery.

## Acceptance evidence

- AC-01/02: preparation dispatch and Orca reseed persist artifact identity version 1 plus the complete three-digest identity; resume no longer calls `renderExecutionOwnerPrompt`.
- AC-03: `TestReadExecutionResumeArtifactsRejectsSealedIdentityDrift` independently rejects prompt bytes, packet bytes, issue digest, and stored prompt digest drift before Orca inventory or mutation.
- AC-04: `TestReadExecutionResumeArtifactsUsesDurableIdentityAcrossTemplateUpgrade` changes the embedded template after sealing and resumes the intact older prompt.
- AC-05: core and CLI status tests route only unversioned all-empty legacy bindings through preview; contract tests reject versioned all-empty, unversioned-complete, partial, and future-version identities. A new binary read of live #248 generation 1 emitted the exact preview command without mutating state.
- AC-06: focused tests, full tests, full race tests, vet, build, contract/response goldens, diff check, and `validate-skill.py skills/issueops` pass.
- AC-07: independent implementation review, PR CI, merge, exact-head dual-host smoke, and cleanup remain publication gates.
- AC-08: after merge, live #248 will follow its emitted recovery commands until a real Orca owner claims the replacement generation and performs a production mutation.

## Verification

RED first failed because `OrcaBinding` had no durable digest fields. GREEN added only the stable contract, producer, verifier, recovery projection, conversion, tests, and operating docs required by that failure. Brooks then identified that undifferentiated all-empty state could hide a post-upgrade producer bug; a second RED failed on the absent version marker, and GREEN made identity version 1 mandatory for every current producer while preserving only unversioned all-empty legacy recovery. A first full-suite run exposed the pre-existing actor-free resume CLI fixture as a current-identity fixture missing the new fields; the fixture now tests both current resume and legacy preview projection.

- `go test ./internal/contract/issueops ./internal/contract/issueopslease ./internal/adapter/outbound/issueopspreparation ./internal/application/issueopslease ./internal/core/issueops ./internal/adapter/inbound/issueopspreparation ./internal/adapter/inbound/issueopslease ./internal/adapter/inbound/issueopscompletion ./cmd/issueops/issueopsapp -count=1`
- `go test ./cmd/issueops/contractgolden -run Golden -count=1`
- `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1`
- `go test ./... -count=1`
- `go test -race ./... -count=1`
- `go vet ./...`
- `go build -o /tmp/issueops-254 ./cmd/issueops`
- `python3 scripts/validate-skill.py skills/issueops`
- `git diff --check`

The IssueOps reference RED retrieval test incorrectly chose the revoke/finalize path for an already holderless legacy binding. The same fresh-agent scenario after the reference change returned the exact `status → preview → reseed → resume` chain and rejected recomputing trust from current worktree/template bytes.
