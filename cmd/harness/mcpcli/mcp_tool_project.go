package mcpcli

import (
	"time"

	"agent-harness/cmd/harness/apidoc"
	"agent-harness/internal/core"
)

func handleProjectMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "harness_inspect":
		return mcpToolPayload(InspectHarness(stringArg(call.Arguments, "repo")))
	case "atomic_commit_preflight":
		return mcpToolPayload(core.GitPreflight(ResolveTarget(stringArg(call.Arguments, "path")), HarnessRoot()))
	case "commit_policy":
		text, err := ReadHarnessFile(".agent-harness", "COMMIT_POLICY.md")
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "Cannot read commit policy", Data: err.Error()})
		}
		return mcpToolDirect(TextResult(text))
	case "skill_manifest":
		return mcpToolPayload(core.ListSkills(HarnessRoot(), skillName))
	case "docs_index":
		return mcpToolPayload(core.DocsIndex(HarnessRoot(), Version))
	case "project_docs_route":
		result, err := core.RouteProjectDocs(ResolveTarget(stringArg(call.Arguments, "repo")), stringArgWithDefault(call.Arguments, "task", "general"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs route failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_bootstrap_plan":
		result, err := core.BootstrapProjectDocs(core.ProjectDocsBootstrapRequest{RepoRoot: ResolveTarget(stringArg(call.Arguments, "repo")), Write: false})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs bootstrap plan failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_read":
		result, err := core.ReadProjectDoc(ResolveTarget(stringArg(call.Arguments, "repo")), stringArg(call.Arguments, "rel_path"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs read failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_update":
		result, err := core.UpdateProjectDoc(core.ProjectDocsUpdateRequest{
			RepoRoot:       ResolveTarget(stringArg(call.Arguments, "repo")),
			RelPath:        stringArg(call.Arguments, "rel_path"),
			Content:        stringArg(call.Arguments, "content"),
			ExpectedSHA256: stringArg(call.Arguments, "expected_sha256"),
			Summary:        stringArg(call.Arguments, "summary"),
			Evidence:       stringSliceArg(call.Arguments, "evidence"),
			Confirm:        boolArg(call.Arguments, "confirm"),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs update failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_record":
		result, err := core.AppendProjectDocsRecord(core.ProjectDocsRecordRequest{
			RepoRoot:     ResolveTarget(stringArg(call.Arguments, "repo")),
			Kind:         stringArg(call.Arguments, "kind"),
			Title:        stringArg(call.Arguments, "title"),
			Summary:      stringArg(call.Arguments, "summary"),
			Context:      stringArg(call.Arguments, "context"),
			Resolution:   stringArg(call.Arguments, "resolution"),
			Decision:     stringArg(call.Arguments, "decision"),
			Evidence:     stringSliceArg(call.Arguments, "evidence"),
			Alternatives: stringSliceArg(call.Arguments, "alternatives"),
			Consequences: stringArg(call.Arguments, "consequences"),
			Source:       stringArgWithDefault(call.Arguments, "source", "mcp"),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs record failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "api_doc_review":
		timeout, err := time.ParseDuration(stringArgWithDefault(call.Arguments, "timeout", apidoc.DefaultReviewTimeout.String()))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "API doc review failed", Data: "invalid timeout: " + err.Error()})
		}
		result, err := apidoc.RunReviewWithOptions(apidoc.ReviewOptions{
			Repo:       ResolveTarget(stringArg(call.Arguments, "repo")),
			Model:      stringArgWithDefault(call.Arguments, "model", apidoc.DefaultReviewModel),
			Effort:     stringArgWithDefault(call.Arguments, "reasoning", apidoc.DefaultReviewReasoning),
			Timeout:    timeout,
			Files:      stringSliceArg(call.Arguments, "files"),
			All:        boolArg(call.Arguments, "all"),
			DiffFile:   stringArg(call.Arguments, "diff_file"),
			PromptFile: stringArg(call.Arguments, "prompt_file"),
			JSON:       true,
		})
		if err != nil && !apidoc.IsReviewGateError(err) {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "API doc review failed", Data: result})
		}
		return mcpToolPayload(result)
	case "api_doc_static_check":
		result, err := apidoc.RunStaticCheckWithOptions(apidoc.StaticOptions{
			Repo:  ResolveTarget(stringArg(call.Arguments, "repo")),
			Files: stringSliceArg(call.Arguments, "files"),
			All:   boolArg(call.Arguments, "all"),
			JSON:  true,
		})
		if err != nil && !apidoc.IsStaticGateError(err) {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "API doc static check failed", Data: result})
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
