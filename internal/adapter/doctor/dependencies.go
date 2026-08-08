package doctor

import (
	"agent-harness/internal/adapter/lifecycle"
	"agent-harness/internal/adapter/projectdoc"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

type ProjectLifecycleStatePlan = lifecycle.ProjectLifecycleStatePlan

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycle.ValidateProjectLifecycleState(repoRoot)
}

func ProjectDocNames() []string {
	return projectdoc.ProjectDocNames()
}
