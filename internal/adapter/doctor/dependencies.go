package doctor

import (
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	projectdoc "agent-harness/internal/domain/projectdoc"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir

type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan

// lifecycle state 검증은 사용자 상태를 읽는 I/O다. doctor는 그 구현을 모르고
// composition root가 주입한 함수만 호출한다.
var validateProjectLifecycleState = func(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return ProjectLifecycleStatePlan{}, nil
}

// ConfigureLifecycle는 composition root가 실제 구현을 꽂는 진입점이다.
func ConfigureLifecycle(validate func(string) (ProjectLifecycleStatePlan, error)) {
	if validate != nil {
		validateProjectLifecycleState = validate
	}
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return validateProjectLifecycleState(repoRoot)
}

func ProjectDocNames() []string {
	return projectdoc.ProjectDocNames()
}
