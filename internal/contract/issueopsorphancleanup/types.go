// 고아 워크트리 정리의 요청·결과 DTO다. 정리를 수행하는 쪽은 I/O를 하지만
// 요청을 만들고 결과를 읽는 쪽은 그 구현을 알 필요가 없다.
package issueopsorphancleanup

import issueopscontract "agent-harness/internal/contract/issueops"

// Request identifies one recordless local worktree and its already-merged
// remote artifact. It deliberately does not contain a user-supplied merged
// boolean: provider readback is the only merge evidence accepted by Preview.
type Request struct {
	ID           string
	RepoRoot     string
	WorktreePath string
	Branch       string
	Artifact     issueopscontract.IssueOpsRemoteArtifactVerification
}
type ApplyRequest struct {
	Confirm     bool
	Fingerprint string
}
type Result struct {
	OK                   bool     `json:"ok"`
	Preview              bool     `json:"preview"`
	Applied              bool     `json:"applied"`
	Confirmed            bool     `json:"confirmed"`
	Ready                bool     `json:"ready"`
	ID                   string   `json:"id"`
	RepoRoot             string   `json:"repo_root"`
	WorktreePath         string   `json:"worktree_path"`
	Branch               string   `json:"branch"`
	Provider             string   `json:"provider"`
	ArtifactKind         string   `json:"artifact_kind"`
	RemoteArtifactURL    string   `json:"remote_artifact_url"`
	InventoryRefreshed   bool     `json:"inventory_refreshed"`
	RecordAbsent         bool     `json:"record_absent"`
	TargetWorktreeCount  int      `json:"target_worktree_count"`
	TargetCanonical      bool     `json:"target_canonical"`
	TargetClean          bool     `json:"target_clean"`
	LocalBranchOID       string   `json:"local_branch_oid"`
	HeadSHA              string   `json:"head_sha"`
	RecoveryPath         string   `json:"recovery_path"`
	RecoveryHead         string   `json:"recovery_head"`
	RemoteMerged         bool     `json:"remote_merged"`
	LocalWorktreeRemoved bool     `json:"local_worktree_removed"`
	LocalBranchRemoved   bool     `json:"local_branch_removed"`
	RemoteBranchDeletion string   `json:"remote_branch_deletion"`
	Fingerprint          string   `json:"fingerprint"`
	Missing              []string `json:"missing"`
	Warnings             []string `json:"warnings"`
}
