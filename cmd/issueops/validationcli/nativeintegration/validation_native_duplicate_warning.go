package nativeintegration

import "strings"

type ClaudeMCPDuplicateWarning struct {
	Server      string   `json:"server"`
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions"`
}

func DetectClaudeMCPDuplicateWarnings(output string) []ClaudeMCPDuplicateWarning {
	warnings := []ClaudeMCPDuplicateWarning{}
	current := -1
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "└"))
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "│"))
		if strings.Contains(trimmed, "[Warning]") && strings.Contains(trimmed, "defined in multiple scopes") {
			server := ""
			if before, after, ok := strings.Cut(trimmed, `Server "`); ok {
				_ = before
				if name, _, ok := strings.Cut(after, `"`); ok {
					server = name
				}
			}
			warnings = append(warnings, ClaudeMCPDuplicateWarning{
				Server:      server,
				Message:     strings.TrimSpace(trimmed),
				Suggestions: []string{},
			})
			current = len(warnings) - 1
			continue
		}
		if current >= 0 && strings.Contains(trimmed, "Suggestion:") {
			_, suggestion, _ := strings.Cut(trimmed, "Suggestion:")
			warnings[current].Suggestions = append(warnings[current].Suggestions, strings.TrimSpace(suggestion))
		}
	}
	return warnings
}

func ClaudeMCPDuplicateWarningFixture() string {
	return `MCP Config Diagnostics

For help configuring MCP servers, see: https://code.claude.com/docs/en/mcp

[Conflicting scopes]
 └ [Warning] Server "issueops" is defined in multiple scopes with different endpoints: user (/Users/example/issueops/bin/issueops mcp), project (./bin/issueops mcp). OAuth tokens are stored per endpoint, so authenticating in one context will not carry over.
   Suggestion: Keep the correct endpoint and remove the others: ` + "`claude mcp remove issueops -s user`" + ` or ` + "`claude mcp remove issueops -s project`" + `
`
}

func detectClaudeMCPDuplicateWarnings(output string) []ClaudeMCPDuplicateWarning {
	return DetectClaudeMCPDuplicateWarnings(output)
}

func claudeMCPDuplicateWarningFixture() string {
	return ClaudeMCPDuplicateWarningFixture()
}
