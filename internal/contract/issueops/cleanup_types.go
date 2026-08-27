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
	// SupersededBy는 원래 artifact가 closed-unmerged이고 후속 artifact가 그
	// 변경을 명시적으로 대체해 머지된 경우에 그 후속 artifact URL이다. 이 값이
	// 있으면 merged 게이트를 replacement 증거로 대신 충족할 수 있다(#283).
	// 증거는 provider readback으로 검증되며, 검증 실패는 통과가 아니라 거부다.
	SupersededBy string
	Apply        bool
	Confirm      bool
	Fingerprint  string
}

// CleanupRemoteBranchRequest는 머지 검증된 사이클의 원격 브랜치를 typed 경로로
// 삭제하는 입력이다(이슈 #116). cleanup 명령군 순서는
// status → close-children → remote-branch → finish다.
//
// 이 표면은 source checkout 전용이다(cwd = record.Repo). 워크트리 cwd에서의
// 호출은 lease 가드가 미분류 셸로 차단한다.
type CleanupRemoteBranchRequest struct {
	ID string
	// SupersededBy는 원격 tip이 기록된 머지 head보다 전진했고, 그 전진분이
	// 후속 merged artifact로 재통합된 경우에 그 artifact URL이다. 기록된 base
	// 브랜치가 이미 삭제됐거나 후속 머지 대상이 base가 아니면 ancestry로는
	// 판정할 수 없어, provider readback 증거가 유일한 근거가 된다(#323).
	SupersededBy string
	Apply        bool
	Confirm      bool
	Fingerprint  string
}

type CleanupAbandonResult struct {
	OK      bool     `json:"ok"`
	ID      string   `json:"id"`
	Preview bool     `json:"preview"`
	Reason  string   `json:"reason,omitempty"`
	Missing []string `json:"missing,omitempty"`
	// UnresolvedChildren는 no_children을 유발한 자식들이다. 개수만 알려주면
	// 사용자가 무엇을 먼저 끝내야 할지 알 수 없다(#437).
	UnresolvedChildren   []string `json:"unresolved_children,omitempty"`
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
	// WorkspaceProcesses/OrcaTerminals는 preview가 관측한 apply ①′ 종료 대상이고,
	// *Stopped는 apply가 실제로 종료한 집합이다(#477).
	WorkspaceProcesses        []CleanupWorkspaceProcess `json:"workspace_processes,omitempty"`
	OrcaTerminals             []string                  `json:"orca_terminals,omitempty"`
	WorkspaceProcessesStopped []CleanupWorkspaceProcess `json:"workspace_processes_stopped,omitempty"`
	OrcaTerminalsStopped      int                       `json:"orca_terminals_stopped,omitempty"`
	WorktreeRemoved           bool                      `json:"worktree_removed,omitempty"`
	BranchDeleted             bool                      `json:"branch_deleted,omitempty"`
	RecordDeleted             bool                      `json:"record_deleted,omitempty"`
	AbandonedAt               string                    `json:"abandoned_at,omitempty"`
	FailedStep                string                    `json:"failed_step,omitempty"`
	NextCommand               string                    `json:"next_command,omitempty"`
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
	WorkspaceProcesses []CleanupWorkspaceProcess `json:"workspace_processes,omitempty"`
	// OrcaTerminals는 preview가 관측한 워크트리 Orca 터미널 handle이고,
	// WorkspaceProcessesStopped/OrcaTerminalsStopped는 apply ①′가 실제로 종료한
	// 집합이다(#477).
	OrcaTerminals             []string                  `json:"orca_terminals,omitempty"`
	WorkspaceProcessesStopped []CleanupWorkspaceProcess `json:"workspace_processes_stopped,omitempty"`
	OrcaTerminalsStopped      int                       `json:"orca_terminals_stopped,omitempty"`
	OrcaWorktreeID            string                    `json:"orca_worktree_id,omitempty"`
	OrcaRemoved               bool                      `json:"orca_removed,omitempty"`
	WorktreeRemoved           bool                      `json:"worktree_removed,omitempty"`
	BranchDeleted             bool                      `json:"branch_deleted,omitempty"`
	AuditReflected            bool                      `json:"audit_reflected,omitempty"`
	AuditError                string                    `json:"audit_error,omitempty"`
	RecordDeleted             bool                      `json:"record_deleted,omitempty"`
	FailedStep                string                    `json:"failed_step,omitempty"`
	NextCommand               string                    `json:"next_command,omitempty"`
	// SupersededBy는 merged 게이트를 replacement 증거로 충족했을 때 그 artifact
	// URL이다. 무엇을 근거로 통과했는지 결과만 보고 알 수 있어야 한다.
	SupersededBy string `json:"superseded_by,omitempty"`
	// SupersedeError는 replacement 증거가 제시됐으나 검증에 실패한 사유다.
	// 비어 있으면 증거가 아예 없었다는 뜻이다.
	SupersedeError string `json:"supersede_error,omitempty"`
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
	// SupersededBy는 게이트 ⑩을 replacement 증거로 통과했을 때 그 artifact URL이다.
	SupersededBy string `json:"superseded_by,omitempty"`
	// SupersedeError는 replacement 증거가 제시됐으나 검증에 실패한 사유다.
	SupersedeError string `json:"supersede_error,omitempty"`
}

// CleanupLinkedBranchRequest는 `createLinkedBranch`가 남긴 ref-null 고아
// 레코드를 typed 경로로 정리하는 입력이다(#306 AC-04).
//
// 지울 대상을 사용자가 지정하지 않는다는 점이 이 표면의 핵심이다. ref가 없는
// 레코드는 이름이 없으므로 사람이 지목하면 오지목을 검증할 방법이 없다.
// preview가 이슈를 읽어 후보를 **하나로 확정**했을 때만 그 노드 id가 결속된
// fingerprint를 발급하고, apply는 그 fingerprint로만 진행한다.
type CleanupLinkedBranchRequest struct {
	ID          string
	Apply       bool
	Confirm     bool
	Fingerprint string
}

// CleanupLinkedBranchResult는 관측·분류·처분을 한 응답에 담는다.
type CleanupLinkedBranchResult struct {
	OK      bool     `json:"ok"`
	ID      string   `json:"id"`
	Preview bool     `json:"preview"`
	Missing []string `json:"missing,omitempty"`
	// State는 도메인 분류다: absent | healthy | orphan_ref_null | mismatched | ambiguous.
	// 삭제가 허용되는 값은 orphan_ref_null 하나뿐이다.
	State string `json:"state"`
	// StateReason은 왜 그 분류인지다. 지울 수 없는 이유가 사용자에게 보이지
	// 않으면 raw GraphQL로 우회하게 된다 — 그것이 이 이슈가 금지하는 경로다.
	StateReason     string `json:"state_reason,omitempty"`
	IssueURL        string `json:"issue_url,omitempty"`
	RequestedBranch string `json:"requested_branch,omitempty"`
	SealedBase      string `json:"sealed_base,omitempty"`
	// LinkedBranchID는 preview가 확정한 삭제 대상 노드다. 확정하지 못하면 비어 있다.
	LinkedBranchID string `json:"linked_branch_id,omitempty"`
	LinkedCount    int    `json:"linked_count"`
	// RemoteRefOID는 refs/heads/<RequestedBranch>의 같은 시점 관측이다.
	RemoteRefOID  string `json:"remote_ref_oid,omitempty"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	AlreadyAbsent bool   `json:"already_absent,omitempty"`
	Deleted       bool   `json:"deleted,omitempty"`
	DeletedAt     string `json:"deleted_at,omitempty"`
	// AuditRecorded는 처분을 durable audit에 남겼는지다(AC-06).
	AuditRecorded bool   `json:"audit_recorded,omitempty"`
	AuditError    string `json:"audit_error,omitempty"`
	ObserveError  string `json:"observe_error,omitempty"`
	FailedStep    string `json:"failed_step,omitempty"`
	NextCommand   string `json:"next_command,omitempty"`
}

// AwaitBranchLinkRequest는 coordinator가 만들 linked branch가 나타날 때까지
// 경계 있게 기다리는 읽기 전용 요청이다(#319).
type AwaitBranchLinkRequest struct {
	ID string
	// Timeout은 Go duration 문자열이다. 비어 있으면 기본값을 쓴다.
	Timeout string
}

// AwaitBranchLinkResult는 대기 결과와 그 근거가 된 관측이다.
type AwaitBranchLinkResult struct {
	OK      bool     `json:"ok"`
	ID      string   `json:"id"`
	Missing []string `json:"missing,omitempty"`
	// Linked는 봉인된 정체성과 일치하는 링크를 관측했는지다.
	Linked          bool   `json:"linked"`
	AlreadyVerified bool   `json:"already_verified,omitempty"`
	TimedOut        bool   `json:"timed_out,omitempty"`
	IssueURL        string `json:"issue_url,omitempty"`
	Branch          string `json:"branch,omitempty"`
	SealedBase      string `json:"sealed_base,omitempty"`
	LinkedBranchID  string `json:"linked_branch_id,omitempty"`
	ObservedOID     string `json:"observed_oid,omitempty"`
	// State와 StateReason은 마지막 관측의 분류다. 기다리다 실패했을 때
	// 무엇을 보고 있었는지가 남아야 다음 사람이 다시 조사하지 않는다.
	State          string `json:"state,omitempty"`
	StateReason    string `json:"state_reason,omitempty"`
	Attempts       int    `json:"attempts"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	NextCommand    string `json:"next_command,omitempty"`
}

// CleanupWorkspaceProcess는 cleanup preview가 관측한 워크트리 점유 프로세스 하나다.
// receipt(pid·시작 시각·실행 파일)는 apply에서 PID 재사용을 배제하고, Descendants와
// Collateral(워크트리를 점유하지 않는 자손 수)은 공유 서버처럼 자손이 많은
// 점유자를 apply 전에 알아보게 한다(#477).
type CleanupWorkspaceProcess struct {
	PID         int    `json:"pid"`
	Command     string `json:"command"`
	StartedAt   string `json:"started_at"`
	Executable  string `json:"executable"`
	Descendants int    `json:"descendants"`
	Collateral  int    `json:"collateral"`
}
