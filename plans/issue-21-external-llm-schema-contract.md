# Issue #21 Plan: external LLM schema contract

## Goal

Make structured `agy -p` usage reliable when the provider has no native structured-output API. Every external LLM path that expects machine-readable JSON must inject the expected schema/example into the prompt and parse either strict JSON or fenced `json` JSON, while preserving read-only evaluator constraints.

## Scope

- Inspect all `RunExternalLLMPrint` call sites and classify them as structured JSON gates or free-form generation.
- Add shared core helpers for structured external LLM prompts and JSON extraction.
- Apply the helpers to IssueOps remote scoring and IssueOps benchmark judge.
- Keep self-verify's existing noisy JSON extraction behavior, but align prompt schema wording or parser helper use where it is low risk.
- Do not force JSON-only behavior onto draft wiki free-form writing paths.

## Steps

1. Add focused tests for the current gaps.
   - Verify IssueOps remote scoring prompt includes a fenced `json` schema/example and schema-enforcement wording.
   - Verify IssueOps benchmark judge prompt includes the same contract.
   - Verify fake `agy` fenced `json` output is accepted for IssueOps remote scoring and benchmark judge.
   - Verify malformed fenced or extra-field output remains rejected.

2. Implement shared structured-output helper.
   - Build a fenced `json` response schema/example section for prompts.
   - Decode strict JSON first.
   - If strict decoding fails, extract a single fenced `json` block and decode it with `DisallowUnknownFields`.
   - Reject empty, ambiguous, or trailing JSON/data outputs.

3. Wire structured gates.
   - Update IssueOps remote scoring prompt and decoder.
   - Update IssueOps benchmark judge prompt and decoder.
   - Preserve execution class and read-only normalization.

4. Verify real-world behavior.
   - Run focused Go tests.
   - Run full Go tests.
   - If `agy` is installed and usable, run a tiny read-only schema smoke prompt. If unavailable, record the fake `agy` evidence.

## Verification

```bash
go test ./internal/core -run 'ExternalLLM|IssueOps.*Agy|IssueOpsRemote' -count=1
go test ./cmd/harness -run 'SelfVerifyLLM|Golden' -count=1
go test ./... -count=1
git diff --check
```

## IssueOps Notes

- Remote issue: https://github.com/m16khb/agent-harness/issues/21
- Expected worktree: `/Users/m16khb/Workspace/agent-harness.worktrees/fix-21-external-llm-schema-contract`
- Source checkout must remain clean on `main`.
