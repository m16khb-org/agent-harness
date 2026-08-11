package issueopsinventory

import issueopscontract "agent-harness/internal/contract/issueops"

type Record = issueopscontract.IssueOpsRecord

type ListResult struct {
	OK             bool        `json:"ok"`
	GeneratedAt    string      `json:"generated_at"`
	ScannedRecords int         `json:"scanned_records"`
	Entries        []ListEntry `json:"entries"`
}

type ListEntry struct {
	ID                    string                         `json:"id"`
	Repo                  string                         `json:"repo"`
	Branch                string                         `json:"branch,omitempty"`
	Phase                 issueopscontract.IssueOpsPhase `json:"phase"`
	Mode                  string                         `json:"mode,omitempty"`
	LeaseStatus           string                         `json:"lease_status,omitempty"`
	HolderHost            string                         `json:"holder_host,omitempty"`
	HolderSession         string                         `json:"holder_session,omitempty"`
	OwnerModel            string                         `json:"owner_model,omitempty"`
	WorkspaceRoot         string                         `json:"workspace_root,omitempty"`
	RemoteArtifactURL     string                         `json:"remote_artifact_url,omitempty"`
	UpdatedAt             string                         `json:"updated_at,omitempty"`
	Claimable             bool                           `json:"claimable,omitempty"`
	CleanupCandidate      bool                           `json:"cleanup_candidate,omitempty"`
	CompletionUnreflected bool                           `json:"completion_unreflected,omitempty"`
	Invalid               bool                           `json:"invalid,omitempty"`
}

const (
	LeaseStatusClaimable = issueopscontract.LeaseStatusClaimable
	PhaseDone            = issueopscontract.IssueOpsPhaseDone
)
