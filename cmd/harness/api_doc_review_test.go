package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAPIDocReviewPromptIsFrameworkAgnostic(t *testing.T) {
	prompt := buildAPIDocReviewPrompt([]string{"internal/api/user_handler.go", "openapi.yaml"}, "diff --git ...", "Require tenant headers.")
	for _, want := range []string{"framework-agnostic", "swaggo", "OpenAPI/Swagger specs", "Do not force NestJS decorators onto Go", "@ApiOperation", "Require tenant headers."} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestAPIDocCandidateDetectionCoversGoAndOpenAPIWithoutMarkdown(t *testing.T) {
	cases := map[string]bool{
		"internal/api/user_handler.go":        true,
		"src/users/users.controller.ts":       true,
		"api/openapi.yaml":                    true,
		"docs/swagger.json":                   true,
		"agent_docs/API_GUIDE.md":             false,
		"package.json":                        false,
		"internal/service/payment_service.go": false,
	}
	for file, want := range cases {
		if got := isAPIDocCandidate(file); got != want {
			t.Fatalf("isAPIDocCandidate(%q)=%v want %v", file, got, want)
		}
	}
}

func TestRunAPIDocReviewSkipsWhenNoCandidateFiles(t *testing.T) {
	root := t.TempDir()
	runGitForContract(t, root, "init")
	runGitForContract(t, root, "config", "user.email", "test@example.com")
	runGitForContract(t, root, "config", "user.name", "Tester")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, root, "add", "README.md")
	result, err := runAPIDocReviewWithOptions(apiDocReviewOptions{Repo: root, Model: "gpt-5.5", Effort: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Skipped || result.Reason != "no_api_doc_candidate_files" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAPIDocReviewSchemaRequiresNullableLine(t *testing.T) {
	schema := apiDocReviewSchema()
	properties := schema["properties"].(map[string]any)
	findings := properties["findings"].(map[string]any)
	items := findings["items"].(map[string]any)
	required := items["required"].([]string)
	if !containsString(required, "line") {
		t.Fatalf("line must be required for strict Codex response schema: %+v", required)
	}
}
