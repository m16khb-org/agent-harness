package feedbackcleanup

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscore "agent-harness/internal/adapter/issueops"
	issueopscontract "agent-harness/internal/contract/issueops"
	orphancontract "agent-harness/internal/contract/issueopsorphancleanup"
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
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
				mergeCalls++
				return issueopscontract.CleanupRemoteBranchArtifactHead{}, nil
			}
			deps.Provider = func(string) (port.IssueProvider, error) {
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
			deps.Provider = func(string) (port.IssueProvider, error) {
				providerCalls++
				return &cleanupStatusProvider{}, nil
			}
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
				return issueopscontract.CleanupRemoteBranchArtifactHead{}, tc.err
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
		processes   []issueopscontract.CleanupWorkspaceProcess
		wantReady   bool
		wantMissing string
		wantWarning string
	}{
		{
			name:       "open issue requires issue closed without cleanup recommendation",
			issueState: "open", issueBody: port.IssueBodyCompletionStartMarker,
			mergedBase: "main", wantMissing: "issue_closed",
		},
		{
			name:       "closed issue matches finish ready",
			issueState: "closed", issueBody: port.IssueBodyCompletionStartMarker,
			mergedBase: "main", wantReady: true,
		},
		{
			name:       "base drift is projected",
			issueState: "closed", issueBody: port.IssueBodyCompletionStartMarker,
			mergedBase: "release", wantMissing: "base_branch_drifted",
		},
		{
			// 점유 프로세스는 더 이상 차단 사유가 아니라 apply가 종료할 대상이다.
			// status는 준비됨을 보고하되 무엇이 종료될지 경고로 알린다(#477).
			name:       "workspace holder becomes a stop warning",
			issueState: "closed", issueBody: port.IssueBodyCompletionStartMarker,
			mergedBase: "main", processes: []issueopscontract.CleanupWorkspaceProcess{{PID: 4321, Command: "codex", StartedAt: "2026-08-27T00:00:01Z", Executable: "codex"}},
			wantReady: true, wantWarning: "4321:codex:2026-08-27T00:00:01Z",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HARNESS_STATE_DIR", t.TempDir())
			record := cleanupStatusRecord(t, true, true)
			var printed []any
			provider := &cleanupStatusProvider{snapshot: port.ExecutionIssueSnapshot{
				URL: record.IssueURL, Body: tc.issueBody, State: tc.issueState,
			}}
			deps := cleanupStatusDeps(&printed)
			deps.Provider = func(name string) (port.IssueProvider, error) {
				if name != "github" {
					t.Fatalf("provider name = %q", name)
				}
				return provider, nil
			}
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
				return issueopscontract.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "abc123", BaseRefName: tc.mergedBase}, nil
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
			// 이 테스트 프로세스가 Orca 터미널 안에서 돌더라도 요청자 터미널 join을
			// 시도하지 않게 env를 비운다(join은 adapter 테스트가 고정한다).
			t.Setenv("ORCA_PANE_KEY", "")
			t.Setenv("ORCA_TERMINAL_HANDLE", "")
			processCalls := 0
			deps.InspectCleanupProcesses = func(root string) (port.CleanupWorkspaceOccupancy, error) {
				processCalls++
				if root != record.WorktreePath {
					t.Fatalf("process root = %q, want %q", root, record.WorktreePath)
				}
				ancestry := map[int][]int{os.Getpid(): {1}}
				for _, process := range tc.processes {
					ancestry[process.PID] = []int{1}
				}
				return port.CleanupWorkspaceOccupancy{Occupants: tc.processes, Ancestry: ancestry}, nil
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
		provider func(string) (port.IssueProvider, error)
		want     string
	}{
		{
			name: "provider resolution",
			provider: func(string) (port.IssueProvider, error) {
				return nil, errors.New("provider unavailable")
			},
			want: "provider unavailable",
		},
		{
			name: "issue readback",
			provider: func(string) (port.IssueProvider, error) {
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
			deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
				return issueopscontract.CleanupRemoteBranchArtifactHead{BaseRefName: "main"}, nil
			}
			err := RunCleanup([]string{"status", "--id", record.ID, "--merged"}, deps)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error must not normalize into blocked status: %v", err)
			}
		})
	}
}

func TestRunCleanupFinishUsesSupersedingArtifactBaseBranch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)
	replacement := "https://github.com/acme/repo/pull/454"
	provider := &cleanupStatusProvider{snapshot: port.ExecutionIssueSnapshot{
		URL: record.IssueURL, Body: port.IssueBodyCompletionStartMarker, State: "closed",
	}}
	deps := cleanupStatusDeps(nil)
	deps.Provider = func(string) (port.IssueProvider, error) { return provider, nil }
	observed := []string{}
	deps.VerifyMergedHead = func(artifact issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
		observed = append(observed, artifact.URL)
		if artifact.URL == record.RemoteArtifact.URL {
			return issueopscontract.CleanupRemoteBranchArtifactHead{}, errors.New("original child PR is not merged")
		}
		if artifact.URL != replacement {
			t.Fatalf("unexpected replacement artifact: %+v", artifact)
		}
		return issueopscontract.CleanupRemoteBranchArtifactHead{BaseRefName: "main"}, nil
	}

	previous := cleanupDeps
	t.Cleanup(func() { cleanupDeps = previous })
	wired := cleanupDeps
	var captured issueopscontract.CleanupFinishRequest
	wired.CleanupFinish = func(_ context.Context, _ string, req issueopscontract.CleanupFinishRequest, _ Deps, _ port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
		captured = req
		return issueopscontract.CleanupFinishResult{OK: true, ID: req.ID, Preview: true}, nil
	}
	ConfigureCleanup(wired)

	if err := RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--superseded-by", replacement, "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	if strings.Join(observed, "\n") != record.RemoteArtifact.URL+"\n"+replacement {
		t.Fatalf("merge observations = %v", observed)
	}
	if captured.Merged || captured.SupersededBy != replacement || captured.MergedBaseBranch != "main" {
		t.Fatalf("superseding finish request lost replacement merge evidence: %+v", captured)
	}
}

func TestRunCleanupOrphanDefaultsToPreviewAndGatesApply(t *testing.T) {
	var printed []any
	var previews []orphancontract.Request
	var applies []orphancontract.ApplyRequest
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON: func(value any) error {
			printed = append(printed, value)
			return nil
		},
		OrphanPreview: func(_ context.Context, request orphancontract.Request) (orphancontract.Result, error) {
			previews = append(previews, request)
			return orphancontract.Result{OK: true, Preview: true, Ready: true, Fingerprint: "preview-fingerprint"}, nil
		},
		OrphanApply: func(_ context.Context, request orphancontract.Request, apply orphancontract.ApplyRequest) (orphancontract.Result, error) {
			previews = append(previews, request)
			applies = append(applies, apply)
			return orphancontract.Result{OK: true, Confirmed: true, Applied: true}, nil
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
	record, err := issueopscore.StartIssueOps(issueopscore.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "1234-feedback-cleanup"})
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
		record.Phase = issueopscore.IssueOpsPhaseDone
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
	written, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record)
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
		InspectCleanupProcesses: func(string) (port.CleanupWorkspaceOccupancy, error) {
			return port.CleanupWorkspaceOccupancy{Ancestry: map[int][]int{os.Getpid(): {1}}}, nil
		},
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
	snapshot  port.ExecutionIssueSnapshot
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

// RunCleanup의 close-children 디스패치 경로를 잠근다: merged 확인 플래그가
// 어댑터 요청으로 전달되고, JSON/텍스트 출력과 에러 경로가 계약대로 나간다.
func TestRunCleanupCloseChildrenDispatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	var printed []any
	var printedErrors []error
	var requests []issueopscontract.IssueOpsCloseChildrenRequest
	previous := cleanupDeps
	t.Cleanup(func() { cleanupDeps = previous })
	wired := cleanupDeps
	wired.IssueOpsStateRoot = func() string { return os.Getenv("HARNESS_STATE_DIR") }
	wired.ReadIssueOps = issueopscore.ReadIssueOps
	wired.CloseIssueOpsChildren = func(_ string, _ string, req issueopscontract.IssueOpsCloseChildrenRequest, _ func(string) (port.IssueProvider, error)) (issueopscontract.IssueOpsCloseChildrenResult, error) {
		requests = append(requests, req)
		return issueopscontract.IssueOpsCloseChildrenResult{ClosedCount: 1, Children: []issueopscontract.IssueOpsCloseChildResult{{URL: "https://example.com/i/1", Closed: true, State: "closed"}}}, nil
	}
	ConfigureCleanup(wired)
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(value any) error { printed = append(printed, value); return nil },
		PrintError: func(err error) error { printedErrors = append(printedErrors, err); return nil },
		VerifyMerged: func(issueopscontract.IssueOpsRemoteArtifactVerification) error {
			return nil
		},
	}
	if err := RunCleanup([]string{"close-children", "--id", record.ID, "--merged", "--confirm", "--json"}, deps); err != nil {
		t.Fatalf("close-children json: %v", err)
	}
	// fixture 레코드에는 RemoteArtifact가 없으므로 merged 검증은 false로
	// 강등된다(fail-closed). 요청 플래그 전달과 confirm만 계약이다.
	if len(requests) != 1 || requests[0].Merged || !requests[0].MergeEvidenceRequested || !requests[0].Confirm {
		t.Fatalf("request contract wrong: %#v", requests[0])
	}
	if len(printed) != 1 {
		t.Fatalf("json output missing: %d", len(printed))
	}
	// 에러 경로: 어댑터가 실패하면 JSON 에러 프린트 후 원본 에러 복귀.
	failing := cleanupDeps
	failing.IssueOpsStateRoot = func() string { return os.Getenv("HARNESS_STATE_DIR") }
	failing.ReadIssueOps = issueopscore.ReadIssueOps
	failing.CloseIssueOpsChildren = func(string, string, issueopscontract.IssueOpsCloseChildrenRequest, func(string) (port.IssueProvider, error)) (issueopscontract.IssueOpsCloseChildrenResult, error) {
		return issueopscontract.IssueOpsCloseChildrenResult{}, errors.New("provider refused")
	}
	ConfigureCleanup(failing)
	if err := RunCleanup([]string{"close-children", "--id", record.ID, "--merged", "--json"}, deps); err == nil || err.Error() != "provider refused" {
		t.Fatalf("adapter error must propagate: %v", err)
	}
	if len(printedErrors) != 1 {
		t.Fatalf("json error print missing: %d", len(printedErrors))
	}
}

// 도움말 진입과 알 수 없는 하위명령 경로.
func TestRunCleanupHelpAndUnknownSubcommand(t *testing.T) {
	if err := RunCleanup([]string{"--help"}, Deps{ParseFlags: parseFeedbackCleanupFlags}); err != nil {
		t.Fatalf("help must not error: %v", err)
	}
	if err := RunCleanup([]string{"nonexistent"}, Deps{ParseFlags: parseFeedbackCleanupFlags}); err == nil || !strings.Contains(err.Error(), "unknown issueops cleanup subcommand") {
		t.Fatalf("unknown subcommand must fail closed: %v", err)
	}
}

// cleanup abandon CLI의 모드 배타/조합 검증 경로를 잠근다.
func TestRunCleanupAbandonModeDiscipline(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	deps := Deps{ParseFlags: parseFeedbackCleanupFlags}
	if err := RunCleanup([]string{"abandon", "--id", "io-x", "--reason", "stale cycle", "--preview", "--apply", "--json"}, deps); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("preview+apply must be rejected: %v", err)
	}
	if err := RunCleanup([]string{"abandon", "--id", "io-x", "--reason", "stale cycle"}, deps); err == nil || !strings.Contains(err.Error(), "exactly one mode") {
		t.Fatalf("modeless abandon must be rejected: %v", err)
	}
}

// abandon 요청이 어댑터로 정확히 전달되는지 잠근다(성공 프린트 경로 포함).
func TestRunCleanupAbandonDispatchesToAdapter(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	var printed []any
	var requests []issueopscontract.CleanupAbandonRequest
	previous := cleanupDeps
	t.Cleanup(func() { cleanupDeps = previous })
	wired := cleanupDeps
	wired.IssueOpsStateRoot = func() string { return os.Getenv("HARNESS_STATE_DIR") }
	wired.ReadIssueOps = issueopscore.ReadIssueOps
	wired.CleanupAbandon = func(_ context.Context, _ string, req issueopscontract.CleanupAbandonRequest, _ Deps) (issueopscontract.CleanupAbandonResult, error) {
		requests = append(requests, req)
		return issueopscontract.CleanupAbandonResult{OK: true, ID: req.ID}, nil
	}
	ConfigureCleanup(wired)
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(value any) error { printed = append(printed, value); return nil },
		PrintError: func(err error) error { return nil },
	}
	if err := RunCleanup([]string{"abandon", "--id", record.ID, "--reason", "dogfood verification", "--preview", "--json"}, deps); err != nil {
		t.Fatalf("abandon preview: %v", err)
	}
	if len(requests) != 1 || requests[0].ID != record.ID || requests[0].Reason != "dogfood verification" || requests[0].Apply {
		t.Fatalf("abandon request wrong: %#v", requests[0])
	}
	if len(printed) != 1 {
		t.Fatalf("json output missing: %d", len(printed))
	}
}

// cleanup remote-branch CLI의 모드 배타 규율과 어댑터 디스패치를 잠근다.
func TestRunCleanupRemoteBranchDisciplineAndDispatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	deps := Deps{ParseFlags: parseFeedbackCleanupFlags}
	if err := RunCleanup([]string{"remote-branch", "--id", record.ID, "--preview", "--apply", "--json"}, deps); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("preview+apply must be rejected: %v", err)
	}
	if err := RunCleanup([]string{"remote-branch", "--id", record.ID}, deps); err == nil || !strings.Contains(err.Error(), "exactly one mode") {
		t.Fatalf("modeless remote-branch must be rejected: %v", err)
	}

	var requests []issueopscontract.CleanupRemoteBranchRequest
	var printed []any
	var printedErrors []error
	previous := cleanupDeps
	t.Cleanup(func() { cleanupDeps = previous })
	wired := cleanupDeps
	wired.IssueOpsStateRoot = func() string { return os.Getenv("HARNESS_STATE_DIR") }
	wired.ReadIssueOps = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		return record, nil
	}
	wired.ResolveRecordProvider = func(issueopscontract.IssueOpsRecord) string { return "github" }
	wired.CleanupRemoteBranch = func(_ context.Context, _ string, req issueopscontract.CleanupRemoteBranchRequest, _ Deps, _ port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
		requests = append(requests, req)
		return issueopscontract.CleanupRemoteBranchResult{OK: true, ID: req.ID, Fingerprint: "abc"}, nil
	}
	ConfigureCleanup(wired)
	printDeps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(value any) error { printed = append(printed, value); return nil },
		PrintError: func(err error) error { printedErrors = append(printedErrors, err); return nil },
		Provider:   func(string) (port.IssueProvider, error) { return nil, nil },
		VerifyMergedHead: func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
			return issueopscontract.CleanupRemoteBranchArtifactHead{}, nil
		},
	}
	if err := RunCleanup([]string{"remote-branch", "--id", record.ID, "--preview", "--json"}, printDeps); err != nil {
		t.Fatalf("remote-branch preview: %v", err)
	}
	if len(requests) != 1 || requests[0].ID != record.ID || requests[0].Apply || requests[0].SupersededBy != "" {
		t.Fatalf("remote-branch request wrong: %#v", requests[0])
	}
	if len(printed) != 1 {
		t.Fatalf("json output missing: %d", len(printed))
	}
}

// cleanup linked-branch CLI의 모드 배타와 어댑터 디스패치를 잠근다.
func TestRunCleanupLinkedBranchDisciplineAndDispatch(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := feedbackCleanupIssueOpsRecord(t)
	deps := Deps{ParseFlags: parseFeedbackCleanupFlags}
	if err := RunCleanup([]string{"linked-branch", "--id", record.ID, "--preview", "--apply", "--json"}, deps); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("preview+apply must be rejected: %v", err)
	}
	if err := RunCleanup([]string{"linked-branch", "--id", record.ID}, deps); err == nil || !strings.Contains(err.Error(), "exactly one mode") {
		t.Fatalf("modeless linked-branch must be rejected: %v", err)
	}

	var requests []issueopscontract.CleanupLinkedBranchRequest
	var printed []any
	previous := cleanupDeps
	t.Cleanup(func() { cleanupDeps = previous })
	wired := cleanupDeps
	wired.IssueOpsStateRoot = func() string { return os.Getenv("HARNESS_STATE_DIR") }
	wired.ReadIssueOps = func(string, string) (issueopscontract.IssueOpsRecord, error) {
		return record, nil
	}
	wired.CleanupLinkedBranch = func(_ context.Context, _ string, req issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
		requests = append(requests, req)
		return issueopscontract.CleanupLinkedBranchResult{OK: true, ID: req.ID, State: "absent"}, nil
	}
	ConfigureCleanup(wired)
	printDeps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON:  func(value any) error { printed = append(printed, value); return nil },
	}
	if err := RunCleanup([]string{"linked-branch", "--id", record.ID, "--preview", "--json"}, printDeps); err != nil {
		t.Fatalf("linked-branch preview: %v", err)
	}
	if len(requests) != 1 || requests[0].ID != record.ID || requests[0].Apply || requests[0].Confirm {
		t.Fatalf("linked-branch request wrong: %#v", requests[0])
	}
	if len(printed) != 1 {
		t.Fatalf("json output missing: %d", len(printed))
	}
}

// status는 finish preview의 점유·터미널 관측을 "무엇이 종료될지" 경고로 투영한다
// (#477, plans/285 parity: schema는 그대로, 점유는 Warnings로).
func TestCleanupStatusWarningsProjectStoppedProcesses(t *testing.T) {
	warnings := cleanupStatusWarnings(issueopscontract.CleanupFinishResult{
		WorkspaceProcesses: []issueopscontract.CleanupWorkspaceProcess{
			{PID: 4321, Command: "codex", StartedAt: "2026-08-27T00:00:01Z", Executable: "codex"},
			{PID: 5555, Command: "zsh", StartedAt: "2026-08-27T00:00:02Z", Executable: "zsh"},
		},
		OrcaTerminals: []string{"term_a"},
	})
	if len(warnings) != 3 || warnings[0] != "4321:codex:2026-08-27T00:00:01Z" || warnings[1] != "5555:zsh:2026-08-27T00:00:02Z" {
		t.Fatalf("occupants must be projected as pid:command:started_at: %v", warnings)
	}
	if !strings.Contains(warnings[2], "프로세스 2개") || !strings.Contains(warnings[2], "Orca 터미널 1개") {
		t.Fatalf("the summary warning must state what apply will stop: %q", warnings[2])
	}
	if quiet := cleanupStatusWarnings(issueopscontract.CleanupFinishResult{}); len(quiet) != 0 {
		t.Fatalf("quiet previews carry no stop warning: %v", quiet)
	}
}
