package issueopspublication_test

import (
	"bytes"
	"encoding/json"
	"testing"

	publication "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

func TestProviderCreateRequestJSONMatchesPort(t *testing.T) {
	tests := []struct {
		name        string
		contract    publication.ProviderCreateRequest
		port        port.IssueProviderCreatePullRequestRequest
		literalJSON string
	}{
		{
			name: "all fields",
			contract: publication.ProviderCreateRequest{
				Repo: "/repo", ProjectKey: "github.com/acme/repo", Title: "title", Body: "body",
				HeadBranch: "195-branch", BaseBranch: "117-parent", Labels: []string{"enhancement"},
				Assignees: []string{"maintainer"}, Draft: true, ExpectedHeadSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Confirm: true, Host: "codex", SessionID: "session", AgentID: "agent", CWD: "/repo.worktrees/195-branch",
			},
			port: port.IssueProviderCreatePullRequestRequest{
				Repo: "/repo", ProjectKey: "github.com/acme/repo", Title: "title", Body: "body",
				HeadBranch: "195-branch", BaseBranch: "117-parent", Labels: []string{"enhancement"},
				Assignees: []string{"maintainer"}, Draft: true, ExpectedHeadSHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Confirm: true, Host: "codex", SessionID: "session", AgentID: "agent", CWD: "/repo.worktrees/195-branch",
			},
			literalJSON: "{\"repo\":\"/repo\",\"project_key\":\"github.com/acme/repo\",\"title\":\"title\",\"body\":\"body\",\"head_branch\":\"195-branch\",\"base_branch\":\"117-parent\",\"labels\":[\"enhancement\"],\"assignees\":[\"maintainer\"],\"draft\":true,\"expected_head_sha\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"confirm\":true,\"host\":\"codex\",\"session_id\":\"session\",\"agent_id\":\"agent\",\"cwd\":\"/repo.worktrees/195-branch\"}",
		},
		{
			name: "optional fields omitted",
			contract: publication.ProviderCreateRequest{
				Repo: "/repo", Title: "title", Body: "body", HeadBranch: "195-branch", BaseBranch: "117-parent",
				Labels: []string{}, Confirm: false,
			},
			port: port.IssueProviderCreatePullRequestRequest{
				Repo: "/repo", Title: "title", Body: "body", HeadBranch: "195-branch", BaseBranch: "117-parent",
				Labels: []string{}, Confirm: false,
			},
			literalJSON: "{\"repo\":\"/repo\",\"title\":\"title\",\"body\":\"body\",\"head_branch\":\"195-branch\",\"base_branch\":\"117-parent\",\"labels\":[],\"assignees\":null,\"draft\":false,\"confirm\":false}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mustMarshal(t, test.contract)
			want := mustMarshal(t, test.port)
			if !bytes.Equal(got, want) || string(got) != test.literalJSON {
				t.Fatalf("provider request JSON drift\ngot=%s\nport=%s\nliteral=%s", got, want, test.literalJSON)
			}
		})
	}
}

func TestProviderCreateResultJSONMatchesPort(t *testing.T) {
	tests := []struct {
		name        string
		contract    publication.ProviderCreateResult
		port        port.IssueProviderCreatePullRequestResult
		literalJSON string
	}{
		{
			name:        "created",
			contract:    publication.ProviderCreateResult{OK: true, URL: "https://github.com/acme/repo/pull/1", Number: "1", Preview: "preview"},
			port:        port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/1", Number: "1", Preview: "preview"},
			literalJSON: "{\"ok\":true,\"url\":\"https://github.com/acme/repo/pull/1\",\"number\":\"1\",\"preview\":\"preview\"}",
		},
		{
			name:        "empty preview omitted",
			contract:    publication.ProviderCreateResult{},
			port:        port.IssueProviderCreatePullRequestResult{},
			literalJSON: "{\"ok\":false,\"url\":\"\",\"number\":\"\"}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mustMarshal(t, test.contract)
			want := mustMarshal(t, test.port)
			if !bytes.Equal(got, want) || string(got) != test.literalJSON {
				t.Fatalf("provider result JSON drift\ngot=%s\nport=%s\nliteral=%s", got, want, test.literalJSON)
			}
		})
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
