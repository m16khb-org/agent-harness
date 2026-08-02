package issueopslease

import (
	"fmt"
	"strings"
)

const (
	DenyResumeLease              DenyCode = "resume_lease"
	DenyResumeOwnerContradiction DenyCode = "resume_owner_contradiction"
	DenyResumeOwnerLive          DenyCode = "resume_owner_live"
	DenyResumeTerminalIdentity   DenyCode = "resume_terminal_identity"
	DenyResumeRuntimeIdentity    DenyCode = "resume_runtime_identity"
	DenyResumeStage              DenyCode = "resume_stage"
)

type ResumeDisposition string

const (
	ResumeExistingBinding ResumeDisposition = "existing_binding"
	ResumeReuseTerminal   ResumeDisposition = "reuse_terminal"
	ResumeCreateTerminal  ResumeDisposition = "create_terminal"
)

type ResumeInventory struct {
	RuntimeID    string
	TerminalLive bool
	TaskLive     bool
	TerminalID   string
}

type ResumeRequest struct {
	ExpectedGeneration uint64
	Lease              Lease
	BindingGeneration  uint64
	BindingRuntimeID   string
	BindingTerminalID  string
	CanonicalCWD       bool
	ModeOrca           bool
	BindingPresent     bool
	PendingAbsent      bool
	RuntimeCompatible  bool
	Inventory          ResumeInventory
}

type ResumePlan struct {
	Disposition         ResumeDisposition
	RuntimeID           string
	ReusedTerminalPTYID string
}

func PlanResume(request ResumeRequest) (ResumePlan, error) {
	if !request.ModeOrca || !request.BindingPresent {
		return ResumePlan{}, Deny(DenyResumeLease, fmt.Errorf("execution resume requires an existing Orca binding"))
	}
	if !request.PendingAbsent {
		return ResumePlan{}, Deny(DenyResumeLease, fmt.Errorf("execution resume is blocked by a pending external intent; run execution reconcile"))
	}
	if request.Lease.Generation != request.ExpectedGeneration || request.Lease.Status != "claimable" || request.Lease.Holder != nil || strings.TrimSpace(request.Lease.ClaimTokenSHA256) == "" {
		return ResumePlan{}, Deny(DenyResumeLease, fmt.Errorf("execution resume requires a holderless claimable lease"))
	}
	if !request.CanonicalCWD {
		return ResumePlan{}, Deny(DenyCanonicalCWD, fmt.Errorf("execution resume cwd must be the canonical worktree"))
	}
	if !request.RuntimeCompatible {
		return ResumePlan{}, Deny(DenyResumeRuntimeIdentity, fmt.Errorf("Orca runtime identity is incompatible"))
	}

	runtimeID := strings.TrimSpace(request.Inventory.RuntimeID)
	if runtimeID == "" {
		runtimeID = strings.TrimSpace(request.BindingRuntimeID)
	}
	if request.Inventory.TaskLive {
		if !request.Inventory.TerminalLive {
			return ResumePlan{}, Deny(DenyResumeOwnerContradiction, fmt.Errorf("Orca owner inventory has a live task without a live terminal"))
		}
		if request.BindingGeneration != request.Lease.Generation {
			return ResumePlan{}, Deny(DenyResumeOwnerLive, fmt.Errorf("previous Orca owner task is still live"))
		}
		return ResumePlan{Disposition: ResumeExistingBinding, RuntimeID: runtimeID}, nil
	}
	if request.Inventory.TerminalLive {
		terminalID := strings.TrimSpace(request.Inventory.TerminalID)
		if terminalID == "" || terminalID != strings.TrimSpace(request.BindingTerminalID) {
			return ResumePlan{}, Deny(DenyResumeTerminalIdentity, fmt.Errorf("Orca owner terminal identity changed"))
		}
		return ResumePlan{Disposition: ResumeReuseTerminal, RuntimeID: runtimeID, ReusedTerminalPTYID: terminalID}, nil
	}
	return ResumePlan{Disposition: ResumeCreateTerminal, RuntimeID: runtimeID}, nil
}

type ResumeStageAction string

const (
	ResumeStageAdopt     ResumeStageAction = "adopt"
	ResumeStageInvoke    ResumeStageAction = "invoke"
	ResumeStageReconcile ResumeStageAction = "reconcile"
)

type ResumeStageRequest struct {
	CandidateCount     int
	AuthoritativeZero  bool
	InvocationState    string
	InvocationAttempts int
}

type ResumeStagePlan struct {
	Action         ResumeStageAction
	CandidateIndex int
	Reason         string
}

func PlanResumeStage(request ResumeStageRequest) (ResumeStagePlan, error) {
	if request.CandidateCount > 1 {
		return ResumeStagePlan{}, Deny(DenyResumeStage, fmt.Errorf("multiple-candidates"))
	}
	if request.CandidateCount == 1 {
		return ResumeStagePlan{Action: ResumeStageAdopt, CandidateIndex: 0}, nil
	}
	if !request.AuthoritativeZero {
		return ResumeStagePlan{Action: ResumeStageReconcile, Reason: "non-authoritative-zero"}, nil
	}
	if request.InvocationState != "not_invoked_proven" {
		return ResumeStagePlan{Action: ResumeStageReconcile, Reason: "unknown-invocation"}, nil
	}
	if request.InvocationAttempts >= 2 {
		return ResumeStagePlan{}, Deny(DenyResumeStage, fmt.Errorf("retry-exhausted"))
	}
	return ResumeStagePlan{Action: ResumeStageInvoke}, nil
}
