package doctor

import (
	"agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/outbound/state"
	"agent-harness/internal/adapter/projectdoc"
	statecontract "agent-harness/internal/contract/state"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

type ProjectLifecycleStatePlan = lifecycle.ProjectLifecycleStatePlan

func StateDir() string {
	return state.StateDir()
}

func StateDoctor() (statecontract.StateDoctorResult, error) {
	return state.StateDoctor()
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycle.ValidateProjectLifecycleState(repoRoot)
}

func ProjectDocNames() []string {
	return projectdoc.ProjectDocNames()
}
