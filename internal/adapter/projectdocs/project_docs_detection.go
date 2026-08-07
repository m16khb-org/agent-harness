package projectdocs

import (
	"agent-harness/internal/adapter/projectdocs/detection"
)

func detectFrameworks(root string, files []string, addEvidence func(string)) []string {
	return detection.Frameworks(root, files, addEvidence)
}

func detectMonorepo(root string, files []string, addEvidence func(string)) bool {
	return detection.Monorepo(root, files, addEvidence)
}

func inferProjectTypes(root string, signals ProjectSignals, frameworks []string, monorepo bool, addEvidence func(string)) []string {
	return detection.ProjectTypes(root, signals.Languages, frameworks, monorepo, addEvidence)
}
