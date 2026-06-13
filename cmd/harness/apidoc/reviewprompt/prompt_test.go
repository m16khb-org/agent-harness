package reviewprompt

import (
	"strings"
	"testing"
)

func TestBuildIncludesFilesDiffExtraPromptAndContract(t *testing.T) {
	prompt := Build([]string{"src/user.controller.ts", "src/user.dto.ts"}, "@Get(':id')", "Use local style")
	for _, want := range []string{
		"strict, framework-agnostic pre-commit reviewer",
		"Use local style",
		"- src/user.controller.ts",
		"- src/user.dto.ts",
		"@Get(':id')",
		"Respond only with JSON",
		"Business-logic public error contracts",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestSchemaAndBulletLines(t *testing.T) {
	schema := Schema()
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Fatalf("unexpected schema root: %#v", schema)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing: %#v", schema)
	}
	if _, ok := props["findings"]; !ok {
		t.Fatalf("findings schema missing: %#v", props)
	}
	if got := bulletLines(nil); got != "- <none>" {
		t.Fatalf("empty bulletLines = %q", got)
	}
	if got := bulletLines([]string{"a", "b"}); got != "- a\n- b" {
		t.Fatalf("bulletLines = %q", got)
	}
}
