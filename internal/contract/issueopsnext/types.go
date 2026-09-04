// Package issueopsnext holds the wire contract of `issueops next`:
// the read-only projection that says which stage of the ten-stage cycle the
// caller is in and which command advances it.
//
// 이 패키지는 stage key와 명령 문자열만 소유한다. 그 키를 스킬 이름이나
// 한국어 label로 바꾸는 표는 라우터 스킬이 소유한다 — 소유자를 하나로 두기
// 위해서다.
package issueopsnext

import issueopscontract "issueops/internal/contract/issueops"

type Record = issueopscontract.IssueOpsRecord
type Phase = issueopscontract.IssueOpsPhase
type Readiness = issueopscontract.IssueOpsReadiness

const (
	PhaseProblem             = issueopscontract.IssueOpsPhaseProblem
	PhaseGrill               = issueopscontract.IssueOpsPhaseGrill
	PhasePlan                = issueopscontract.IssueOpsPhasePlan
	PhaseCompatibilityReview = issueopscontract.IssueOpsPhaseCompatibilityReview
	PhaseImplement           = issueopscontract.IssueOpsPhaseImplement
	PhaseAISlopClean         = issueopscontract.IssueOpsPhaseAISlopClean
	PhaseFeedback            = issueopscontract.IssueOpsPhaseFeedback
	PhasePR                  = issueopscontract.IssueOpsPhasePR
	PhaseDone                = issueopscontract.IssueOpsPhaseDone
)

const (
	LeaseStatusClaimable = issueopscontract.LeaseStatusClaimable
	LeaseStatusActive    = issueopscontract.LeaseStatusActive
	LeaseStatusRevoking  = issueopscontract.LeaseStatusRevoking
	LeaseStatusReleased  = issueopscontract.LeaseStatusReleased
)

// NextCommandKind는 next_command를 그대로 실행해도 되는지를 말한다. template은
// `<...>`나 `$ACTOR_FLAGS` 자리표시자가 남아 있어 사람이 값을 채워야 한다.
const (
	NextCommandKindExact    = "exact"
	NextCommandKindTemplate = "template"
)

// CWDRole은 호출 디렉터리가 사이클의 어느 쪽인지를 말한다.
const (
	CWDRoleSource   = "source"
	CWDRoleWorktree = "worktree"
	CWDRoleOther    = "other"
)

type Result struct {
	OK              bool     `json:"ok"`
	GeneratedAt     string   `json:"generated_at"`
	CWD             string   `json:"cwd"`
	CWDRole         string   `json:"cwd_role"`
	SourceRoot      string   `json:"source_root"`
	Selected        *Entry   `json:"selected,omitempty"`
	Candidates      []Entry  `json:"candidates,omitempty"`
	Stage           Stage    `json:"stage"`
	Lease           Lease    `json:"lease"`
	Missing         []string `json:"missing"`
	NextCommand     string   `json:"next_command,omitempty"`
	NextCommandKind string   `json:"next_command_kind,omitempty"`
	Exits           Exits    `json:"exits"`
	Review          Review   `json:"review"`
	Warnings        []string `json:"warnings,omitempty"`
}

// Review는 코드가 소유하는 host별 리뷰어 기본값이다(internal/port/orca.go).
// 스킬이 모델 이름을 적어 두면 그 값이 곧 낡으므로 CLI가 돌려준다.
type Review struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
}

type Entry struct {
	ID                string `json:"id"`
	Phase             string `json:"phase"`
	Branch            string `json:"branch,omitempty"`
	WorkspaceRoot     string `json:"workspace_root,omitempty"`
	IssueURL          string `json:"issue_url,omitempty"`
	RemoteArtifactURL string `json:"remote_artifact_url,omitempty"`
}

type Stage struct {
	Key   string `json:"key"`
	Index int    `json:"index"`
}

type Lease struct {
	Status       string `json:"status,omitempty"`
	Generation   uint64 `json:"generation,omitempty"`
	HolderIsSelf bool   `json:"holder_is_self"`
	HolderLive   *bool  `json:"holder_live,omitempty"`
}

// Exits는 어느 단계에서든 빠져나가는 세 경로다. 사이클이 없으면 전부 빈 값이다.
type Exits struct {
	PauseCommand    string `json:"pause_command,omitempty"`
	AbandonCommand  string `json:"abandon_command"`
	TakeoverCommand string `json:"takeover_command,omitempty"`
}

// Stage key는 라우터가 스킬·label·선택지로 바꾸는 유일한 입력이다. 새 키를
// 늘리면 라우터의 `## 단계 표`에도 같은 키의 행이 있어야 한다.
const (
	StageNone              = "none"
	StageAmbiguous         = "ambiguous"
	StageInvalid           = "invalid"
	StageBlockedPending    = "blocked.pending"
	StageBlockedRoot       = "blocked.root_conflict"
	StageBlockedHolderLive = "blocked.holder_live"
	StageTakeover          = "takeover"
	StageClaim             = "claim"
	StageIssue             = "issue"
	StagePrepare           = "prepare"
	StagePlanWrite         = "plan.write"
	StagePlanDesign        = "plan.design"
	StagePlanReview        = "plan.review"
	StagePlanHandoff       = "plan.handoff"
	StageImplementEnter    = "implement.enter"
	StageImplement         = "implement"
	StageClean             = "clean"
	StageDocs              = "docs"
	StageVerify            = "verify"
	StageCommitPush        = "commit-push"
	StagePRCreate          = "pr.create"
	StagePRComplete        = "pr.complete"
	StageDone              = "done"
	StageUnknown           = "unknown"
)
