package issueopsapp

import (
	"issueops/cmd/issueops/riskqa"
	"issueops/cmd/issueops/selfworkflow"
)

type RiskQATierPlan = riskqa.RiskQATierPlan

type riskQATierDeps struct {
	plan func(string) RiskQATierPlan
	run  func(root string, command string) StepResult
}

func validateRiskQATier(root string) StepResult {
	return riskqa.Validate(root)
}

func validateRiskQATierEvidence(root string) selfworkflow.SelfVerifyRiskQAEvidence {
	step, coversFullGoTest := riskqa.ValidateForSelfVerify(root)
	return selfworkflow.SelfVerifyRiskQAEvidence{
		Step:             step,
		CoversFullGoTest: coversFullGoTest,
	}
}

func validateRiskQATierWithDeps(root string, deps riskQATierDeps) StepResult {
	return riskqa.ValidateWithDeps(root, riskqa.Deps{Plan: deps.plan, Run: deps.run})
}

func planRiskQATier(root string) RiskQATierPlan {
	return riskqa.Plan(root)
}

func planRiskQATierFromPaths(paths []string) RiskQATierPlan {
	return riskqa.PlanFromPaths(paths)
}

func parseGitStatusPath(line string) string {
	return riskqa.ParseGitStatusPath(line)
}

func riskQATierPlanJSON(plan RiskQATierPlan) string {
	return riskqa.PlanJSON(plan)
}
