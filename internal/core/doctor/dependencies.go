package doctor

import (
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/projectdoc"
	"agent-harness/internal/core/state"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

type ProjectLifecycleStatePlan = lifecycle.ProjectLifecycleStatePlan

func StateDir() string {
	return state.StateDir()
}

func StateDoctor() (state.StateDoctorResult, error) {
	return state.StateDoctor()
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycle.ValidateProjectLifecycleState(repoRoot)
}

func ProjectDocNames() []string {
	return projectdoc.ProjectDocNames()
}
