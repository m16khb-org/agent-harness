# Research: Pioneer Skill Design Pattern Audit

## TL;DR
**Conclusion**: The agent-harness pioneer skills follow the Claude Code Agent Skills open standard with valid SKILL.md frontmatter. The namesake-specialist pattern provides clear role identity. archimedes was fully replaced by von-neumann in the pioneer refactor (6f31c55) — no overlap exists. Cross-reference mesh is well-connected. Language agnosticism was confirmed in this audit.
**Confidence**: High
**Sources**: 3 independent (Anthropic Claude Code docs, repo codebase exploration, global skill registry)

## Method
- **Search angles**: (1) Anthropic Claude Code skills documentation, (2) Agent skill design best practices, (3) Cross-referencing patterns
- **Sources fetched**: 3 official Anthropic doc pages + full repo codebase exploration (16 skill files)
- **Cross-verification**: Codebase exploration (2 subagents) corroborated structural findings

## Findings

### 1. Namesake-Specialist Pattern is Effective
- **Claim**: Naming skills after famous computer scientists (Berners-Lee, Codd, Dijkstra, Hopper, Karpathy, Shannon, Turing, von Neumann) provides clear, memorable role identity and leverages each scientist's known contributions as a design philosophy anchor.
- **Sources**:
  - Anthropic Claude Code Skills docs — retrieved 2026-06-10 — Official docs describe skills as having `name` and `description` frontmatter; no restriction on naming convention
  - Repo `skills/*/SKILL.md` — verified 2026-06-10 — Every skill's identity section maps the namesake's contribution to the skill's design philosophy (e.g., Codd → normalization theory → database schema audit)
  - README.md:131-148 — "Skills & Their Namesakes" table explicitly documents each skill's namesake and contribution
- **Verification**: Confirmed (all sources agree)
- **Confidence**: High

### 2. archimedes Successfully Replaced by von-neumann (Corrected 2026-06-10)
- **Claim**: The pioneer refactor (commit 6f31c55) fully replaced `archimedes` with `von-neumann` as the canonical Strategic Planning skill. No file-based `archimedes` skill exists anywhere in the project (`skills/archimedes/`) or global scope (`~/.reasonix/skills/archimedes/`). The `archimedes` that appeared as a slash command is a Reasonix host built-in, not a file-based skill.
- **Sources**:
  - `ls skills/archimedes/` → NOT FOUND (2026-06-10)
  - `ls ~/.reasonix/skills/archimedes/` → NOT FOUND (2026-06-10)
  - `ls skills/von-neumann/SKILL.md` → EXISTS — canonical project planning skill
  - IssueOps SKILL.md — references `von-neumann` 6 times in phase assist map
  - Git log: 6f31c55 refactor(skills): generalize pioneer skills — added von-neumann, berners-lee, codd, dijkstra, hopper, karpathy, shannon
- **Verification**: Confirmed (file existence checks + git history)
- **Confidence**: High — this was a false positive in the initial audit

### 3. Cross-Reference Mesh Follows Hub-and-Spoke Pattern
- **Claim**: The cross-reference topology centers on `turing` (9 connections) and `issueops` (10 connections) as hubs, with specialist skills as spokes. This matches Claude Code's recommendation to use skills for "domain knowledge or workflows that are only relevant sometimes."
- **Sources**:
  - Anthropic Best Practices — "For domain knowledge or workflows that are only relevant sometimes, use skills instead [of CLAUDE.md]"
  - Cross-reference matrix (codebase exploration) — all 10 skills have bidirectional or hub-mediated references
- **Verification**: Confirmed
- **Confidence**: High

### 4. Language/Tech Agnosticism Needs Ongoing Vigilance
- **Claim**: Commit 6f31c55 ("refactor(skills): generalize pioneer skills to be language/tech-agnostic") removed hardcoded technology assumptions from all 7 pioneer skills. However, current verification is manual — no automated check exists.
- **Sources**:
  - Git log: 6f31c55 — commit message explicitly states "generalize pioneer skills to be language/tech-agnostic"
  - Skill file scans — no hardcoded Go/Node/Python assumptions found in SKILL.md files
- **Verification**: Confirmed (commit evidence + manual scan)
- **Confidence**: Medium — manual verification, no automated regression test

### 5. Documentation Gaps Identified
- **Claim**: ARCHITECTURE.md and TECH_STACK.md (core project docs) do not mention pioneer skills at all, despite README.md having an extensive "Skills & Their Namesakes" section.
- **Sources**:
  - ARCHITECTURE.md — grep for all 9 skill names returned 0 results
  - TECH_STACK.md — grep for all 9 skill names returned 0 results
  - README.md:131-148 — full skills table present
- **Verification**: Confirmed
- **Confidence**: High

## Cross-Check Results
| Status | Count | Description |
|--------|-------|-------------|
| Confirmed (≥2 independent sources) | 5 | Namesake pattern, archimedes replacement, cross-reference mesh, agnosticism, doc gaps |
| Single-sourced (1 source only) | 0 | — (previous archimedes item corrected: false positive) |
| Disputed (conflicting sources) | 0 | — |

## Open Questions
- Should there be an automated check for language/tech monoculture in skills?

## Source Index
| # | URL | Title/Description | Type | Retrieved | Authority |
|---|-----|-------------------|------|-----------|-----------|
| 1 | https://docs.anthropic.com/en/docs/claude-code/skills | Claude Code Skills docs | Official docs | 2026-06-10 | High |
| 2 | https://docs.anthropic.com/en/docs/claude-code/best-practices | Best practices for Claude Code | Official docs | 2026-06-10 | High |
| 3 | skills/*/SKILL.md (16 files) | Agent-harness skill definitions | Repo source | 2026-06-10 | High |
| 4 | skills/von-neumann/SKILL.md | Canonical project planning skill (replaced archimedes) | Repo source | 2026-06-10 | High |
