package summary

import (
	"agent-harness/cmd/harness/commandstep"
	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/adapter/failurecause"
	"testing"
)

func TestSummarizeSelfVerificationAddsOrthogonalFailureCause(t *testing.T) {
	ok := SummarizeSelfVerification(model.SelfAugmentResult{OK: true}, 95)
	if ok.FailureCause != failurecause.None || ok.FailureCauseEvidence == nil {
		t.Fatalf("success cause = %#v", ok)
	}
	failed := SummarizeSelfVerification(model.SelfAugmentResult{Runs: []model.SelfAugmentIteration{{Steps: []commandstep.StepResult{{Label: "probe", FailureEvidence: []failurecause.Evidence{{Cause: failurecause.Transport, Code: "framing", Source: "mcp"}}}}}}}, 95)
	if failed.FailureCause != failurecause.Transport {
		t.Fatalf("cause = %s", failed.FailureCause)
	}
}
