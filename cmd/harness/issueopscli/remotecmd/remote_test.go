package remotecmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops"
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
	if err := Run([]string{"verify-artifact", "--id", record.ID, "--provider", "github", "--kind", "pr", "--url", "https://github.com/acme/repo/pull/1", "--label", "bug", "--assignee", "octocat", "--json"}, deps); err != nil {
		t.Fatalf("verify-artifact returned error: %v", err)
	}
	if err := Run([]string{"create-issue", "--id", record.ID, "--title", "Title", "--body", "Body", "--label", "bug", "--json"}, deps); err != nil {
		t.Fatalf("create-issue dry-run returned error: %v", err)
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
	if len(printed) != 3 {
		t.Fatalf("expected three JSON dry-run outputs, got %d", len(printed))
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
	record, err = core.LinkIssueOpsChild(core.IssueOpsStateRoot(), record.ID, "https://github.com/acme/repo/issues/1235", "child")
	if err != nil {
		t.Fatalf("LinkIssueOpsChild: %v", err)
	}
	record.Phase = issueops.IssueOpsPhasePR
	record, err = issueops.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatalf("WriteIssueOps: %v", err)
	}
	return record
}
