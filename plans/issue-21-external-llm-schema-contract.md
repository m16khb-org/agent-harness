# Issue #21 Plan: external LLM schema contract

## Goal

Make structured `agy -p` usage reliable when the provider has no native structured-output API. Every external LLM path that expects machine-readable JSON must inject the expected schema/example into the prompt and parse either strict JSON or fenced `json` JSON, while preserving read-only evaluator constraints.

## Scope

- Inspect all `RunExternalLLMPrint` call sites and classify them as structured JSON gates or free-form generation.
- Add shared core helpers for structured external LLM prompts and JSON extraction.
- Apply the helpers to IssueOps remote scoring and IssueOps benchmark judge.
- Keep self-verify's existing noisy JSON extraction behavior, but align prompt schema wording or parser helper use where it is low risk.
- Also enforce the schema contract on commit suggest, lint diagnose, and draft wiki suggestion paths.
- Add follow-up verification coverage for the actual `agy -p` invocation mode/model settings.

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

4. Wire remaining `agy -p` paths.
   - Update commit suggest, lint diagnose, and draft wiki prompts with field/type schemas.
   - Parse their responses from strict JSON or fenced `json` JSON.
   - Preserve generated user-facing content inside a typed string field.

5. Verify real-world behavior.
   - Run focused Go tests.
   - Run full Go tests.
   - Run real production-prompt `agy` checks across the affected commands.
   - Record follow-up evidence for whether print-mode `agy` uses the configured `Gemini 3.5 Flash (Medium)` and fast mode settings.

## Verification

```bash
go test ./internal/core -run 'ExternalLLM|IssueOps.*Agy|IssueOpsRemote' -count=1
go test ./cmd/harness -run 'SelfVerifyLLM|Golden' -count=1
go test ./... -count=1
git diff --check
```

## IssueOps Notes

- Remote issue: `#21`
- Expected worktree: `../agent-harness.worktrees/fix-21-external-llm-schema-contract`
- Source checkout must remain clean on `main`.
