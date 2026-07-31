# IssueOps Child Help Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task in the current main session. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `issueops child --help`가 실제 child parser와 동일한 actor 플래그 계약을 canonical usage catalog에서 렌더하도록 수정한다.

**Architecture:** `internal/adapter/cli`의 기존 `IssueOpsUsageLines`와 `IssueOpsUsageKey`를 단일 원본으로 유지한다. `cmd/harness/issueopscli`는 정확한 child 명령 key 여섯 개만 선택해 공용 `IssueOpsActorFlagLegend`와 결합하며, parser·lifecycle·cleanup 구현은 변경하지 않는다.

**Tech Stack:** Go 1.26, 표준 `flag`/`strings` 패키지, Go `testing`, 기존 IssueOps CLI adapter.

## Global Constraints

- 모든 새 코드 주석은 한글로 작성한다.
- 로컬 전체 `go test ./...`와 전체 race는 실행하지 않는다.
- parser capability, actor 권한, write lease, persisted schema, cleanup 의미를 변경하지 않는다.
- child help의 명령 순서는 canonical catalog 순서를 따른다.
- `link-child`는 child 하위 명령이 아니므로 child help에 포함하지 않는다.
- 구현은 현재 메인 세션이 직접 수행하고 구현 sub-agent에 위임하지 않는다.

---

### Task 0: Publish the approved implementation plan

**Files:**
- Create: `docs/superpowers/plans/2026-07-31-issueops-child-help-parity.md`

**Interfaces:**
- Consumes: 승인된 design spec와 Brooks revise findings.
- Produces: 구현 전에 branch에 추적되는 decision-complete plan commit.

- [ ] **Step 1: 계획 자체검토를 다시 실행한다**

Run:

```bash
git diff --check
rg -n 'T[B]D|T[O]DO|F[I]XME|implement l[a]ter|fill in det[a]ils|Similar to T[a]sk' \
  docs/superpowers/plans/2026-07-31-issueops-child-help-parity.md
```

Expected: `git diff --check`는 exit 0이고 `rg`는 match 없이 exit 1이다.

- [ ] **Step 2: plan 파일만 별도 docs commit으로 기록한다**

```bash
git add -- docs/superpowers/plans/2026-07-31-issueops-child-help-parity.md
git diff --cached --check
git diff --cached
git commit -m "docs(issueops): define child help implementation plan" -m "Lore:
- Intent: Make the reviewed child-help implementation sequence executable and auditable.
- Why: TDD, exact scope evidence, implementation review, and lifecycle cleanup need a durable plan before code changes.
- Changes:
  - Define the canonical help projection RED/GREEN cycle.
  - Separate publication evidence from post-merge lifecycle cleanup.
- Verify: plan placeholder scan and git diff checks pass; Brooks review passes.
- Risk: Low; documentation-only plan commit."
```

### Task 1: Canonical child help projection

**Files:**
- Create: `cmd/harness/issueopscli/issueops_child_usage_parity_test.go`
- Modify: `cmd/harness/issueopscli/issueops_cli_support.go:29-37`
- Modify: `cmd/harness/issueopscli/issueops_subcommands.go:111-115`

**Interfaces:**
- Consumes: `cliadapter.IssueOpsUsageLines() []string`, `cliadapter.IssueOpsUsageKey(string) string`, `cliadapter.IssueOpsActorFlagLegend string`.
- Produces: `issueOpsChildUsageText() string`, child help의 완성된 문자열.

- [ ] **Step 1: canonical catalog와 child help의 정확한 parity를 요구하는 실패 테스트를 작성한다**

```go
package issueopscli

import (
	"strings"
	"testing"

	cliadapter "agent-harness/internal/adapter/cli"
)

func TestIssueOpsChildUsageMatchesCanonicalCatalog(t *testing.T) {
	counts := map[string]int{
		"child start":  0,
		"child status": 0,
		"child list":   0,
		"child accept": 0,
		"child reject": 0,
		"child drop":   0,
	}
	var lines []string
	for _, line := range cliadapter.IssueOpsUsageLines() {
		key := cliadapter.IssueOpsUsageKey(line)
		if _, selected := counts[key]; !selected {
			continue
		}
		counts[key]++
		lines = append(lines, line)
	}
	for key, count := range counts {
		if count != 1 {
			t.Fatalf("canonical catalog에서 %q 줄을 정확히 하나 기대했지만 %d개다", key, count)
		}
	}
	want := "Usage:\n" + strings.Join(lines, "\n") + "\n\n" +
		cliadapter.IssueOpsActorFlagLegend
	got := strings.TrimSuffix(captureStdoutForContract(t, func() error {
		return runIssueOpsChild([]string{"--help"})
	}), "\n")
	if got != want {
		t.Fatalf("child help가 canonical catalog projection과 다르다\nwant:\n%s\n\ngot:\n%s", want, got)
	}
	for _, line := range lines {
		if !strings.Contains(line, " RECORD_ACTOR_FLAGS ") {
			t.Fatalf("child usage 줄이 RECORD_ACTOR_FLAGS를 숨긴다: %s", line)
		}
	}
	if legendLine(got, "RECORD_ACTOR_FLAGS") == "" {
		t.Fatal("child help가 RECORD_ACTOR_FLAGS 범례를 정의하지 않는다")
	}
	if strings.Contains(got, "\n  agent-harness issueops link-child ") {
		t.Fatal("child help에 비-child link-child 명령이 포함됐다")
	}
}
```

- [ ] **Step 2: 새 focused test가 현재 코드에서 RED임을 확인한다**

Run:

```bash
go test ./cmd/harness/issueopscli -run TestIssueOpsChildUsageMatchesCanonicalCatalog -count=1
```

Expected: test가 실행되고 기존 별도 상수 출력이 canonical projection과 달라 실패한다. diff에는
여섯 줄의 `RECORD_ACTOR_FLAGS`와 공용 범례 누락이 보여야 한다.

- [ ] **Step 3: child help를 canonical catalog의 정확한 key projection으로 구현한다**

`cmd/harness/issueopscli/issueops_cli_support.go`의 `issueOpsChildUsage` 상수를 다음 함수로
대체한다.

```go
// issueOpsChildUsageText는 canonical catalog에서 child 하위 명령만 골라 렌더한다.
// usage 문장을 다시 적지 않아 parser/help 계약의 별도 drift를 막는다(#207).
func issueOpsChildUsageText() string {
	var lines []string
	for _, line := range cliadapter.IssueOpsUsageLines() {
		switch cliadapter.IssueOpsUsageKey(line) {
		case "child start", "child status", "child list",
			"child accept", "child reject", "child drop":
			lines = append(lines, line)
		}
	}
	return "Usage:\n" +
		strings.Join(lines, "\n") + "\n\n" +
		cliadapter.IssueOpsActorFlagLegend
}
```

`cmd/harness/issueopscli/issueops_subcommands.go`의 help 분기는 계산된 문자열을 출력한다.

```go
func runIssueOpsChild(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(issueOpsChildUsageText())
		return nil
	}
```

- [ ] **Step 4: RED test와 기존 usage parity tests를 GREEN으로 만든다**

Run:

```bash
go test ./cmd/harness/issueopscli -run 'TestIssueOpsChildUsageMatchesCanonicalCatalog|TestUsageTextsDefineActorFlagShorthand|TestCommandsRequiringActorDiscloseItInUsage|TestIssueOpsUsageMatchesAdapterUsage' -count=1
```

Expected: `ok agent-harness/cmd/harness/issueopscli`.

- [ ] **Step 5: 범위 package와 contract golden을 focused 검증한다**

Run:

```bash
go test ./cmd/harness/issueopscli/... ./internal/adapter/cli/... -count=1
go test ./cmd/harness/contractgolden -run Golden -count=1
go build -o bin/agent-harness ./cmd/harness
./bin/agent-harness issueops child --help
```

Expected:

- 모든 test command가 exit 0이다.
- build가 성공한다.
- child 여섯 usage 줄 모두 `RECORD_ACTOR_FLAGS`를 포함한다.
- 출력 하단에 `RECORD_ACTOR_FLAGS: --host codex|claude ... --cwd PATH` 범례가 있다.
- `link-child`는 출력에 없다.

- [ ] **Step 6: 구현과 직접 테스트를 하나의 atomic commit으로 커밋한다**

```bash
git add -- \
  cmd/harness/issueopscli/issueops_child_usage_parity_test.go \
  cmd/harness/issueopscli/issueops_cli_support.go \
  cmd/harness/issueopscli/issueops_subcommands.go
git diff --cached --check
git diff --cached
git commit -m "fix(issueops): align child help actor flags" -m "Lore:
- Intent: Render child help from the canonical IssueOps usage contract.
- Why: Child parsers accept RECORD_ACTOR_FLAGS but their nearest help surface hides them and the legend.
- Changes:
  - Project the six child command lines from the canonical catalog.
  - Add focused parity coverage for commands, actor flags, legend, and link-child exclusion.
- Verify: focused issueopscli, adapter CLI, contract golden, build, and child help readback pass.
- Risk: Low; parser, lifecycle, persistence, and cleanup behavior are unchanged."
```

### Task 2: IssueOps review and publication evidence

**Files:**
- Modify only if review finds a valid defect: the three Task 1 paths.
- No planned production file outside Task 1.

**Interfaces:**
- Consumes: Task 1 commit and its focused verification output.
- Produces: IssueOps compatibility record, implementation review, PR evidence, and a merge-ready branch.

- [ ] **Step 1: 호환성 검토를 IssueOps에 기록한다**

Record:

- backward compatibility: 기존 parser와 JSON/persisted state는 불변이다.
- side effect: child help 출력에 이미 지원되는 actor 계약만 추가된다.
- rollback: Task 1 commit revert.
- verification: focused parity, golden, build, runtime help readback.

- [ ] **Step 2: AI slop clean에서 변경 범위를 다시 제한한다**

Check:

```bash
git status --short
git diff --name-only origin/main...HEAD | sort
git diff --check origin/main...HEAD
```

Expected:

- `git status --short`가 비어 있다.
- changed path 목록은 다음 다섯 경로와 정확히 같다.

```text
cmd/harness/issueopscli/issueops_child_usage_parity_test.go
cmd/harness/issueopscli/issueops_cli_support.go
cmd/harness/issueopscli/issueops_subcommands.go
docs/superpowers/plans/2026-07-31-issueops-child-help-parity.md
docs/superpowers/specs/2026-07-31-issueops-child-help-parity-design.md
```

- production diff는 parser/lifecycle/cleanup을 건드리지 않는다.

- [ ] **Step 3: 독립 implementation review를 실행하고 IssueOps에 기록한다**

Read-only reviewer는 `origin/main...HEAD`의 다섯 changed path와 focused verification 출력을
검토한다. verdict가 `revise` 또는 `stop`이면 publication을 중단하고 유효한 finding을
TDD로 수정한 뒤 review를 다시 실행한다. `pass`일 때만 메인 holder가 다음 record를 남긴다.

```bash
/Users/habin/workspace/agent-harness/bin/agent-harness issueops implementation-review record \
  --id io-60af1c5c4367 \
  --verdict pass \
  --finding '독립 구현 검토에서 blocking finding이 없다.' \
  --evidence 'child help actual stdout이 canonical six-command projection 및 actor legend와 일치한다.' \
  --evidence 'focused issueopscli, adapter CLI, contract golden, build 검증이 통과했다.' \
  --reviewer-host codex \
  --reviewer-model gpt-5.6-sol \
  --reviewer-effort xhigh \
  --host codex \
  --session-id 019fa0f7-65da-7cb3-a7b6-6db05b21f4b5 \
  --cwd /Users/habin/workspace/agent-harness.worktrees/207-issueops-child-help-parity \
  --json
```

- [ ] **Step 4: 원격 PR과 CI를 검증한다**

Expected:

- PR base는 `main`, head는 `207-issueops-child-help-parity`다.
- PR label은 `bug`, assignee는 `m16khb`다.
- 원격 CI 전체가 성공한다.

## Post-plan IssueOps lifecycle checklist

이 절차는 feature 구현 task가 아니라 원격 publication이 성공한 뒤 수행하는 lifecycle
마감이다. CI와 merge authority에 순차 의존하므로 Task 2와 한 단계로 묶지 않는다.

- [ ] 원격 CI 성공 뒤 PR을 `main`에 merge하고 merge SHA를 다시 읽는다.
- [ ] #207 completion을 원격 이슈에 반영하고 CLOSED 상태를 검증한다.
- [ ] remote branch 삭제를 별도 확인한다.
- [ ] IssueOps `cleanup finish` preview/apply로 canonical worktree·local branch·lifecycle record를
  한 번에 정리하고, 이후 세 대상의 부재를 각각 Git/IssueOps inventory로 검증한다.
- [ ] 최신 `main`으로 설치·daemon/MCP를 갱신하고 child help readback을 다시 확인한다.
