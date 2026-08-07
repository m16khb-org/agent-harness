package remotecmd

import (
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
)

// 우산 브랜치 게이트는 provider 호출 이전에 선다. 자식이 만들어진 뒤에는 위상을
// 되돌릴 수 없기 때문이다(#129 AC-01).
func TestCreateChildRequiresPreparedUmbrellaBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "78-umbrella"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	record, err = issueopscore.LinkIssueOpsIssue(issueopscore.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/78")
	if err != nil {
		t.Fatalf("LinkIssueOpsIssue: %v", err)
	}

	var printedErrors []error
	deps := Deps{
		PrintJSON: func(any) error { return nil },
		PrintError: func(err error) error {
			printedErrors = append(printedErrors, err)
			return nil
		},
	}

	err = Run([]string{"create-child", "--id", record.ID, "--title", "자식 작업", "--body", "본문",
		"--label", "bug", "--assignee", "octocat", "--json"}, deps)
	if err == nil {
		t.Fatal("create-child must be blocked until the umbrella cycle prepares its own branch")
	}
	if !strings.Contains(err.Error(), "branch prepare") {
		t.Fatalf("error %q must name the command that resolves it", err)
	}
	if len(printedErrors) != 1 {
		t.Fatalf("the blocked run must report through the JSON error surface: %v", printedErrors)
	}
}

// 우산 브랜치가 준비되면 종전 경로가 그대로 열린다.
func TestCreateChildProceedsWithPreparedUmbrellaBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)

	var printed []any
	deps := Deps{
		PrintJSON:  func(value any) error { printed = append(printed, value); return nil },
		PrintError: func(error) error { return nil },
	}

	if err := Run([]string{"create-child", "--id", record.ID, "--title", "자식 작업", "--body", "본문",
		"--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("a prepared umbrella branch must not block child creation: %v", err)
	}
	if len(printed) != 1 {
		t.Fatalf("expected the dry-run preview output, got %d", len(printed))
	}
}
