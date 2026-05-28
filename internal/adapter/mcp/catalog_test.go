package mcp

import "testing"

func TestAdapterOwnedToolsHaveWriteSemantics(t *testing.T) {
	seen := map[string]bool{}
	for _, tool := range AdapterOwnedTools() {
		if tool.Name == "" || tool.Description == "" || tool.InputSchema == nil {
			t.Fatalf("incomplete tool descriptor: %+v", tool)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.Name == "worker_enqueue" && !contains(tool.Description, "never executes shell") {
			t.Fatalf("worker enqueue description must state shell safety: %s", tool.Description)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
