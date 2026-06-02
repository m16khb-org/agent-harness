# Gemini Code Assist 리뷰 설정 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**목표:** Gemini Code Assist가 `agent-harness` PR을 리뷰할 때 저장소의 핵심 위험과 운영 규칙을 반영하도록 `.gemini/` 설정과 스타일 가이드를 추가한다.

**아키텍처:** GitHub App 설치나 CI 자동화는 외부 연결 절차로 남기고, 저장소에는 Gemini가 읽는 정적 설정만 추가한다. 설정은 `.gemini/config.yaml`, 리뷰 기준은 `.gemini/styleguide.md`에 둔다.

**기술 스택:** Gemini Code Assist repository configuration, YAML, Markdown.

**IssueOps Context:** Issue `https://github.com/m16khb/agent-harness/issues/5`; branch `chore/5-configure-gemini-code-assist-review`; worktree `/Users/m16khb/Workspace/agent-harness.worktrees/chore-5-configure-gemini-code-assist-review`.

---

## 파일 구조

- Create: `.gemini/config.yaml` - Gemini Code Assist PR 리뷰 설정과 ignore pattern.
- Create: `.gemini/styleguide.md` - agent-harness 전용 리뷰 기준.
- Create: `plans/issue-5-gemini-code-assist-review.md` - IssueOps 내부 구현 계획. GitHub issue 본문에는 링크하지 않는다.

---

### Task 1: Gemini 저장소 설정 추가

**Files:**
- Create: `.gemini/config.yaml`

- [ ] **Step 1: 설정 파일 작성**

`.gemini/config.yaml`에 코드 리뷰 활성화, PR open summary/review 활성화, draft PR 포함, 생성물/런타임 파일 ignore pattern을 작성한다.

- [ ] **Step 2: YAML 파싱 검증**

Run: `python3 -c 'import yaml; yaml.safe_load(open(".gemini/config.yaml")); print("yaml ok")'`

Expected: `yaml ok`

---

### Task 2: agent-harness 리뷰 스타일 가이드 추가

**Files:**
- Create: `.gemini/styleguide.md`

- [ ] **Step 1: 스타일 가이드 작성**

다음 리뷰 기준을 포함한다.

- Go core behavior는 가능한 host-neutral `internal/core`/`internal/port`에 둔다.
- host adapter는 정책을 복제하지 않고 얇은 wrapper로 유지한다.
- Codex/Claude hook stdout schema는 호스트별 호환성을 검증한다.
- user state/log/queue/audit output은 secret redaction을 거친다.
- IssueOps 작업은 sibling worktree 규칙을 지킨다.
- PR 범위 밖의 리팩터링과 speculative abstraction을 지적한다.

- [ ] **Step 2: 필수 문구 검증**

Run: `rg -n "host-neutral|hook stdout|redaction|IssueOps|worktree|speculative" .gemini/styleguide.md`

Expected: 각 필수 주제가 출력된다.

---

### Task 3: 최종 검증과 PR 준비

**Files:**
- Create: `.gemini/config.yaml`
- Create: `.gemini/styleguide.md`
- Create: `plans/issue-5-gemini-code-assist-review.md`

- [ ] **Step 1: 변경 파일 확인**

Run: `git status --short`

Expected: `.gemini/config.yaml`, `.gemini/styleguide.md`, `plans/issue-5-gemini-code-assist-review.md`만 변경된다.

- [ ] **Step 2: 전체 영향 확인**

Run: `git diff --check`

Expected: 출력 없음.

- [ ] **Step 3: PR 본문은 한글로 작성**

PR에는 issue #5, 변경 요약, 검증 명령, Gemini App/Enterprise 연결은 외부 설치가 필요하다는 제한을 한글로 적는다. GitHub issue 본문에는 계획 링크를 작성하지 않는다.
