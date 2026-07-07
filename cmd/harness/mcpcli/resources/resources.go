package resources

import (
	"encoding/json"

	mcpadapter "agent-harness/internal/adapter/mcp"
	"agent-harness/internal/core"
)

type Config struct {
	HarnessRoot     string
	Version         string
	SkillName       string
	ReadHarnessFile func(parts ...string) (string, error)
}

type ReadError struct {
	Code    int
	Message string
	Data    any
}

func MCPResources() []map[string]any {
	return mcpadapter.ResourceMaps(mcpadapter.Resources())
}

func apiDocGuidanceText() string {
	return `# API Documentation Guidance

Use deterministic ` + "`agent-harness api-doc static-check`" + `/MCP ` + "`api_doc_static_check`" + ` first, then ` + "`agent-harness api-doc review`" + `/MCP ` + "`api_doc_review`" + ` to render the host-agent prompt/schema or record a supplied result file whenever endpoint, controller, handler, DTO, schema, or OpenAPI files change.

Default scope is staged API candidate files. Do not fail unrelated legacy Swagger/OpenAPI debt.
Use ` + "`--all`" + ` or MCP ` + "`all: true`" + ` only for an explicit full tracked-file review.

The static check catches deterministic omissions such as missing operation descriptions, path/query/header/body documentation, 400/401 responses, and DTO required/optional decorator mismatches where the framework convention is known.

The agent reviewer must inspect directly related business logic, not only decorators or comments. If the changed endpoint can return domain errors such as 400 validation, 401 auth, 403 forbidden, 404 not found, 409 conflict, or equivalent framework errors, those responses must be documented in the OpenAPI spec.

Clean Swagger/OpenAPI output should include concise operation summaries, consistent sectioned descriptions, documented path/query/header/body parameters, accurate required/optional schemas, explicit success and error responses, and response descriptions or examples where the target project convention supports them.

For NestJS projects following the nextcandle-api style, prefer Markdown-section operation descriptions such as purpose, request rules/processing, and auth/cautions; keep public/admin documents audience-filtered when the project has that split.
`
}

func projectDocUpkeepText() string {
	return `# Project Doc Upkeep Guidance

After first bootstrap, .agent-harness documents are living project operating docs. Agents should keep them current through MCP instead of relying on static template text.

Use this flow:

1. Call project_docs_route with the current task to choose only relevant docs.
2. Read the selected docs. When a document needs updating, call project_docs_read first and keep the returned sha256.
3. Update one document at a time with project_docs_update, passing expected_sha256, a consensus-preserving summary, concrete evidence, and confirm=true only when the full replacement content preserves stronger existing guidance.
4. Use project_docs_record(kind=caution) for solved false cases, repeated failures, and risk notes.
5. Use project_docs_record(kind=adr) for decisions, rationale, rejected alternatives, and consequences.

Do not invent repo facts. If evidence is missing, mark the section as "Unknown / not confirmed" and explain how to verify. Do not overwrite user decisions or stronger local docs with generated template language.
`
}

func HandleResourceRead(params json.RawMessage, config Config) (any, *ReadError) {
	var req struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &ReadError{Code: -32602, Message: "Invalid params", Data: err.Error()}
	}
	if req.URI == "harness://docs" {
		result := core.DocsIndex(config.HarnessRoot, config.Version)
		b, _ := json.MarshalIndent(result, "", "  ")
		return content(req.URI, "application/json", string(b)), nil
	}
	if req.URI == "harness://project-docs" {
		result, err := core.RouteProjectDocs(".", "general")
		if err != nil {
			return nil, &ReadError{Code: -32000, Message: "Cannot read project docs route", Data: err.Error()}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return content(req.URI, "application/json", string(b)), nil
	}
	if req.URI == "harness://project-doc-upkeep" {
		return content(req.URI, "text/markdown", projectDocUpkeepText()), nil
	}
	if req.URI == "harness://api-doc-guidance" {
		return content(req.URI, "text/markdown", apiDocGuidanceText()), nil
	}
	if req.URI == "harness://command-policy" {
		b, _ := json.MarshalIndent(core.CommandPolicySummary(), "", "  ")
		return content(req.URI, "application/json", string(b)), nil
	}
	if req.URI == "harness://state" {
		result, err := core.StateList()
		if err != nil {
			return nil, &ReadError{Code: -32000, Message: "Cannot read state index", Data: err.Error()}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return content(req.URI, "application/json", string(b)), nil
	}
	rel, ok := harnessFileResource(req.URI, config.SkillName)
	if !ok {
		return nil, &ReadError{Code: -32602, Message: "Unknown resource", Data: req.URI}
	}
	text, err := config.ReadHarnessFile(rel...)
	if err != nil {
		return nil, &ReadError{Code: -32000, Message: "Cannot read resource", Data: err.Error()}
	}
	return content(req.URI, "text/markdown", text), nil
}

func content(uri string, mimeType string, text string) map[string]any {
	return map[string]any{"contents": []map[string]any{{"uri": uri, "mimeType": mimeType, "text": text}}}
}

func harnessFileResource(uri string, skillName string) ([]string, bool) {
	switch uri {
	case "harness://commit-policy":
		return []string{".agent-harness", "COMMIT_POLICY.md"}, true
	case "harness://skill/atomic-commit-push":
		if skillName == "" {
			skillName = "atomic-commit-push"
		}
		return []string{"skills", skillName, "SKILL.md"}, true
	case "harness://agents":
		return []string{"AGENTS.md"}, true
	default:
		return nil, false
	}
}
