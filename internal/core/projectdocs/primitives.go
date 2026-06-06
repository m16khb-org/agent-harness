package projectdocs

import "agent-harness/internal/core/projectdoc"

const ProjectDocsDir = projectdoc.ProjectDocsDir

const agentsStartMarker = projectdoc.AgentsStartMarker
const agentsEndMarker = projectdoc.AgentsEndMarker
const behavioralGuidelines = projectdoc.BehavioralGuidelines
const solidDesignPatternGuidance = projectdoc.SolidDesignPatternGuidance

func normalizeProjectDocRelPath(relPath string) (string, error) {
	return projectdoc.NormalizeRelPath(relPath)
}

func nonEmptyStrings(values []string) []string {
	return projectdoc.NonEmptyStrings(values)
}

func appendUnique(values []string, value string) []string {
	return projectdoc.AppendUnique(values, value)
}

func plannedFileAction(path, content string) string {
	return projectdoc.PlannedFileAction(path, content)
}

func sha256Hex(content string) string {
	return projectdoc.SHA256Hex(content)
}

func ensureDocMetaFrontmatter(name, content string) string {
	return projectdoc.EnsureMetaFrontmatter(name, content)
}
