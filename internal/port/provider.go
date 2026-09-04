package port

import (
	"context"
	"fmt"
)

type IssueProviderCreateError struct {
	Invoked bool
	Err     error
}

type IssueProviderCreateIssueContexter interface {
	CreateIssueContext(context.Context, IssueProviderCreateIssueRequest) (IssueProviderCreateIssueResult, error)
}

type IssueProviderIssueCreateCandidate struct {
	URL   string
	Title string
	Body  string
}

type IssueProviderFindIssueCreateCandidatesRequest struct {
	Repo             string
	ProjectAuthority string
	Marker           string
}

type IssueProviderIssueCreateReconciler interface {
	FindIssueCreateCandidates(context.Context, IssueProviderFindIssueCreateCandidatesRequest) (IssueProviderFindIssueCreateCandidatesResult, error)
}

type IssueProviderFindIssueCreateCandidatesResult struct {
	Candidates []IssueProviderIssueCreateCandidate
	Truncated  bool
}

func (e *IssueProviderCreateError) Error() string {
	return fmt.Sprintf("provider create failed: %v", e.Err)
}
func (e *IssueProviderCreateError) Unwrap() error { return e.Err }

// IssueProviderCreateIssueRequest describes a request to create a remote issue.
type IssueProviderCreateIssueRequest struct {
	Repo       string   `json:"repo"`        // local repo path for provider auth context
	ProjectKey string   `json:"project_key"` // verified canonical HOST/OWNER[/NAMESPACE]/REPO authority
	Title      string   `json:"title"`       // issue title
	Body       string   `json:"body"`        // issue body (markdown)
	Labels     []string `json:"labels"`      // labels to apply
	Assignees  []string `json:"assignees"`   // assignee usernames
	Confirm    bool     `json:"confirm"`     // must be true to execute; false = dry-run preview
}

// IssueProviderCreateIssueResult reports the outcome of a remote issue creation.
type IssueProviderCreateIssueResult struct {
	OK        bool     `json:"ok"`
	Provider  string   `json:"provider"`
	URL       string   `json:"issue_url"`         // created issue URL
	Number    string   `json:"issue_number"`      // issue number
	Labels    []string `json:"labels"`            // canonical requested labels
	Assignees []string `json:"assignees"`         // canonical requested assignees
	Preview   string   `json:"preview,omitempty"` // dry-run preview of what would be created
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
	Host            string   `json:"host,omitempty"`
	SessionID       string   `json:"session_id,omitempty"`
	AgentID         string   `json:"agent_id,omitempty"`
	CWD             string   `json:"cwd,omitempty"`
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
	// State는 provider가 보고한 아티팩트 수명 상태다(gitlab: opened/merged/closed,
	// github: open/merged/closed). draft 판정은 수명 상태에 종속되므로 후보 검증이
	// 이 값을 함께 본다.
	State string
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

// Managed issue-body section kinds shared by core callers and provider
// adapters. The set is intentionally closed (no open extension point).
const (
	IssueBodySectionDevilsAdvocate = "devils-advocate"
	IssueBodySectionCompletion     = "completion"

	// IssueBodyCompletionStartMarker is the durable delimiter cleanup finish
	// readback-checks before destructive local cleanup (설계 v5 WS3).
	IssueBodyCompletionStartMarker = "<!-- issueops:completion:start -->"
)

// IssueProviderArtifactDigest names one staged artifact and its content digest
// for durable preservation in the issue body.
type IssueProviderArtifactDigest struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// IssueProviderCompletionSection carries the completion payload rendered into
// the managed completion section. Blocks with empty values still render with a
// placeholder so the section shape stays machine-checkable.
type IssueProviderCompletionSection struct {
	FinalHead           string                        `json:"final_head"`
	RemoteArtifactURL   string                        `json:"remote_artifact_url"`
	VerificationSummary []string                      `json:"verification_summary"`
	ArtifactManifest    []IssueProviderArtifactDigest `json:"artifact_manifest"`
	TuringSummary       string                        `json:"turing_summary"`
	SpecBody            string                        `json:"spec_body"`
	PlanBody            string                        `json:"plan_body"`
	CleanupAudit        string                        `json:"cleanup_audit,omitempty"`
	// MissingArtifacts lists required sealed artifacts (plan) that were absent
	// from the workspace artifact directory when the section was gathered (#482).
	MissingArtifacts []string `json:"missing_artifacts,omitempty"`
}

// IssueProviderUpdateIssueBodySectionRequest describes reflecting one managed,
// delimited section (devils-advocate | completion) of an existing remote issue
// body. Exactly the payload matching Section is consumed.
type IssueProviderUpdateIssueBodySectionRequest struct {
	Repo       string                          `json:"repo"`                 // local repo path for provider auth context
	IssueURL   string                          `json:"issue_url"`            // issue whose body is updated
	Section    string                          `json:"section"`              // devils-advocate | completion
	Findings   []string                        `json:"findings,omitempty"`   // devils-advocate payload
	Completion *IssueProviderCompletionSection `json:"completion,omitempty"` // completion payload
	Confirm    bool                            `json:"confirm"`              // must be true to write; false = dry-run preview
}

// IssueProviderUpdateIssueBodySectionResult reports the outcome of a body update.
type IssueProviderUpdateIssueBodySectionResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url,omitempty"`
	Updated bool   `json:"updated"`
	Preview string `json:"preview,omitempty"`
}

// IssueProviderCloseIssueRequest describes closing the parent/primary issue
// after its implementation PR/MR merge was verified by the caller.
type IssueProviderCloseIssueRequest struct {
	Repo     string `json:"repo"`
	IssueURL string `json:"issue_url"`
	// Reason is "completed" (the default when empty) or "not_planned". A cycle
	// abandoned before merge closes its issue as not planned; the merged
	// cleanup path keeps the completed default.
	Reason  string `json:"reason,omitempty"`
	Confirm bool   `json:"confirm"`
}

// IssueProviderCloseIssueResult reports the close mutation and the state
// readback that verified it.
type IssueProviderCloseIssueResult struct {
	OK            bool   `json:"ok"`
	Provider      string `json:"provider"`
	IssueURL      string `json:"issue_url,omitempty"`
	Closed        bool   `json:"closed"`
	AlreadyClosed bool   `json:"already_closed,omitempty"`
	State         string `json:"state,omitempty"`
	Preview       string `json:"preview,omitempty"`
}

// IssueProviderClosePullRequestRequest describes closing an unmerged PR/MR.
// An IssueOps cycle that is abandoned before merge has to leave the draft it
// published closed; the merged path is `cleanup finish`, not this one.
type IssueProviderClosePullRequestRequest struct {
	Repo        string `json:"repo"`
	ArtifactURL string `json:"artifact_url"`
	Kind        string `json:"kind"` // pr | mr
	Confirm     bool   `json:"confirm"`
}

// IssueProviderClosePullRequestResult reports the close mutation and the state
// readback that verified it. Merged reports the one state that must never be
// closed; the caller decides what to do about it.
type IssueProviderClosePullRequestResult struct {
	OK            bool   `json:"ok"`
	Provider      string `json:"provider"`
	ArtifactURL   string `json:"artifact_url,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Closed        bool   `json:"closed"`
	AlreadyClosed bool   `json:"already_closed,omitempty"`
	Merged        bool   `json:"merged,omitempty"`
	State         string `json:"state,omitempty"`
	Preview       string `json:"preview,omitempty"`
}

// IssueProviderPullRequestCloser is an optional capability for the same reason
// the artifact-body capabilities below are optional: IssueProvider is
// implemented by every provider fake in the test suite, so widening it would
// break them all. Callers type-assert and fail closed when an adapter does not
// implement it.
type IssueProviderPullRequestCloser interface {
	ClosePullRequest(IssueProviderClosePullRequestRequest) (IssueProviderClosePullRequestResult, error)
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
	CloseIssue(req IssueProviderCloseIssueRequest) (IssueProviderCloseIssueResult, error)
	UpdateIssueBodySection(req IssueProviderUpdateIssueBodySectionRequest) (IssueProviderUpdateIssueBodySectionResult, error)
}

// Artifact-body sync capabilities.
//
// These are optional interfaces rather than IssueProvider methods on purpose:
// IssueProvider is implemented by every provider fake in the test suite, so
// widening it would break them all. Callers type-assert and fail closed when an
// adapter does not implement the capability, which is the same shape the
// context and reconcile capabilities above already use.

// IssueProviderArtifactBodyRequest reads one remote artifact's current body.
type IssueProviderArtifactBodyRequest struct {
	Repo string `json:"repo"` // local repo path for provider auth context
	Kind string `json:"kind"` // issue | child | pr | mr
	URL  string `json:"url"`  // artifact URL
}

// IssueProviderArtifactBody is the live body plus the lifecycle state a caller
// needs to decide whether the artifact may still be edited.
type IssueProviderArtifactBody struct {
	Provider string `json:"provider"`
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Body     string `json:"body"`
	State    string `json:"state,omitempty"`
}

type IssueProviderArtifactBodyReader interface {
	ReadArtifactBody(context.Context, IssueProviderArtifactBodyRequest) (IssueProviderArtifactBody, error)
}

// IssueProviderReplaceArtifactBodyRequest replaces a remote artifact's body
// wholesale. Managed-section preservation is decided before this call; the
// adapter writes exactly the body it is given.
type IssueProviderReplaceArtifactBodyRequest struct {
	Repo    string `json:"repo"`
	Kind    string `json:"kind"`
	URL     string `json:"url"`
	Body    string `json:"body"`
	Confirm bool   `json:"confirm"` // must be true to write; false = dry-run preview
}

// IssueProviderReplaceArtifactBodyResult reports the write and the readback
// that verified it. VerifiedBodySHA256 is hashed from a fresh read, never from
// the request, so a silently-rejected edit cannot look successful.
type IssueProviderReplaceArtifactBodyResult struct {
	Provider           string `json:"provider"`
	OK                 bool   `json:"ok"`
	URL                string `json:"url,omitempty"`
	Updated            bool   `json:"updated"`
	VerifiedBodySHA256 string `json:"verified_body_sha256,omitempty"`
	Preview            string `json:"preview,omitempty"`
}

type IssueProviderArtifactBodyReplacer interface {
	ReplaceArtifactBody(context.Context, IssueProviderReplaceArtifactBodyRequest) (IssueProviderReplaceArtifactBodyResult, error)
}

// IssueProviderChildHierarchyRequest asks whether ChildURL is a provider-native
// child of ParentIssueURL. Syncing a child body verifies this first so a typo'd
// URL cannot rewrite an unrelated issue.
type IssueProviderChildHierarchyRequest struct {
	Repo           string `json:"repo"`
	ParentIssueURL string `json:"parent_issue_url"`
	ChildURL       string `json:"child_url"`
}

type IssueProviderChildHierarchyResult struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Verified bool   `json:"hierarchy_verified"`
}

type IssueProviderChildHierarchyVerifier interface {
	VerifyChildHierarchy(context.Context, IssueProviderChildHierarchyRequest) (IssueProviderChildHierarchyResult, error)
}
