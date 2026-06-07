package mcpcli

import (
	"time"

	"agent-harness/cmd/harness/apidoc"
	"agent-harness/cmd/harness/mcpcli/argmap"
	"agent-harness/internal/core"
)

func handleProjectMCPToolCall(call MCPToolCall) MCPToolOutcome {
	switch call.Name {
	case "harness_inspect":
		return mcpToolPayload(InspectHarness(argmap.String(call.Arguments, "repo")))
	case "atomic_commit_preflight":
		return mcpToolPayload(core.GitPreflight(ResolveTarget(argmap.String(call.Arguments, "path")), HarnessRoot()))
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
		result, err := core.RouteProjectDocs(ResolveTarget(argmap.String(call.Arguments, "repo")), argmap.StringDefault(call.Arguments, "task", "general"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs route failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_bootstrap_plan":
		result, err := core.BootstrapProjectDocs(core.ProjectDocsBootstrapRequest{RepoRoot: ResolveTarget(argmap.String(call.Arguments, "repo")), Write: false})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs bootstrap plan failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_read":
		result, err := core.ReadProjectDoc(ResolveTarget(argmap.String(call.Arguments, "repo")), argmap.String(call.Arguments, "rel_path"))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs read failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_update":
		result, err := core.UpdateProjectDoc(core.ProjectDocsUpdateRequest{
			RepoRoot:       ResolveTarget(argmap.String(call.Arguments, "repo")),
			RelPath:        argmap.String(call.Arguments, "rel_path"),
			Content:        argmap.String(call.Arguments, "content"),
			ExpectedSHA256: argmap.String(call.Arguments, "expected_sha256"),
			Summary:        argmap.String(call.Arguments, "summary"),
			Evidence:       argmap.StringSlice(call.Arguments, "evidence"),
			Confirm:        argmap.Bool(call.Arguments, "confirm"),
		})
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs update failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "project_docs_record":
		result, err := core.AppendProjectDocsRecord(core.ProjectDocsRecordRequest{
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
			return mcpToolFailure(&RPCError{Code: -32602, Message: "Project docs record failed", Data: err.Error()})
		}
		return mcpToolPayload(result)
	case "api_doc_review":
		timeout, err := time.ParseDuration(argmap.StringDefault(call.Arguments, "timeout", apidoc.DefaultReviewTimeout.String()))
		if err != nil {
			return mcpToolFailure(&RPCError{Code: -32602, Message: "API doc review failed", Data: "invalid timeout: " + err.Error()})
		}
		result, err := apidoc.RunReviewWithOptions(apidoc.ReviewOptions{
			Repo:       ResolveTarget(argmap.String(call.Arguments, "repo")),
			Model:      argmap.StringDefault(call.Arguments, "model", apidoc.DefaultReviewModel),
			Effort:     argmap.StringDefault(call.Arguments, "reasoning", apidoc.DefaultReviewReasoning),
			Timeout:    timeout,
			Files:      argmap.StringSlice(call.Arguments, "files"),
			All:        argmap.Bool(call.Arguments, "all"),
			DiffFile:   argmap.String(call.Arguments, "diff_file"),
			PromptFile: argmap.String(call.Arguments, "prompt_file"),
			JSON:       true,
		})
		if err != nil && !apidoc.IsReviewGateError(err) {
			return mcpToolFailure(&RPCError{Code: -32000, Message: "API doc review failed", Data: result})
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
			return mcpToolFailure(&RPCError{Code: -32000, Message: "API doc static check failed", Data: result})
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
