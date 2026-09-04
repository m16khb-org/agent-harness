# Pioneer holdout reproduction harness

These directories are **reproduction harnesses**, NOT held-out test cases.

A holdout's epistemic value is that the skill under test has never seen it. The
moment a holdout fixture is committed into the same repo whose pioneer skills
operate on it, that property is gone — a future run can read the committed
fixture. So this tree is honestly named a *reproduction harness*: it lets a
fresh checkout re-stage the **input** of each pioneer holdout case so the
scenario can be re-run, but it is no longer a blind holdout.

## What is and is NOT committed here

- **Committed (input only):** the broken/seed files (`run.sh`, `greet.py`,
  `startup.py`, `profile.txt`, README/notes) or a `setup.sh` that rebuilds a
  git working-tree state, plus a `TASK.md` stating the user request.
- **Committed (redacted run manifest):** `evaluation-manifest.json` records
  36 primary/boundary/operational case rows (three per namesake): fixture
  paths/hashes, isolated child IDs, pass/blocked/fail verdicts, host/model route,
  capability-block reasons, and SHA-256/byte-count receipts for 24 unique child
  executions. Primary uses 12 independent children; each namesake's boundary
  and operational prompts share one additional child. It never stores answer
  content, never presents 36 rows as 36 independent executions, and marks every
  committed case as `hidden_holdout=false`.
- **Committed (reviewable evidence records):** `evidence-records/<skill>.json`
  binds each namesake's two fresh-context runs to all three case hashes,
  deterministic assertion IDs, semantic grade, and host-capability outcome.
  The manifest stores and validates each evidence-record SHA-256. These records
  are evaluation receipts, not answer text.
- **NOT committed (answer):** root-cause analysis, exact fix, evaluator prose,
  and answer artifacts. Those live in
  `.issueops/evidence/pioneer-skills-quality/reruns/<skill>/result.yaml`,
  which stays blanket-gitignored (`.gitignore: evidence`). A Go test
  (`internal/holdoutdeleak`) mechanically asserts no file here leaks an answer
  token and that the evidence tree stays untracked.

## Coverage and honesty caveats

- Each namesake has `TASK.md` (primary), `BOUNDARY.md`, and `OPERATIONAL.md`.
  The latter two are independent prompts rather than answer variants.
- **6 of 12 primary cases are filesystem fixtures** (algorithm-optimization, issueops-debugging, code-quality-metrics, git-operations,
  verified-execution, implementation-planning). `code-quality-metrics` and `git-operations` need a git working-tree state,
  so they ship a `setup.sh` builder rather than a committable nested `.git`.
- **5 are in-prompt** (requirements-analysis, design-review, database-design, meeting-notes, prompt-engineering): there is no
  filesystem input beyond the prompt text in `TASK.md`.
- **1 is live-web** (web-research): it depended on live network state and is
  **NOT reproducible offline** — recorded for provenance only.
- **Provenance gap:** these inputs are RE-AUTHORED from the recorded specs. The
  original `/tmp/pioneer-holdouts` working dirs that produced the recorded
  scores are gone, so a re-run reproduces the *case*, not the exact recorded
  *run*.
- **Contamination caveat:** these are generic engineering tasks (a missing
  config file, a typo, an O(n²) loop). "Post-cutoff" dating would only defend
  against verbatim memorization, not against a model solving a generic bug from
  priors. Scores reflect baseline competence on the case, not held-out
  generalization.
