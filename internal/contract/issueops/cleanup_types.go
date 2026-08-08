// cleanup 요청·결과 DTO다. 정리를 수행하는 쪽은 I/O를 하지만 요청을 만들고
// 결과를 읽는 쪽은 그 구현을 알 필요가 없다.
package issueops

// IssueOpsActor identifies the native host session that is authorized to make
// a durable mutation for one IssueOps cycle. CWD is deliberately part of the
// request identity: a valid session in the source checkout is not authority
// for the isolated workspace, and vice versa.
type IssueOpsActor struct {
	Host                  string                 `json:"host"`
	SessionID             string                 `json:"session_id"`
	AgentID               string                 `json:"agent_id,omitempty"`
	CWD                   string                 `json:"cwd"`
	NativeProcessAncestry []NativeProcessReceipt `json:"-"`
}

// CleanupAbandonRequest는 폐기된 비-done 사이클의 로컬 worktree, branch,
// record 수명 종료(issue #106, #293)의 입력이다.
//
// 이 경로는 cleanup finish와 두 가지 축에서 다르다.
//   - 원격 무접촉: 이슈 본문·PR/MR·원격 브랜치 어느 것도 읽지도 쓰지도 않는다.
//     ReflectCleanupAudit를 재사용하지 않는 이유는 빈 completion payload가 열린
//     이슈에 가짜 "완료 기록" 섹션을 append하고, 그 마커만 보는
//     `completion_reflected` 게이트가 미래 사이클의 파괴적 finish를 영구
//     개방하기 때문이다(brooks F3).
//   - 보존 불변식 C2-F6("레코드 삭제 전 보존")의 의도적 예외: 원격에 남길
//     자리가 없으므로 삭제 대상 레코드 전문을 결과 JSON에 담는 것이 유일한
//     보존 채널이다(brooks F7 완화책).
type CleanupAbandonRequest struct {
	ID          string
	Reason      string
	Apply       bool
	Confirm     bool
	Fingerprint string
	// ArtifactUnmerged는 레코드의 remote artifact가 병합되지 않았음을 호출자가
	// 실제로 관측했다는 뜻이다. 관측 실패와 미관측은 모두 false로 남아 게이트가
	// 닫힌 상태를 유지한다(fail-closed). 이 값은 fingerprint 입력이 아니다 —
	// 네트워크 관측을 인벤토리에 섞으면 일시적 원격 오류가 preview 재발급
	// 루프를 만든다(finish의 remote_branch_absent와 같은 규율).
	ArtifactUnmerged bool
}

// CleanupFinishRequest는 record-backed 머지 후 정리(설계 v5 WS3)의 입력이다.
// Merged/CompletionReflected/IssueClosed는 caller가 원격 readback으로 검증한
// 값이며, readback 실패는 caller에서 fail-closed로 끝난다(값이 오지 않는다).
type CleanupFinishRequest struct {
	ID                  string
	CWD                 string
	Merged              bool
	CompletionReflected bool
	IssueClosed         bool
	// MergedBaseBranch는 머지 readback이 함께 관측한 원격 artifact의 현재 base
	// ref다. done 전이는 draft PR 생성 직후에 일어나고 finish는 머지 이후에
	// 실행되므로 그 사이 구간에서 base가 바뀔 수 있다. 레코드의 base 값은 done
	// 시점에 검증된 과거이므로 drift를 구조적으로 검출하지 못한다 — 원격 관측만이
	// 유효한 증거다.
	MergedBaseBranch string
	Apply            bool
	Confirm          bool
	Fingerprint      string
}

// CleanupRemoteBranchRequest는 머지 검증된 사이클의 원격 브랜치를 typed 경로로
// 삭제하는 입력이다(이슈 #116). cleanup 명령군 순서는
// status → close-children → remote-branch → finish다.
//
// 이 표면은 source checkout 전용이다(cwd = record.Repo). 워크트리 cwd에서의
// 호출은 lease 가드가 미분류 셸로 차단한다.
type CleanupRemoteBranchRequest struct {
	ID          string
	Apply       bool
	Confirm     bool
	Fingerprint string
}

type CleanupAbandonResult struct {
	OK                   bool     `json:"ok"`
	ID                   string   `json:"id"`
	Preview              bool     `json:"preview"`
	Reason               string   `json:"reason,omitempty"`
	Missing              []string `json:"missing,omitempty"`
	ReasonError          string   `json:"reason_error,omitempty"`
	PendingIntentError   string   `json:"pending_intent_error,omitempty"`
	OrcaResidueError     string   `json:"orca_residue_error,omitempty"`
	Fingerprint          string   `json:"fingerprint,omitempty"`
	WorktreePath         string   `json:"worktree_path,omitempty"`
	Branch               string   `json:"branch,omitempty"`
	WorktreePresent      bool     `json:"worktree_present"`
	BranchPresent        bool     `json:"branch_present"`
	WorktreeCanonical    bool     `json:"worktree_canonical"`
	WorktreeClean        bool     `json:"worktree_clean"`
	WorktreeHead         string   `json:"worktree_head,omitempty"`
	BranchOID            string   `json:"branch_oid,omitempty"`
	BranchCheckoutPath   string   `json:"branch_checkout_path,omitempty"`
	RemovalPlan          []string `json:"removal_plan,omitempty"`
	RemoteBranchDeletion string   `json:"remote_branch_deletion"`
	PendingOperationID   string   `json:"pending_operation_id,omitempty"`
	IntentRowsDeleted    []string `json:"intent_rows_deleted,omitempty"`
	WorktreeRemoved      bool     `json:"worktree_removed,omitempty"`
	BranchDeleted        bool     `json:"branch_deleted,omitempty"`
	RecordDeleted        bool     `json:"record_deleted,omitempty"`
	AbandonedAt          string   `json:"abandoned_at,omitempty"`
	FailedStep           string   `json:"failed_step,omitempty"`
	NextCommand          string   `json:"next_command,omitempty"`
	// Record는 삭제 대상 레코드 전문이다. C2-F6 예외의 유일한 보존 채널이므로
	// preview와 apply 양쪽 결과에 담는다 — preview에만 담으면 preview 이후
	// apply 직전까지의 변경분이 어디에도 남지 않는다.
	Record *IssueOpsRecord `json:"record,omitempty"`
}
type CleanupFinishResult struct {
	OK              bool     `json:"ok"`
	ID              string   `json:"id"`
	Preview         bool     `json:"preview"`
	Missing         []string `json:"missing,omitempty"`
	Fingerprint     string   `json:"fingerprint,omitempty"`
	WorktreePath    string   `json:"worktree_path,omitempty"`
	Branch          string   `json:"branch,omitempty"`
	WorktreePresent bool     `json:"worktree_present"`
	BranchPresent   bool     `json:"branch_present"`
	// WorkspaceProcesses는 workspace_processes_quiescent가 막았을 때 그 판정의
	// 근거를 담는다. 게이트는 이미 PID와 명령명을 관측하는데 개수만 쓰고 버려서,
	// 차단당한 사용자가 lsof를 직접 돌려야 했다 — 그 lsof마저 워크트리 경로를
	// 인자로 주면 lifecycle 가드에 걸린다(이슈 #154).
	WorkspaceProcesses []string `json:"workspace_processes,omitempty"`
	OrcaWorktreeID     string   `json:"orca_worktree_id,omitempty"`
	OrcaRemoved        bool     `json:"orca_removed,omitempty"`
	WorktreeRemoved    bool     `json:"worktree_removed,omitempty"`
	BranchDeleted      bool     `json:"branch_deleted,omitempty"`
	AuditReflected     bool     `json:"audit_reflected,omitempty"`
	AuditError         string   `json:"audit_error,omitempty"`
	RecordDeleted      bool     `json:"record_deleted,omitempty"`
	FailedStep         string   `json:"failed_step,omitempty"`
	NextCommand        string   `json:"next_command,omitempty"`
}
type CleanupRemoteBranchResult struct {
	OK                  bool     `json:"ok"`
	ID                  string   `json:"id"`
	Preview             bool     `json:"preview"`
	Missing             []string `json:"missing,omitempty"`
	ArtifactError       string   `json:"artifact_error,omitempty"`
	RemoteIdentityError string   `json:"remote_identity_error,omitempty"`
	Fingerprint         string   `json:"fingerprint,omitempty"`
	Branch              string   `json:"branch,omitempty"`
	RemoteOID           string   `json:"remote_oid,omitempty"`
	ArtifactHeadBranch  string   `json:"artifact_head_branch,omitempty"`
	ArtifactHeadOID     string   `json:"artifact_head_oid,omitempty"`
	RemoteBranchPresent bool     `json:"remote_branch_present"`
	// RemoteTipReachedBase는 게이트 ⑩이 OID 일치가 아니라 ancestry로 통과했음을
	// 밝힌다. 두 근거는 강도가 다르므로 무엇으로 통과했는지 남긴다 — OID 일치는
	// "머지된 그 커밋 그대로"이고, ancestry는 "다른 커밋이지만 이미 base에 있다"다
	// (이슈 #153).
	RemoteTipReachedBase bool   `json:"remote_tip_reached_base,omitempty"`
	AlreadyAbsent        bool   `json:"already_absent,omitempty"`
	Deleted              bool   `json:"deleted,omitempty"`
	DeletedAt            string `json:"deleted_at,omitempty"`
	AuditReflected       bool   `json:"audit_reflected,omitempty"`
	AuditError           string `json:"audit_error,omitempty"`
	FailedStep           string `json:"failed_step,omitempty"`
	NextCommand          string `json:"next_command,omitempty"`
}
