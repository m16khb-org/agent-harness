package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAPIDocReviewPromptIsFrameworkAgnostic(t *testing.T) {
	prompt := buildAPIDocReviewPrompt([]string{"internal/api/user_handler.go", "openapi.yaml"}, "diff --git ...", "Require tenant headers.")
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
	result, err := runAPIDocReviewWithOptions(apiDocReviewOptions{Repo: root, Model: "gpt-5.5", Effort: "medium"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !result.Skipped || result.Reason != "no_api_doc_candidate_files" {
		t.Fatalf("unexpected result: %+v", result)
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

func TestRunAPIDocRoutesStaticCheckAndUsageErrors(t *testing.T) {
	root := t.TempDir()
	if err := runAPIDoc(nil); err == nil || !strings.Contains(err.Error(), "missing api-doc subcommand") {
		t.Fatalf("expected missing api-doc subcommand error, got %v", err)
	}
	if err := runAPIDoc([]string{"unknown"}); err == nil || !strings.Contains(err.Error(), `unknown api-doc subcommand "unknown"`) {
		t.Fatalf("expected unknown api-doc subcommand error, got %v", err)
	}

	out := captureStatusVerifyStdout(t, func() error {
		return runAPIDoc([]string{"static-check", "--repo", root, "--json"})
	})

	var result apiDocStaticResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode api-doc static JSON: %v\n%s", err, out)
	}
	if !result.OK || !result.Skipped || result.Reason != "no_api_doc_candidate_files" {
		t.Fatalf("unexpected static-check result: %#v", result)
	}
}

func TestRunAPIDocStaticCheckTextReportsViolations(t *testing.T) {
	root := t.TempDir()
	controller := filepath.Join(root, "src", "users", "users.controller.ts")
	if err := os.MkdirAll(filepath.Dir(controller), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(controller, []byte(`import { Controller, Get, Param } from '@nestjs/common'

@Controller('users')
export class UsersController {
  @Get(':id')
  find(@Param('id') id: string) {
    return { id }
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureTraceGuardPolicyStdout(t, func() error {
		return runAPIDocStaticCheck([]string{"--repo", root, "src/users/users.controller.ts"})
	})

	if err == nil || !isAPIDocStaticGateError(err) {
		t.Fatalf("expected static gate error, got %T %v", err, err)
	}
	if !strings.Contains(out, "API documentation static check found") || !strings.Contains(out, "missing_api_operation") || !strings.Contains(out, "missing_api_param") {
		t.Fatalf("unexpected static-check text output:\n%s", out)
	}
}

func TestRunAPIDocCheckJSONSkipsReviewWhenStaticFails(t *testing.T) {
	root := t.TempDir()
	dto := filepath.Join(root, "src", "users", "dto", "create-user.dto.ts")
	if err := os.MkdirAll(filepath.Dir(dto), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dto, []byte(`export class CreateUserDto {
  name: string
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStatusVerifyStdout(t, func() error {
		err := runAPIDocCheck([]string{"--repo", root, "--json", "src/users/dto/create-user.dto.ts"})
		if err == nil || !strings.Contains(err.Error(), "api documentation static check failed") {
			t.Fatalf("expected static check failure, got %v", err)
		}
		return nil
	})

	var result apiDocCheckResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode api-doc check JSON: %v\n%s", err, out)
	}
	if result.OK || result.Reason != "static_check_failed" || !result.Review.Skipped || result.Review.Reason != "static_check_failed" {
		t.Fatalf("unexpected api-doc check result: %#v", result)
	}
}

func TestRunAPIDocReviewRejectsInvalidTimeout(t *testing.T) {
	if err := runAPIDocReview([]string{"--timeout", "not-a-duration"}); err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("expected invalid duration error, got %v", err)
	}
}
