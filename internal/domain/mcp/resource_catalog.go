package mcp

func Resources() []Resource {
	return []Resource{
		{URI: "issueops://commit-policy", Name: "Commit policy", Description: "Conventional Commit + Lore body policy.", MimeType: "text/markdown"},
		{URI: "issueops://skill/atomic-commit-push", Name: "atomic-commit-push skill", Description: "Shared native skill instructions.", MimeType: "text/markdown"},
		{URI: "issueops://agents", Name: "Agent root rules", Description: "AGENTS.md root operating contract.", MimeType: "text/markdown"},
		{URI: "issueops://docs", Name: "Agent docs index", Description: "JSON index of harness agent-facing markdown docs.", MimeType: "application/json"},
		{URI: "issueops://project-docs", Name: "Project docs route", Description: "JSON default routing for AGENTS.md and .issueops project docs in the current workspace.", MimeType: "application/json"},
		{URI: "issueops://project-doc-upkeep", Name: "Project doc upkeep guidance", Description: "How agents should keep .issueops docs current through MCP route/read/update/record while preserving user consensus.", MimeType: "text/markdown"},
		{URI: "issueops://api-doc-guidance", Name: "API documentation guidance", Description: "Framework-agnostic Swagger/OpenAPI review guidance, including business-logic error response coverage.", MimeType: "text/markdown"},
		{URI: "issueops://command-policy", Name: "Command policy summary", Description: "JSON summary of command policy boundaries and fake runner behavior.", MimeType: "application/json"},
		{URI: "issueops://state", Name: "State checkpoint index", Description: "JSON index of issueops state checkpoints.", MimeType: "application/json"},
	}
}
