package installutil

import "testing"

func TestHookGroupContainsAgentHarnessDoesNotMatchThirdPartyHarnessHook(t *testing.T) {
	group := map[string]any{"hooks": []any{map[string]any{"command": "third-party-harness hook observe"}}}
	if HookGroupContainsAgentHarness(group) {
		t.Fatalf("third-party harness hook was classified as managed: %#v", group)
	}
}

func TestHookGroupContainsAgentHarnessRecognizesQuotedPathWithSpaces(t *testing.T) {
	group := map[string]any{"hooks": []any{map[string]any{"command": "'/source with spaces/bin/issueops' hook session-start --host codex"}}}
	if !HookGroupContainsAgentHarness(group) {
		t.Fatalf("quoted issueops command was not classified as managed: %#v", group)
	}
}
