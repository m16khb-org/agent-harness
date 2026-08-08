package doctor

import (
	"agent-harness/internal/adapter/lifecycle"
	projectdoc "agent-harness/internal/domain/projectdoc"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

type ProjectLifecycleStatePlan = lifecycle.ProjectLifecycleStatePlan

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycle.ValidateProjectLifecycleState(repoRoot)
}

func ProjectDocNames() []string {
	return projectdoc.ProjectDocNames()
}
