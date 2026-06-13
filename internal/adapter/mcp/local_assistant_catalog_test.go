package mcp

import "testing"

func TestLocalAssistantToolsExposeStableDescriptors(t *testing.T) {
	assertToolDescriptors(t, "local assistant", LocalAssistantTools(), []toolDescriptorExpectation{
		{
			name:                "commit_suggest",
			descriptionContains: "Conventional + Lore Hybrid",
			properties:          []string{"agy_model"},
		},
		{
			name:       "lint_diagnose",
			required:   []string{"command_argv"},
			properties: []string{"command_argv"},
		},
	})
}
