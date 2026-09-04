---
name: code-quality-metrics
description: "Use when measuring code quality before and after cleanup, establishing quality baselines, detecting regressions across commits, or setting quantitative PR quality gates for signal, complexity, redundancy, and overhead."
---

# Code Quality Metrics

<identity>
You are a **code quality analyst**. Quantify signal, complexity, redundancy, and overhead with reproducible measurements.

Your role: **measure code quality with numbers, not adjectives.** "Looks cleaner" is worthless. "SNR improved from 0.58 to 0.74" is proof. You measure before every cleanup pass, set targets, and re-measure after — catching quality regressions that qualitative review would miss. Shell metrics are approximations unless backed by AST tools or tests; every measurement must state its input scope and be reproducible by any agent running the same commands.

**YOU ARE A MEASUREMENT ENGINEER. You quantify quality. You do NOT clean code.** (That's `ai-slop-clean`/`algorithm-optimization`'s job.)
</identity>

<mission>
Deliver **reproducible quantitative quality metrics.** Signal-to-noise ratio per diff. Cyclomatic entropy distribution. Redundancy ratio. Channel overhead. Every metric has a command or tool-backed procedure that produces it. Every measurement is comparable across commits and sessions when it uses the same scope and tool versions. Every quality regression is detectable as a metric change.
</mission>

## IssueOps Benchmark Artifact Contract

When Code Quality Metrics contributes to an IssueOps artifact or benchmark response, include a compact labeled evidence block. This is a measurement artifact, not a cleanup plan.

```text
Diff inventory: <staged, unstaged, and untracked files from git status>
SNR before/after: <baseline and post-cleanup values, or baseline only before cleanup>
Secondary metric: <entropy, redundancy, or channel overhead result>
Heuristic caveat: <AST-backed or approximate shell metric limitation>
No-input guard: <zero-diff/zero-line handling result>
```

Never use `git diff` alone as the scope when untracked files exist, and never divide by zero when there is no measurable input.

---

## The Code Quality Metrics Method: 4 Phases

```
Phase 0: BASELINE  — Measure current quality metrics
Phase 1: REGRESSION — Compare against previous checkpoint; detect degradation
Phase 2: TARGET     — Set quantitative goals for the cleanup pass
Phase 3: GATE       — Re-measure after cleanup; confirm improvement
```

---

## The 4 Quality Metrics

> **Command portability (read first).** The measurement commands below use POSIX ERE patterns like `grep '^\+[^+]'`.
> On hosts where `grep` is aliased to `ugrep` or `rg` (common in agent shells), these patterns FAIL with "invalid
> syntax" and the command silently returns a false `insufficient-input`/zero. If a pattern errors or a non-empty
> diff measures as zero, re-run with `command grep` (bypasses the alias) — e.g. `git diff HEAD | command grep -E '^\+[^+]'`.
>
> **Scope (code-only metrics).** SNR/Entropy/Redundancy/Channel-overhead assume source code. If the change set is
> predominantly markdown/docs/config (no source lines), say so and report these metrics as N/A — a "noise = comment"
> SNR is meaningless for prose. Fall back to a plain line-delta summary for non-code diffs.

### Metric 1: SNR — Signal-to-Noise Ratio

```
SNR = signal_lines / (signal_lines + noise_lines)

Signal lines: Lines that, if removed, cause a test failure or change observable behavior.
Noise lines:  Comments restating code, dead code, unreachable branches, pass-through wrappers,
              debug prints, commented-out code, speculative abstractions.
```

**How to measure current worktree changes:**

```bash
# 0. Capture the measurement scope, including staged, unstaged, and untracked files.
git status --short

# 1. Estimate changed tracked lines. This covers staged and unstaged tracked changes.
git diff --stat HEAD | tail -1

# 2. List untracked files separately. Include them in the input list and inspect them
# before scoring; `git diff` cannot measure files Git does not know about.
git ls-files --others --exclude-standard

# 3. Capture added tracked lines once so a failed git diff cannot become a
# plausible zero. awk returns success when there are no matching lines.
TRACKED_DIFF=$(git diff HEAD --) || exit
ADDED_LINES=$(printf '%s\n' "$TRACKED_DIFF" | command awk '/^\+[^+]/')

# 4. Count matches while distinguishing grep's legitimate no-match status (1)
# from a real grep error (>1).
count_matches() {
  pattern=$1
  count=$(command grep -cE "$pattern")
  status=$?
  case "$status" in
    0) printf '%s\n' "$count" ;;
    1) printf '0\n' ;;
    *) return "$status" ;;
  esac
}

NOISE=$(printf '%s\n' "$ADDED_LINES" | count_matches '^\+[[:space:]]*(//|#|/\*|\*|console\.(log|debug|info)|print\(|log\.(debug|info|warn))') || exit
TOTAL=$(printf '%s\n' "$ADDED_LINES" | command awk 'NF { count++ } END { print count + 0 }') || exit
SIGNAL=$((TOTAL - NOISE))
if [ "$TOTAL" -eq 0 ]; then
  echo "SNR: insufficient-input (signal=0, noise=0, total=0)"
else
  SNR=$(echo "scale=2; $SIGNAL / $TOTAL" | bc)
  echo "SNR: $SNR (signal=$SIGNAL, noise=$NOISE, total=$TOTAL)"
fi
```

**Interpretation:**
| SNR | Quality | Action |
|-----|---------|--------|
| ≥ 0.85 | Excellent | No cleanup needed |
| 0.70–0.85 | Good | Optional cleanup |
| 0.50–0.70 | Needs cleanup | Run ai-slop-clean before PR |
| < 0.50 | Poor | Block PR until cleanup improves SNR |

### Metric 2: Entropy — Cyclomatic Complexity Distribution

```
Entropy score = count of functions exceeding complexity ceiling.

Ceilings:
- Low-risk ceiling: 6 (yellow flag)
- High-risk ceiling: 12 (red flag — must be refactored)

Heuristic (does not require AST parser):
  Branch points per function = if + else + for + while + case + && + ||
  Count via grep -c on function bodies.
```

**How to measure:**

```bash
# Count branch points per function (Go example; adapt pattern to your language)
# Universal: count if/else/for/while/case in each function body
# Go: grep '^func '
# Python: grep '^def \|^    def '
# JavaScript/TypeScript: grep 'function \|=> {'
# Rust: grep '^fn \|^pub fn '

for file in *.go; do
  [ -f "$file" ] || continue
  rg -n '^func ' "$file" | while IFS=: read -r line signature; do
    name="$file:$line $signature"
    body=$(sed -n "${line},/^}/p" "$file")
    branches=$(echo "$body" | grep -cE '(^|[^[:alnum:]_])(if|else|for|while|case)([^[:alnum:]_]|$)')
    if [ "$branches" -gt 6 ]; then echo "WARN: $name — $branches branch points (ceiling: 6)"; fi
    if [ "$branches" -gt 12 ]; then echo "FAIL: $name — $branches branch points (ceiling: 12)"; fi
  done
done
# This file-aware heuristic stops at a top-level Go closing brace. Adapt both
# the function-line and closing-brace patterns for other languages.
```

### Metric 3: Redundancy — Duplicate Code Ratio

```
Redundancy ratio = pairs of blocks with >80% token similarity / total block pairs.

Heuristic (same-file only, no AST):
  Compare adjacent functions for line-count similarity and token overlap.
```

**How to measure:**

```bash
# Quick redundancy check: find functions with similar line counts (first-order approximation)
# Adapt the function-line pattern to your language:
#   Go: '^func '     Python: '^def \|^    def '     JS/TS: 'function \|=> {'     Rust: '^fn \|^pub fn '
for file in *.go; do
  [ -f "$file" ] || continue
  {
    rg -n '^func ' "$file" | while IFS=: read -r line signature; do
      name=$(printf '%s\n' "$signature" | sed 's/^func //' | cut -d'(' -f1)
      length=$(sed -n "${line},/^}/p" "$file" | wc -l | tr -d ' ')
      printf '%s:%s:%s:%s\n' "$file" "$line" "$length" "$name"
    done
  } | sort -t: -k3,3n
done
# Output fields are file:start-line:function-length:name. Within each file,
# adjacent rows with similar lengths are first-order redundancy candidates.
# For reliable results, use AST-based tools (below).
```

**For real measurement**, use AST-based tools:
```bash
# Go: golangci-lint dupl (threshold comes from the project config/tool default)
golangci-lint run --enable-only dupl ./...

# JavaScript/TypeScript: jscpd
npx jscpd --min-lines 6 --min-tokens 50 src/
```

### Metric 4: Channel Overhead — Boilerplate-to-Logic Ratio

```
Channel overhead = boilerplate_lines / (boilerplate_lines + logic_lines)

Boilerplate: imports, package/namespace declarations, serialization annotations,
             getter/setter pairs, constructor passthrough, DI wiring, middleware chain setup.
Logic:       Business rules, algorithmic code, type contracts, error handling with domain meaning.
```

**How to measure:**

```bash
# Quick overhead estimate per file
git diff --name-only HEAD
git ls-files --others --exclude-standard

FILE_LIST=$(mktemp)
trap 'rm -f "$FILE_LIST"' EXIT
git diff --name-only -z HEAD > "$FILE_LIST" || exit
git ls-files --others --exclude-standard -z >> "$FILE_LIST" || exit

while IFS= read -r -d '' f; do
  [ -f "$f" ] || continue
  TOTAL=$(wc -l < "$f")
  [ "$TOTAL" -gt 0 ] || { echo "SKIP EMPTY: $f"; continue; }
  BOILER=$(command grep -cE '^[[:space:]]*(import|package|@|//|/\*|type.*struct|func.*return|func \(.*\) .*return)' "$f")
  grep_status=$?
  case "$grep_status" in
    0) : ;;
    1) BOILER=0 ;;
    *) exit "$grep_status" ;;
  esac
  LOGIC=$((TOTAL - BOILER))
  OVERHEAD=$(echo "scale=2; $BOILER / $TOTAL" | bc)
  if (( $(echo "$OVERHEAD > 0.5" | bc -l) )); then
    echo "HIGH OVERHEAD: $f — $BOILER/$TOTAL lines boilerplate (overhead=$OVERHEAD)"
  fi
done < "$FILE_LIST"
```

---

## Phase 0: BASELINE — Measure Before Touching

Always measure before any cleanup pass. Record the baseline in a structured snapshot.

Write a valid JSON snapshot using the values captured above. Use `.issueops/evidence/code-quality-metrics/` only when the
project ignores that path; otherwise use issueops state or a temporary path so runtime evidence is not accidentally
committed. The object below is a schema example; replace its sample values rather than executing it as a shell heredoc.

```json
{
  "measured_at": "1970-01-01T00:00:00Z",
  "scope": "0123456789abcdef0123456789abcdef01234567",
  "git_status_short": "<captured from git status --short>",
  "changed_files": ["<staged, unstaged, and untracked files measured>"],
  "snr": {
    "signal_lines": 0,
    "noise_lines": 0,
    "snr": 0.0,
    "passed": false
  },
  "entropy": {
    "functions_exceeding_6": 0,
    "functions_exceeding_12": 0,
    "total_functions": 0,
    "passed": false
  },
  "redundancy": {
    "duplicate_blocks": 0,
    "passed": false
  },
  "channel_overhead": {
    "files_over_50pct_boilerplate": 0,
    "total_files": 0,
    "passed": false
  }
}
```

**Baseline pass/fail criteria:**
- SNR ≥ 0.60 (for pre-cleanup baseline; gate requires ≥ target)
- Zero functions exceeding 12 branch points
- Zero duplicate blocks (same-file > 6 identical lines)
- Zero files > 50% boilerplate

---

## Phase 1: REGRESSION — Compare Against History

Load the previous Code Quality Metrics snapshot from issueops state (or `.issueops/evidence/code-quality-metrics/`). Compare:

```bash
# Load previous checkpoint
issueops state read --key code-quality-metrics-latest 2>/dev/null || echo '{"snr":{"snr":0}}'

# Compare SNR
# Regression: SNR dropped by >0.10 → ai-slop-clean must target recovery first
# Improvement: SNR increased → record the improvement
```

**Regression signals that block PR:**
- SNR dropped > 0.10 from previous checkpoint
- Entropy score increased (more functions > 12 complexity)
- New redundant blocks detected
- Channel overhead increased > 10%

---

## Phase 2: TARGET — Set Quantitative Goals

Based on the baseline and regression check, produce a target card for the cleanup pass:

```markdown
## Code Quality Metrics Target Card

| Metric | Baseline | Target | Threshold |
|--------|----------|--------|-----------|
| SNR | 0.58 | ≥ 0.75 | ≥ 0.60 |
| Entropy (>6) | 4 functions | 0 functions | ≤ 1 |
| Entropy (>12) | 1 function | 0 functions | 0 |
| Redundancy | 2 blocks | 0 blocks | ≤ 1 |
| Overhead | 3 files | 0 files | ≤ 2 |

**Priority order** (fix these first):
1. SNR: remove noise lines (comments, dead code, debug prints)
2. Entropy: refactor functions > 12 branch points
3. Redundancy: deduplicate similar blocks
4. Overhead: reduce boilerplate in high-overhead files
```

Feed this target card to `ai-slop-clean` and `algorithm-optimization`. They clean; you re-measure.

---

## Phase 3: GATE — Re-measure After Cleanup

After cleanup is complete, re-run Phase 0 with the same commands. Compare baseline vs. after:

```bash
# Re-measure SNR
SIGNAL_AFTER=<N>
NOISE_AFTER=<N>
TOTAL_AFTER=$((SIGNAL_AFTER + NOISE_AFTER))
if [ "$TOTAL_AFTER" -eq 0 ]; then
  echo "SNR: insufficient-input (signal=0, noise=0, total=0)"
  exit 0
fi
SNR_AFTER=$(echo "scale=2; $SIGNAL_AFTER / $TOTAL_AFTER" | bc)

echo "SNR: $SNR_BASELINE → $SNR_AFTER"
echo "Pass: $(echo "$SNR_AFTER >= $SNR_TARGET" | bc -l)"
```

**Gate results format (for Verified Execution Quality Gate):**
```json
{
  "shannonAudit": {
    "snr": {"before": 0.58, "after": 0.74, "target": 0.60, "passed": true},
    "entropy": {"before": {"above_6": 4, "above_12": 1}, "after": {"above_6": 0, "above_12": 0}, "passed": true},
    "redundancy": {"before": 2, "after": 0, "passed": true},
    "channel_overhead": {"before": 3, "after": 0, "passed": true}
  },
  "overall": "PASS"
}
```

---

## Integration with IssueOps Workflow

```
ai-slop-clean phase:
  1. Code Quality Metrics Phase 0: BASELINE — measure SNR before touching anything
  2. Code Quality Metrics Phase 1: REGRESSION — compare against previous checkpoint
  3. Code Quality Metrics Phase 2: TARGET — produce target card with priority order
  4. ai-slop-clean: execute cleanup per target card priorities
  5. Code Quality Metrics Phase 3: GATE — re-measure, confirm improvement
  6. Record gate results as IssueOps feedback
```

---

## Quick Measurement Checklist (Pre-PR)

Run these commands and check against thresholds:

```bash
# 1. SNR (simple approximation)
git status --short
git diff --stat HEAD | tail -1
git ls-files --others --exclude-standard
# → If diff is large (>200 lines), run full SNR measurement

# 2. Oversized files (>250 LOC, adapt extension to language)
find . \( -name '*.go' -o -name '*.py' -o -name '*.ts' -o -name '*.rs' -o -name '*.java' \) \
  -not -path '*_test*' -not -path '*.test.*' | xargs wc -l | awk '$1 > 250 {print $2, $1 " lines (cap: 250)"}'
# → Flag any file > 250 LOC regardless of language

# 3. Deeply nested functions (Go example; adapt function-line pattern)
grep -rn '^func ' --include='*.go' . | while read line; do
  file=$(echo "$line" | cut -d: -f1)
  start=$(echo "$line" | cut -d: -f2)
  body=$(tail -n +$start "$file" | head -80)
  depth=$(echo "$body" | awk '{if(/^[\t ]*if|^[\t ]*for|^[\t ]*while/) d++} END{print d}')
  if [ "$depth" -gt 4 ]; then echo "WARN: $line — nesting depth ~$depth (cap: 4)"; fi
done
# This is necessarily language-specific (function detection). Use language-native tools for accuracy.

# 4. Test coverage (adapt to your language's coverage tool)
# Go:   go test -cover ./...
# Py:   pytest --cov --cov-report=term
# Node: npx jest --coverage
# Rust: cargo tarpaulin
go test -cover ./... 2>&1 | grep -E 'coverage: [0-9]'
# → Flag packages < 60% coverage

# 5. Dead code via unused. Use an installed or project-local tool. Ask before
# installing global tools; do not run `go install ...@latest` as a default.
if command -v staticcheck >/dev/null 2>&1; then
  # Preserve staticcheck's exit status and inspect all reported findings.
  # Filter U1000 only after recording whether the complete command succeeded.
  staticcheck ./...
else
  echo "staticcheck unavailable; use project-local tooling or ask before installing"
fi
# → Flag unused code
```

---

## Relationship with Other Skills

| Skill | How Code Quality Metrics integrates |
|-------|----------------------|
| **verified-execution** | Measurements from Code Quality Metrics feed into Verified Execution's Final Quality Gate as `shannonAudit`. Gate fails if SNR, entropy, redundancy, or overhead metrics don't meet targets. |
| **algorithm-optimization** | Algorithm Optimization uses Code Quality Metrics's entropy and redundancy measurements to prioritize algorithmic simplification. Code Quality Metrics's "high-entropy functions" become Algorithm Optimization's refactoring targets. |
| **issueops-debugging** | Code Quality Metrics's "signal lines" heuristic helps Debugging isolate failure causes: if a bug appeared but signal lines didn't change, the bug is likely environmental. |
| **implementation-planning** | Code Quality Metrics's baseline measurement during planning sets quality expectations for the plan's verification strategy. |
| **ai-slop-clean** | Code Quality Metrics measures before and after; ai-slop-clean performs the cleanup. Code Quality Metrics provides the target card; ai-slop-clean executes it. |

---

## Critical Rules

**NEVER:**
- Report "looks cleaner" as a quality improvement — use numbers
- Skip baseline measurement before cleanup (regression detection is impossible without it)
- Measure current work with `git diff` alone when untracked files exist
- Divide by zero or invent SNR when there are no changed lines
- Install global tools without explicit user approval
- Accept a cleanup pass that doesn't improve SNR
- Use a single metric in isolation — all 4 must pass
- Measure after cleanup without measuring before
- Claim quality improvement without re-running the exact same measurement commands

**ALWAYS:**
- Measure baseline BEFORE any cleanup (Phase 0)
- Capture `git status --short` and the staged/unstaged/untracked file list
- Label grep/shell metrics as approximate unless AST or test-backed evidence is used
- Compare against previous checkpoint (Phase 1)
- Set quantitative targets before cleanup begins (Phase 2)
- Re-measure with identical commands after cleanup (Phase 3)
- Record before/after metrics in the Code Quality Metrics snapshot
- Feed gate results to Verified Execution's quality gate record

**SHANNON'S PRINCIPLE:** "Information is the resolution of uncertainty." If you haven't reduced the uncertainty about whether the code is clean, you haven't measured anything.

## Stop Rules

- All 4 metrics pass thresholds + gate results recorded: **DONE**.
- SNR cannot be improved (all noise is structural, unavoidable): document the ceiling, mark SNR threshold as "waived with justification."
- Entropy metrics unchanged after algorithm-optimization refactoring: the functions are at their minimal complexity. Document.
- Measurement commands fail or produce inconsistent results: debug the measurement, not the code.

---

## IssueOps Integration

When an IssueOps cycle exists:

1. Code Quality Metrics runs at the start of `ai-slop-clean` phase: Phase 0 + 1 + 2
2. Code Quality Metrics runs at the end of `ai-slop-clean` phase: Phase 3 (GATE)
3. Record gate results:
   ```bash
   issueops feedback add --id "$ISSUEOPS_ID" --source code-quality-metrics \
     --body "SNR: 0.58→0.74. Entropy: 1 high-risk function→0. Redundancy: 2→0. Overhead: 3→0. GATE: PASS." --json
   ```
