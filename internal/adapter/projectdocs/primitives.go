package projectdocs

import projectdocdomain "agent-harness/internal/domain/projectdoc"

const ProjectDocsDir = projectdocdomain.ProjectDocsDir

const agentsStartMarker = projectdocdomain.AgentsStartMarker
const agentsEndMarker = projectdocdomain.AgentsEndMarker
const behavioralGuidelines = projectdocdomain.BehavioralGuidelines
const solidDesignPatternGuidance = projectdocdomain.SolidDesignPatternGuidance

func normalizeProjectDocRelPath(relPath string) (string, error) {
	return projectdocdomain.NormalizeRelPath(relPath)
}

func nonEmptyStrings(values []string) []string {
	return projectdocdomain.NonEmptyStrings(values)
}

func appendUnique(values []string, value string) []string {
	return projectdocdomain.AppendUnique(values, value)
}

func plannedFileAction(path, content string) string {
	return PlannedFileAction(path, content)
}

func sha256Hex(content string) string {
	return projectdocdomain.SHA256Hex(content)
}

func ensureDocMetaFrontmatter(name, content string) string {
	return projectdocdomain.EnsureMetaFrontmatter(name, content)
}
