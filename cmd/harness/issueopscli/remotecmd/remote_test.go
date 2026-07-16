package remotecmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-harness/internal/core"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/testsupport"
)

func TestRunScoreWithJudgeNoneAndErrorPaths(t *testing.T) {
	input := filepath.Join(t.TempDir(), "score.json")
	if err := os.WriteFile(input, []byte(`{"provider":"github","threshold":0.5,"issue":{"title":"Fix login bug","body":"login fails"},"issue_candidates":[{"id":"1","title":"Fix login bug","body":"same","score":0.9}],"label_candidates":[{"name":"bug","score":0.8}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var printed []any
	var printedErrors []error
	deps := Deps{
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		PrintError: func(err error) error {
			printedErrors = append(printedErrors, err)
			return nil
		},
	}
	if err := Run([]string{"score", "--input", input, "--judge", "none", "--json"}, deps); err != nil {
		t.Fatalf("score json returned error: %v", err)
	}
	if err := Run([]string{"remote-score", "--input", input, "--judge", "none"}, deps); err != nil {
		t.Fatalf("score text returned error: %v", err)
	}
	if len(printed) != 1 {
		t.Fatalf("expected one JSON result, got %d", len(printed))
	}
	if err := Run([]string{"score", "--input", filepath.Join(t.TempDir(), "missing"), "--judge", "none", "--json"}, deps); err == nil {
		t.Fatal("expected missing input error")
	}
	if len(printedErrors) != 1 {
		t.Fatalf("expected printed error, got %d", len(printedErrors))
	}
	if err := Run([]string{"score", "--input", input, "--judge", "bad"}, deps); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported judge error, got %v", err)
	}
}

func TestRunVerifyArtifactAndRemoteCreateDryRuns(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	var records []core.IssueOpsRecord
	var printed []any
	deps := Deps{
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		PrintResult: func(record core.IssueOpsRecord, jsonOut bool, err error) error {
			if err != nil {
				return err
			}
			records = append(records, record)
			return nil
		},
		VerifyLive: func(req core.IssueOpsRemoteArtifactVerificationRequest) error {
			if req.Provider == "" || req.URL == "" {
				return errors.New("bad request")
			}
			return nil
		},
	}
	if err := Run([]string{"verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/acme/repo/pull/1", "--target-branch", "main", "--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("verify-artifact returned error: %v", err)
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--body", "Body", "--label", "bug", "--json"}, deps); err != nil {
		t.Fatalf("create-issue dry-run returned error: %v", err)
	}
	if err := Run([]string{"create-child", "--id", record.ID, "--title", "Child", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("create-child dry-run returned error: %v", err)
	}
	if err := Run([]string{"create-pr", "--id", record.ID, "--title", "PR", "--body", "Body", "--head", record.Branch, "--base", "main", "--json"}, deps); err != nil {
		t.Fatalf("create-pr dry-run returned error: %v", err)
	}
	if err := Run([]string{"sync-graph", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("sync-graph dry-run returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one verified record, got %d", len(records))
	}
	if records[0].RemoteArtifact == nil || records[0].RemoteArtifact.TargetBranch != "main" {
		t.Fatalf("verify-artifact did not persist target branch: %#v", records[0].RemoteArtifact)
	}
	if len(printed) != 4 {
		t.Fatalf("expected four JSON dry-run outputs, got %d", len(printed))
	}
}

func TestRunRenderTemplateAndCreateIssueBodyFileTemplateValidation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	bodyFile := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(bodyFile, []byte("## 문제\n\n본문\n\n## Plan Link\n\n/path/to/plan.md\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var printed []any
	deps := Deps{
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		PrintError: func(err error) error {
			return nil
		},
	}

	if err := Run([]string{
		"render-template",
		"--kind", "issue",
		"--template", "feature",
		"--provider", "github",
		"--title", "원격 템플릿 계약",
		"--field", "problem=본문 품질이 흔들린다.",
		"--field", "current_evidence=임의 body만 받는다.",
		"--field", "acceptance_criteria=렌더러 결과가 고정된다.",
		"--field", "non_goals=provider 정책 복제 제외",
		"--field", "implementation_scope=core와 CLI",
		"--field", "verification=go test ./...",
		"--field", "risks=golden drift",
		"--field", "feedback_log=없음",
		"--json",
	}, deps); err != nil {
		t.Fatalf("render-template returned error: %v", err)
	}
	if len(printed) != 1 {
		t.Fatalf("expected render-template JSON output, got %d", len(printed))
	}

	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--body", "Body", "--body-file", bodyFile}, deps); err == nil || !strings.Contains(err.Error(), "body and body-file are mutually exclusive") {
		t.Fatalf("expected mutual exclusion error, got %v", err)
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--template", "feature", "--body-file", bodyFile, "--label", "bug", "--assignee", "octocat", "--confirm"}, deps); err == nil || !strings.Contains(err.Error(), "plan_link_section_forbidden") {
		t.Fatalf("expected template validation error for forbidden Plan Link, got %v", err)
	}
}

func TestRunRemoteCreatePRConfirmRequiresLabelAndAssignee(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	deps := Deps{PrintError: func(err error) error { return nil }}
	tests := [][]string{
		{"create-pr", "--id", record.ID, "--title", "PR", "--head", record.Branch, "--base", "main", "--assignee", "octocat", "--confirm"},
		{"create-pr", "--id", record.ID, "--title", "PR", "--head", record.Branch, "--base", "main", "--label", "bug", "--confirm"},
	}
	for _, args := range tests {
		if err := Run(args, deps); err == nil {
			t.Fatalf("expected confirm validation error for args %v", args)
		}
	}
}

func TestRunRemoteCreatePRPreservesLegacyBodyFileAndRejectsSupervisedBodyFile(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	bodyFile := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(bodyFile, []byte("untrusted"), 0o600); err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	var capturedBody string
	deps := Deps{
		PrintError: func(error) error { return nil },
		CreatePullRequest: func(_ string, req core.IssueProviderCreatePullRequestRequest) (core.IssueProviderCreatePullRequestResult, error) {
			providerCalls++
			capturedBody = req.Body
			return core.IssueProviderCreatePullRequestResult{}, nil
		},
	}
	err := Run([]string{"create-pr", "--id", record.ID, "--title", "PR", "--head", record.Branch, "--base", "main", "--body-file", bodyFile}, deps)
	if err != nil || providerCalls != 1 || capturedBody != "untrusted" {
		t.Fatalf("legacy body-file behavior changed: calls=%d body=%q err=%v", providerCalls, capturedBody, err)
	}
	supervised := record
	supervised.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{}
	if supervised.ExecutionHandoff == nil || strings.TrimSpace(bodyFile) == "" {
		t.Fatal("invalid supervised body-file test setup")
	}
	if err := rejectSupervisedPullRequestBodyFile(supervised, bodyFile); err == nil || !strings.Contains(err.Error(), "body-file is forbidden") {
		t.Fatalf("supervised body-file rejection = %v", err)
	}
}

func TestRunRemoteCreatePRLegacyAtMeReachesProviderWithoutStateMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	db, err := sqlstore.Open(core.IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	before, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok {
		t.Fatalf("read legacy bytes: ok=%v err=%v", ok, err)
	}
	var captured core.IssueProviderCreatePullRequestRequest
	deps := Deps{
		PrintJSON: func(value any) error {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(value)
		},
		PrintError: func(error) error { return nil },
		CreatePullRequest: func(_ string, req core.IssueProviderCreatePullRequestRequest) (core.IssueProviderCreatePullRequestResult, error) {
			captured = req
			return core.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/16"}, nil
		},
	}
	baseArgs := []string{"create-pr", "--id", record.ID, "--title", "PR", "--body", " legacy body ", "--head", record.Branch, "--base", "main", "--label", " bug ", "--assignee", "@me", "--confirm"}
	for _, golden := range []struct {
		name string
		args []string
	}{
		{name: "testdata/legacy_create_pr_text.golden.txt", args: baseArgs},
		{name: "testdata/legacy_create_pr_json.golden.json", args: append(append([]string(nil), baseArgs...), "--json")},
	} {
		got := testsupport.CaptureStdout(t, func() error { return Run(golden.args, deps) })
		want, readErr := os.ReadFile(golden.name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal([]byte(got), want) {
			t.Fatalf("legacy CLI output changed for %s\nwant=%q\n got=%q", golden.name, want, got)
		}
	}
	if captured.Assignees[0] != "@me" || captured.Draft || captured.Body != "legacy body" {
		t.Fatalf("legacy provider request changed: %#v", captured)
	}
	after, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok || !reflect.DeepEqual(after, before) {
		t.Fatalf("legacy create mutated IssueOps bytes: ok=%v err=%v", ok, err)
	}
}

func TestRunRemoteCreatePRDryRunRedactsProviderArgvForGitHubAndGitLab(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	secret := "api_key=opaque-token password=opaque-password Authorization: Bearer opaque-bearer /tmp/secret.pem"
	for _, providerName := range []string{"github", "gitlab"} {
		t.Run(providerName, func(t *testing.T) {
			var printed core.IssueProviderCreatePullRequestResult
			deps := Deps{PrintJSON: func(value any) error {
				printed = value.(core.IssueProviderCreatePullRequestResult)
				return nil
			}}
			err := Run([]string{"create-pr", "--id", record.ID, "--provider", providerName, "--title", "PR", "--body", secret, "--head", record.Branch, "--base", "main", "--json"}, deps)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(printed.Preview, "opaque-") || !strings.Contains(printed.Preview, "<redacted>") || len(printed.Preview) > 4200 {
				t.Fatalf("%s CLI dry-run preview was not bounded and redacted: %q", providerName, printed.Preview)
			}
		})
	}
}

func TestPullRequestDraftIsSupervisedOnly(t *testing.T) {
	legacy := core.IssueOpsRecord{}
	if pullRequestDraft(legacy) {
		t.Fatal("legacy nil-handoff PR/MR became draft")
	}
	supervised := legacy
	supervised.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{}
	if !pullRequestDraft(supervised) {
		t.Fatal("supervised PR/MR was not forced to draft")
	}
}

func TestRunRemoteCreateChildConfirmRecordsChildLink(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	binDir := t.TempDir()
	writeFakeGhForCreateChild(t, binDir)
	t.Setenv("PATH", binDir)
	var printed []any
	deps := Deps{
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		PrintError: func(err error) error {
			return nil
		},
	}

	if err := Run([]string{"create-child", "--id", record.ID, "--title", "Child", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--confirm", "--json"}, deps); err != nil {
		t.Fatalf("create-child confirm returned error: %v", err)
	}
	updated, err := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.IssueLinks) != 1 || updated.IssueLinks[0].Type != "child" || updated.IssueLinks[0].URL != "https://github.com/acme/repo/issues/34" {
		t.Fatalf("child link not recorded: %+v", updated.IssueLinks)
	}
	if len(printed) != 1 {
		t.Fatalf("expected one JSON result, got %d", len(printed))
	}
}

func TestRunRemoteCreateChildRequiresParentLabelsAndAssignees(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	tests := [][]string{
		{"create-child", "--id", record.ID, "--title", "Child", "--assignee", "octocat"},
		{"create-child", "--id", record.ID, "--title", "Child", "--label", "bug"},
	}
	for _, args := range tests {
		if err := Run(args, Deps{}); err == nil {
			t.Fatalf("expected validation error for args %v", args)
		}
	}
}

func TestRemoteHelpersAndBoundaries(t *testing.T) {
	var deps Deps
	if err := deps.printJSON(map[string]any{"ok": true}); err != nil {
		t.Fatalf("default printJSON: %v", err)
	}
	if err := deps.printError(errors.New("x")); err == nil {
		t.Fatal("default printError should return input error")
	}
	if err := deps.printResult(core.IssueOpsRecord{}, false, errors.New("x")); err == nil {
		t.Fatal("default printResult should return input error")
	}
	if err := deps.verifyLive(core.IssueOpsRemoteArtifactVerificationRequest{}); err != nil {
		t.Fatalf("default verifyLive should allow: %v", err)
	}
	var flags repeatedFlag
	if flags.String() != "" {
		t.Fatal("empty repeated flag string should be empty")
	}
	_ = flags.Set("a")
	_ = flags.Set("b")
	if flags.String() != "a,b" {
		t.Fatalf("repeated flag string = %q", flags.String())
	}
	item := core.IssueOpsRemoteScoredItem{ID: "1", URL: "url", Title: "Title", Score: 0.9}
	if formatIssueOpsRemoteIssueRef(item) != "1 (Title)" || formatIssueOpsRemoteIssueRef(core.IssueOpsRemoteScoredItem{Title: "Title"}) != "Title" {
		t.Fatal("unexpected issue ref formatting")
	}
	if firstNonEmptyMain("", " a ") != "a" {
		t.Fatal("firstNonEmptyMain should trim")
	}
	if _, err := readIssueOpsRemoteScoringRequestFile(""); err == nil {
		t.Fatal("empty input should fail")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readIssueOpsRemoteScoringRequestFile(bad); err == nil {
		t.Fatal("bad scoring JSON should fail")
	}
	if resolveRecordProvider(core.IssueOpsRecord{BranchPrepare: &core.IssueOpsBranchPrepare{Provider: "gitlab"}}) != "gitlab" {
		t.Fatal("branch prepare provider should win")
	}
	if resolveRecordProvider(core.IssueOpsRecord{RemoteArtifact: &core.IssueOpsRemoteArtifactVerification{Provider: "github"}}) != "github" {
		t.Fatal("remote artifact provider should be used")
	}
	if resolveRecordProvider(core.IssueOpsRecord{IssueURL: "https://gitlab.com/acme/repo/-/issues/1"}) != "gitlab" {
		t.Fatal("gitlab issue URL should infer provider")
	}
	if err := Run(nil, deps); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if err := Run([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops remote") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func remoteIssueOpsRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	record := remoteIssueOpsRecordWithoutChild(t)
	var err error
	record, err = core.LinkIssueOpsChild(core.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1235", "child")
	if err != nil {
		t.Fatalf("LinkIssueOpsChild: %v", err)
	}
	record.Phase = core.IssueOpsPhasePR
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatalf("WriteIssueOps: %v", err)
	}
	return record
}

func remoteIssueOpsRecordWithoutChild(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	repo := t.TempDir()
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: "1234-remote-cmd"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	record, err = core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1234")
	if err != nil {
		t.Fatalf("LinkIssueOpsIssue: %v", err)
	}
	record, err = core.PrepareIssueOpsBranch(core.IssueOpsStateRoot(), record.ID, core.IssueOpsBranchPrepareRequest{
		Provider:     "github",
		IssueURL:     "https://github.com/acme/repo/issues/1234",
		Branch:       record.Branch,
		BaseBranch:   "main",
		LinkVerified: true,
	})
	if err != nil {
		t.Fatalf("PrepareIssueOpsBranch: %v", err)
	}
	return record
}

func writeFakeGhForCreateChild(t *testing.T, binDir string) {
	t.Helper()
	path := filepath.Join(binDir, "gh")
	script := `#!/bin/sh
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/acme/repo/issues/34\n'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/34" ]; then
  printf '{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}]}'
  exit 0
fi
if [ "$1 $2" = "api -X" ] && [ "$3" = "POST" ]; then
  printf '{"ok":true}'
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "repos/acme/repo/issues/1234/sub_issues" ]; then
  printf '[{"id":987,"number":34,"html_url":"https://github.com/acme/repo/issues/34"}]'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestRunRemoteCreateIssueConfirmVerifiesLiveIssue(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	binDir := t.TempDir()
	writeFakeGhForCreateIssue(t, binDir)
	t.Setenv("PATH", binDir)

	var verified []core.IssueOpsRemoteArtifactVerificationRequest
	deps := Deps{
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		VerifyLive: func(req core.IssueOpsRemoteArtifactVerificationRequest) error {
			verified = append(verified, req)
			return nil
		},
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--confirm", "--json"}, deps); err != nil {
		t.Fatalf("create-issue confirm returned error: %v", err)
	}
	if len(verified) != 1 {
		t.Fatalf("expected one live verification, got %d", len(verified))
	}
	got := verified[0]
	if got.Kind != "issue" || got.Provider != "github" || got.URL != "https://github.com/acme/repo/issues/77" {
		t.Fatalf("unexpected verify request: %+v", got)
	}
	if strings.Join(got.Labels, ",") != "bug" || strings.Join(got.Assignees, ",") != "octocat" {
		t.Fatalf("labels/assignees not forwarded to verification: %+v", got)
	}
}

func TestRunRemoteCreateIssueConfirmFailsWhenLiveVerificationFails(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	binDir := t.TempDir()
	writeFakeGhForCreateIssue(t, binDir)
	t.Setenv("PATH", binDir)

	deps := Deps{
		PrintError: func(error) error { return nil },
		VerifyLive: func(core.IssueOpsRemoteArtifactVerificationRequest) error {
			return errors.New("remote artifact missing verified label(s): bug")
		},
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--confirm"}, deps); err == nil || !strings.Contains(err.Error(), "missing verified label") {
		t.Fatalf("expected live verification failure to propagate, got %v", err)
	}
}

func writeFakeGhForCreateIssue(t *testing.T, binDir string) {
	t.Helper()
	path := filepath.Join(binDir, "gh")
	script := `#!/bin/sh
if [ "$1 $2" = "issue create" ]; then
  printf 'https://github.com/acme/repo/issues/77\n'
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
