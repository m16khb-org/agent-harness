package summary

import (
	"issueops/cmd/issueops/commandstep"
	"issueops/cmd/issueops/selfworkflow/model"
	"issueops/internal/contract/failurecause"
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
