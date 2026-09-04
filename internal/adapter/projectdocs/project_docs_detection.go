package projectdocs

import (
	"issueops/internal/adapter/projectdocs/detection"
	projectdoc "issueops/internal/domain/projectdoc"
)

func detectFrameworks(root string, files []string, addEvidence func(string)) []string {
	return detection.Frameworks(root, files, addEvidence)
}

func detectMonorepo(root string, files []string, addEvidence func(string)) bool {
	return detection.Monorepo(root, files, addEvidence)
}

func inferProjectTypes(root string, signals projectdoc.ProjectSignals, frameworks []string, monorepo bool, addEvidence func(string)) []string {
	return detection.ProjectTypes(root, signals.Languages, frameworks, monorepo, addEvidence)
}
