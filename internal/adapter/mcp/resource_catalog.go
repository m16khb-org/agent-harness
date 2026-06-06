package mcp

func Resources() []Resource {
	return []Resource{
		{URI: "harness://commit-policy", Name: "Commit policy", Description: "Conventional Commit + Lore body policy.", MimeType: "text/markdown"},
		{URI: "harness://skill/atomic-commit-push", Name: "atomic-commit-push skill", Description: "Shared native skill instructions.", MimeType: "text/markdown"},
		{URI: "harness://agents", Name: "Agent root rules", Description: "AGENTS.md root operating contract.", MimeType: "text/markdown"},
		{URI: "harness://docs", Name: "Agent docs index", Description: "JSON index of harness agent-facing markdown docs.", MimeType: "application/json"},
		{URI: "harness://project-docs", Name: "Project docs route", Description: "JSON default routing for AGENTS.md and .agent-harness project docs in the current workspace.", MimeType: "application/json"},
		{URI: "harness://project-doc-upkeep", Name: "Project doc upkeep guidance", Description: "How agents should keep .agent-harness docs current through MCP route/read/update/record while preserving user consensus.", MimeType: "text/markdown"},
		{URI: "harness://api-doc-guidance", Name: "API documentation guidance", Description: "Framework-agnostic Swagger/OpenAPI review guidance, including business-logic error response coverage.", MimeType: "text/markdown"},
		{URI: "harness://command-policy", Name: "Command policy summary", Description: "JSON summary of command policy boundaries and fake runner behavior.", MimeType: "application/json"},
		{URI: "harness://state", Name: "State checkpoint index", Description: "JSON index of harness state checkpoints.", MimeType: "application/json"},
	}
}
