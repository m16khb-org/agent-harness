package issueopslease

import "fmt"

type ReconcileStageAction string

const (
	ReconcileStageAdopt    ReconcileStageAction = "adopt"
	ReconcileStageInvoke   ReconcileStageAction = "invoke"
	ReconcileStagePreserve ReconcileStageAction = "preserve"
)

type ReconcileStageRequest struct {
	Stage              string
	CandidateCount     int
	AuthoritativeZero  bool
	InvocationState    string
	InvocationAttempts int
}

type ReconcileStagePlan struct {
	Action         ReconcileStageAction
	CandidateIndex int
	Reason         string
}

func PlanReconcileStage(request ReconcileStageRequest) (ReconcileStagePlan, error) {
	if request.CandidateCount < 0 {
		return ReconcileStagePlan{}, fmt.Errorf("candidate count must not be negative")
	}
	if request.CandidateCount > 1 {
		return ReconcileStagePlan{Action: ReconcileStagePreserve, Reason: "multiple-candidates"}, nil
	}
	if request.CandidateCount == 1 {
		return ReconcileStagePlan{Action: ReconcileStageAdopt, CandidateIndex: 0}, nil
	}
	if !request.AuthoritativeZero {
		return ReconcileStagePlan{Action: ReconcileStagePreserve, Reason: "non-authoritative-zero"}, nil
	}
	if request.InvocationState != "not_invoked_proven" && request.Stage != "run_bind" {
		return ReconcileStagePlan{Action: ReconcileStagePreserve, Reason: "unknown-invocation"}, nil
	}
	if request.InvocationAttempts >= 2 {
		return ReconcileStagePlan{Action: ReconcileStagePreserve, Reason: "retry-exhausted"}, nil
	}
	return ReconcileStagePlan{Action: ReconcileStageInvoke}, nil
}
