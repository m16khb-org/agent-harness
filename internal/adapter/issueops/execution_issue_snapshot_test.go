package issueops

import (
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// parseGitLabExecutionIssueIdentity는 실행 스냅샷 신원 검증의 근간이다.
// 정식 HTTPS URL만 받아들이고, 프로젝트 경로/IID 정규형을 강제한다.
func TestParseGitLabExecutionIssueIdentity(t *testing.T) {
	valid := map[string]gitLabExecutionIssueIdentity{
		"https://gitlab.com/acme/repo/-/issues/42":    {authority: "gitlab.com", project: "acme/repo", iid: "42"},
		"https://GitLab.com/acme/repo/-/work_items/7": {authority: "gitlab.com", project: "acme/repo", iid: "7"},
		"https://gl.example.co.kr/a/b/c/-/issues/1":   {authority: "gl.example.co.kr", project: "a/b/c", iid: "1"},
	}
	for raw, want := range valid {
		got, err := parseGitLabExecutionIssueIdentity(raw)
		if err != nil || got != want {
			t.Fatalf("parse(%q) = %+v, %v want %+v", raw, got, err, want)
		}
	}
	invalid := []string{
		"", "  https://gitlab.com/a/b/-/issues/1  ",
		"http://gitlab.com/a/b/-/issues/1",
		"https://gitlab.com/a/b/-/issues/1?x=1",
		"https://gitlab.com/a/b/-/issues/1#frag",
		"https://user@gitlab.com/a/b/-/issues/1",
		"https://gitlab.com/a/b/-/merge_requests/1",
		"https://gitlab.com/a/b/-/issues/0",
		"https://gitlab.com/a/b/-/issues/007",
		"https://gitlab.com/a/b/-/issues/1.0",
		"https://gitlab.com/-/issues/1",
		"https://gitlab.com/a/../b/-/issues/1",
		"https://gitlab.com/a/b/-/issues/1/",
		// 인코딩된 슬래시(%2F)는 디코딩 결과가 경로 구분자라 프로젝트
		// 경로 정규형 위반으로 거부된다(fail-closed 계약).
		"https://gitlab.com/acme/re%2Fpo/-/issues/9",
	}
	for _, raw := range invalid {
		if _, err := parseGitLabExecutionIssueIdentity(raw); err == nil {
			t.Fatalf("parse(%q) must fail closed", raw)
		}
	}
}

func TestSameGitLabExecutionIssueIdentity(t *testing.T) {
	base := "https://gitlab.com/acme/repo/-/issues/42"
	if !sameGitLabExecutionIssueIdentity(base, "https://gitlab.com/acme/repo/-/issues/42") {
		t.Fatal("identical identities must match")
	}
	if sameGitLabExecutionIssueIdentity(base, "https://gitlab.com/acme/repo/-/issues/43") ||
		sameGitLabExecutionIssueIdentity(base, "https://other.com/acme/repo/-/issues/42") ||
		sameGitLabExecutionIssueIdentity(base, "https://gitlab.com/acme/other/-/issues/42") ||
		sameGitLabExecutionIssueIdentity(base, "not-a-url") ||
		sameGitLabExecutionIssueIdentity("broken", base) {
		t.Fatal("distinct or invalid identities must not match")
	}
}

func TestValidateGitLabExecutionIssueSnapshot(t *testing.T) {
	base := "https://gitlab.com/acme/repo/-/issues/42"
	if err := validateGitLabExecutionIssueSnapshot(base, port.ExecutionIssueSnapshot{
		URL: base, Body: "설명", State: "opened",
	}); err != nil {
		t.Fatalf("valid snapshot must pass: %v", err)
	}
	cases := []struct {
		name string
		snap port.ExecutionIssueSnapshot
	}{
		{"url mismatch", port.ExecutionIssueSnapshot{URL: "https://gitlab.com/acme/repo/-/issues/43", Body: "b", State: "opened"}},
		{"empty body", port.ExecutionIssueSnapshot{URL: base, Body: "  ", State: "opened"}},
		{"oversized body", port.ExecutionIssueSnapshot{URL: base, Body: strings.Repeat("x", executionIssueSnapshotBodyLimit+1), State: "opened"}},
		{"bad state", port.ExecutionIssueSnapshot{URL: base, Body: "b", State: "merged"}},
	}
	for _, tc := range cases {
		if err := validateGitLabExecutionIssueSnapshot(base, tc.snap); err == nil {
			t.Fatalf("%s must fail closed", tc.name)
		}
	}
}

func TestValidateExecutionIssueSnapshotActionAndRecord(t *testing.T) {
	for _, action := range []string{string(ExecutionActionPrepare), string(ExecutionActionClaim)} {
		if err := validateExecutionIssueSnapshotAction(ExecutionActionRequest{Action: action}); err != nil {
			t.Fatalf("action %s must pass: %v", action, err)
		}
	}
	if err := validateExecutionIssueSnapshotAction(ExecutionActionRequest{
		Action: ExecutionActionReconcile, Confirm: true, Preview: false,
	}); err != nil {
		t.Fatalf("confirm reconcile must pass: %v", err)
	}
	if err := validateExecutionIssueSnapshotAction(ExecutionActionRequest{Action: ExecutionActionReconcile}); err == nil {
		t.Fatal("preview reconcile must fail")
	}
	if err := validateExecutionIssueSnapshotAction(ExecutionActionRequest{
		Action: ExecutionActionReplace, ReplaceAction: ExecutionReplaceFinalize,
	}); err != nil {
		t.Fatalf("replace finalize must pass: %v", err)
	}
	if err := validateExecutionIssueSnapshotAction(ExecutionActionRequest{Action: ExecutionActionRelease}); err == nil {
		t.Fatal("release must fail closed")
	}

	// reconcile은 orca pending worktree_create에서만 허용된다.
	if err := validateExecutionIssueSnapshotRecord(
		ExecutionActionRequest{Action: ExecutionActionReconcile, Confirm: true},
		issueops.IssueOpsRecord{Execution: &issueops.Execution{
			Mode:    issueops.ExecutionModeOrca,
			Pending: &issueops.ExternalIntent{Kind: "worktree_create"},
		}},
	); err != nil {
		t.Fatalf("orca pending record must pass: %v", err)
	}
	for name, record := range map[string]issueops.IssueOpsRecord{
		"direct mode": {Execution: &issueops.Execution{Mode: issueops.ExecutionModeDirect, Pending: &issueops.ExternalIntent{Kind: "worktree_create"}}},
		"no pending":  {Execution: &issueops.Execution{Mode: issueops.ExecutionModeOrca}},
		"other kind":  {Execution: &issueops.Execution{Mode: issueops.ExecutionModeOrca, Pending: &issueops.ExternalIntent{Kind: "gh_run"}}},
	} {
		if err := validateExecutionIssueSnapshotRecord(
			ExecutionActionRequest{Action: ExecutionActionReconcile, Confirm: true}, record,
		); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}
}

func TestWithExecutionIssueSnapshotSource(t *testing.T) {
	if got := withExecutionIssueSnapshotSource(ExecutionPrepareResult{}, ""); got.(ExecutionPrepareResult).IssueSnapshotSource != "" {
		t.Fatal("empty source must not decorate")
	}
	prepared := withExecutionIssueSnapshotSource(ExecutionPrepareResult{}, "glab_mcp").(ExecutionPrepareResult)
	if prepared.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("prepare decoration wrong: %+v", prepared)
	}
	exe := withExecutionIssueSnapshotSource(ExecutionResult{}, "glab_cli").(ExecutionResult)
	if exe.IssueSnapshotSource != "glab_cli" {
		t.Fatalf("execution decoration wrong: %+v", exe)
	}
	replace := withExecutionIssueSnapshotSource(ExecutionReplaceResult{}, "s").(ExecutionReplaceResult)
	if replace.IssueSnapshotSource != "s" {
		t.Fatalf("replace decoration wrong: %+v", replace)
	}
	reconcile := withExecutionIssueSnapshotSource(ExecutionReconcileResult{}, "s").(ExecutionReconcileResult)
	if reconcile.IssueSnapshotSource != "s" {
		t.Fatalf("reconcile decoration wrong: %+v", reconcile)
	}
	if got := withExecutionIssueSnapshotSource(42, "s"); got != 42 {
		t.Fatal("unknown types must pass through unchanged")
	}
}
