package core

import (
	"agent-harness/internal/adapter/projectbootstrap"
	"agent-harness/internal/adapter/projectdoc"
	"agent-harness/internal/adapter/projectdocs"
)

const ProjectDocsDir = projectdoc.ProjectDocsDir
const agentsStartMarker = projectdoc.AgentsStartMarker
const agentsEndMarker = projectdoc.AgentsEndMarker
const behavioralGuidelines = projectdoc.BehavioralGuidelines
const solidDesignPatternGuidance = projectdoc.SolidDesignPatternGuidance

type ProjectDocCatalogEntry = projectdoc.ProjectDocCatalogEntry
type ProjectDocsPlannedFile = projectdoc.ProjectDocsPlannedFile
type ProjectSignals = projectdocs.ProjectSignals
type EvidenceCommand = projectdocs.EvidenceCommand
type ProjectProfile = projectdocs.ProjectProfile
type ProjectVCSProfile = projectdocs.ProjectVCSProfile
type ProjectDocsRouteResult = projectdocs.ProjectDocsRouteResult
type ProjectDocRouteEntry = projectdocs.ProjectDocRouteEntry
type ProjectDocsRecordRequest = projectdocs.ProjectDocsRecordRequest
type ProjectDocsRecordResult = projectdocs.ProjectDocsRecordResult
type ProjectDocsReadResult = projectdocs.ProjectDocsReadResult
type ProjectDocsUpdateRequest = projectdocs.ProjectDocsUpdateRequest
type ProjectDocsUpdateResult = projectdocs.ProjectDocsUpdateResult
type ProjectDocsBootstrapRequest = projectbootstrap.ProjectDocsBootstrapRequest
type ProjectDocsBootstrapResult = projectbootstrap.ProjectDocsBootstrapResult

func DocMetaDescription(name string) (string, bool) {
	return projectdoc.DocMetaDescription(name)
}

func DiscoverProjectDocs(repoRoot string) []ProjectDocCatalogEntry {
	return projectdoc.DiscoverProjectDocs(repoRoot)
}

func FormatProjectDocCatalog(entries []ProjectDocCatalogEntry) string {
	return projectdoc.FormatProjectDocCatalog(entries)
}

func AnalyzeProjectSignals(root string) ProjectSignals {
	return projectdocs.AnalyzeProjectSignals(root)
}

func RouteProjectDocs(repoRoot, task string) (ProjectDocsRouteResult, error) {
	return projectdocs.RouteProjectDocs(repoRoot, task)
}

func ReadProjectDoc(repoRoot, relPath string) (ProjectDocsReadResult, error) {
	return projectdocs.ReadProjectDoc(repoRoot, relPath)
}

func UpdateProjectDoc(req ProjectDocsUpdateRequest) (ProjectDocsUpdateResult, error) {
	return projectdocs.UpdateProjectDoc(req)
}

func AppendProjectDocsRecord(req ProjectDocsRecordRequest) (ProjectDocsRecordResult, error) {
	return projectdocs.AppendProjectDocsRecord(req)
}

func BootstrapProjectDocs(req ProjectDocsBootstrapRequest) (ProjectDocsBootstrapResult, error) {
	return projectbootstrap.BootstrapProjectDocs(req)
}

func renderProjectDocs(root string, signals ProjectSignals) map[string]string {
	return projectdocs.RenderProjectDocs(root, signals)
}

func renderAgentsWithBlock(root, existing string) string {
	return projectdocs.RenderAgentsWithBlock(root, existing)
}

func parseDocFrontmatter(content string) (name, description, body string, ok bool) {
	return projectdoc.ParseFrontmatter(content)
}

func ensureDocMetaFrontmatter(name, content string) string {
	return projectdoc.EnsureMetaFrontmatter(name, content)
}

func ProjectDocNames() []string {
	return projectdoc.ProjectDocNames()
}

func prefixedProjectDocNames() []string {
	return projectdoc.PrefixedProjectDocNames()
}

func normalizeProjectDocRelPath(relPath string) (string, error) {
	return projectdoc.NormalizeRelPath(relPath)
}

func nonEmptyStrings(items []string) []string {
	return projectdoc.NonEmptyStrings(items)
}

func appendUnique(items []string, v string) []string {
	return projectdoc.AppendUnique(items, v)
}

func plannedFileAction(path, content string) string {
	return projectdoc.PlannedFileAction(path, content)
}

func sha256Hex(content string) string {
	return projectdoc.SHA256Hex(content)
}
