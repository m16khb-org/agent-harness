// IssueOps CLI가 읽는 목록·정리·검토 DTO다.
package issueops

// IssueOpsImplementationReviewRequest는 구현 diff에 대한 brooks 리뷰 기록이다.
type IssueOpsImplementationReviewRequest struct {
	Verdict        string
	Findings       []string
	Evidence       []string
	ReviewerHost   string
	ReviewerModel  string
	ReviewerEffort string
}

// IssueOpsListResult는 집계와 그 비용을 함께 노출한다 — scanned_records가
// O(N) 전량 읽기 비용을 관측 가능하게 한다(브룩스 2차 F9).
type IssueOpsListResult struct {
	OK             bool                `json:"ok"`
	GeneratedAt    string              `json:"generated_at"`
	ScannedRecords int                 `json:"scanned_records"`
	Entries        []IssueOpsListEntry `json:"entries"`
}

// IssueOpsPruneResult reports one prune preview or confirmed prune pass over
// done cycles.
type IssueOpsPruneResult struct {
	OK        bool     `json:"ok"`
	StateRoot string   `json:"state_root"`
	MaxAge    string   `json:"max_age"`
	Cutoff    string   `json:"cutoff"`
	DryRun    bool     `json:"dry_run"`
	Pruned    []string `json:"pruned"`
	Kept      []string `json:"kept"`
}

// IssueOpsListEntry는 한 사이클의 조망 행이다. 메인(planner) 세션이 여러
// 사이클을 한 번에 파악하는 read-only 표면으로, lease를 잡거나 repair를
// 수행하지 않는다(설계 v5 WS6).
type IssueOpsListEntry struct {
	ID                    string        `json:"id"`
	Repo                  string        `json:"repo"`
	Branch                string        `json:"branch,omitempty"`
	Phase                 IssueOpsPhase `json:"phase"`
	Mode                  string        `json:"mode,omitempty"`
	LeaseStatus           string        `json:"lease_status,omitempty"`
	HolderHost            string        `json:"holder_host,omitempty"`
	HolderSession         string        `json:"holder_session,omitempty"`
	OwnerModel            string        `json:"owner_model,omitempty"`
	WorkspaceRoot         string        `json:"workspace_root,omitempty"`
	RemoteArtifactURL     string        `json:"remote_artifact_url,omitempty"`
	UpdatedAt             string        `json:"updated_at,omitempty"`
	Claimable             bool          `json:"claimable,omitempty"`
	CleanupCandidate      bool          `json:"cleanup_candidate,omitempty"`
	CompletionUnreflected bool          `json:"completion_unreflected,omitempty"`
	Invalid               bool          `json:"invalid,omitempty"`
}

// 설계 검토 증거 예시 문구는 CLI 도움말과 어댑터가 함께 쓰는 어휘다.
const IssueOpsDesignReviewEvidenceExample = "design review checked alternatives and risks"
