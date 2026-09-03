// Package issueopsbodysync carries the DTOs for refreshing the body of a remote
// IssueOps artifact whose content has drifted from the cycle record.
//
// The vertical exists because the only pre-existing body mutation,
// port.IssueProviderUpdateIssueBodySection, owns a closed set of managed
// sections and cannot touch the authored body at all.
package issueopsbodysync

// Artifact kinds whose body can be synced. Issue and child are provider issues;
// pr and mr are the GitHub and GitLab names for the same publication.
const (
	KindIssue = "issue"
	KindChild = "child"
	KindPR    = "pr"
	KindMR    = "mr"
)

// Drift classifies how far the live remote body has moved from the body the
// harness last recorded for the artifact.
type Drift string

const (
	// DriftInSync means the proposed body, once merged, equals the live body.
	DriftInSync Drift = "in_sync"
	// DriftStale means only the harness-authored content differs: the live body
	// is exactly what the harness last wrote, and the proposal moves it forward.
	DriftStale Drift = "stale"
	// DriftRemoteEdited means the live body is not what the harness last wrote,
	// so somebody edited it outside the harness.
	DriftRemoteEdited Drift = "remote_edited"
)

// Region is one managed block that survives a body replacement verbatim.
type Region struct {
	// Name is the marker identity, e.g. "issueops:completion".
	Name string
	// Block is the exact text spliced back into the merged body.
	Block string
}

// Plan is the deterministic comparison of a proposed body against the live
// remote body. Preview and confirm compute it identically; only confirm writes.
type Plan struct {
	Drift             Drift
	RemoteBodySHA256  string
	MergedBodySHA256  string
	MergedBody        string
	PreservedSections []string
}

// Result is the CLI and MCP response for one sync attempt.
type Result struct {
	OK                 bool     `json:"ok"`
	ID                 string   `json:"id"`
	Provider           string   `json:"provider"`
	Kind               string   `json:"kind"`
	URL                string   `json:"url"`
	Confirm            bool     `json:"confirm"`
	Updated            bool     `json:"updated"`
	Drift              Drift    `json:"drift"`
	RecordedBodySHA256 string   `json:"recorded_body_sha256,omitempty"`
	RemoteBodySHA256   string   `json:"remote_body_sha256"`
	MergedBodySHA256   string   `json:"merged_body_sha256"`
	ExpectedBodySHA256 string   `json:"expected_body_sha256"`
	PreservedSections  []string `json:"preserved_sections,omitempty"`
	RecordedAt         string   `json:"recorded_at,omitempty"`
	AgeDays            int      `json:"age_days,omitempty"`
	AcceptRemoteEdits  bool     `json:"accept_remote_edits,omitempty"`
	Preview            string   `json:"preview,omitempty"`
}

// Command is one sync request. Kind is what the caller asked for (KindIssue or
// KindPR); the resolved artifact kind can differ, because a URL under a linked
// parent resolves to KindChild and a GitLab publication resolves to KindMR.
type Command struct {
	ID                 string
	Kind               string
	URL                string
	ProposedBody       string
	ExpectedBodySHA256 string
	AcceptRemoteEdits  bool
	ExpectedGeneration uint64
	Confirm            bool
}
