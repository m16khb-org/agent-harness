package main

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCodexAPIDocReviewUsesSchemaOutputAndMapsVerdicts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", writeAPIDocFakeCodex(t, t.TempDir())+string(os.PathListSeparator)+os.Getenv("PATH"))
	options := apiDocReviewOptions{Repo: root, Model: "test-model", Effort: "low", Timeout: 5 * time.Second}

	t.Setenv("API_DOC_FAKE_RESULT", `{"verdict":"pass","summary":"ok"}`)
	pass, err := runCodexAPIDocReview(options, []string{"api/openapi.yaml"}, "diff", "extra")
	if err != nil || !pass.OK || pass.Model != "test-model" || pass.Effort != "low" || !containsString(pass.Files, "api/openapi.yaml") {
		t.Fatalf("expected passing fake codex review, result=%+v err=%v", pass, err)
	}

	t.Setenv("API_DOC_FAKE_RESULT", `{"verdict":"fail","summary":"missing response","findings":[{"file":"api/openapi.yaml","severity":"error","message":"missing"}]}`)
	fail, err := runCodexAPIDocReview(options, []string{"api/openapi.yaml"}, "diff", "")
	if !errors.Is(err, errAPIDocReviewGateFailed) || fail.OK || fail.Verdict != "fail" || len(fail.Findings) != 1 {
		t.Fatalf("expected failing fake codex review gate, result=%+v err=%v", fail, err)
	}

	t.Setenv("API_DOC_FAKE_EXIT", "7")
	result, err := runCodexAPIDocReview(options, []string{"api/openapi.yaml"}, "diff", "")
	if err == nil || result.OK || !strings.Contains(err.Error(), "codex failed") || !strings.Contains(result.Summary, "codex denied") {
		t.Fatalf("expected fake codex process error, result=%+v err=%v", result, err)
	}
}

func writeAPIDocFakeCodex(t *testing.T, dir string) string {
	t.Helper()
	body := `#!/bin/sh
set -eu
if [ -n "${API_DOC_FAKE_EXIT:-}" ]; then
  echo codex denied >&2
  exit "$API_DOC_FAKE_EXIT"
fi
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --output-last-message) out="$2"; shift 2 ;;
    *) shift ;;
  esac
done
[ -n "$out" ]
printf '%s\n' "$API_DOC_FAKE_RESULT" > "$out"
`
	path := dir + "/codex"
	writeFileForWrapperTest(t, path, body)
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}
