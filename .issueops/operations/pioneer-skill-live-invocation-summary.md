# Pioneer Skill Live Invocation Summary

This is the durable summary of representative live invocations used to inform the quality-improvement plan.

Full local evidence, including command outputs, is in `.issueops/evidence/pioneer-skills-quality/task-0-live-invocation-record.md` when available in this workspace.

## Summary Table

| Skill | Request actually tried | Response / artifact observed | Quality judgement | Improvement required |
|-------|------------------------|------------------------------|-------------------|----------------------|
| `web-research` | Verify whether `curl_cffi` can impersonate browser TLS/JA3/HTTP2 fingerprints and assess the skill's fetch-escalation safety. | Found primary web evidence from the GitHub project, official impersonate guide, and PyPI. The skill can cross-check the claim, but its own guidance still pushes auto-install and browser/TLS impersonation too readily. | Research workflow is useful; safety boundary is weak. | Gate dependency installation and impersonation; stop at auth/paywall/CAPTCHA/access-control; replace hard-coded `web_fetch` assumptions. |
| `database-design` | Optimize `SELECT * FROM orders WHERE user_id=1 ORDER BY created_at DESC LIMIT 2`. | SQLite plan changed from table scan + temp B-tree to `SEARCH orders USING COVERING INDEX idx_orders_user_created (user_id=?)`. | Core query-optimization request succeeds. | Keep method; split dense core content; add explicit write-penalty output. |
| `algorithm-optimization` | Decide whether to optimize `parseGitStatus`. | The function is already simple O(n), and there is no profiling evidence; correct response is "do not optimize speculatively." | Good request fit; prevents bad optimization. | Fix scaling-test explanation and add a no-change response template. |
| `debugging` | Diagnose `go test ./definitely-not-a-package -count=1`. | Reproduced missing-directory failure; current `project lint-diagnose --json -- ...` gives a useful root-cause diagnosis. | Method works with current CLI, but skill docs point to stale `--command-argv`. | Replace stale CLI form and label MCP form separately. |
| `prompt-engineering` | Improve a prompt that asks the model to show chain-of-thought. | Current skill guidance points toward chain-of-thought/show-your-work patterns; safer response should use private reasoning plus concise rationale and verification summary. | Strong prompt workflow; reasoning-privacy defect. | Replace user-visible CoT guidance; make tool schemas host-neutral. |
| `code-quality-metrics` | Measure quality of current uncommitted work. | `git status` shows untracked plan/fixture files; `git diff` sees no tracked diff. The current SNR path can miss real work. | Measurement workflow fails for untracked changes. | Include staged/unstaged/untracked inputs, zero-total guard, no global `go install`, label grep metrics as heuristic. |
| `git-operations` | Preflight current repo before advanced git work. | Captured branch `main`, recent commits, worktree state, and untracked files. | Strong git-state verification. | Add explicit confirmation ladder before destructive recovery; prefer non-interactive agent-safe rebase paths. |
| `verified-execution` | Treat the quality audit as an evidence-bound goal. | Turing's method exposed that static review was insufficient, but its own examples include stale `issueops heartbeat`, `remove-ai-slops`, and old state syntax. | Evidence philosophy is valuable; operational contract is stale and too heavy for small docs tasks. | Fix stale commands and make the final gate proportionate to task risk. |
| `implementation-planning` | Create a decision-complete plan for pioneer skill quality improvement. | Produced a useful plan structure, but the first draft overclaimed qualitative evaluation before actual invocation evidence existed. | Useful planner; needs stronger evidence-maturity gate. | Require request/response/evaluation evidence maturity in plans; narrow activation; remove nonexistent `implementation-planning plan` CLI. |

## Completion Status

Representative live invocation: complete for all 9 pioneer skills.

Full qualitative gate: incomplete until all 27 cases in `.issueops/operations/pioneer-skill-quality-cases.md` are executed and recorded in the same request -> response/artifact -> judgement -> improvement format.
