package lifecycle

import (
	"errors"

	issueopscontract "agent-harness/internal/contract/issueops"
)

var errIssueOpsNotConfigured = errors.New("issueops runtime is not configured")

// IssueOps 사이클 읽기·쓰기는 상태 저장소를 다루는 I/O다. lifecycle은 그 구현을
// 모르고 composition root가 주입한 함수만 호출한다.
var issueOpsDeps = neutralIssueOpsDeps()

// IssueOpsDeps는 composition root가 실제 어댑터를 꽂는 진입점이다.
type IssueOpsDeps struct {
	ActiveIssueOpsCycleForBranch              func(repo, branch string) (issueopscontract.IssueOpsRecord, bool)
	ActiveIssueOpsLinkedWorktreeCyclesForRepo func(repo string) []issueopscontract.IssueOpsRecord
	AdvanceIssueOpsPhase                      func(stateRoot, id, to string) (issueopscontract.IssueOpsRecord, error)
	IssueOpsPhaseExpectsWorktree              func(phase issueopscontract.IssueOpsPhase) bool
	IssueOpsStateRoot                         func() string
	LinkIssueOpsIssue                         func(stateRoot, id, issueURL string) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsPlan                          func(stateRoot, id, planPath string) (issueopscontract.IssueOpsRecord, error)
	LinkIssueOpsWorktree                      func(stateRoot, id, worktreePath string) (issueopscontract.IssueOpsRecord, error)
	ListIssueOpsIDs                           func(stateRoot string) ([]string, error)
	ScanIssueOps                              func(stateRoot string) ([]issueopscontract.IssueOpsRecord, error)
	ScanReadableIssueOps                      func(stateRoot string) ([]issueopscontract.IssueOpsRecord, error)
	NewIssueOpsID                             func(repo, branch string) string
	PrepareIssueOpsBranch                     func(stateRoot, id string, req issueopscontract.IssueOpsBranchPrepareRequest) (issueopscontract.IssueOpsRecord, error)
	ReadIssueOps                              func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsDesignReview                func(stateRoot, id string, req issueopscontract.IssueOpsDesignReviewRequest) (issueopscontract.IssueOpsRecord, error)
	RecordIssueOpsIntent                      func(stateRoot, id string, req issueopscontract.IssueOpsIntentRecordRequest) (issueopscontract.IssueOpsRecord, error)
	SealedOwnerContextPacketPath              func(record issueopscontract.IssueOpsRecord) string
	StartIssueOps                             func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error)
	WriteIssueOps                             func(stateRoot string, record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error)
}

func ConfigureIssueOps(deps IssueOpsDeps) {
	if deps.ActiveIssueOpsCycleForBranch != nil {
		issueOpsDeps.ActiveIssueOpsCycleForBranch = deps.ActiveIssueOpsCycleForBranch
	}
	if deps.ActiveIssueOpsLinkedWorktreeCyclesForRepo != nil {
		issueOpsDeps.ActiveIssueOpsLinkedWorktreeCyclesForRepo = deps.ActiveIssueOpsLinkedWorktreeCyclesForRepo
	}
	if deps.AdvanceIssueOpsPhase != nil {
		issueOpsDeps.AdvanceIssueOpsPhase = deps.AdvanceIssueOpsPhase
	}
	if deps.IssueOpsPhaseExpectsWorktree != nil {
		issueOpsDeps.IssueOpsPhaseExpectsWorktree = deps.IssueOpsPhaseExpectsWorktree
	}
	if deps.IssueOpsStateRoot != nil {
		issueOpsDeps.IssueOpsStateRoot = deps.IssueOpsStateRoot
	}
	if deps.LinkIssueOpsIssue != nil {
		issueOpsDeps.LinkIssueOpsIssue = deps.LinkIssueOpsIssue
	}
	if deps.LinkIssueOpsPlan != nil {
		issueOpsDeps.LinkIssueOpsPlan = deps.LinkIssueOpsPlan
	}
	if deps.LinkIssueOpsWorktree != nil {
		issueOpsDeps.LinkIssueOpsWorktree = deps.LinkIssueOpsWorktree
	}
	if deps.ListIssueOpsIDs != nil {
		issueOpsDeps.ListIssueOpsIDs = deps.ListIssueOpsIDs
	}
	if deps.ScanIssueOps != nil {
		issueOpsDeps.ScanIssueOps = deps.ScanIssueOps
	}
	if deps.ScanReadableIssueOps != nil {
		issueOpsDeps.ScanReadableIssueOps = deps.ScanReadableIssueOps
	}
	if deps.NewIssueOpsID != nil {
		issueOpsDeps.NewIssueOpsID = deps.NewIssueOpsID
	}
	if deps.PrepareIssueOpsBranch != nil {
		issueOpsDeps.PrepareIssueOpsBranch = deps.PrepareIssueOpsBranch
	}
	if deps.ReadIssueOps != nil {
		issueOpsDeps.ReadIssueOps = deps.ReadIssueOps
	}
	if deps.RecordIssueOpsDesignReview != nil {
		issueOpsDeps.RecordIssueOpsDesignReview = deps.RecordIssueOpsDesignReview
	}
	if deps.RecordIssueOpsIntent != nil {
		issueOpsDeps.RecordIssueOpsIntent = deps.RecordIssueOpsIntent
	}
	if deps.SealedOwnerContextPacketPath != nil {
		issueOpsDeps.SealedOwnerContextPacketPath = deps.SealedOwnerContextPacketPath
	}
	if deps.StartIssueOps != nil {
		issueOpsDeps.StartIssueOps = deps.StartIssueOps
	}
	if deps.WriteIssueOps != nil {
		issueOpsDeps.WriteIssueOps = deps.WriteIssueOps
	}
}

// 배선 누락이 패닉이 아니라 명시적 오류로 드러나도록 중립 기본값을 둔다.
func neutralIssueOpsDeps() IssueOpsDeps {
	return IssueOpsDeps{
		ActiveIssueOpsCycleForBranch: func(repo, branch string) (issueopscontract.IssueOpsRecord, bool) {
			return issueopscontract.IssueOpsRecord{}, false
		},
		ActiveIssueOpsLinkedWorktreeCyclesForRepo: func(repo string) []issueopscontract.IssueOpsRecord { return nil },
		AdvanceIssueOpsPhase: func(stateRoot, id, to string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		IssueOpsPhaseExpectsWorktree: func(phase issueopscontract.IssueOpsPhase) bool { return false },
		IssueOpsStateRoot:            func() string { return "" },
		LinkIssueOpsIssue: func(stateRoot, id, issueURL string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		LinkIssueOpsPlan: func(stateRoot, id, planPath string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		LinkIssueOpsWorktree: func(stateRoot, id, worktreePath string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		ListIssueOpsIDs: func(stateRoot string) ([]string, error) { return nil, errIssueOpsNotConfigured },
		ScanIssueOps: func(stateRoot string) ([]issueopscontract.IssueOpsRecord, error) {
			return nil, errIssueOpsNotConfigured
		},
		ScanReadableIssueOps: func(stateRoot string) ([]issueopscontract.IssueOpsRecord, error) {
			return nil, errIssueOpsNotConfigured
		},
		NewIssueOpsID: func(repo, branch string) string { return "" },
		PrepareIssueOpsBranch: func(stateRoot, id string, req issueopscontract.IssueOpsBranchPrepareRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		ReadIssueOps: func(stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		RecordIssueOpsDesignReview: func(stateRoot, id string, req issueopscontract.IssueOpsDesignReviewRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		RecordIssueOpsIntent: func(stateRoot, id string, req issueopscontract.IssueOpsIntentRecordRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		SealedOwnerContextPacketPath: func(record issueopscontract.IssueOpsRecord) string { return "" },
		StartIssueOps: func(stateRoot string, req issueopscontract.IssueOpsStartRequest) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
		WriteIssueOps: func(stateRoot string, record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{}, errIssueOpsNotConfigured
		},
	}
}
