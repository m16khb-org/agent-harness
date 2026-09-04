package issueopscompletion

import (
	"strings"
	"testing"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

func TestValidateArtifactAcceptsCanonicalCurrentArtifact(t *testing.T) {
	record := currentArtifactRecord()
	if err := ValidateArtifact(record, record.Artifact.URL); err != nil {
		t.Fatal(err)
	}

	record.IssueURL = "https://github.enterprise:443/acme/repo/issues/232"
	record.Artifact.URL = "https://github.enterprise/acme/repo/pull/7"
	if err := ValidateArtifact(record, record.Artifact.URL); err != nil {
		t.Fatalf("GitHub Enterprise artifact: %v", err)
	}

	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/232"
	record.Artifact.Provider = "gitlab"
	record.Artifact.Kind = "mr"
	record.Artifact.URL = "https://gitlab.example.com/acme/repo/-/merge_requests/7"
	if err := ValidateArtifact(record, record.Artifact.URL); err != nil {
		t.Fatal(err)
	}
}

func TestValidateArtifactRejectsNonCanonicalOrMismatchedEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*completioncontract.RecordSnapshot)
		url    string
		want   string
	}{
		{name: "missing artifact", mutate: func(r *completioncontract.RecordSnapshot) { r.Artifact = nil }, want: "durable verified"},
		{name: "target", mutate: func(r *completioncontract.RecordSnapshot) { r.Artifact.TargetBranch = "release" }, want: "target branch"},
		{name: "project", mutate: func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/other/repo/pull/7" }, url: "https://github.com/other/repo/pull/7", want: "linked issue project"},
		{name: "project port", mutate: func(r *completioncontract.RecordSnapshot) {
			r.IssueURL = "https://github.com:8443/acme/repo/issues/232"
		}, want: "linked issue project"},
		{name: "linked provider", mutate: func(r *completioncontract.RecordSnapshot) {
			r.IssueURL = "https://gitlab.example.com/acme/repo/issues/232"
			r.Artifact.URL = "https://gitlab.example.com/acme/repo/pull/7"
		}, url: "https://gitlab.example.com/acme/repo/pull/7", want: "linked issue provider"},
		{name: "linked issue authority", mutate: func(r *completioncontract.RecordSnapshot) {
			r.IssueURL = "https://github.com/acme/repo/issues/232?redirect=other"
		}, want: "linked issue project"},
		{name: "labels", mutate: func(r *completioncontract.RecordSnapshot) { r.Artifact.Labels = []string{" issueops "} }, want: "labels are not canonical"},
		{name: "assignee placeholder", mutate: func(r *completioncontract.RecordSnapshot) { r.Artifact.Assignees = []string{"self"} }, want: "verified provider user"},
		{name: "requested url", url: "https://github.com/acme/repo/pull/8", want: "must match the durable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := currentArtifactRecord()
			requested := record.Artifact.URL
			if test.mutate != nil {
				test.mutate(&record)
			}
			if test.url != "" {
				requested = test.url
			}
			if err := ValidateArtifact(record, requested); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func currentArtifactRecord() completioncontract.RecordSnapshot {
	return completioncontract.RecordSnapshot{
		Phase: "pr", IssueURL: "https://github.com/acme/repo/issues/232", BaseBranch: "228-clean-break",
		Artifact: &completioncontract.RemoteArtifact{
			Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/7",
			Labels: []string{"issueops"}, Assignees: []string{"m16khb"},
			VerifiedAt: "2026-08-02T00:00:00Z", TargetBranch: "228-clean-break",
		},
	}
}

// 코드가 이슈와 다른 프로젝트에 있는 사이클은 봉인한 code project key와
// 대조한다. 이 경로가 없으면 verify까지 통과한 아티팩트가 completion에서 다시
// 막혀 사이클이 done에 도달하지 못한다.
func TestValidateArtifactBindsToSealedCodeProject(t *testing.T) {
	record := currentArtifactRecord()
	record.IssueURL = "https://github.com/acme/planning/issues/232"

	if err := ValidateArtifact(record, record.Artifact.URL); err == nil {
		t.Fatal("without a sealed code project the artifact must still match the issue project")
	}

	record.CodeProjectKey = "github.com/acme/repo"
	if err := ValidateArtifact(record, record.Artifact.URL); err != nil {
		t.Fatalf("artifact in the sealed code project must be accepted: %v", err)
	}

	record.CodeProjectKey = "github.com/acme/other"
	if err := ValidateArtifact(record, record.Artifact.URL); err == nil {
		t.Fatal("artifact outside the sealed code project must be rejected")
	}
}
