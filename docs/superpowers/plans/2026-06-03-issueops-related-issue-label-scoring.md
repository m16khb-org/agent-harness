# IssueOps Related Issue And Label Scoring

## Goal

Make `$issueops` proactively score related issues and labels when creating or updating remote GitHub/GitLab issues, and make requirement-changing feedback update the issue body because the issue is the source of truth.

## Scope

- Add a shared external LLM print wrapper for all `agy -p` usage.
- Route existing `agy` call sites through the wrapper with `--dangerously-skip-permissions`.
- Add `agent-harness issueops remote score` with deterministic scoring and active `agy` judge support.
- Update the IssueOps skill so agents propose related issue/label scoring and issue-body SOT updates without waiting for the user to suggest them.
- Cover GitHub and GitLab apply instructions.

## Tasks

1. Core wrapper and scoring DTOs
   - Add `RunExternalLLMPrint`.
   - Add remote issue/label scoring request and result DTOs.
   - Verify with `go test ./internal/core -count=1`.

2. CLI surface and contracts
   - Add `issueops remote score`.
   - Update usage and response contract goldens.
   - Verify with `go test ./cmd/harness -run 'IssueOps|Golden' -count=1`.

3. Skill behavior
   - Document threshold-based related issue and label scoring.
   - Document issue-body SOT updates during feedback loops.
   - Verify with targeted skill text checks and full Go tests.

4. Remote issue sync
   - Update GitHub issue #11 body whenever implementation requirements change.
   - Verify with `gh issue view 11 --json body`.

## Verification

- `go test ./internal/core -count=1`
- `go test ./cmd/harness -run 'IssueOps|Golden' -count=1`
- `go test ./... -count=1`
- `go build -o bin/agent-harness ./cmd/harness`
- `python3 skills/issueops/scripts/remote_artifact_gate.py --kind issue --title "$TITLE" --body-file "$BODY_FILE"`
