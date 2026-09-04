package feedbackcleanup

import (
	"context"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// --keep-remote-branch는 게이트를 여는 선택이므로 어댑터까지 그대로 도착해야
// 하고, 붙이지 않은 실행에서는 절대 켜지면 안 된다. 이 플래그가 조용히 켜지면
// 파괴는 없지만 원격 브랜치가 추적 없이 남는다.
func TestRunCleanupFinishPropagatesKeepRemoteBranchExactly(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)
	provider := &cleanupStatusProvider{snapshot: port.ExecutionIssueSnapshot{
		URL: record.IssueURL, Body: port.IssueBodyCompletionStartMarker, State: "closed",
	}}
	deps := cleanupStatusDeps(nil)
	deps.Provider = func(string) (port.IssueProvider, error) { return provider, nil }
	deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscontract.CleanupRemoteBranchArtifactHead, error) {
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

	if err := RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--keep-remote-branch", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	if !captured.KeepRemoteBranch {
		t.Fatalf("원격 브랜치를 남기겠다는 선택이 어댑터까지 전달되지 않았다: %+v", captured)
	}

	captured = issueopscontract.CleanupFinishRequest{}
	if err := RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	if captured.KeepRemoteBranch {
		t.Fatalf("플래그 없는 실행은 기본값 fail-closed를 유지해야 한다: %+v", captured)
	}
}
