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
- **NOT committed (answer):** the recorded scores, root-cause analysis, exact
  fix, gaming-resistance notes and provenance. Those live in
  `.agent-harness/evidence/pioneer-skills-quality/reruns/<skill>/result.yaml`,
  which stays blanket-gitignored (`.gitignore: evidence`). A Go test
  (`internal/holdoutdeleak`) mechanically asserts no file here leaks an answer
  token and that the evidence tree stays untracked.

## Coverage and honesty caveats

- **6 of 9 are filesystem fixtures** (dijkstra, hopper, shannon, torvalds,
  turing, von-neumann). `shannon` and `torvalds` need a git working-tree state,
  so they ship a `setup.sh` builder rather than a committable nested `.git`.
- **2 are in-prompt** (codd, karpathy): there is no filesystem input, only the
  prompt text in `TASK.md`.
- **1 is live-web** (berners-lee): it depended on live network state and is
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
