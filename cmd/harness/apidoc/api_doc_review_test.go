package apidoc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAPIDocReviewPromptIsFrameworkAgnostic(t *testing.T) {
	prompt := buildAPIDocReviewPrompt([]string{"internal/api/user_handler.go", "openapi.yaml"}, "diff --git ...", "Require tenant headers.", "")
	for _, want := range []string{"## Identity", "## Objective", "## Operating Phases", "## Output Contract", "framework-agnostic", "swaggo", "OpenAPI/Swagger specs", "Do not force NestJS decorators onto Go", "@ApiOperation", "business logic", "404", "409", "Require tenant headers."} {
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
		".agent-harness/API_GUIDE.md":         false,
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
	result, err := runAPIDocReviewWithOptions(apiDocReviewOptions{Repo: root})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Skipped || result.Reason != "no_api_doc_candidate_files" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAPIDocReviewRendersPromptWithoutSpawning(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent-harness", "OPEN_API_SPEC.md"), []byte("repo-specific api contract rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diffFile := filepath.Join(root, "api.diff")
	if err := os.WriteFile(diffFile, []byte("diff --git a/api/openapi.yaml b/api/openapi.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")

	result, err := runAPIDocReviewWithOptions(apiDocReviewOptions{Repo: root, Files: []string{"api/openapi.yaml"}, DiffFile: diffFile})
	if !errors.Is(err, ErrReviewResultRequired) {
		t.Fatalf("expected host-agent result requirement, result=%+v err=%v", result, err)
	}
	if result.OK || result.Verdict != "pending" || result.Reason != "host_agent_result_required" {
		t.Fatalf("unexpected pending review result: %+v", result)
	}
	if !strings.Contains(result.Prompt, "repo-specific api contract rules") || !strings.Contains(result.Prompt, "api/openapi.yaml") || !strings.Contains(result.Prompt, "diff --git") {
		t.Fatalf("rendered prompt missing review inputs:\n%s", result.Prompt)
	}
	if result.Schema == nil || result.Schema["type"] != "object" {
		t.Fatalf("rendered schema missing: %+v", result.Schema)
	}
}

func TestAPIDocReviewRecordsSuppliedResult(t *testing.T) {
	root := t.TempDir()
	diffFile := filepath.Join(root, "api.diff")
	if err := os.WriteFile(diffFile, []byte("diff --git a/api/openapi.yaml b/api/openapi.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultFile := filepath.Join(root, "review.json")
	if err := os.WriteFile(resultFile, []byte(`{"verdict":"pass","summary":"documented","findings":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "")

	result, err := runAPIDocReviewWithOptions(apiDocReviewOptions{Repo: root, Files: []string{"api/openapi.yaml"}, DiffFile: diffFile, ResultFile: resultFile})
	if err != nil {
		t.Fatalf("expected supplied result to pass, result=%+v err=%v", result, err)
	}
	if !result.OK || result.Verdict != "pass" || result.ResultFile != resultFile || !containsString(result.Files, "api/openapi.yaml") {
		t.Fatalf("supplied result not recorded: %+v", result)
	}
}

func TestAPIDocAllModeUsesTrackedCandidateContent(t *testing.T) {
	root := t.TempDir()
	runGitForContract(t, root, "init")
	runGitForContract(t, root, "config", "user.email", "test@example.com")
	runGitForContract(t, root, "config", "user.name", "Tester")
	if err := os.MkdirAll(filepath.Join(root, "src", "users"), 0o755); err != nil {
		t.Fatal(err)
	}
	controller := filepath.Join(root, "src", "users", "users.controller.ts")
	if err := os.WriteFile(controller, []byte("export class UsersController {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForContract(t, root, "add", ".")
	runGitForContract(t, root, "commit", "-m", "test")

	files := trackedAPIDocFiles(root)
	if len(files) != 1 || files[0] != "src/users/users.controller.ts" {
		t.Fatalf("unexpected tracked API files: %+v", files)
	}
	content, err := apiDocInput(root, files, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "--- FILE: src/users/users.controller.ts ---") || !strings.Contains(content, "UsersController") {
		t.Fatalf("all mode content missing controller: %s", content)
	}
}

func TestAPIDocStaticCheckFindsNestOmissions(t *testing.T) {
	root := t.TempDir()
	runGitForContract(t, root, "init")
	if err := os.MkdirAll(filepath.Join(root, "src", "users", "dto"), 0o755); err != nil {
		t.Fatal(err)
	}
	controller := `import { Body, Controller, Get, Param, Query } from '@nestjs/common'
import { ApiBearerAuth, ApiOperation, ApiResponse } from '@nestjs/swagger'

@ApiBearerAuth('JWT-auth')
@Controller('users')
export class UsersController {
  @Get(':id')
  @ApiOperation({ summary: '사용자 조회', description: '사용자 조회' })
  @ApiResponse({ status: 200, description: 'ok' })
  find(@Param('id') id: string, @Query('expand') expand?: string) {
    return { id, expand }
  }
}
`
	dto := `export class CreateUserDto {
  name: string
  nickname?: string
}
`
	if err := os.WriteFile(filepath.Join(root, "src", "users", "users.controller.ts"), []byte(controller), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "users", "dto", "create-user.dto.ts"), []byte(dto), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := runAPIDocStaticCheckWithOptions(apiDocStaticOptions{Repo: root, Files: []string{"src/users/users.controller.ts", "src/users/dto/create-user.dto.ts"}})
	if err == nil {
		t.Fatal("expected static check to fail")
	}
	codes := map[string]bool{}
	for _, v := range result.Violations {
		codes[v.Code] = true
	}
	for _, want := range []string{"invalid_api_operation_description_format", "missing_api_param", "missing_api_query", "missing_400_response", "missing_401_response", "missing_api_property", "missing_api_property_optional", "missing_is_optional"} {
		if !codes[want] {
			t.Fatalf("missing violation %s in %+v", want, result.Violations)
		}
	}
}

func TestAPIDocStaticCheckSkipsSwaggerDecoratorsInContractTestsMode(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src", "users"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `---
api_doc_mode: contract-tests
---
`
	if err := os.WriteFile(filepath.Join(root, ".agent-harness", "OPEN_API_SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	controller := `import { Controller, Get, Query } from '@nestjs/common'

@Controller('users')
export class UsersController {
  @Get()
  find(@Query('name') name?: string) {
    return { name }
  }
}
`
	path := filepath.Join(root, "src", "users", "users.controller.ts")
	if err := os.WriteFile(path, []byte(controller), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := runAPIDocStaticCheckWithOptions(apiDocStaticOptions{
		Repo:  root,
		Files: []string{"src/users/users.controller.ts"},
	})
	if err != nil {
		t.Fatalf("contract-tests mode should skip Swagger decorator checks: %v", err)
	}
	if !result.OK || !result.Skipped || result.Reason != "contract_tests_mode" {
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

func TestAPIDocReviewExtraPromptUsesExplicitPromptFileFirst(t *testing.T) {
	root := t.TempDir()
	promptFile := filepath.Join(root, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("explicit prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent-harness", "OPEN_API_SPEC.md"), []byte("repo spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := apiDocReviewExtraPrompt(apiDocReviewOptions{Repo: root, PromptFile: promptFile})
	if err != nil {
		t.Fatal(err)
	}

	if got != "explicit prompt\n" {
		t.Fatalf("expected explicit prompt, got %q", got)
	}
}

func TestAPIDocReviewExtraPromptFallsBackToRepoSpec(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agent-harness"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agent-harness", "OPEN_API_SPEC.md"), []byte("repo spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := apiDocReviewExtraPrompt(apiDocReviewOptions{Repo: root})
	if err != nil {
		t.Fatal(err)
	}

	if got != "repo spec\n" {
		t.Fatalf("expected repo spec, got %q", got)
	}
}

func TestAPIDocReviewExtraPromptReturnsEmptyWhenNoPromptExists(t *testing.T) {
	got, err := apiDocReviewExtraPrompt(apiDocReviewOptions{Repo: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	if got != "" {
		t.Fatalf("expected empty prompt, got %q", got)
	}
}
