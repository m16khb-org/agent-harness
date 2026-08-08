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
	RuntimeID                 string
	TerminalLive              bool
	TerminalInventoryComplete bool
	TaskLive                  bool
	TerminalID                string
	TaskStatus                string
	DispatchStatus            string
	DispatchAssigneeHandle    string
	DispatchAssigneePresent   bool
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
	Inventory          ResumeInventory
}

type ResumePlan struct {
	Disposition         ResumeDisposition
	RuntimeID           string
	ReusedTerminalPTYID string
}

type HolderlessRuntimeRolloverRequest struct {
	Lease            Lease
	BindingRuntimeID string
	Inventory        ResumeInventory
}

func ValidateHolderlessRuntimeRollover(request HolderlessRuntimeRolloverRequest) error {
	sealedRuntimeID := strings.TrimSpace(request.BindingRuntimeID)
	observedRuntimeID := strings.TrimSpace(request.Inventory.RuntimeID)
	if sealedRuntimeID == "" || observedRuntimeID == "" {
		return Deny(DenyResumeRuntimeIdentity, fmt.Errorf("Orca runtime identity is incomplete"))
	}
	if observedRuntimeID == sealedRuntimeID {
		return nil
	}
	if request.Lease.Holder != nil || (request.Lease.Status != "released" && request.Lease.Status != "claimable") {
		return Deny(DenyResumeRuntimeIdentity, fmt.Errorf("Orca runtime rollover requires a holderless released or claimable lease"))
	}
	if !request.Inventory.TerminalInventoryComplete || request.Inventory.TerminalID != "" || request.Inventory.TerminalLive ||
		request.Inventory.TaskLive || !terminalTaskStatus(request.Inventory.TaskStatus) ||
		!recoverableStaleDispatchStatus(request.Inventory.DispatchStatus) || strings.TrimSpace(request.Inventory.DispatchAssigneeHandle) == "" ||
		request.Inventory.DispatchAssigneePresent {
		return Deny(DenyResumeRuntimeIdentity, fmt.Errorf("Orca runtime rollover owner evidence is not holderless and settled"))
	}
	return nil
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
	runtimeID := strings.TrimSpace(request.Inventory.RuntimeID)
	if runtimeID == "" {
		runtimeID = strings.TrimSpace(request.BindingRuntimeID)
	}
	if runtimeID != strings.TrimSpace(request.BindingRuntimeID) {
		if err := ValidateHolderlessRuntimeRollover(HolderlessRuntimeRolloverRequest{
			Lease: request.Lease, BindingRuntimeID: request.BindingRuntimeID, Inventory: request.Inventory,
		}); err != nil {
			return ResumePlan{}, err
		}
		return ResumePlan{Disposition: ResumeCreateTerminal, RuntimeID: runtimeID}, nil
	}
	terminalID := strings.TrimSpace(request.Inventory.TerminalID)
	if terminalID != "" && !request.Inventory.TerminalLive {
		return ResumePlan{}, Deny(DenyResumeTerminalIdentity, fmt.Errorf("Orca owner terminal is present but not live"))
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
		if terminalID == "" || terminalID != strings.TrimSpace(request.BindingTerminalID) {
			return ResumePlan{}, Deny(DenyResumeTerminalIdentity, fmt.Errorf("Orca owner terminal identity changed"))
		}
		return ResumePlan{Disposition: ResumeReuseTerminal, RuntimeID: runtimeID, ReusedTerminalPTYID: terminalID}, nil
	}
	return ResumePlan{Disposition: ResumeCreateTerminal, RuntimeID: runtimeID}, nil
}

func terminalTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed":
		return true
	default:
		return false
	}
}

func recoverableStaleDispatchStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "circuit_broken":
		return true
	case "dispatched":
		// #261의 종료 task는 assignee가 없는 dispatched 장부 행을 남긴다. 이
		// 상태는 다른 absence 증거가 모두 있을 때만 old-runtime residue다.
		return true
	default:
		return false
	}
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
