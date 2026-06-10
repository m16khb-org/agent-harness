# Pioneer Skill Documentation Audit & Update

## TL;DR
> **Summary**: Update ARCHITECTURE.md and TECH_STACK.md to reference the 10 pioneer skills, verify language/tech agnosticism, and correct the archimedes false positive in research artifacts.
> **Deliverables**: Updated ARCHITECTURE.md, updated TECH_STACK.md, monoculture scan report, corrected research report
> **Effort**: Quick
> **Parallel**: NO (sequential dependency: scan → update → verify)
> **Critical Path**: T1 → T2, T3 → T4

## Context
### Original Request
IssueOps cycle `io-086195b77c2d` — pioneer skill ecosystem audit. GitHub issue #5.
### Interview Summary
- archimedes↔von-neumann was a **false positive**: archimedes only exists as Reasonix host built-in slash command, not as a file-based skill. Pioneer refactor (6f31c55) fully replaced it with von-neumann. No action needed.
- Real gaps: ARCHITECTURE.md and TECH_STACK.md (core project docs) have zero mentions of pioneer skills, while README.md has a full "Skills & Their Namesakes" table.
- Language/tech agnosticism was addressed in 6f31c55 but lacks automated verification.
### Gap Analysis
- ARCHITECTURE.md currently focuses on Go core / MCP / adapter boundaries — adding a "Skills layer" section fits naturally after section 7 (Codex/Claude integration map)
- TECH_STACK.md currently lists upstream companion tools but not project-native skills — adding a "Project skills" subsection under section 2.1 is the natural fit
- Both files use Korean as primary language — new content must match
- Monoculture scan: grep for `Go\|Node\.js\|Python` and `typescript\|TypeScript\|javascript\|JavaScript` patterns

## Work Objectives
### Core Objective
Update ARCHITECTURE.md and TECH_STACK.md so pioneer skills are discoverable from core project documentation, matching README.md coverage.
### Deliverables
- ARCHITECTURE.md section 9: "Pioneer Skills Layer"
- TECH_STACK.md section 2.2: "Project pioneer skills"
- Monoculture verification report (inline in plan)
- Corrected berners-lee research report
### Definition of Done
```bash
grep -c "von-neumann\|turing\|berners-lee\|codd\|dijkstra\|hopper\|karpathy\|shannon\|torvalds\|issueops" .agent-harness/ARCHITECTURE.md
# Expected: >0 matches for at least 3 skill names

grep -c "von-neumann\|turing\|berners-lee\|codd\|dijkstra\|hopper\|karpathy\|shannon\|torvalds\|issueops" .agent-harness/TECH_STACK.md
# Expected: >0 matches for at least 3 skill names

grep -ri 'go\|node\.js\|python\|typescript\|javascript' skills/*/SKILL.md | grep -v '#' | grep -v 'description:'
# Expected: no hardcoded language assumptions (0 matches for implementation-context mentions)
```
### Must Have
- ARCHITECTURE.md: pioneer skills section describing role, location, cross-reference mesh
- TECH_STACK.md: pioneer skills entry in technology stack table
- Verified zero hardcoded language/tech assumptions
### Must NOT Have
- Changes to any skill's SKILL.md content
- New skills or methodology changes
- Changes to Go core code

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after (documentation changes; verify with grep/structural checks)
- QA policy: Every task has agent-executed scenarios
- Evidence: `.agent-harness/evidence/task-{N}-pioneer-docs.{ext}`

## Execution Strategy
### Parallel Execution Waves
Wave 1: T1 (monoculture scan) — no dependencies
Wave 2: T2 (ARCHITECTURE.md), T3 (TECH_STACK.md) — after T1 confirms clean baseline
Wave 3: T4 (correct research report), T5 (final verification) — after T2, T3

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|-----------|--------|---------------------|
| T1   | —         | —      | —                    |
| T2   | T1        | T5     | T3                   |
| T3   | T1        | T5     | T2                   |
| T4   | T2, T3    | T5     | —                    |
| T5   | T2, T3, T4| —     | —                    |

## TODOs

- [ ] 1. Run language/tech monoculture scan

  **What to do**: Scan all 10 pioneer SKILL.md files for hardcoded language/technology assumptions (Go, Node.js, Python, TypeScript, JavaScript). Exclude YAML frontmatter and skill description lines from false positives.
  **Must NOT do**: Do not report "Go" mentions in code examples about the harness itself (which is written in Go) — only flag assumptions that would make the skill unusable for non-Go projects.

  **Recommended Agent**: quick
    Reason: Single grep command, no code changes.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: — | Blocked By: —

  **References**:
  - Commits: `6f31c55` — "refactor(skills): generalize pioneer skills to be language/tech-agnostic"
  - Files: `skills/*/SKILL.md`

  **Acceptance Criteria** (agent-executable only):
  - [ ] `grep -ri 'node\.js\|Node\.js\|typescript\|TypeScript\|javascript\|JavaScript' skills/*/SKILL.md | grep -v 'description:' | grep -v 'name:' | grep -v '# '` returns 0 results for implementation-assumption mentions
  - [ ] Report saved to `.agent-harness/evidence/task-1-monoculture-scan.txt`

  **QA Scenarios**:
  ```
  Scenario: Scan all skill files for language assumptions
    Channel: bash
    Steps:
      1. Run: grep -rniE '(node\.?js|typescript|javascript|python|golang|rust|java|ruby|php|csharp|c\+\+)' skills/*/SKILL.md | grep -vi 'description:' | grep -vi 'name:' | grep -vi '# research\|# relationship\|# issueops'
      2. For each match, classify as: "false-positive" (meta-reference), "code-example" (harness self-reference), or "assumption" (prescribes a language for skill users)
    Expected: Zero "assumption" classifications. All matches are false-positives or harness self-references.
    Evidence: .agent-harness/evidence/task-1-monoculture-scan.txt

  Scenario: Verify Go mentions are harness self-references only
    Channel: bash
    Steps: Run: grep -rni 'go\|golang' skills/*/SKILL.md | grep -vi 'description:\|name:\|# '
    Expected: All matches are references to agent-harness itself (written in Go) or general English words ("go to", "to go"), not language prescriptions.
    Evidence: .agent-harness/evidence/task-1-monoculture-scan.txt (appended)
  ```

  **Commit**: NO (evidence-only task)

- [ ] 2. Update ARCHITECTURE.md — add Pioneer Skills section

  **What to do**: Add a new section after section 7 (Codex / Claude integration map) titled "9. Pioneer Skills Layer". The section must: (a) list all 10 pioneer skills with their roles, (b) describe the cross-reference hub-and-spoke topology (turing + issueops as hubs), (c) note that skills live in `skills/` as single source of truth, (d) reference README.md "Skills & Their Namesakes" table for namesake details.
  **Must NOT do**: Do not modify sections 1-8. Do not change any existing content. Match existing Korean language style.

  **Recommended Agent**: quick
    Reason: Single-file markdown insertion, no code logic.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T5 | Blocked By: T1

  **References**:
  - File: `.agent-harness/ARCHITECTURE.md` — existing structure and Korean style
  - File: `README.md:131-148` — "Skills & Their Namesakes" table
  - File: `skills/issueops/SKILL.md:44-54` — phase-by-phase skill mapping table

  **Acceptance Criteria** (agent-executable only):
  - [ ] Section "9. Pioneer Skills Layer" exists in ARCHITECTURE.md
  - [ ] Lists at least 8 of 10 pioneer skills by name
  - [ ] Mentions cross-reference hub-and-spoke topology
  - [ ] References `skills/` directory as source of truth

  **QA Scenarios**:
  ```
  Scenario: Verify new section exists and is well-formed
    Channel: bash
    Steps:
      1. grep -n "^## 9. Pioneer Skills Layer" .agent-harness/ARCHITECTURE.md
      2. Count skill name mentions: grep -c "von-neumann\|turing\|berners-lee\|codd\|dijkstra\|hopper\|karpathy\|shannon\|torvalds" .agent-harness/ARCHITECTURE.md
    Expected: Section header found. At least 8 skill names mentioned (total count >= 8).
    Evidence: .agent-harness/evidence/task_two_architecture_section.txt
  ```

  **Commit**: YES | Message: `docs(architecture): add pioneer skills layer section` | Files: `.agent-harness/ARCHITECTURE.md`

- [ ] 3. Update TECH_STACK.md — add pioneer skills entry

  **What to do**: Add a subsection "2.2 Project pioneer skills" after section 2.1 (Upstream companion dependencies). Include a table listing each pioneer skill, its role, and its `skills/` path. Reference the README.md namesake table for detailed descriptions.
  **Must NOT do**: Do not modify sections 1, 2, 3-5. Do not change existing content.

  **Recommended Agent**: quick
    Reason: Single-file markdown table insertion.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T5 | Blocked By: T1

  **References**:
  - File: `.agent-harness/TECH_STACK.md` — existing sections 2 and 2.1
  - File: `README.md:131-148` — namesake table for role descriptions

  **Acceptance Criteria** (agent-executable only):
  - [ ] Section "2.2 Project pioneer skills" exists in TECH_STACK.md
  - [ ] Table with skill name, role, and skills/ path for at least 8 skills
  - [ ] References README.md for detailed namesake descriptions

  **QA Scenarios**:
  ```
  Scenario: Verify new section exists
    Channel: bash
    Steps:
      1. grep -n "2.2 Project pioneer skills" .agent-harness/TECH_STACK.md
      2. grep -c "skills/" .agent-harness/TECH_STACK.md
    Expected: Section header found. Multiple skills/ path references.
    Evidence: .agent-harness/evidence/task-3-techstack-section.txt
  ```

  **Commit**: YES | Message: `docs(tech-stack): add pioneer skills section` | Files: `.agent-harness/TECH_STACK.md`

- [ ] 4. Correct berners-lee research report

  **What to do**: Update `.agent-harness/research/pioneer-skill-audit.md` to correct the archimedes false positive: (a) change Finding 2 from "archimedes↔von-neumann Overlap" to "archimedes Successfully Replaced by von-neumann", (b) update Cross-Check Results table, (c) remove archimedes from Open Questions.
  **Must NOT do**: Do not change other findings (1, 3, 4, 5 remain valid).

  **Recommended Agent**: quick
    Reason: Small targeted edits to existing markdown report.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: T5 | Blocked By: T2, T3

  **References**:
  - File: `.agent-harness/research/pioneer-skill-audit.md`
  - Verification: `ls skills/archimedes` → NOT FOUND; `ls ~/.reasonix/skills/archimedes` → NOT FOUND

  **Acceptance Criteria** (agent-executable only):
  - [ ] Finding 2 renamed to "archimedes Successfully Replaced by von-neumann"
  - [ ] Confidence changed from "Medium" to "High"
  - [ ] archimedes removed from Open Questions
  - [ ] Cross-Check: previously "Single-sourced" item removed or reclassified

  **QA Scenarios**:
  ```
  Scenario: Verify corrections applied
    Channel: bash
    Steps:
      1. grep "archimedes" .agent-harness/research/pioneer-skill-audit.md
      2. grep "Single-sourced" .agent-harness/research/pioneer-skill-audit.md
    Expected: archimedes mentioned only in corrected Finding 2 (describing the replacement). Single-sourced count = 0.
    Evidence: .agent-harness/evidence/task_four_corrected_research.txt
  ```

  **Commit**: YES | Message: `docs(research): correct archimedes false positive in pioneer audit` | Files: `.agent-harness/research/pioneer-skill-audit.md`

- [ ] 5. Final verification gate

  **What to do**: Run all acceptance criteria commands from T1-T4. Verify ARCHITECTURE.md and TECH_STACK.md are well-formed. Run `git diff --stat` to confirm only target files changed. Run `go test ./... -count=1` to confirm no regressions.
  **Must NOT do**: Do not run full self-verify loop (out of scope). Do not commit — waiting for user approval.

  **Recommended Agent**: quick
    Reason: Verification-only, no file changes.

  **Parallelization**: Can Parallel: NO | Wave 3 | Blocks: — | Blocked By: T2, T3, T4

  **References**:
  - All previous task evidence files
  - Command: `go test ./... -count=1`

  **Acceptance Criteria** (agent-executable only):
  - [ ] `grep -c "von-neumann\|turing\|berners-lee" .agent-harness/ARCHITECTURE.md` >= 3
  - [ ] `grep -c "von-neumann\|turing\|berners-lee" .agent-harness/TECH_STACK.md` >= 3
  - [ ] `go test ./... -count=1` passes (no regressions from doc-only changes)
  - [ ] `git diff --stat` shows only: ARCHITECTURE.md, TECH_STACK.md, .agent-harness/research/pioneer-skill-audit.md, .agent-harness/plans/, .agent-harness/evidence/

  **QA Scenarios**:
  ```
  Scenario: All docs valid and no regressions
    Channel: bash
    Steps:
      1. grep pioneer skills in both ARCHITECTURE.md and TECH_STACK.md
      2. go test ./... -count=1
      3. git diff --stat
    Expected: Skill names found in both files. All tests pass. Diff contains only documentation files.
    Evidence: .agent-harness/evidence/task-5-final-gate.txt
  ```

  **Commit**: NO (user decision gate)

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
- [ ] F1. Plan Compliance Audit — every TODO executed as specified?
- [ ] F2. Code Quality Review — no AI slop, no dead code, no overbroad abstractions?
- [ ] F3. Real Manual QA — every scenario PASS with captured evidence?
- [ ] F4. Scope Fidelity Check — no scope creep, no missed deliverables?

## Commit Strategy
Two commits, atomic per conventional commit policy:
1. `docs(architecture): add pioneer skills layer section`
2. `docs(tech-stack): add pioneer skills section`
3. `docs(research): correct archimedes false positive in pioneer audit`
(Evidence files in `.agent-harness/evidence/` not committed)

## Success Criteria
- ARCHITECTURE.md section 9 references pioneer skills
- TECH_STACK.md section 2.2 references pioneer skills
- Zero hardcoded language/tech assumptions in any skill file
- Research report corrected for archimedes false positive
- All Go tests pass (no regressions)
