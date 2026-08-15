package remotecmd

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	port "agent-harness/internal/port"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
	var records []issueopscontract.IssueOpsRecord
	var printed []any
	deps := Deps{
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		PrintResult: func(record issueopscontract.IssueOpsRecord, jsonOut bool, err error) error {
			if err != nil {
				return err
			}
			records = append(records, record)
			return nil
		},
		VerifyLive: func(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
			if req.Provider == "" || req.URL == "" {
				return errors.New("bad request")
			}
			return nil
		},
		Publication: PublicationHandlers{Create: func(context.Context, string, issueopscore.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{OK: true, Preview: "would create pull request"}, nil
		}},
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--body", "Body", "--label", "bug", "--json"}, deps); err != nil {
		t.Fatalf("create-issue dry-run returned error: %v", err)
	}
	if err := Run([]string{"create-child", "--id", record.ID, "--title", "Child", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("create-child dry-run returned error: %v", err)
	}
	if err := Run([]string{"create-pr", "--id", record.ID, "--title", "PR", "--body", "Body", "--head", record.Branch, "--base", "main", "--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("create-pr dry-run returned error: %v", err)
	}
	if err := Run([]string{"verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/acme/repo/pull/1", "--target-branch", "main", "--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("verify-artifact returned error: %v", err)
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
	createPayload, err := json.Marshal(printed[0])
	if err != nil {
		t.Fatal(err)
	}
	var createShape map[string]any
	if err := json.Unmarshal(createPayload, &createShape); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ok", "provider", "issue_url", "issue_number", "labels", "assignees", "preview"} {
		if _, ok := createShape[key]; !ok {
			t.Fatalf("create-issue response missing %q: %s", key, createPayload)
		}
	}
	if _, legacy := createShape["url"]; legacy {
		t.Fatalf("create-issue response exposes legacy url key: %s", createPayload)
	}
}

func TestRunRemoteCreateIssueRejectsSecretLikeFields(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	tests := []struct {
		name string
		args []string
	}{
		{name: "title", args: []string{"--title", "token=private-value"}},
		{name: "body", args: []string{"--title", "Title", "--body", "password=private-value"}},
		{name: "label", args: []string{"--title", "Title", "--label", "api_key=private-value"}},
		{name: "assignee", args: []string{"--title", "Title", "--assignee", "credential=private-value"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"create-issue", "--id", record.ID}, test.args...)
			err := Run(args, Deps{})
			if err == nil || !strings.Contains(err.Error(), "issue create "+test.name+" contains secret-like content") {
				t.Fatalf("error = %v", err)
			}
		})
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

func TestRunRemoteCreateIssueValidatesTitleBeforeProviderInference(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{
		Repo:   t.TempDir(),
		Branch: "1234-title-validation",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = Run([]string{"create-issue", "--id", record.ID}, Deps{})

	if err == nil || !strings.Contains(err.Error(), "issue title is required") {
		t.Fatalf("error = %v, want title validation before provider inference", err)
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

func TestRunRemoteCreatePRDryRunRejectsSecretLikeContentBeforeProviderCall(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	secret := "api_key=opaque-token password=opaque-password Authorization: Bearer opaque-bearer /tmp/secret.pem"
	deps := Deps{
		Publication: PublicationHandlers{Create: func(context.Context, string, issueopscore.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{}, errors.New("remote create title or body contains secret-like content")
		}},
	}
	err := Run([]string{"create-pr", "--id", record.ID, "--provider", "github", "--title", "PR", "--body", secret, "--head", record.Branch, "--base", "main", "--json"}, deps)
	if err == nil || !strings.Contains(err.Error(), "secret-like") {
		t.Fatalf("error=%v", err)
	}
}

func TestRunRemoteCreatePRUsesPublicationHandlerForPreviewAndConfirm(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	ancestry, err := issueopscore.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	var process issueopscontract.NativeProcessReceipt
	for _, receipt := range ancestry {
		if receipt.PID == os.Getpid() {
			process = receipt
			break
		}
	}
	if process.PID == 0 {
		t.Fatalf("current process receipt missing from ancestry: %#v", ancestry)
	}
	handlerCalls := 0
	var printed []any
	deps := Deps{
		Publication: PublicationHandlers{Create: func(_ context.Context, stateRoot string, request issueopscore.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			handlerCalls++
			if stateRoot != issueopscore.IssueOpsStateRoot() || request.ID != record.ID || request.Provider != "github" || request.Title != "PR" {
				t.Fatalf("stateRoot=%q request=%#v", stateRoot, request)
			}
			if request.Confirm {
				return port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/195", Number: "195"}, nil
			}
			return port.IssueProviderCreatePullRequestResult{OK: true, Preview: "would create pull request"}, nil
		}},
		ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
			return append([]issueopscontract.NativeProcessReceipt(nil), ancestry...), nil
		},
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
	}
	baseArgs := []string{
		"create-pr", "--id", record.ID, "--provider", "github", "--title", "PR", "--body", "Body",
		"--head", record.Branch, "--base", "main", "--label", "bug", "--assignee", "octocat", "--json",
	}
	if err := Run(baseArgs, deps); err != nil {
		t.Fatal(err)
	}
	confirmArgs := append(append([]string{}, baseArgs...),
		"--host", "codex", "--session-id", "session-195", "--session-pid", strconv.Itoa(process.PID),
		"--session-started-at", process.StartedAt, "--session-executable", process.Executable,
		"--cwd", record.Repo, "--confirm",
	)
	if err := Run(confirmArgs, deps); err != nil {
		t.Fatal(err)
	}
	if handlerCalls != 2 || len(printed) != 2 {
		t.Fatalf("handlerCalls=%d printed=%#v", handlerCalls, printed)
	}
	preview := printed[0].(port.IssueProviderCreatePullRequestResult)
	created := printed[1].(port.IssueProviderCreatePullRequestResult)
	if preview.Preview != "would create pull request" || created.URL != "https://github.com/acme/repo/pull/195" {
		t.Fatalf("preview=%#v created=%#v", preview, created)
	}
}

func TestRunRemoteCreatePRFailsClosedWithoutPublicationHandler(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	err := Run([]string{
		"create-pr", "--id", record.ID, "--provider", "github", "--title", "PR", "--body", "Body",
		"--head", record.Branch, "--base", "main", "--label", "bug", "--assignee", "octocat",
	}, Deps{})
	if !errors.Is(err, issueopscore.ErrRemotePullRequestCreateHandlerUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestRunRemoteCreatePRObservesAncestryOnlyForConfirmedMutation(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecord(t)
	observeCalls := 0
	providerCalls := 0
	deps := Deps{
		ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
			observeCalls++
			return nil, errors.New("ps unavailable")
		},
		Publication: PublicationHandlers{Create: func(context.Context, string, issueopscore.RemotePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			providerCalls++
			return port.IssueProviderCreatePullRequestResult{OK: true, Preview: "would create pull request"}, nil
		}},
	}
	baseArgs := []string{
		"create-pr", "--id", record.ID, "--provider", "github", "--title", "PR", "--body", "Body",
		"--head", record.Branch, "--base", "main", "--label", "bug", "--assignee", "octocat",
		"--host", "codex", "--session-id", "session-1", "--session-pid", "42",
		"--session-started-at", "2026-07-23T00:00:00Z", "--session-executable", "/bin/codex", "--cwd", record.Repo,
	}
	if err := Run(baseArgs, deps); err != nil {
		t.Fatalf("create-pr dry-run must not require process observation: %v", err)
	}
	if observeCalls != 0 {
		t.Fatalf("create-pr dry-run observed process ancestry %d times", observeCalls)
	}
	providerCalls = 0

	err := Run(append(append([]string{}, baseArgs...), "--confirm"), deps)
	if err == nil || !strings.Contains(err.Error(), "observe native process ancestry") || !strings.Contains(err.Error(), "ps unavailable") {
		t.Fatalf("create-pr confirm did not propagate process observation failure: %v", err)
	}
	if observeCalls != 1 || providerCalls != 0 {
		t.Fatalf("observation/provider calls = %d/%d, want 1/0", observeCalls, providerCalls)
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
	updated, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
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

func TestRunRemoteCreateChildConfirmUsesActiveLeaseActor(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	worktree, ancestry := activateRemoteIssueOpsRecordForCurrentProcess(t, &record)

	binDir := t.TempDir()
	writeFakeGhForCreateChild(t, binDir)
	t.Setenv("PATH", binDir)
	err := Run([]string{
		"create-child", "--id", record.ID, "--title", "Child", "--body", "Body",
		"--label", "bug", "--assignee", "octocat", "--host", "codex",
		"--session-id", "session-1", "--cwd", worktree, "--confirm", "--json",
	}, Deps{
		PrintError: func(error) error { return nil },
		ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
			return ancestry, nil
		},
	})
	if err != nil {
		t.Fatalf("create-child confirm with current lease actor returned error: %v", err)
	}
	updated, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.IssueLinks) != 1 || updated.IssueLinks[0].URL != "https://github.com/acme/repo/issues/34" {
		t.Fatalf("child link not recorded with active lease: %+v", updated.IssueLinks)
	}
}

func TestRunRemoteCreateChildRejectsWrongActorBeforeProviderCall(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordWithoutChild(t)
	worktree, ancestry := activateRemoteIssueOpsRecordForCurrentProcess(t, &record)
	t.Setenv("PATH", t.TempDir())

	err := Run([]string{
		"create-child", "--id", record.ID, "--title", "Child", "--body", "Body",
		"--label", "bug", "--assignee", "octocat", "--host", "codex",
		"--session-id", "wrong-session", "--cwd", worktree, "--confirm", "--json",
	}, Deps{
		PrintError: func(error) error { return nil },
		ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) {
			return ancestry, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "current write lease holder") {
		t.Fatalf("wrong actor must fail before provider execution, got %v", err)
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
	if err := deps.printResult(issueopscontract.IssueOpsRecord{}, false, errors.New("x")); err == nil {
		t.Fatal("default printResult should return input error")
	}
	if err := deps.verifyLive(context.Background(), issueopscontract.IssueOpsRemoteArtifactVerificationRequest{}); err == nil {
		t.Fatal("default verifyLive must fail closed")
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
	item := issueopscore.IssueOpsRemoteScoredItem{ID: "1", URL: "url", Title: "Title", Score: 0.9}
	if formatIssueOpsRemoteIssueRef(item) != "1 (Title)" || formatIssueOpsRemoteIssueRef(issueopscore.IssueOpsRemoteScoredItem{Title: "Title"}) != "Title" {
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
	if issueopscore.ResolveRecordProvider(issueopscontract.IssueOpsRecord{BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{Provider: "gitlab"}}) != "gitlab" {
		t.Fatal("branch prepare provider should win")
	}
	if issueopscore.ResolveRecordProvider(issueopscontract.IssueOpsRecord{RemoteArtifact: &issueopscontract.IssueOpsRemoteArtifactVerification{Provider: "github"}}) != "github" {
		t.Fatal("remote artifact provider should be used")
	}
	if issueopscore.ResolveRecordProvider(issueopscontract.IssueOpsRecord{IssueURL: "https://gitlab.com/acme/repo/-/issues/1"}) != "gitlab" {
		t.Fatal("gitlab issue URL should infer provider")
	}
	if err := Run(nil, deps); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if err := Run([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops remote") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestDurableIssueCreateFailureRedactsAndCapsDiagnostics(t *testing.T) {
	got := durableIssueCreateFailure(errors.New(
		"token=super-secret https://internal.example/path " + strings.Repeat("x", 4096),
	))

	if strings.Contains(got, "super-secret") || strings.Contains(got, "internal.example") {
		t.Fatalf("durable diagnostic was not redacted: %q", got)
	}
	if len(got) > 2048 {
		t.Fatalf("durable diagnostic length = %d, want <= 2048", len(got))
	}
}

func TestReflectDevilsAdvocateAcceptsHolderActorFlags(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	err := Run([]string{
		"reflect-devils-advocate",
		"--id", "io-missing",
		"--provider", "gitlab",
		"--host", "codex",
		"--session-id", "session-1",
		"--agent-id", "agent-1",
		"--cwd", t.TempDir(),
		"--confirm",
	}, Deps{})
	if err == nil {
		t.Fatal("존재하지 않는 lifecycle은 실패해야 한다")
	}
	if strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("reflect-devils-advocate가 holder actor flag를 등록하지 않았다: %v", err)
	}
}

func TestRemoteNativeActorIncludesCurrentProcessAncestry(t *testing.T) {
	actor, err := (Deps{}).remoteNativeActor("codex", "session-1", "agent-1", 42, "2026-07-23T00:00:00Z", "/bin/codex", true)
	if err != nil {
		t.Fatal(err)
	}
	if actor.Host != "codex" || actor.SessionID != "session-1" || actor.AgentID != "agent-1" {
		t.Fatalf("native actor identity was not preserved: %#v", actor)
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID != 42 {
		t.Fatalf("native actor process receipt was not preserved: %#v", actor.SessionProcess)
	}
	if len(actor.ProcessAncestry) == 0 || actor.ProcessAncestry[0].PID != os.Getpid() {
		t.Fatalf("native actor did not capture the current process ancestry: %#v", actor.ProcessAncestry)
	}
}

func remoteIssueOpsRecord(t *testing.T) issueopscontract.IssueOpsRecord {
	t.Helper()
	record := remoteIssueOpsRecordWithoutChild(t)
	var err error
	record, err = issueopscore.LinkIssueOpsChild(issueopscore.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1235", "child")
	if err != nil {
		t.Fatalf("LinkIssueOpsChild: %v", err)
	}
	record.Phase = issueopscore.IssueOpsPhasePR
	record, err = issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatalf("WriteIssueOps: %v", err)
	}
	return record
}

func remoteIssueOpsRecordWithoutChild(t *testing.T) issueopscontract.IssueOpsRecord {
	t.Helper()
	repo := t.TempDir()
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "1234-remote-cmd"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	record, err = issueopscore.LinkIssueOpsIssue(issueopscore.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1234")
	if err != nil {
		t.Fatalf("LinkIssueOpsIssue: %v", err)
	}
	record, err = issueopscore.PrepareIssueOpsBranch(issueopscore.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsBranchPrepareRequest{
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

func remoteIssueOpsRecordForCreate(t *testing.T) issueopscontract.IssueOpsRecord {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init"},
		{"remote", "add", "origin", "https://github.com/acme/repo.git"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: "1234-remote-create"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	return record
}

func activateRemoteIssueOpsRecordForCurrentProcess(t *testing.T, record *issueopscontract.IssueOpsRecord) (string, []issueopscontract.NativeProcessReceipt) {
	t.Helper()
	ancestry, err := issueopscore.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	var process issueopscontract.NativeProcessReceipt
	for _, receipt := range ancestry {
		if receipt.PID == os.Getpid() {
			process = receipt
			break
		}
	}
	if process.PID == 0 {
		t.Fatalf("current process receipt missing from ancestry: %#v", ancestry)
	}
	worktree := filepath.Join(record.Repo, "worktree")
	record.WorktreePath = worktree
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
			BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-02T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{
			Generation: 1, Status: issueopscontract.LeaseStatusActive, ClaimedAt: "2026-08-02T00:00:00Z",
			Holder: &issueopscontract.NativeActor{Host: "codex", SessionID: "session-1", SessionProcess: &process},
		},
	}
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), *record); err != nil {
		t.Fatal(err)
	}
	return worktree, ancestry
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
	record := remoteIssueOpsRecordForCreate(t)
	binDir := t.TempDir()
	writeFakeGhForCreateIssue(t, binDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var verified []issueopscontract.IssueOpsRemoteArtifactVerificationRequest
	deps := Deps{
		PrintJSON:  func(any) error { return nil },
		PrintError: func(error) error { return nil },
		VerifyLive: func(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
			verified = append(verified, req)
			return nil
		},
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--provider", "github", "--title", "Title", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--confirm", "--json"}, deps); err != nil {
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
	stored, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IssueURL != "https://github.com/acme/repo/issues/77" || stored.IssueCreateIntent == nil || stored.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted {
		t.Fatalf("created issue receipt was not committed atomically: %+v", stored)
	}
}

func TestRunRemoteCreateIssueConfirmFailsWhenLiveVerificationFails(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := remoteIssueOpsRecordForCreate(t)
	binDir := t.TempDir()
	writeFakeGhForCreateIssue(t, binDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	deps := Deps{
		PrintError: func(error) error { return nil },
		VerifyLive: func(issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
			return errors.New("remote artifact missing verified label(s): bug")
		},
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--provider", "github", "--title", "Title", "--body", "Body", "--label", "bug", "--assignee", "octocat", "--confirm"}, deps); err == nil || !strings.Contains(err.Error(), "missing verified label") {
		t.Fatalf("expected live verification failure to propagate, got %v", err)
	}
	stored, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IssueURL != "" || stored.IssueCreateIntent == nil || stored.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentVerificationFailed {
		t.Fatalf("verification failure did not remain recoverable: %+v", stored)
	}
}

func TestRunRemoteReconcileIssueRequiresExactlyOneMatchingCandidate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, body := remoteIssueOpsRecordWithCreateIntent(t)
	binDir := t.TempDir()
	writeFakeGhForReconcileIssue(t, binDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	t.Setenv("RECONCILE_JSON", `[]`)
	if err := Run([]string{"reconcile-issue", "--id", record.ID, "--json"}, Deps{}); err == nil || !strings.Contains(err.Error(), "found 0 live candidates") {
		t.Fatalf("expected zero-candidate block, got %v", err)
	}

	row := map[string]string{"url": "https://github.com/acme/repo/issues/77", "title": "Title", "body": body}
	rows, err := json.Marshal([]map[string]string{row, row})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RECONCILE_JSON", string(rows))
	if err := Run([]string{"reconcile-issue", "--id", record.ID, "--json"}, Deps{}); err == nil || !strings.Contains(err.Error(), "found 2 live candidates") {
		t.Fatalf("expected many-candidate block, got %v", err)
	}
}

func TestRunRemoteReconcileIssueAdoptsDelayedUniqueVerifiedCandidate(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, body := remoteIssueOpsRecordWithCreateIntent(t)
	binDir := t.TempDir()
	writeFakeGhForReconcileIssue(t, binDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	t.Setenv("RECONCILE_JSON", `[]`)
	if err := Run([]string{"reconcile-issue", "--id", record.ID}, Deps{}); err == nil {
		t.Fatal("expected delayed zero-candidate result to remain unresolved")
	}
	rows, err := json.Marshal([]map[string]string{{
		"url":   "https://github.com/acme/repo/issues/77",
		"title": "Title",
		"body":  body,
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RECONCILE_JSON", string(rows))
	var verified int
	var printed []any
	deps := Deps{
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		VerifyLive: func(issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
			verified++
			return nil
		},
	}
	if err := Run([]string{"reconcile-issue", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("preview unique candidate: %v", err)
	}
	if err := Run([]string{"reconcile-issue", "--id", record.ID, "--confirm", "--json"}, deps); err != nil {
		t.Fatalf("reconcile unique candidate: %v", err)
	}
	if len(printed) != 2 {
		t.Fatalf("printed %d reconcile results, want 2", len(printed))
	}
	preview, ok := printed[0].(issueopscontract.IssueOpsIssueCreateReconcileResult)
	if !ok || !preview.OK || preview.CandidateCount != 1 || preview.CandidateURL == "" || !preview.WouldAdopt ||
		preview.IssueURL != "" || preview.IssueCreateIntent == nil {
		t.Fatalf("preview reconcile result = %#v", printed[0])
	}
	confirmed, ok := printed[1].(issueopscontract.IssueOpsIssueCreateReconcileResult)
	if !ok || !confirmed.OK || confirmed.CandidateCount != 1 || confirmed.CandidateURL == "" || confirmed.WouldAdopt ||
		confirmed.IssueURL == "" || confirmed.IssueCreateIntent == nil ||
		confirmed.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted {
		t.Fatalf("confirmed reconcile result = %#v", printed[1])
	}
	if verified != 1 {
		t.Fatalf("expected one live verification, got %d", verified)
	}
	stored, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.IssueURL != "https://github.com/acme/repo/issues/77" || stored.IssueCreateIntent == nil || stored.IssueCreateIntent.Status != issueopscontract.IssueCreateIntentCompleted {
		t.Fatalf("unique candidate was not adopted atomically: %+v", stored)
	}
}

func TestRunRemoteReconcileIssueRejectsTruncatedAndForeignProjectCandidates(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record, body := remoteIssueOpsRecordWithCreateIntent(t)
	binDir := t.TempDir()
	writeFakeGhForReconcileIssue(t, binDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	rows := make([]map[string]string, 100)
	for index := range rows {
		rows[index] = map[string]string{
			"url":   fmt.Sprintf("https://github.com/acme/repo/issues/%d", index+1),
			"title": "Title",
			"body":  body,
		}
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RECONCILE_JSON", string(raw))
	if err := Run([]string{"reconcile-issue", "--id", record.ID}, Deps{}); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("error = %v, want truncated search rejection", err)
	}

	raw, err = json.Marshal([]map[string]string{{
		"url":   "https://github.com/other/repo/issues/77",
		"title": "Title",
		"body":  body,
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RECONCILE_JSON", string(raw))
	if err := Run([]string{"reconcile-issue", "--id", record.ID, "--confirm"}, Deps{
		VerifyLive: func(issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
			t.Fatal("foreign project candidate reached live verification")
			return nil
		},
	}); err == nil || !strings.Contains(err.Error(), "sealed authority") {
		t.Fatalf("error = %v, want foreign project rejection", err)
	}
}

func remoteIssueOpsRecordWithCreateIntent(t *testing.T) (issueopscontract.IssueOpsRecord, string) {
	t.Helper()
	record := remoteIssueOpsRecordForCreate(t)
	operationID := "0123456789abcdef0123456789abcdef"
	marker := "<!-- agent-harness:issue-create:" + operationID + " -->"
	body := "Body\n\n" + marker
	digest := sha256.Sum256([]byte(body))
	updated, err := issueopscore.BeginIssueCreateIntent(issueopscore.IssueOpsStateRoot(), record.ID, issueopscontract.IssueOpsIssueCreateIntentRequest{
		OperationID:      operationID,
		Provider:         "github",
		ProjectAuthority: "github.com/acme/repo",
		Title:            "Title",
		BodySHA256:       fmt.Sprintf("%x", digest[:]),
		Labels:           []string{"bug"},
		Assignees:        []string{"octocat"},
		StartedAt:        "2026-08-14T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	return updated, body
}

func writeFakeGhForReconcileIssue(t *testing.T, binDir string) {
	t.Helper()
	path := filepath.Join(binDir, "gh")
	script := `#!/bin/sh
if [ "$1 $2" = "issue list" ]; then
  printf '%s' "$RECONCILE_JSON"
  exit 0
fi
echo "unexpected gh call: $*" >&2
exit 2
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
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
