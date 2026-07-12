package port

import "fmt"

type IssueProviderCreateError struct {
	Invoked bool
	Err     error
}

func (e *IssueProviderCreateError) Error() string {
	return fmt.Sprintf("provider create failed: %v", e.Err)
}
func (e *IssueProviderCreateError) Unwrap() error { return e.Err }

// IssueProviderCreateIssueRequest describes a request to create a remote issue.
type IssueProviderCreateIssueRequest struct {
	Repo      string   `json:"repo"`      // local repo path for provider auth context
	Title     string   `json:"title"`     // issue title
	Body      string   `json:"body"`      // issue body (markdown)
	Labels    []string `json:"labels"`    // labels to apply
	Assignees []string `json:"assignees"` // assignee usernames
	Confirm   bool     `json:"confirm"`   // must be true to execute; false = dry-run preview
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
	Repo            string   `json:"repo"`                  // local repo path
	ProjectKey      string   `json:"project_key,omitempty"` // verified canonical HOST/OWNER[/NAMESPACE]/REPO authority
	Title           string   `json:"title"`                 // PR title
	Body            string   `json:"body"`                  // PR body (markdown)
	HeadBranch      string   `json:"head_branch"`           // source branch
	BaseBranch      string   `json:"base_branch"`           // target branch
	Labels          []string `json:"labels"`
	Assignees       []string `json:"assignees"`
	Draft           bool     `json:"draft"`
	ExpectedHeadSHA string   `json:"expected_head_sha,omitempty"`
	Confirm         bool     `json:"confirm"`
}

// IssueProviderCreatePullRequestResult reports the outcome.
type IssueProviderCreatePullRequestResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`
	Number  string `json:"number"`
	Preview string `json:"preview,omitempty"`
}

type IssueProviderReconcilePullRequestRequest struct {
	Repo            string
	ProjectKey      string
	HeadBranch      string
	BaseBranch      string
	ExpectedHeadSHA string
	Title           string
	BodySHA256      string
	Labels          []string
	Assignees       []string
	Draft           bool
}

type IssueProviderReconcilePullRequestCandidate struct {
	URL              string
	ProjectKey       string
	SourceProjectKey string
	HeadBranch       string
	BaseBranch       string
	HeadSHA          string
	Title            string
	BodySHA256       string
	Labels           []string
	Assignees        []string
	Draft            bool
}

type IssueProviderReconcilePullRequestResult struct {
	Candidates        []IssueProviderReconcilePullRequestCandidate
	AuthoritativeZero bool
}

type IssueProviderRemoteCreateReconciler interface {
	ReconcilePullRequest(IssueProviderReconcilePullRequestRequest) (IssueProviderReconcilePullRequestResult, error)
}

// IssueProviderCreateChildRequest describes a provider-native child work item
// creation request under an already-linked parent issue.
type IssueProviderCreateChildRequest struct {
	Repo           string   `json:"repo"`             // local repo path for provider auth context
	ParentIssueURL string   `json:"parent_issue_url"` // existing parent issue URL
	Title          string   `json:"title"`            // child title
	Body           string   `json:"body"`             // child body (markdown)
	Labels         []string `json:"labels"`           // labels to apply
	Assignees      []string `json:"assignees"`        // assignee usernames
	Confirm        bool     `json:"confirm"`          // must be true to execute; false = dry-run preview
}

// IssueProviderCreateChildResult reports provider-native child creation,
// hierarchy attachment, and verification.
type IssueProviderCreateChildResult struct {
	OK                bool     `json:"ok"`
	Provider          string   `json:"provider"`
	ChildURL          string   `json:"child_url,omitempty"`
	ChildNumber       string   `json:"child_number,omitempty"`
	HierarchyVerified bool     `json:"hierarchy_verified"`
	Labels            []string `json:"labels,omitempty"`
	Assignees         []string `json:"assignees,omitempty"`
	Preview           string   `json:"preview,omitempty"`
}

// IssueProviderCloseChildRequest describes closing an existing provider-native
// child work item after its implementation PR/MR was merged into the parent
// work branch.
type IssueProviderCloseChildRequest struct {
	Repo           string `json:"repo"`
	ParentIssueURL string `json:"parent_issue_url"`
	ChildURL       string `json:"child_url"`
	Confirm        bool   `json:"confirm"`
}

// IssueProviderCloseChildResult reports child hierarchy verification and final
// closed-state verification.
type IssueProviderCloseChildResult struct {
	OK                bool   `json:"ok"`
	Provider          string `json:"provider"`
	ChildURL          string `json:"child_url,omitempty"`
	HierarchyVerified bool   `json:"hierarchy_verified"`
	Closed            bool   `json:"closed"`
	AlreadyClosed     bool   `json:"already_closed,omitempty"`
	State             string `json:"state,omitempty"`
	Preview           string `json:"preview,omitempty"`
}

// IssueProviderUpdateIssueBodySectionRequest describes reflecting findings into a
// managed, delimited section of an existing remote issue body.
type IssueProviderUpdateIssueBodySectionRequest struct {
	Repo     string   `json:"repo"`      // local repo path for provider auth context
	IssueURL string   `json:"issue_url"` // issue whose body is updated
	Findings []string `json:"findings"`  // devil's-advocate findings to render
	Confirm  bool     `json:"confirm"`   // must be true to write; false = dry-run preview
}

// IssueProviderUpdateIssueBodySectionResult reports the outcome of a body update.
type IssueProviderUpdateIssueBodySectionResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url,omitempty"`
	Updated bool   `json:"updated"`
	Preview string `json:"preview,omitempty"`
}

// IssueProvider is implemented by provider-specific adapters such as GitHub and GitLab.
// Every mutating operation requires Confirm=true; without it, only a dry-run preview
// is returned.
type IssueProvider interface {
	Name() string
	CreateIssue(req IssueProviderCreateIssueRequest) (IssueProviderCreateIssueResult, error)
	CreatePullRequest(req IssueProviderCreatePullRequestRequest) (IssueProviderCreatePullRequestResult, error)
	CreateChild(req IssueProviderCreateChildRequest) (IssueProviderCreateChildResult, error)
	CloseChild(req IssueProviderCloseChildRequest) (IssueProviderCloseChildResult, error)
	UpdateIssueBodySection(req IssueProviderUpdateIssueBodySectionRequest) (IssueProviderUpdateIssueBodySectionResult, error)
}
