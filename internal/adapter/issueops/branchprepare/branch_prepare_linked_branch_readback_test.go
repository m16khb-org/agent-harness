package branchprepare

import (
	"strings"
	"testing"
)

const (
	readbackIssueURL = "https://github.com/acme/repo/issues/304"
	readbackBranch   = "304-completion-reseed-stale-receipt"
	readbackBaseSHA  = "5480568a4178d5ea46d5486b97d0ff5223f1c24c"
)

// TestGitHubStepsRequireALinkedBranchReadback는 #306을 고정한다.
// `createLinkedBranch`가 오류 없이 응답하고도 실제 ref를 만들지 않는 부분
// 성공이 관측됐다(`linkedBranch.ref == null`). 생성 단계만 안내하고 끝나면
// 그 상태가 성공으로 통과한다.
func TestGitHubStepsRequireALinkedBranchReadback(t *testing.T) {
	for _, tc := range []struct {
		name    string
		baseSHA string
	}{
		{"sealed base", readbackBaseSHA},
		{"base SHA 없음", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := githubSteps(readbackIssueURL, readbackBranch, "main", tc.baseSHA)

			issueReadback, remoteReadback := "", ""
			for _, step := range steps {
				command := strings.Join(step.Command, " ")
				if step.Strategy == "verify_linked_branch" {
					if strings.Contains(command, "linkedBranches") {
						issueReadback = command
					}
					if strings.Contains(command, "ls-remote") {
						remoteReadback = command
					}
				}
			}
			if issueReadback == "" {
				t.Fatal("issue의 linkedBranches를 다시 읽는 단계가 있어야 한다")
			}
			if remoteReadback == "" {
				t.Fatal("원격 refs/heads/<branch>를 다시 읽는 단계가 있어야 한다")
			}
			if !strings.Contains(remoteReadback, "refs/heads/"+readbackBranch) {
				t.Fatalf("원격 readback은 exact branch ref를 지목해야 한다: %s", remoteReadback)
			}
		})
	}
}

// TestGitHubReadbackNamesThePartialSuccessAndForbidsRetry는 안내가 무엇을
// 확인해야 하는지, 그리고 무엇을 하면 안 되는지 밝히는지 고정한다.
// "다시 읽어라"만으로는 ref:null을 성공으로 넘긴다.
func TestGitHubReadbackNamesThePartialSuccessAndForbidsRetry(t *testing.T) {
	steps := githubSteps(readbackIssueURL, readbackBranch, "main", readbackBaseSHA)

	description := ""
	for _, step := range steps {
		if step.Strategy == "verify_linked_branch" {
			description += " " + step.Description
		}
	}
	for _, needle := range []string{
		"ref", "null", readbackBaseSHA, "retry",
	} {
		if !strings.Contains(strings.ToLower(description), strings.ToLower(needle)) {
			t.Fatalf("readback 안내에 %q가 있어야 한다: %s", needle, description)
		}
	}
}

// TestGitHubStepsKeepFailAsTheLastStep는 readback 추가가 실패 종료 단계를
// 밀어내지 않음을 고정한다. 마지막 단계는 언제나 "여기서 멈춰라"여야 한다.
func TestGitHubStepsKeepFailAsTheLastStep(t *testing.T) {
	for _, baseSHA := range []string{readbackBaseSHA, ""} {
		steps := githubSteps(readbackIssueURL, readbackBranch, "main", baseSHA)
		last := steps[len(steps)-1]
		if last.Strategy != "fail" {
			t.Fatalf("마지막 단계는 fail이어야 한다: %+v", last)
		}
		for index, step := range steps {
			if step.Order != index+1 {
				t.Fatalf("Order는 1부터 연속이어야 한다: index=%d order=%d", index, step.Order)
			}
		}
	}
}
