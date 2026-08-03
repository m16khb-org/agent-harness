# #258 Turing report

## Acceptance matrix

| Acceptance | Test / evidence | Result |
| --- | --- | --- |
| Dispatched owner prioritizes sealed claim over recovery resume | `TestExecutionOwnerPromptSeparatesSealedClaimFromRecoveryResume`; live #248 reseed follows merge | GREEN / live pending |
| Prompt/status priority is contradiction-free | embedded/Karpathy byte parity and adversarial prompt assertions | GREEN |
| Duplicate live-owner resume is explicitly staged | `TestResumeReturnsExistingBindingWithoutAllocatingAnotherLaunch`; CLI/MCP `resume_disposition` assertions | GREEN |
| Dead-owner recovery remains available | `TestPlanResumeDecisionTable`; `TestResumeCreatesTerminalRunTaskDispatchInApplicationOrder` | GREEN |
| GitHub/GitLab identity and direct mode do not regress | full normal and race suites, including owner prompt/direct fixtures | GREEN |
| Live Orca production path | #248 lifecycle `io-268bd6ac6e7a`, new sealed generation | pending |

## RED evidence

- Prompt test failed because `injected sealed claim command가 유일한 owner next
  action` was absent and printed the contradictory recursive-resume instruction.
- Application, CLI, and MCP tests failed to compile because `Disposition` and
  `ResumeDisposition` were absent from their result contracts.

## GREEN implementation

- Made the injected sealed claim the only next action for a dispatched owner;
  status' resume is explicitly coordinator-only recovery.
- Propagated the existing domain disposition through application, inbound
  adapter, CLI JSON, MCP JSON, compatibility contract, and response golden.
- Proved `existing_binding` allocates no operation ID and preserved terminal
  reuse/create recovery decisions without changing durable lease transitions.
- Removed the unused legacy core resume-result assembler so there is one active
  response projection path.

## Verification log

- Targeted resume/prompt/CLI/MCP/contract tests — pass.
- `go test ./... -count=1` — pass.
- `go test -race ./... -count=1` — pass.
- `go vet ./...` — pass.
- `go build -o /tmp/agent-harness-258 ./cmd/harness` — pass.
- `gopls check` on all modified Go files — pass.
- `git diff --check` — pass.
- Fresh implementation review `review_258_owner_claim` — PASS, zero findings.
- Publication and live #248 Orca dogfood remain pending.
