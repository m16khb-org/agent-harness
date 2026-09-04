package artifactverify

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func crossProjectRecord(codeProjectKey string) model.IssueOpsRecord {
	return model.IssueOpsRecord{
		Phase:    model.IssueOpsPhasePR,
		IssueURL: "https://gitlab.example.com/planning/backlog/-/issues/42",
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider:       "gitlab",
			IssueURL:       "https://gitlab.example.com/planning/backlog/-/issues/42",
			Branch:         "42-cross",
			BaseBranch:     "main",
			CodeProjectKey: codeProjectKey,
		},
	}
}

func verificationRequest(url string) model.IssueOpsRemoteArtifactVerificationRequest {
	return model.IssueOpsRemoteArtifactVerificationRequest{
		Provider: "gitlab", Kind: "mr", URL: url,
		Labels: []string{"bug"}, Assignees: []string{"someone"}, TargetBranch: "main",
	}
}

// 이슈가 다른 프로젝트에 있는 사이클은 봉인한 code project key 덕분에 코드
// 프로젝트의 MR을 봉인할 수 있어야 한다. 이 경로가 막히면 remote_artifact가
// 끝내 채워지지 않아 cleanup까지 같은 지점에서 멈춘다.
func TestProjectionAcceptsArtifactInSealedCodeProject(t *testing.T) {
	record := crossProjectRecord("gitlab.example.com/team/service-a")
	got, err := Projection(record, verificationRequest("https://gitlab.example.com/team/service-a/-/merge_requests/7"))
	if err != nil {
		t.Fatalf("artifact in the sealed code project must be accepted: %v", err)
	}
	if got.URL != "https://gitlab.example.com/team/service-a/-/merge_requests/7" {
		t.Fatalf("verification must round-trip the artifact URL: %+v", got)
	}
}

// 봉인이 있어도 아무 프로젝트나 붙일 수는 없다 — 검증 대상이 이슈 프로젝트에서
// 코드 프로젝트로 바뀔 뿐 강도는 그대로다.
func TestProjectionRejectsArtifactOutsideSealedCodeProject(t *testing.T) {
	record := crossProjectRecord("gitlab.example.com/team/service-a")
	_, err := Projection(record, verificationRequest("https://gitlab.example.com/team/service-b/-/merge_requests/7"))
	if err == nil || !strings.Contains(err.Error(), "must match linked issue project") {
		t.Fatalf("artifact outside the sealed code project must be rejected: %v", err)
	}
}

// 봉인이 없으면 이슈 프로젝트가 곧 코드 프로젝트다 — 기존 사이클의 동작.
func TestProjectionWithoutSealedCodeProjectStillBindsToIssueProject(t *testing.T) {
	record := crossProjectRecord("")
	if _, err := Projection(record, verificationRequest("https://gitlab.example.com/team/service-a/-/merge_requests/7")); err == nil {
		t.Fatal("without a sealed code project the artifact must still match the issue project")
	}
	if _, err := Projection(record, verificationRequest("https://gitlab.example.com/planning/backlog/-/merge_requests/7")); err != nil {
		t.Fatalf("same-project artifact must remain accepted: %v", err)
	}
}
