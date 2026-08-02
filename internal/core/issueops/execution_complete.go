package issueops

import "agent-harness/internal/contract/issueops"

// ExecutionCompleteRequest is the transport DTO consumed by the current
// completion inbound adapter.
type ExecutionCompleteRequest struct {
	ID                string               `json:"id"`
	Generation        uint64               `json:"generation"`
	Actor             issueops.NativeActor `json:"actor"`
	CWD               string               `json:"cwd"`
	FinalHead         string               `json:"final_head"`
	TuringReportPath  string               `json:"turing_report_path"`
	Verification      []string             `json:"verification"`
	RemoteArtifactURL string               `json:"remote_artifact_url"`
	Confirm           bool                 `json:"confirm"`
}
