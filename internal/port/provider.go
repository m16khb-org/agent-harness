package port

// IssueProviderCreateIssueRequest describes a request to create a remote issue.
type IssueProviderCreateIssueRequest struct {
	Repo        string   `json:"repo"`        // local repo path for provider auth context
	Title       string   `json:"title"`       // issue title
	Body        string   `json:"body"`        // issue body (markdown)
	Labels      []string `json:"labels"`      // labels to apply
	Assignees   []string `json:"assignees"`   // assignee usernames
	Confirm     bool     `json:"confirm"`     // must be true to execute; false = dry-run preview
}

// IssueProviderCreateIssueResult reports the outcome of a remote issue creation.
type IssueProviderCreateIssueResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`               // created issue URL
	Number  string `json:"number"`            // issue number
	Preview string `json:"preview,omitempty"` // dry-run preview of what would be created
}

// IssueProviderCreatePullRequestRequest describes a request to create a remote PR/MR.
type IssueProviderCreatePullRequestRequest struct {
	Repo        string `json:"repo"`          // local repo path
	Title       string `json:"title"`         // PR title
	Body        string `json:"body"`          // PR body (markdown)
	HeadBranch  string `json:"head_branch"`   // source branch
	BaseBranch  string `json:"base_branch"`   // target branch
	Labels      []string `json:"labels"`
	Assignees   []string `json:"assignees"`
	Confirm     bool   `json:"confirm"`
}

// IssueProviderCreatePullRequestResult reports the outcome.
type IssueProviderCreatePullRequestResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`
	Number  string `json:"number"`
	Preview string `json:"preview,omitempty"`
}

// IssueProvider is implemented by provider-specific adapters such as GitHub and GitLab.
// Every mutating operation requires Confirm=true; without it, only a dry-run preview
// is returned.
type IssueProvider interface {
	Name() string
	CreateIssue(req IssueProviderCreateIssueRequest) (IssueProviderCreateIssueResult, error)
	CreatePullRequest(req IssueProviderCreatePullRequestRequest) (IssueProviderCreatePullRequestResult, error)
}
