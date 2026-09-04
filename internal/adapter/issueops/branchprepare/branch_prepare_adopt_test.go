package branchprepare

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// TestPrepareAdoptsBranchOntoBranchlessRecord는 이슈를 먼저 만드는 흐름이
// 막히지 않는지 본다.
//
// `IssueOpsProblemReadiness`는 "issue_url/branch는 grill artifact다"라고 못박고,
// 사이클이 둘 다 없이 시작해 grill 중에 획득하는 것을 설계로 삼는다. 이슈
// URL에는 `link-issue`라는 사후 연결 표면이 있지만 브랜치에는 없었다. 그래서
// `issueops start`(브랜치 없음) → `remote create-issue --confirm`(되돌릴 수 없는
// 원격 쓰기 성공) → `branch prepare`(거부)로 사이클이 영구히 막혔다.
//
// 브랜치 이름은 `<이슈번호>-` 접두를 강제받으므로 이슈를 만들기 전에는 알 수
// 없다. 따라서 create-issue 쪽을 막으면 그 명령 자체가 무용해진다. 브랜치를
// 채택할 수 있게 하는 것이 설계와 맞는 방향이다.
func TestPrepareAdoptsBranchOntoBranchlessRecord(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-adopt",
		OK:       true,
		Repo:     "/repo/example",
		IssueURL: "https://github.com/example/repo/issues/77",
	})

	record, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-adopt", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/77",
		Branch:     "77-adopt-branch",
		BaseBranch: "main",
	})
	if err != nil {
		t.Fatalf("branchless record must adopt the prepared branch: %v", err)
	}
	if record.Branch != "77-adopt-branch" {
		t.Fatalf("record branch = %q, want the adopted branch", record.Branch)
	}
	if record.BranchPrepare == nil || record.BranchPrepare.Branch != "77-adopt-branch" {
		t.Fatalf("branch prepare state not recorded: %+v", record.BranchPrepare)
	}
}

// TestPrepareRejectsBranchMismatchOnAdoptedRecord는 채택이 일회성인지 본다.
// 한 번 정해진 브랜치는 그 사이클의 정체성이므로 이후 다른 이름으로 바꿔
// 쓸 수 없어야 한다.
func TestPrepareRejectsBranchMismatchOnAdoptedRecord(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-adopt-twice",
		OK:       true,
		Repo:     "/repo/example",
		IssueURL: "https://github.com/example/repo/issues/77",
	})
	base := model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/77",
		Branch:     "77-adopt-branch",
		BaseBranch: "main",
	}
	if _, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-adopt-twice", base); err != nil {
		t.Fatalf("first prepare must adopt: %v", err)
	}
	second := base
	second.Branch = "77-different-branch"
	_, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-adopt-twice", second)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("adopted branch must be immutable afterwards; err=%v", err)
	}
}

// TestPrepareRejectsAdoptionWithoutLinkedIssue는 채택이 무근거로 일어나지
// 않는지 본다. 이슈가 없으면 브랜치 번호를 검증할 근거도 없다.
func TestPrepareRejectsAdoptionWithoutLinkedIssue(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:   "io-adopt-noissue",
		OK:   true,
		Repo: "/repo/example",
	})
	_, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-adopt-noissue", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/77",
		Branch:     "77-adopt-branch",
		BaseBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "issue must be linked") {
		t.Fatalf("adoption requires a linked issue; err=%v", err)
	}
}
