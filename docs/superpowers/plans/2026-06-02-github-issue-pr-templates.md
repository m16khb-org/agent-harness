# GitHub and GitLab Issue and PR/MR Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add GitHub and GitLab issue and PR/MR templates so `agent-harness` remote artifacts preserve IssueOps-quality problem, evidence, scope, and verification data on both hosts.

**Architecture:** Keep this as repository metadata only. GitHub issue forms live under `.github/ISSUE_TEMPLATE/`, the GitHub pull request template lives at `.github/pull_request_template.md`, GitLab issue templates live under `.gitlab/issue_templates/`, and the GitLab merge request template lives under `.gitlab/merge_request_templates/`; no harness core, hook, MCP, or installer behavior changes.

**Tech Stack:** GitHub issue forms YAML, GitLab Markdown description templates, Markdown PR/MR templates, Python YAML validation.

---

### Task 1: Add Issue Forms

**Files:**
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/ISSUE_TEMPLATE/proposal.yml`
- Create: `.github/ISSUE_TEMPLATE/implementation_task.yml`
- Create: `.github/ISSUE_TEMPLATE/config.yml`
- Create: `.gitlab/issue_templates/bug_report.md`
- Create: `.gitlab/issue_templates/feature_request.md`
- Create: `.gitlab/issue_templates/proposal.md`
- Create: `.gitlab/issue_templates/implementation_task.md`

- [ ] **Step 1: Create bug report templates**

Add GitHub and GitLab bug templates requiring reproducible evidence: affected surface, version/context, steps, expected behavior, actual behavior, logs, and verification.

- [ ] **Step 2: Create feature request templates**

Add GitHub and GitLab feature templates requiring problem, user value, proposed behavior, alternatives, acceptance criteria, non-goals, and verification.

- [ ] **Step 3: Create proposal templates**

Add GitHub and GitLab proposal templates for architecture, API, host integration, or policy changes. Require context, decision, alternatives, compatibility, risks, and verification. Include a conditional diagram section only for flows, state transitions, or boundaries where a diagram reduces review effort.

- [ ] **Step 4: Create implementation task templates**

Add GitHub and GitLab IssueOps execution templates using the selected `implementation_task` name. Require problem, current evidence, acceptance criteria, non-goals, implementation scope, verification, risks/tradeoffs, and feedback log. Include a conditional diagram section that explicitly permits `필요 없음` for simple changes.

- [ ] **Step 5: Create `.github/ISSUE_TEMPLATE/config.yml`**

Disable blank issues by default and provide links for questions/security-style reports instead of letting low-context issues bypass the forms.

### Task 2: Add PR Template

**Files:**
- Create: `.github/pull_request_template.md`
- Create: `.gitlab/merge_request_templates/default.md`

- [ ] **Step 1: Create PR/MR templates**

Add Korean IssueOps-oriented sections for intent, linked issue, changes, verification, risk, scope management, conditional diagram use, worktree cleanup, and reviewer notes. Keep wording host-neutral by using PR/MR where possible.

### Task 3: Verify

**Files:**
- Verify: `.github/ISSUE_TEMPLATE/*.yml`
- Verify: `.github/pull_request_template.md`
- Verify: `.gitlab/issue_templates/*.md`
- Verify: `.gitlab/merge_request_templates/*.md`

- [ ] **Step 1: Parse all YAML forms**

Run:

```bash
python3 - <<'PY'
from pathlib import Path
import yaml

for path in sorted(Path(".github/ISSUE_TEMPLATE").glob("*.yml")):
    data = yaml.safe_load(path.read_text())
    assert isinstance(data, dict), path
    if path.name != "config.yml":
        assert data.get("name"), path
        assert data.get("description"), path
        assert isinstance(data.get("body"), list) and data["body"], path
print("issue form yaml ok")
PY
```

Expected: `issue form yaml ok`.

- [ ] **Step 2: Check required sections**

Run:

```bash
rg -n "문제|현재 근거|완료 기준|비목표|구현 범위|검증|위험|트레이드오프|피드백" .github/ISSUE_TEMPLATE/implementation_task.yml
rg -n "문제|현재 근거|완료 기준|비목표|구현 범위|검증|위험|트레이드오프|피드백" .gitlab/issue_templates/implementation_task.md
rg -n "이슈|검증|위험|범위 관리|다이어그램|워크트리 정리" .github/pull_request_template.md .gitlab/merge_request_templates/default.md
```

Expected: each command prints matching lines.

- [ ] **Step 3: Check diff hygiene**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only `.github/`, `.gitlab/`, and this plan are changed.
