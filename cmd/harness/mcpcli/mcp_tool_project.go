package mcpcli

import (
	"agent-harness/cmd/harness/apidoc"
	"agent-harness/cmd/harness/mcpcli/argmap"
	projectbootstrapcontract "agent-harness/internal/contract/projectbootstrap"
	projectdocscontract "agent-harness/internal/contract/projectdocs"
)

func handleProjectMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "harness_inspect":
		return mcpToolPayload(InspectHarness(argmap.String(call.Arguments, "repo")))
	case "atomic_commit_preflight":
		return mcpToolPayload(GitPreflight(ResolveTarget(argmap.String(call.Arguments, "path")), HarnessRoot()))
	case "commit_policy":
		text, err := ReadHarnessFile(".agent-harness", "COMMIT_POLICY.md")
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "Cannot read commit policy", err.Error()))
		}
		return mcpToolDirect(TextResult(text))
	case "skill_manifest":
		return mcpToolPayload(ListSkills(HarnessRoot(), skillName))
	case "docs_index":
		return mcpToolPayload(DocsIndex(HarnessRoot(), Version))
	case "project_docs_route":
		result, err := RouteProjectDocs(ResolveTarget(argmap.String(call.Arguments, "repo")), argmap.StringDefault(call.Arguments, "task", "general"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs route failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_bootstrap_plan":
		result, err := bootstrapProjectDocs(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: ResolveTarget(argmap.String(call.Arguments, "repo")), Write: false})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs bootstrap plan failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_read":
		result, err := ReadProjectDoc(ResolveTarget(argmap.String(call.Arguments, "repo")), argmap.String(call.Arguments, "rel_path"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs read failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_update":
		result, err := UpdateProjectDoc(projectdocscontract.ProjectDocsUpdateRequest{
			RepoRoot:       ResolveTarget(argmap.String(call.Arguments, "repo")),
			RelPath:        argmap.String(call.Arguments, "rel_path"),
			Content:        argmap.String(call.Arguments, "content"),
			ExpectedSHA256: argmap.String(call.Arguments, "expected_sha256"),
			Summary:        argmap.String(call.Arguments, "summary"),
			Evidence:       argmap.StringSlice(call.Arguments, "evidence"),
			Confirm:        argmap.Bool(call.Arguments, "confirm"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs update failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_record":
		result, err := AppendProjectDocsRecord(projectdocscontract.ProjectDocsRecordRequest{
			RepoRoot:     ResolveTarget(argmap.String(call.Arguments, "repo")),
			Kind:         argmap.String(call.Arguments, "kind"),
			Title:        argmap.String(call.Arguments, "title"),
			Summary:      argmap.String(call.Arguments, "summary"),
			Context:      argmap.String(call.Arguments, "context"),
			Resolution:   argmap.String(call.Arguments, "resolution"),
			Decision:     argmap.String(call.Arguments, "decision"),
			Evidence:     argmap.StringSlice(call.Arguments, "evidence"),
			Alternatives: argmap.StringSlice(call.Arguments, "alternatives"),
			Consequences: argmap.String(call.Arguments, "consequences"),
			Source:       argmap.StringDefault(call.Arguments, "source", "mcp"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs record failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "api_doc_review":
		result, err := apidoc.RunReviewWithOptions(apidoc.ReviewOptions{
			Repo:       ResolveTarget(argmap.String(call.Arguments, "repo")),
			Files:      argmap.StringSlice(call.Arguments, "files"),
			All:        argmap.Bool(call.Arguments, "all"),
			DiffFile:   argmap.String(call.Arguments, "diff_file"),
			PromptFile: argmap.String(call.Arguments, "prompt_file"),
			ResultFile: argmap.String(call.Arguments, "result_file"),
			JSON:       true,
		})
		if err != nil && !apidoc.IsReviewGateError(err) {
			return mcpToolFailure(newProtocolError(-32000, "API doc review failed", result))
		}
		return mcpToolPayload(result)
	case "api_doc_static_check":
		result, err := apidoc.RunStaticCheckWithOptions(apidoc.StaticOptions{
			Repo:  ResolveTarget(argmap.String(call.Arguments, "repo")),
			Files: argmap.StringSlice(call.Arguments, "files"),
			All:   argmap.Bool(call.Arguments, "all"),
			JSON:  true,
		})
		if err != nil && !apidoc.IsStaticGateError(err) {
			return mcpToolFailure(newProtocolError(-32000, "API doc static check failed", result))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
