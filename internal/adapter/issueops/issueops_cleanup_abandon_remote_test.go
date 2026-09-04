package issueops

import (
	"context"
	"strings"
	"testing"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/port"
)

// fakeAbandonRemote는 호출 순서를 기록한다. 원격 효과는 되돌릴 수 없으므로
// 어떤 순서로 무엇을 불렀는지가 계약의 일부다.
type fakeAbandonRemote struct {
	calls        []string
	artifactBody port.IssueProviderArtifactBody
	issueBody    port.IssueProviderArtifactBody
	closePR      port.IssueProviderClosePullRequestResult
	closeIssue   port.IssueProviderCloseIssueResult
	closePRErr   error
	readErr      error
}

func (p *fakeAbandonRemote) Name() string { return "github" }
func (p *fakeAbandonRemote) CreateIssue(port.IssueProviderCreateIssueRequest) (port.IssueProviderCreateIssueResult, error) {
	return port.IssueProviderCreateIssueResult{}, nil
}
func (p *fakeAbandonRemote) CreatePullRequest(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
	return port.IssueProviderCreatePullRequestResult{}, nil
}
func (p *fakeAbandonRemote) CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{}, nil
}
func (p *fakeAbandonRemote) CloseChild(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	return port.IssueProviderCloseChildResult{}, nil
}
func (p *fakeAbandonRemote) UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{}, nil
}
func (p *fakeAbandonRemote) CloseIssue(req port.IssueProviderCloseIssueRequest) (port.IssueProviderCloseIssueResult, error) {
	p.calls = append(p.calls, "close_issue:"+req.Reason)
	return p.closeIssue, nil
}
func (p *fakeAbandonRemote) ClosePullRequest(port.IssueProviderClosePullRequestRequest) (port.IssueProviderClosePullRequestResult, error) {
	p.calls = append(p.calls, "close_pr")
	return p.closePR, p.closePRErr
}
func (p *fakeAbandonRemote) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	p.calls = append(p.calls, "read:"+req.Kind)
	if p.readErr != nil {
		return port.IssueProviderArtifactBody{}, p.readErr
	}
	if req.Kind == "issue" {
		return p.issueBody, nil
	}
	return p.artifactBody, nil
}

// remoteAbandonGit은 원격 브랜치 관측과 삭제를 기록한다.
type remoteAbandonGit struct {
	branchOID    string
	remoteOID    string
	commands     []string
	deleteFailed bool
}

func (g *remoteAbandonGit) run(_ string, args ...string) (int, string) {
	joined := strings.Join(args, " ")
	g.commands = append(g.commands, joined)
	switch {
	case len(args) > 0 && args[0] == "rev-parse":
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	case len(args) > 0 && args[0] == "ls-remote":
		if g.remoteOID == "" {
			return 0, ""
		}
		return 0, g.remoteOID + "\trefs/heads/106-abandon"
	case len(args) > 0 && args[0] == "push":
		if g.deleteFailed {
			return 1, "remote rejected"
		}
		return 0, ""
	}
	return 0, ""
}

func remoteAbandonRecord(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot, record := abandonTestRecord(t)
	record.IssueURL = "https://github.com/acme/repo/issues/106"
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{
		Provider: "github", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main",
	}
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
		Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/107",
	}
	if err := withIssueOpsLock(context.Background(), stateRoot, record.ID, func(context.Context) error {
		_, e := writeIssueOps(stateRoot, record)
		return e
	}); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record
}

func remoteAbandonRequest(id string, apply bool, fingerprint string) CleanupAbandonRequest {
	req := abandonRequest(id, apply, fingerprint)
	req.ArtifactUnmerged = true
	req.ClosePR, req.CloseIssue, req.DeleteRemoteBranch = true, true, true
	return req
}

func remoteAbandonDeps(git *remoteAbandonGit, remote port.IssueProvider) CleanupAbandonDeps {
	return CleanupAbandonDeps{
		Processes: quietCleanupProcesses(), Git: git.run, Orca: authoritativeZeroOrca(), Remote: remote,
	}
}

// 플래그가 없으면 원격을 전혀 조회하지 않는다. 이 성질이 깨지면 원격 정체가
// 없는 사이클의 폐기가 provider 오류로 막힌다.
func TestCleanupAbandonRemoteUntouchedWithoutFlags(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{}
	git := &remoteAbandonGit{}
	plain := abandonRequest(record.ID, false, "")
	plain.ArtifactUnmerged = true

	result, err := CleanupAbandon(context.Background(), stateRoot, plain, remoteAbandonDeps(git, remote))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(remote.calls) != 0 {
		t.Fatalf("no flag must mean no provider call, got %v", remote.calls)
	}
	if len(result.RemoteEffects) != 0 {
		t.Fatalf("no flag must plan no remote effect, got %v", result.RemoteEffects)
	}
	for _, command := range git.commands {
		if strings.HasPrefix(command, "ls-remote") {
			t.Fatalf("no flag must not observe the remote branch, ran %v", git.commands)
		}
	}
}

// 플래그가 다른 preview는 다른 승인이다. fingerprint가 같으면 한쪽 승인으로
// 다른 쪽을 실행할 수 있다.
func TestCleanupAbandonRemoteFlagsChangeTheFingerprint(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{
		artifactBody: port.IssueProviderArtifactBody{State: "OPEN"},
		issueBody:    port.IssueProviderArtifactBody{State: "OPEN"},
	}
	plain := abandonRequest(record.ID, false, "")
	plain.ArtifactUnmerged = true

	bare, err := CleanupAbandon(context.Background(), stateRoot, plain, remoteAbandonDeps(&remoteAbandonGit{}, remote))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	withPR := abandonRequest(record.ID, false, "")
	withPR.ArtifactUnmerged, withPR.ClosePR = true, true
	flagged, err := CleanupAbandon(context.Background(), stateRoot, withPR, remoteAbandonDeps(&remoteAbandonGit{}, remote))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bare.Fingerprint == flagged.Fingerprint {
		t.Fatalf("a remote effect must change the approval fingerprint: %s", bare.Fingerprint)
	}
	if len(flagged.RemoteEffects) != 1 || flagged.RemoteEffects[0] != "close_pr" {
		t.Fatalf("preview must plan close_pr, got %v", flagged.RemoteEffects)
	}
	if !strings.Contains(flagged.NextCommand, "--close-pr") {
		t.Fatalf("the apply command must carry the same flag: %q", flagged.NextCommand)
	}
}

// apply는 원격을 먼저, 순서대로 건드린 뒤 로컬을 지운다. 레코드가 사라진
// 뒤에는 --id 기반 원격 정리 경로가 없기 때문이다.
func TestCleanupAbandonRemoteAppliesEffectsBeforeLocalDeletion(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{
		artifactBody: port.IssueProviderArtifactBody{State: "OPEN"},
		issueBody:    port.IssueProviderArtifactBody{State: "OPEN"},
		closePR:      port.IssueProviderClosePullRequestResult{OK: true, Closed: true, State: "CLOSED"},
		closeIssue:   port.IssueProviderCloseIssueResult{OK: true, Closed: true, State: "CLOSED"},
	}
	git := &remoteAbandonGit{remoteOID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	preview, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, false, ""), remoteAbandonDeps(git, remote))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.RemoteEffects) != 3 {
		t.Fatalf("preview must plan three effects, got %v", preview.RemoteEffects)
	}

	// apply는 mutation 직전에 다시 관측한다. 그 재관측까지 포함한 순서를 보려고
	// preview가 남긴 호출 기록을 비운다.
	remote.calls = nil
	applied, err := CleanupAbandon(context.Background(), stateRoot,
		remoteAbandonRequest(record.ID, true, preview.Fingerprint), remoteAbandonDeps(git, remote))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !applied.RecordDeleted || !applied.PRClosed || !applied.IssueClosed || !applied.RemoteBranchDeleted {
		t.Fatalf("apply must complete every requested effect: %+v", applied)
	}
	wantCalls := []string{"read:pr", "read:issue", "close_pr", "close_issue:not_planned"}
	if strings.Join(remote.calls, ",") != strings.Join(wantCalls, ",") {
		t.Fatalf("provider call order = %v, want %v", remote.calls, wantCalls)
	}
	pushIndex := -1
	for index, command := range git.commands {
		if strings.HasPrefix(command, "push origin --delete") {
			pushIndex = index
			if !strings.Contains(command, "--force-with-lease=refs/heads/106-abandon:"+git.remoteOID) {
				t.Fatalf("remote branch deletion must use a lease against the observed OID: %q", command)
			}
		}
	}
	if pushIndex < 0 {
		t.Fatalf("remote branch deletion never ran: %v", git.commands)
	}
}

// 이미 닫힌 아티팩트는 다시 닫지 않고 넘어간다. 폐기가 부분 실패 뒤 재실행돼도
// 같은 결과여야 한다.
func TestCleanupAbandonRemoteSkipsAlreadyClosedArtifacts(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{
		artifactBody: port.IssueProviderArtifactBody{State: "CLOSED"},
		issueBody:    port.IssueProviderArtifactBody{State: "CLOSED"},
	}
	result, err := CleanupAbandon(context.Background(), stateRoot,
		func() CleanupAbandonRequest {
			req := remoteAbandonRequest(record.ID, false, "")
			req.DeleteRemoteBranch = false
			return req
		}(), remoteAbandonDeps(&remoteAbandonGit{}, remote))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "close_pr:already_closed,close_issue:already_closed"
	if strings.Join(result.RemoteEffects, ",") != want {
		t.Fatalf("effects = %v, want %s", result.RemoteEffects, want)
	}
}

// 머지된 아티팩트는 플래그와 무관하게 막힌다. 그 사이클의 정답은
// reflect-completion → cleanup finish다.
func TestCleanupAbandonRemoteRefusesAMergedArtifact(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{artifactBody: port.IssueProviderArtifactBody{State: "MERGED"}}
	req := remoteAbandonRequest(record.ID, false, "")
	req.CloseIssue, req.DeleteRemoteBranch = false, false

	result, err := CleanupAbandon(context.Background(), stateRoot, req, remoteAbandonDeps(&remoteAbandonGit{}, remote))
	if err == nil {
		t.Fatal("a merged artifact must not be abandonable")
	}
	if !containsString(result.Missing, "remote_artifact_unmerged") {
		t.Fatalf("missing = %v, want remote_artifact_unmerged", result.Missing)
	}
	if len(result.RemoteEffects) != 0 {
		t.Fatalf("a blocked preview must not advertise effects: %v", result.RemoteEffects)
	}
}

// 원격 관측이 실패하면 무엇을 지울지 설명할 수 없다. 닫힌 채로 둔다.
func TestCleanupAbandonRemoteFailsClosedOnUnreadableRemote(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{readErr: context.DeadlineExceeded}
	req := remoteAbandonRequest(record.ID, false, "")
	req.DeleteRemoteBranch = false

	result, err := CleanupAbandon(context.Background(), stateRoot, req, remoteAbandonDeps(&remoteAbandonGit{}, remote))
	if err == nil {
		t.Fatal("an unreadable remote must block the preview")
	}
	for _, key := range []string{"remote_artifact_readable", "issue_readable"} {
		if !containsString(result.Missing, key) {
			t.Fatalf("missing = %v, want %s", result.Missing, key)
		}
	}
}

// 원격 브랜치 삭제가 실패하면 레코드도 워크트리도 남는다. 사람이 원격 상태를
// 보고 다시 결정할 수 있어야 한다.
func TestCleanupAbandonRemoteBranchDeleteFailurePreservesTheRecord(t *testing.T) {
	stateRoot, record := remoteAbandonRecord(t)
	remote := &fakeAbandonRemote{
		artifactBody: port.IssueProviderArtifactBody{State: "OPEN"},
		issueBody:    port.IssueProviderArtifactBody{State: "OPEN"},
		closePR:      port.IssueProviderClosePullRequestResult{OK: true, Closed: true, State: "CLOSED"},
		closeIssue:   port.IssueProviderCloseIssueResult{OK: true, Closed: true, State: "CLOSED"},
	}
	git := &remoteAbandonGit{remoteOID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", deleteFailed: true}
	preview, err := CleanupAbandon(context.Background(), stateRoot, remoteAbandonRequest(record.ID, false, ""), remoteAbandonDeps(git, remote))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot,
		remoteAbandonRequest(record.ID, true, preview.Fingerprint), remoteAbandonDeps(git, remote))
	if err == nil {
		t.Fatal("a failed remote branch deletion must stop the abandon")
	}
	if applied.FailedStep != issueops.CleanupFailureStepRemoteBranchDelete {
		t.Fatalf("failed step = %q, want remote_branch_delete", applied.FailedStep)
	}
	if applied.RecordDeleted {
		t.Fatal("the record must survive a remote failure")
	}
	kept, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatalf("record must still be readable: %v", readErr)
	}
	if kept.CleanupAbandonFailure == nil || kept.CleanupAbandonFailure.Step != issueops.CleanupFailureStepRemoteBranchDelete {
		t.Fatalf("failure receipt = %+v, want a remote_branch_delete receipt", kept.CleanupAbandonFailure)
	}
	if !strings.Contains(strings.Join(applied.RemoteEffects, ","), "close_pr") {
		t.Fatalf("effects applied before the failure must be reported: %v", applied.RemoteEffects)
	}
}
