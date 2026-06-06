package apidoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	if err == nil || !IsStaticGateError(err) {
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
