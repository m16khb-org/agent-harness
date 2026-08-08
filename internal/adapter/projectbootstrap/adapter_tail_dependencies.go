package projectbootstrap

import (
	projectdoc "agent-harness/internal/domain/projectdoc"
)

// 설치 계획 수립과 프로젝트 문서 관측은 파일시스템에 닿는다. 구현은 composition
// root가 설치한다.
var (
	AnalyzeProjectSignals func(root string) projectdoc.ProjectSignals
	RenderAgentsWithBlock func(root, existing string) string
	RenderProjectDocs     func(root string, signals projectdoc.ProjectSignals) map[string]string
)
