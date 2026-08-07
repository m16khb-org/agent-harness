package feedbackcleanup

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "agent-harness/internal/contract/issueops"

	"agent-harness/internal/adapter/core"
	"agent-harness/internal/adapter/issueops/orphancleanup"
	"agent-harness/internal/port"
)

func TestRunFeedbackAddAndMarkIssueUpdated(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	var printed []issueopscontract.IssueOpsRecord
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintResult: func(record issueopscontract.IssueOpsRecord, jsonOut bool, err error) error {
			if err != nil {
				return err
			}
			printed = append(printed, record)
			return nil
		},
	}
	if err := RunFeedback([]string{"add", "--id", record.ID, "--source", "review", "--body", "fix this", "--classification", "contract_change", "--json"}, deps); err != nil {
		t.Fatalf("RunFeedback add returned error: %v", err)
	}
	if err := RunFeedback([]string{"mark-issue-updated", "--id", record.ID}, deps); err != nil {
		t.Fatalf("RunFeedback mark returned error: %v", err)
	}
	if len(printed) != 2 {
		t.Fatalf("expected two printed records, got %d", len(printed))
	}
	if len(printed[0].Feedback) != 1 || printed[0].Feedback[0].Classification != "contract_change" {
		t.Fatalf("unexpected feedback record: %#v", printed[0].Feedback)
	}
	if printed[1].Feedback[0].IssueUpdatedAt == "" {
		t.Fatalf("expected issue updated timestamp: %#v", printed[1].Feedback[0])
	}
}

func TestRunCleanupStatusAndJSONError(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	var statuses []any
	var printedErrors []error
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON: func(value any) error {
			statuses = append(statuses, value)
			return nil
		},
		PrintError: func(err error) error {
			printedErrors = append(printedErrors, err)
			return nil
		},
		VerifyMerged: func(issueopscontract.IssueOpsRemoteArtifactVerification) error {
			return nil
		},
	}
	if err := RunCleanup([]string{"status", "--id", record.ID, "--json"}, deps); err != nil {
		t.Fatalf("RunCleanup status returned error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected one status output, got %d", len(statuses))
	}
	if err := RunCleanup([]string{"status", "--id", "missing", "--json"}, deps); err == nil {
		t.Fatal("expected missing status error")
	}
	if len(printedErrors) != 1 {
		t.Fatalf("expected JSON error to be printed, got %d", len(printedErrors))
	}
}

func TestCleanupMergedAndCommandBoundaries(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	verified := 0
	deps := Deps{
		ParseFlags:  parseFeedbackCleanupFlags,
		PrintResult: func(issueopscontract.IssueOpsRecord, bool, error) error { return nil },
		VerifyMerged: func(issueopscontract.IssueOpsRemoteArtifactVerification) error {
			verified++
			return nil
		},
	}
	if CleanupMerged(record.ID, false, deps) {
		t.Fatal("unrequested merge confirmation should be false")
	}
	if verified != 0 {
		t.Fatalf("merge verifier should not run without --merged, ran %d times", verified)
	}
	if CleanupMerged("missing", true, deps) {
		t.Fatal("missing record should not verify merged")
	}
	if verified != 0 {
		t.Fatalf("merge verifier should not run for missing records, ran %d times", verified)
	}
	if err := RunFeedback(nil, deps); err != nil {
		t.Fatalf("help feedback returned error: %v", err)
	}
	if err := RunCleanup(nil, deps); err != nil {
		t.Fatalf("help cleanup returned error: %v", err)
	}
	if err := RunFeedback([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops feedback") {
		t.Fatalf("expected unknown feedback error, got %v", err)
	}
	if err := RunCleanup([]string{"unknown"}, deps); err == nil || !strings.Contains(err.Error(), "unknown issueops cleanup") {
		t.Fatalf("expected unknown cleanup error, got %v", err)
	}
	if err := RunFeedback([]string{"add", "--bad"}, deps); err == nil {
		t.Fatal("expected parse flag error")
	}
}

func TestRunCleanupStatusSkipsRemoteObservationUntilFinishEligible(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	cases := []struct {
		name    string
		prepare func(*testing.T) issueopscontract.IssueOpsRecord
		args    func(issueopscontract.IssueOpsRecord) []string
		missing string
	}{
		{
			name: "early phase with merged flag",
			prepare: func(t *testing.T) issueopscontract.IssueOpsRecord {
				return cleanupStatusRecord(t, false, true)
			},
			args: func(record issueopscontract.IssueOpsRecord) []string {
				return []string{"status", "--id", record.ID, "--merged", "--json"}
			},
			missing: "pr_phase",
		},
		{
			name: "done without artifact",
			prepare: func(t *testing.T) issueopscontract.IssueOpsRecord {
				return cleanupStatusRecord(t, true, false)
			},
			args: func(record issueopscontract.IssueOpsRecord) []string {
				return []string{"status", "--id", record.ID, "--merged", "--json"}
			},
			missing: "remote_artifact",
		},
		{
			name: "done with artifact but no merged flag",
			prepare: func(t *testing.T) issueopscontract.IssueOpsRecord {
				return cleanupStatusRecord(t, true, true)
			},
			args: func(record issueopscontract.IssueOpsRecord) []string {
				return []string{"status", "--id", record.ID, "--json"}
			},
			missing: "remote_artifact_merged",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := tc.prepare(t)
			mergeCalls := 0
			providerCalls := 0
			var printed []any
			deps := cleanupStatusDeps(&printed)
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
				mergeCalls++
				return core.IssueOpsCleanupRemoteBranchArtifactHead{}, nil
			}
			deps.Provider = func(string) (core.IssueProvider, error) {
				providerCalls++
				return &cleanupStatusProvider{}, nil
			}

			if err := RunCleanup(tc.args(record), deps); err != nil {
				t.Fatalf("RunCleanup status: %v", err)
			}
			if mergeCalls != 0 || providerCalls != 0 {
				t.Fatalf("remote observation must be skipped: merge=%d provider=%d", mergeCalls, providerCalls)
			}
			status := printedCleanupStatus(t, printed)
			if status.Merged || !containsCleanupStatusValue(status.Missing, tc.missing) {
				t.Fatalf("structural status not preserved: %+v", status)
			}
		})
	}
}

func TestRunCleanupStatusFailsClosedOnMergedReadbackErrors(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)

	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "unmerged", err: errors.New("pull request is not merged")},
		{name: "readback failure", err: errors.New("provider readback unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			providerCalls := 0
			deps := cleanupStatusDeps(nil)
			deps.Provider = func(string) (core.IssueProvider, error) {
				providerCalls++
				return &cleanupStatusProvider{}, nil
			}
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
				return core.IssueOpsCleanupRemoteBranchArtifactHead{}, tc.err
			}

			err := RunCleanup([]string{"status", "--id", record.ID, "--merged"}, deps)
			if err == nil || !strings.Contains(err.Error(), tc.err.Error()) {
				t.Fatalf("merged readback must remain a real error: %v", err)
			}
			if providerCalls != 1 {
				t.Fatalf("expected one provider resolution, got %d", providerCalls)
			}
		})
	}
}

func TestRunCleanupStatusProjectsFinishReadinessParity(t *testing.T) {
	for _, tc := range []struct {
		name        string
		issueState  string
		issueBody   string
		mergedBase  string
		processes   []string
		wantReady   bool
		wantMissing string
		wantWarning string
	}{
		{
			name:       "open issue requires issue closed without cleanup recommendation",
			issueState: "open", issueBody: core.IssueBodyCompletionStartMarker,
			mergedBase: "main", wantMissing: "issue_closed",
		},
		{
			name:       "closed issue matches finish ready",
			issueState: "closed", issueBody: core.IssueBodyCompletionStartMarker,
			mergedBase: "main", wantReady: true,
		},
		{
			name:       "base drift is projected",
			issueState: "closed", issueBody: core.IssueBodyCompletionStartMarker,
			mergedBase: "release", wantMissing: "base_branch_drifted",
		},
		{
			name:       "workspace holder becomes warning",
			issueState: "closed", issueBody: core.IssueBodyCompletionStartMarker,
			mergedBase: "main", processes: []string{"4321:codex"},
			wantMissing: "workspace_processes_quiescent", wantWarning: "4321:codex",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			record := cleanupStatusRecord(t, true, true)
			var printed []any
			provider := &cleanupStatusProvider{snapshot: core.ExecutionIssueSnapshot{
				URL: record.IssueURL, Body: tc.issueBody, State: tc.issueState,
			}}
			deps := cleanupStatusDeps(&printed)
			deps.Provider = func(name string) (core.IssueProvider, error) {
				if name != "github" {
					t.Fatalf("provider name = %q", name)
				}
				return provider, nil
			}
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
				return core.IssueOpsCleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "abc123", BaseRefName: tc.mergedBase}, nil
			}
			gitCalls := 0
			deps.CleanupFinishGit = func(_ string, args ...string) (int, string) {
				gitCalls++
				switch args[0] {
				case "status", "ls-remote":
					return 0, ""
				case "rev-parse":
					return 0, "abc123\n"
				default:
					t.Fatalf("unexpected git call: %v", args)
					return 1, ""
				}
			}
			processCalls := 0
			deps.InspectCleanupProcesses = func(root string) ([]string, error) {
				processCalls++
				if root != record.WorktreePath {
					t.Fatalf("process root = %q, want %q", root, record.WorktreePath)
				}
				return tc.processes, nil
			}

			if err := RunCleanup([]string{"status", "--id", record.ID, "--merged", "--json"}, deps); err != nil {
				t.Fatalf("ordinary finish preview block must normalize to status: %v", err)
			}
			status := printedCleanupStatus(t, printed)
			if status.Ready != tc.wantReady || !status.OK || !status.Merged {
				t.Fatalf("projected booleans = %+v", status)
			}
			if status.ID != record.ID || status.WorktreePath != record.WorktreePath || status.Branch != record.Branch || status.RemoteArtifactURL != record.RemoteArtifact.URL {
				t.Fatalf("schema projection lost identity fields: %+v", status)
			}
			if tc.wantMissing != "" && !containsCleanupStatusValue(status.Missing, tc.wantMissing) {
				t.Fatalf("missing %q not projected: %+v", tc.wantMissing, status)
			}
			if tc.wantWarning != "" && !containsCleanupStatusValue(status.Warnings, tc.wantWarning) {
				t.Fatalf("warning %q not projected: %+v", tc.wantWarning, status)
			}
			if len(status.Choices) != 3 {
				t.Fatalf("three-choice helper was not applied: %+v", status)
			}
			if tc.wantMissing == "issue_closed" && strings.Contains(strings.Join(status.Choices, "\n"), "정리 진행") {
				t.Fatalf("blocked issue must not recommend cleanup: %+v", status.Choices)
			}
			if gitCalls == 0 || processCalls != 1 || provider.readCalls != 1 {
				t.Fatalf("injected oracle dependencies not exercised: git=%d process=%d issue=%d", gitCalls, processCalls, provider.readCalls)
			}
		})
	}
}

func TestRunCleanupStatusDoesNotNormalizeProviderOrIssueErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider func(string) (core.IssueProvider, error)
		want     string
	}{
		{
			name: "provider resolution",
			provider: func(string) (core.IssueProvider, error) {
				return nil, errors.New("provider unavailable")
			},
			want: "provider unavailable",
		},
		{
			name: "issue readback",
			provider: func(string) (core.IssueProvider, error) {
				return &cleanupStatusProvider{readErr: errors.New("issue unavailable")}, nil
			},
			want: "issue unavailable",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			record := cleanupStatusRecord(t, true, true)
			deps := cleanupStatusDeps(nil)
			deps.Provider = tc.provider
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (core.IssueOpsCleanupRemoteBranchArtifactHead, error) {
				return core.IssueOpsCleanupRemoteBranchArtifactHead{BaseRefName: "main"}, nil
			}
			err := RunCleanup([]string{"status", "--id", record.ID, "--merged"}, deps)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error must not normalize into blocked status: %v", err)
			}
		})
	}
}

func TestRunCleanupOrphanDefaultsToPreviewAndGatesApply(t *testing.T) {
	var printed []any
	var previews []orphancleanup.Request
	var applies []orphancleanup.ApplyRequest
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		OrphanPreview: func(_ context.Context, request orphancleanup.Request) (orphancleanup.Result, error) {
			previews = append(previews, request)
			return orphancleanup.Result{OK: true, Preview: true, Ready: true, Fingerprint: "preview-fingerprint"}, nil
		},
		OrphanApply: func(_ context.Context, request orphancleanup.Request, apply orphancleanup.ApplyRequest) (orphancleanup.Result, error) {
			previews = append(previews, request)
			applies = append(applies, apply)
			return orphancleanup.Result{OK: true, Confirmed: true, Applied: true}, nil
		},
	}
	args := []string{
		"orphan", "--id", "io-f4e347fe9827", "--repo", "/repo", "--worktree", "/repo.worktrees/merged-feature",
		"--branch", "merged-feature", "--provider", "github", "--kind", "pr", "--artifact-url", "https://github.com/example/repo/pull/42", "--json",
	}
	if err := RunCleanup(args, deps); err != nil {
		t.Fatalf("recordless orphan preview: %v", err)
	}
	if len(previews) != 1 || len(applies) != 0 || len(printed) != 1 {
		t.Fatalf("default orphan command must be preview-only: previews=%d applies=%d printed=%d", len(previews), len(applies), len(printed))
	}
	if previews[0].ID != "io-f4e347fe9827" || previews[0].Artifact.URL != "https://github.com/example/repo/pull/42" {
		t.Fatalf("orphan preview request = %#v", previews[0])
	}

	if err := RunCleanup(append(args[:len(args)-1], "--apply"), deps); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("apply without confirm error = %v", err)
	}
	if err := RunCleanup(append(args[:len(args)-1], "--apply", "--confirm", "--fingerprint", "preview-fingerprint", "--json"), deps); err != nil {
		t.Fatalf("confirmed orphan apply: %v", err)
	}
	if len(applies) != 1 || !applies[0].Confirm || applies[0].Fingerprint != "preview-fingerprint" {
		t.Fatalf("apply request = %#v", applies)
	}
}

func feedbackCleanupIssueOpsRecord(t *testing.T) issueopscontract.IssueOpsRecord {
	t.Helper()
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "1234-feedback-cleanup"})
	if err != nil {
		t.Fatalf("StartIssueOps: %v", err)
	}
	return record
}

func cleanupStatusRecord(t *testing.T, done, withArtifact bool) issueopscontract.IssueOpsRecord {
	t.Helper()
	record := feedbackCleanupIssueOpsRecord(t)
	record.IssueURL = "https://github.com/acme/repo/issues/285"
	if done {
		record.Phase = core.IssueOpsPhaseDone
	}
	if withArtifact {
		record.RemoteArtifact = &issueopscontract.IssueOpsRemoteArtifactVerification{
			Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/300",
			Labels: []string{"bug"}, Assignees: []string{"m16khb"},
		}
	}
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch,
		BaseBranch: "main", BaseSHA: strings.Repeat("a", 40), LinkVerified: true,
	}
	worktree := filepath.Join(t.TempDir(), record.Branch)
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.Execution = &issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{
			SourceRoot: record.Repo, Root: worktree, Branch: record.Branch,
			BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-04T00:00:00Z",
		},
		Lease: issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
	}
	written, err := core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatalf("WriteIssueOps: %v", err)
	}
	return written
}

func cleanupStatusDeps(printed *[]any) Deps {
	return Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON: func(value any) error {
			if printed != nil {
				*printed = append(*printed, value)
			}
			return nil
		},
		PrintError: func(error) error { return nil },
		CleanupFinishGit: func(_ string, args ...string) (int, string) {
			switch args[0] {
			case "status", "ls-remote":
				return 0, ""
			case "rev-parse":
				return 0, "abc123\n"
			default:
				return 1, "unexpected git call"
			}
		},
		InspectCleanupProcesses: func(string) ([]string, error) { return nil, nil },
	}
}

func printedCleanupStatus(t *testing.T, printed []any) issueopscontract.IssueOpsCleanupStatus {
	t.Helper()
	if len(printed) != 1 {
		t.Fatalf("printed values = %d, want 1", len(printed))
	}
	status, ok := printed[0].(issueopscontract.IssueOpsCleanupStatus)
	if !ok {
		t.Fatalf("printed type = %T", printed[0])
	}
	return status
}

func containsCleanupStatusValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type cleanupStatusProvider struct {
	snapshot  core.ExecutionIssueSnapshot
	readErr   error
	readCalls int
}

func (p *cleanupStatusProvider) Name() string { return "github" }
func (p *cleanupStatusProvider) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, errors.New("unexpected create issue")
}
func (p *cleanupStatusProvider) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, errors.New("unexpected create pull request")
}
func (p *cleanupStatusProvider) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, errors.New("unexpected create child")
}
func (p *cleanupStatusProvider) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, errors.New("unexpected close child")
}
func (p *cleanupStatusProvider) CloseIssue(port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	return port.IssueProviderCloseIssueResult{}, errors.New("unexpected close issue")
}
func (p *cleanupStatusProvider) UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, errors.New("unexpected update issue body")
}
func (p *cleanupStatusProvider) ReadIssueSnapshot(context.Context, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	p.readCalls++
	return p.snapshot, p.readErr
}

func parseFeedbackCleanupFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}
