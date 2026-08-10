// issueops 실행 API의 요청·결과·핸들러 계약이다. 값을 만들어내는 쪽은 I/O를
// 하지만 읽고 전달하는 쪽은 그 구현을 알 필요가 없다.
package issueops

import (
	"context"
)

type ExecutionReleaseHandler func(context.Context, string, ExecutionReleaseRequest) (ExecutionResult, error)
type ExecutionResumeHandler func(context.Context, string, ExecutionResumeRequest) (ExecutionResumeResult, error)
type ExecutionCompleteHandler func(context.Context, string, ExecutionCompleteRequest) (ExecutionResult, error)
type RemotePullRequestReconcileHandler func(context.Context, string, ExecutionReconcileRequest) (ExecutionReconcileResult, error)

// ExecutionCompleteRequest is the transport DTO consumed by the current
// completion inbound adapter.
type ExecutionCompleteRequest struct {
	ID                string      `json:"id"`
	Generation        uint64      `json:"generation"`
	Actor             NativeActor `json:"actor"`
	CWD               string      `json:"cwd"`
	FinalHead         string      `json:"final_head"`
	TuringReportPath  string      `json:"turing_report_path"`
	Verification      []string    `json:"verification"`
	RemoteArtifactURL string      `json:"remote_artifact_url"`
	Confirm           bool        `json:"confirm"`
}
type ExecutionResult struct {
	OK        bool      `json:"ok"`
	ID        string    `json:"id"`
	Execution Execution `json:"execution"`
	// OrcaTaskSettled와 OrcaTaskError는 기존 JSON 소비자 호환을 위해 남긴다.
	// completion은 Orca task를 종료하지 않으므로 새 completion 응답에서는 생략된다.
	OrcaTaskSettled     bool   `json:"orca_task_settled,omitempty"`
	OrcaTaskError       string `json:"orca_task_error,omitempty"`
	IssueSnapshotSource string `json:"issue_snapshot_source,omitempty"`
	NextCommand         string `json:"next_command,omitempty"`
}
type ExecutionClaimRequest struct {
	ID                  string      `json:"id"`
	Generation          uint64      `json:"generation"`
	Actor               NativeActor `json:"actor"`
	CWD                 string      `json:"cwd"`
	TokenFile           string      `json:"claim_token_file,omitempty"`
	ClaimCurrentToken   bool        `json:"claim_current_token,omitempty"`
	IssueBodySHA256     string      `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256 string      `json:"context_packet_sha256,omitempty"`
}
type ExecutionReleaseRequest struct {
	ID         string      `json:"id"`
	Generation uint64      `json:"generation"`
	Actor      NativeActor `json:"actor"`
	CWD        string      `json:"cwd"`
}
type ExecutionReplaceResult struct {
	OK                    bool      `json:"ok"`
	ID                    string    `json:"id"`
	Action                string    `json:"action"`
	Execution             Execution `json:"execution"`
	InventoryFingerprint  string    `json:"inventory_fingerprint,omitempty"`
	QuiescenceFingerprint string    `json:"quiescence_fingerprint,omitempty"`
	ClaimTokenPath        string    `json:"claim_token_path,omitempty"`
	// 아래 값은 replacement가 새 generation으로 재봉인한 owner artifact의
	// 정체다. owner는 digest들을 claim 명령에 그대로 넣어야 하므로 노출한다.
	IssueBodySHA256     string `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string `json:"owner_prompt_sha256,omitempty"`
	IssueSnapshotSource string `json:"issue_snapshot_source,omitempty"`
	NextCommand         string `json:"next_command,omitempty"`
}

// ExecutionSwitchModeRequest는 준비된 실행의 모드를 바꾸는 입력이다.
//
// prepare가 아니라 별도 명령인 이유는 파괴 범위다. Orca는 기존 브랜치나 경로를
// 입양하지 않고 path_collision으로 거부하므로(orca
// src/main/ipc/workspace-create-error-classifier.ts) 전환은 반드시 기존 워크트리와
// 로컬 브랜치 제거를 동반한다. cleanup abandon과 finish가 각자 이름을 가진 것과
// 같은 이유로, 그 조작을 준비 명령 안에 숨기지 않는다(이슈 #167).
type ExecutionSwitchModeRequest struct {
	ID          string
	Mode        string
	CWD         string
	Apply       bool
	Confirm     bool
	Fingerprint string
	Actor       NativeActor
}

// ExecutionSwitchModeDependencies는 게이트 평가와 정리의 외부 표면이다.
// Git이 nil이면 preflight를 쓴다 — cleanup 경로와 같은 관례다.
type ExecutionSwitchModeDependencies struct {
	Git func(dir string, args ...string) (int, string)
}
type ExecutionSwitchModeResult struct {
	OK              bool     `json:"ok"`
	ID              string   `json:"id"`
	Preview         bool     `json:"preview"`
	CurrentMode     string   `json:"current_mode,omitempty"`
	RequestedMode   string   `json:"requested_mode,omitempty"`
	LeaseGeneration uint64   `json:"lease_generation,omitempty"`
	Missing         []string `json:"missing,omitempty"`
	Fingerprint     string   `json:"fingerprint,omitempty"`
	WorktreeRoot    string   `json:"worktree_root,omitempty"`
	Branch          string   `json:"branch,omitempty"`
	// WorktreePresent와 BranchPresent는 apply가 실제로 지울 대상이다. preview가
	// 이 둘을 보여주지 않으면 사용자는 무엇을 승인하는지 모른다.
	WorktreePresent bool `json:"worktree_present"`
	BranchPresent   bool `json:"branch_present"`
	// BranchFreeError는 orca 전환이 브랜치 이름에 막혔을 때의 원인이다.
	// missing 슬러그만으로는 이름이 로컬에 있는지 원격에 있는지 알 수 없다.
	BranchFreeError string `json:"branch_free_error,omitempty"`
	NextCommand     string `json:"next_command,omitempty"`
	NextAction      string `json:"next_action,omitempty"`
	SwitchedAt      string `json:"switched_at,omitempty"`
}
type ExecutionPrepareRequest struct {
	ID          string      `json:"id"`
	Mode        string      `json:"mode"`
	Actor       NativeActor `json:"actor"`
	CWD         string      `json:"cwd"`
	OwnerHost   string      `json:"owner_host,omitempty"`
	OwnerModel  string      `json:"owner_model,omitempty"`
	OwnerEffort string      `json:"owner_effort,omitempty"`
	// 선택 영수증 입력: 명시적 direct 근거와 준비도 지문을 함께 받는다.
	IssueSnapshotFile            string `json:"issue_snapshot_file,omitempty"`
	DirectReason                 string `json:"direct_reason,omitempty"`
	ExpectedReadinessFingerprint string `json:"expected_readiness_fingerprint,omitempty"`
	Confirm                      bool   `json:"confirm,omitempty"`
}
type ExecutionPrepareResult struct {
	OK            bool   `json:"ok"`
	ID            string `json:"id"`
	Preview       bool   `json:"preview,omitempty"`
	RequestedMode string `json:"requested_mode"`
	ResolvedMode  string `json:"resolved_mode"`
	FallbackCode  string `json:"fallback_code,omitempty"`
	// 준비도 탐침 영수증: 무엇을 시도했고 무엇을 보았는지 보존한다.
	ProbeAttempted       bool       `json:"probe_attempted"`
	ProbeAvailable       bool       `json:"probe_available"`
	ProbeReady           bool       `json:"probe_ready"`
	ProbeCode            string     `json:"probe_code,omitempty"`
	ReadinessFingerprint string     `json:"readiness_fingerprint,omitempty"`
	ExplicitDirectReason string     `json:"explicit_direct_reason,omitempty"`
	Workspace            Workspace  `json:"workspace"`
	Execution            *Execution `json:"execution,omitempty"`
	ClaimTokenPath       string     `json:"claim_token_path,omitempty"`
	IssueBodySHA256      string     `json:"issue_body_sha256,omitempty"`
	ContextPacketPath    string     `json:"context_packet_path,omitempty"`
	ContextPacketSHA256  string     `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath      string     `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256    string     `json:"owner_prompt_sha256,omitempty"`
	IssueSnapshotSource  string     `json:"issue_snapshot_source,omitempty"`
	NextCommand          string     `json:"next_command,omitempty"`
}
type ExecutionReconcileRequest struct {
	ID       string          `json:"id"`
	Preview  bool            `json:"preview,omitempty"`
	Confirm  bool            `json:"confirm,omitempty"`
	Actor    NativeActor     `json:"actor"`
	CWD      string          `json:"cwd"`
	Snapshot *IssueOpsRecord `json:"-"`
}
type ExecutionReconcileResult struct {
	OK                  bool            `json:"ok"`
	ID                  string          `json:"id"`
	Preview             bool            `json:"preview,omitempty"`
	Reconciled          bool            `json:"reconciled"`
	Code                string          `json:"code"`
	Execution           Execution       `json:"execution"`
	Pending             *ExternalIntent `json:"pending,omitempty"`
	IssueSnapshotSource string          `json:"issue_snapshot_source,omitempty"`
	// ExternalStateInspected는 이 결과가 외부 자원을 실제로 조회하고 나온
	// 것인지 밝힌다. preview는 pending kind만 보고 상수 코드를 돌려주므로
	// false다 — 그 구분이 없으면 preview 출력이 "Orca 자원이 이런 상태다"라는
	// 관측 증거로 오독된다(#99의 오진단이 그렇게 생겼다, 이슈 #154).
	//
	// omitempty를 쓰지 않는다. "조회하지 않았다"가 이 필드의 핵심 정보이므로
	// false가 출력에서 사라지면 목적 자체가 무너진다.
	ExternalStateInspected bool `json:"external_state_inspected"`
}
type RemotePullRequestRequest struct {
	ID                 string      `json:"id"`
	Provider           string      `json:"provider"`
	Title              string      `json:"title"`
	Body               string      `json:"body"`
	Head               string      `json:"head"`
	Base               string      `json:"base"`
	Labels             []string    `json:"labels"`
	Assignees          []string    `json:"assignees"`
	ExpectedGeneration uint64      `json:"expected_generation"`
	Actor              NativeActor `json:"actor"`
	CWD                string      `json:"cwd"`
	Confirm            bool        `json:"confirm"`
}
type ExecutionResumeRequest struct {
	ID                 string      `json:"id"`
	ExpectedGeneration uint64      `json:"expected_generation"`
	Actor              NativeActor `json:"actor"`
	CWD                string      `json:"cwd"`
	Confirm            bool        `json:"confirm"`
}
type ExecutionResumeResult struct {
	OK                  bool      `json:"ok"`
	ID                  string    `json:"id"`
	ResumeDisposition   string    `json:"resume_disposition"`
	Execution           Execution `json:"execution"`
	ClaimTokenPath      string    `json:"claim_token_path"`
	IssueBodySHA256     string    `json:"issue_body_sha256"`
	ContextPacketPath   string    `json:"context_packet_path"`
	ContextPacketSHA256 string    `json:"context_packet_sha256"`
	OwnerPromptPath     string    `json:"owner_prompt_path"`
	OwnerPromptSHA256   string    `json:"owner_prompt_sha256"`
	NextCommand         string    `json:"next_command"`
}
type ExecutionSyncBaseRequest struct {
	ID                   string      `json:"id"`
	Mode                 string      `json:"mode"`
	CompletionGeneration uint64      `json:"completion_generation,omitempty"`
	Actor                NativeActor `json:"actor,omitempty"`
	CWD                  string      `json:"cwd"`
	Confirm              bool        `json:"confirm,omitempty"`
	Fingerprint          string      `json:"fingerprint,omitempty"`
}

// ExecutionSyncBaseDeps는 Git 표면 하나만 주입점으로 연다. fetch·merge-tree·
// merge·commit·push가 전부 이 함수를 지나므로 테스트가 순서와 인자를 전수
// 검증할 수 있고, 비대화 env 계약과 timeout도 한 곳에서 강제된다.
type ExecutionSyncBaseDeps struct {
	Git func(ctx context.Context, dir string, args ...string) (int, string)
}
type ExecutionSyncBaseResult struct {
	OK                  bool     `json:"ok"`
	ID                  string   `json:"id"`
	Mode                string   `json:"mode"`
	LeaseGeneration     uint64   `json:"lease_generation,omitempty"`
	Missing             []string `json:"missing,omitempty"`
	Fingerprint         string   `json:"fingerprint,omitempty"`
	Branch              string   `json:"branch,omitempty"`
	BaseBranch          string   `json:"base_branch,omitempty"`
	BaseOID             string   `json:"base_oid,omitempty"`
	WorkOID             string   `json:"work_oid,omitempty"`
	RemoteBranchPresent bool     `json:"remote_branch_present"`
	MergeInProgress     bool     `json:"merge_in_progress"`
	MergeNeeded         bool     `json:"merge_needed"`
	ConflictFiles       []string `json:"conflict_files,omitempty"`
	UntrackedWarnings   []string `json:"untracked_warnings,omitempty"`
	Merged              bool     `json:"merged,omitempty"`
	MergeCommit         string   `json:"merge_commit,omitempty"`
	Pushed              bool     `json:"pushed,omitempty"`
	PushRetryRequired   bool     `json:"push_retry_required,omitempty"`
	Aborted             bool     `json:"aborted,omitempty"`
	FailedStep          string   `json:"failed_step,omitempty"`
	NextCommand         string   `json:"next_command,omitempty"`
	AbortCommand        string   `json:"abort_command,omitempty"`
}
