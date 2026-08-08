package issueopslease

import "fmt"

type ReconcileStageAction string

const (
	ReconcileStageAdopt    ReconcileStageAction = "adopt"
	ReconcileStageInvoke   ReconcileStageAction = "invoke"
	ReconcileStagePreserve ReconcileStageAction = "preserve"
	// ReconcileStageClear는 intent를 재시도하지 않고 제거한다.
	//
	// 실행됐는지 모르는 mutation을 **재시도**하면 중복 자원이 생길 수 있다.
	// 그러나 외부 인벤토리가 authoritative zero를 돌려줬다면 그 mutation은
	// 어떤 자원도 남기지 않았고, 남은 intent는 사실이 아니라 기록일 뿐이다.
	// 그 기록을 지우는 것은 재시도가 아니므로 중복 위험이 없다.
	//
	// 이 액션이 없던 동안 그런 intent는 영원히 preserve되어, lifecycle 전체를
	// cleanup abandon으로 버리는 것 외에 회수 경로가 없었다(#280).
	ReconcileStageClear ReconcileStageAction = "clear"
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
		// 여기까지 왔다는 것은 외부 인벤토리가 authoritative zero라는 뜻이다.
		// 실행 여부를 모르는 mutation을 재시도할 수는 없지만, 자원이 없음이
		// 확정됐으므로 intent는 지울 수 있다. 재시도가 아니라 정리다(#280).
		return ReconcileStagePlan{Action: ReconcileStageClear, Reason: "invocation-left-no-resource"}, nil
	}
	if request.InvocationAttempts >= 2 {
		return ReconcileStagePlan{Action: ReconcileStagePreserve, Reason: "retry-exhausted"}, nil
	}
	return ReconcileStagePlan{Action: ReconcileStageInvoke}, nil
}
