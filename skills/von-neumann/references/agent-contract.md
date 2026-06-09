# Von Neumann Agent Contract Template

Every agent dispatched by Von Neumann receives this contract. The agent has NO interview context — be exhaustive.

## Template

```
TASK: <imperative one-line assignment — what to DO, not what to think about>

DELIVERABLE: <exact file path(s) and what each must contain when done>

SCOPE:
  - Files to create/modify: <exact paths, one per line>
  - Files to read (do NOT modify): <exact paths>
  - Patterns to follow: <file:line references with what to copy and why>
  - Constraints:
    - Do NOT change: <specific files, functions, or behaviors>
    - Do NOT import: <forbidden dependencies>
    - Do NOT use: <forbidden patterns>

CONTEXT:
  - Goal: <what this task contributes to — one sentence>
  - Wave: <N of M> — this task runs in parallel with <other tasks in same wave>
  - Depends on (already complete): <wave N-1 outputs this task can rely on>
  - Previous wave outputs: <relevant file paths and what they provide>

TDD CONTRACT:
  - IF touching existing behavior:
    Characterization test FIRST: pin current observable behavior.
    File: <test file path>
    Test name: <function name>
    Must PASS on unchanged code.
  - RED: Write failing test for <specific new behavior or fix>.
    File: <test file path>
    Test name: <function name>
    Expected failure: <exact assertion message>
    Must fail for the RIGHT reason (not syntax error, not missing import).
  - GREEN: Implement the SMALLEST production change.
    File: <source file path>
    Max lines: ~20. If more → test is too coarse → split into sub-tasks.
  - Run: <exact test command>
    Expected: <N> tests run, <N> passed, 0 failed

VERIFY:
  - Test command: <exact shell command to run full suite>
  - LSP check: <file paths to check for diagnostics>
  - Manual-QA channel: <HTTP call | tmux | browser use | computer use>
  - Channel command: <exact command to exercise the surface>
  - Expected evidence: <.agent-harness/turing/evidence/task-<N>.ext>
  - Binary pass/fail: <what PASS looks like in the artifact>

CLEANUP:
  After verification, tear down:
  - <resource description> → <exact teardown command>
  - ...
  Cleanup receipt format: "cleanup: <actions taken, verified by <check>>"

CONSTRAINTS:
  - Do NOT edit files outside SCOPE.
  - Do NOT "improve" adjacent code, comments, or formatting.
  - Do NOT add new dependencies without explicit permission.
  - If blocked, return BLOCKED: <reason> — do not guess.

OUTPUT:
  Return exactly: DONE: <deliverable summary> | EVIDENCE: <evidence path> | CLEANUP: <receipt>
  Or: BLOCKED: <reason>
  Or: NEEDS_CLARIFICATION: <specific question>
```
