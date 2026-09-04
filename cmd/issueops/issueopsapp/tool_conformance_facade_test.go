package issueopsapp

import "testing"

func TestConformanceModelOverridesRejectMalformedAndDuplicateValues(t *testing.T) {
	models, err := conformanceModelOverrides([]string{"codex=default", "claude=sonnet"})
	if err != nil || models["codex"] != "default" || models["claude"] != "sonnet" {
		t.Fatalf("models=%v err=%v", models, err)
	}
	for _, values := range [][]string{{"missing"}, {"codex="}, {"codex=one", "codex=two"}} {
		if _, err := conformanceModelOverrides(values); err == nil {
			t.Fatalf("invalid overrides accepted: %v", values)
		}
	}
}
