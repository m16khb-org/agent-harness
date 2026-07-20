package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func acceptedPublishedRemoteCreateRecord(t *testing.T, provider string) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := acceptedPublicationHandoff(t, provider)
	target := publicationTargetForProvider(provider)
	sum := sha256.Sum256([]byte(target))
	record.Phase = IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{
		Provider: provider, ProjectKey: remote.ProjectKey(record.IssueURL, provider, "issue"),
		Remote: "origin", PushTargetSHA256: hex.EncodeToString(sum[:]), Branch: record.Branch,
		Base: record.BranchPrepare.BaseBranch, RemoteRef: "refs/heads/" + record.Branch,
		FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-12T00:00:00Z",
	}
	stored, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, stored
}

func remoteCreateClaimRequest(record IssueOpsRecord) IssueOpsRemoteCreateClaimRequest {
	kind := "pr"
	if record.BranchPrepare.Provider == "gitlab" {
		kind = "mr"
	}
	return IssueOpsRemoteCreateClaimRequest{ID: record.ID, Provider: record.BranchPrepare.Provider, Kind: kind, Title: "title", Body: "body", Head: record.Branch, Base: record.BranchPrepare.BaseBranch, Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true}
}

func TestProtocolV2RemoteCreateRequiresOwnerAndLatestReceipt(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	finalHead := strings.TrimSpace(preflight.GitOut(record.ExecutionHandoff.WorkerRoot, "rev-parse", "refs/heads/"+record.Branch))
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
	lease := handoffDispatchFake(record)
	published, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, IssueOpsHandoffPublishRequest{
		ID: record.ID, Confirm: true, Host: owner.Host, SessionID: owner.SessionID, AgentID: owner.AgentID, CWD: owner.CWD,
	}, reader, lease, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	result, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, published.ID, "github", port.IssueProviderCreatePullRequestRequest{
		Repo: published.Repo, Title: "owner PR", Body: "body", HeadBranch: published.Branch, BaseBranch: published.BranchPrepare.BaseBranch,
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
		Host: owner.Host, SessionID: owner.SessionID, AgentID: owner.AgentID, CWD: owner.CWD,
	}, reader, handoffDispatchFake(published), func(request port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		if request.Repo != published.ExecutionHandoff.WorkerRoot || request.ExpectedHeadSHA != finalHead {
			t.Fatalf("provider request did not retain worker authority: %#v", request)
		}
		return port.IssueProviderCreatePullRequestResult{OK: true, URL: "https://github.com/acme/repo/pull/17"}, nil
	})
	if err != nil || result.URL == "" || providerCalls != 1 {
		t.Fatalf("owner remote create result=%#v err=%v calls=%d", result, err, providerCalls)
	}
}

func TestProtocolV2SourceCannotCreateRemotePR(t *testing.T) {
	stateRoot, record, owner := ownershipActiveRecorderRecord(t)
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	_, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, Title: "source PR", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch,
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
		Host: owner.Host, SessionID: "source-session", AgentID: owner.AgentID, CWD: record.Repo,
	}, &publicationRefFake{}, handoffDispatchFake(record), func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		return port.IssueProviderCreatePullRequestResult{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "ownership transfer") || providerCalls != 0 {
		t.Fatalf("source remote create err=%v calls=%d", err, providerCalls)
	}
}

func TestRemoteCreateClaimAllowsOnlyOneConcurrentCallerAndFinalizes(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan IssueOpsRecord, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			defer wg.Done()
			got, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
			results <- got
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	var claimed IssueOpsRecord
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	for got := range results {
		if got.RemoteCreateClaim != nil {
			claimed = got
		}
	}
	if successes != 1 || claimed.RemoteCreateClaim == nil {
		t.Fatalf("concurrent claim successes=%d claim=%#v", successes, claimed.RemoteCreateClaim)
	}
	if err := MarkIssueOpsRemoteCreateUnknown(context.Background(), stateRoot, claimed, "https://github.com/acme/repo/pull/16"); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record)); err == nil {
		t.Fatal("unknown claim allowed retry")
	}
}

func TestRemoteCreateClaimClearsOnlyPreInvocationAndFinalizesArtifact(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
	if err != nil {
		t.Fatal(err)
	}
	if err := ClearIssueOpsRemoteCreateClaimPreInvocation(context.Background(), stateRoot, claimed, claimed.RemoteCreateClaim.ClaimID, &port.IssueProviderCreateError{Invoked: false, Err: context.Canceled}); err != nil {
		t.Fatal(err)
	}
	claimed, err = ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
	if err != nil {
		t.Fatal(err)
	}
	if err := ClearIssueOpsRemoteCreateClaimPreInvocation(context.Background(), stateRoot, claimed, "claim_wrong", &port.IssueProviderCreateError{Invoked: false, Err: context.Canceled}); err == nil {
		t.Fatal("wrong live claim identity cleared reserved claim")
	}
	final, err := FinalizeIssueOpsRemoteCreateClaim(context.Background(), stateRoot, claimed, IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/16", Labels: []string{"bug"}, Assignees: []string{"octocat"}, TargetBranch: "main"})
	if err != nil || final.RemoteArtifact == nil || final.RemoteCreateClaim != nil {
		t.Fatalf("final=%#v err=%v", final, err)
	}
}

func TestInvokedRemoteCreateInvalidKnownURLStillBecomesDurablyUnknown(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkIssueOpsRemoteCreateUnknown(context.Background(), stateRoot, claimed, "https://github.com/other/repo/pull/16"); err == nil {
		t.Fatal("wrong-project known URL did not report validation failure")
	}
	got, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteCreateClaim == nil || got.RemoteCreateClaim.State != "unknown" || got.RemoteCreateClaim.InvocationState != "unknown" || got.RemoteCreateClaim.KnownURL != "" {
		t.Fatalf("invalid known URL claim = %#v", got.RemoteCreateClaim)
	}
	if err := ClearIssueOpsRemoteCreateClaimPreInvocation(context.Background(), stateRoot, got, got.RemoteCreateClaim.ClaimID, &port.IssueProviderCreateError{Invoked: false, Err: context.Canceled}); err == nil {
		t.Fatal("durable invoked-unknown claim was clearable")
	}
}

func TestMarkRemoteCreateUnknownRejectsDifferentKnownURL(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
	if err != nil {
		t.Fatal(err)
	}
	firstURL := "https://github.com/acme/repo/pull/16"
	if err := MarkIssueOpsRemoteCreateUnknown(context.Background(), stateRoot, claimed, firstURL); err != nil {
		t.Fatal(err)
	}
	unknown, err := ReadIssueOps(stateRoot, claimed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkIssueOpsRemoteCreateUnknown(context.Background(), stateRoot, unknown, "https://github.com/acme/repo/pull/17"); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("different known URL overwrite error = %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, claimed.ID)
	if err != nil || persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.KnownURL != firstURL {
		t.Fatalf("different known URL overwrote durable authority: %#v err=%v", persisted.RemoteCreateClaim, err)
	}
}

func TestRemoteCreateReservedCrashWindowBlocksRetryWithoutClearProof(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.RemoteCreateClaim.InvocationState != "reserved" {
		t.Fatalf("crash-window claim state = %#v", claimed.RemoteCreateClaim)
	}
	if _, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record)); err == nil {
		t.Fatal("reserved crash-window claim allowed duplicate create")
	}
	if err := ClearIssueOpsRemoteCreateClaimPreInvocation(context.Background(), stateRoot, claimed, claimed.RemoteCreateClaim.ClaimID, nil); err == nil {
		t.Fatal("reserved crash-window claim cleared without typed live failure")
	}
}

func TestRemoteCreateClaimRequiresSealedCoordinatorNativeSession(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.ExecutionHandoff.CoordinatorSession = nil
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if _, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record)); err == nil {
		t.Fatal("remote create claim accepted mailbox-only coordinator authority")
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(before, after) {
		t.Fatal("rejected mailbox-only claim mutated durable state")
	}
}

func TestRemoteCreateClaimIsAtomicAcrossOSProcesses(t *testing.T) {
	if stateRoot := os.Getenv("ISSUEOPS_CLAIM_HELPER_STATE"); stateRoot != "" {
		record, err := ReadIssueOps(stateRoot, os.Getenv("ISSUEOPS_CLAIM_HELPER_ID"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
		if err == nil {
			fmt.Print("claimed")
			return
		}
		if err.Error() != "remote create is already claimed or requires reconciliation" {
			t.Fatalf("unexpected claim helper error: %v", err)
		}
		fmt.Print("blocked")
		return
	}
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	commands := make([]*exec.Cmd, 2)
	for i := range commands {
		commands[i] = exec.Command(os.Args[0], "-test.run=^TestRemoteCreateClaimIsAtomicAcrossOSProcesses$")
		commands[i].Env = append(os.Environ(), "ISSUEOPS_CLAIM_HELPER_STATE="+stateRoot, "ISSUEOPS_CLAIM_HELPER_ID="+record.ID)
	}
	outputs := make([]string, 2)
	var wg sync.WaitGroup
	for i := range commands {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			out, err := commands[i].CombinedOutput()
			if err != nil {
				outputs[i] = "error:" + err.Error() + ":" + string(out)
				return
			}
			outputs[i] = string(out)
		}(i)
	}
	wg.Wait()
	claimedCount, blockedCount := 0, 0
	for _, output := range outputs {
		if strings.HasPrefix(output, "claimed") {
			claimedCount++
		}
		if strings.HasPrefix(output, "blocked") {
			blockedCount++
		}
	}
	if claimedCount != 1 || blockedCount != 1 {
		t.Fatalf("two-process claim outcomes = %#v", outputs)
	}
}

func TestRemoteCreateHelperOutcomeRejectsUnexpectedErrors(t *testing.T) {
	got, err := remoteCreateHelperOutcome(errors.New("remote create is already claimed or requires reconciliation"))
	if err != nil || got != "blocked" {
		t.Fatalf("claim conflict outcome = %q, %v", got, err)
	}
	got, err = remoteCreateHelperOutcome(errors.New("publication readback failed"))
	if err == nil || got != "" || !strings.Contains(err.Error(), "publication readback failed") {
		t.Fatalf("unexpected error outcome = %q, %v", got, err)
	}
}

func remoteCreateHelperOutcome(err error) (string, error) {
	if err == nil {
		return "created", nil
	}
	if err.Error() == "remote create is already claimed or requires reconciliation" {
		return "blocked", nil
	}
	if err.Error() == "remote create requires phase pr and no existing artifact" {
		return "blocked", nil
	}
	return "", fmt.Errorf("unexpected remote create helper error: %w", err)
}

func waitForRemoteCreateHelperFiles(paths []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		allPresent := true
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				if !os.IsNotExist(err) {
					return err
				}
				allPresent = false
			}
		}
		if allPresent {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for helper files: %v", paths)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCreateRemotePullRequestTwoProcessesInvokeProviderOnce(t *testing.T) {
	if stateRoot := os.Getenv("ISSUEOPS_CREATE_HELPER_STATE"); stateRoot != "" {
		record, err := ReadIssueOps(stateRoot, os.Getenv("ISSUEOPS_CREATE_HELPER_ID"))
		if err != nil {
			t.Fatal(err)
		}
		finalHead := record.ExecutionHandoff.Result.FinalHead
		reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: "https://github.com/acme/repo.git"}
		lease := handoffDispatchFake(record)
		lease.terminals = nil
		readyPath := os.Getenv("ISSUEOPS_CREATE_HELPER_READY")
		releasePath := os.Getenv("ISSUEOPS_CREATE_HELPER_RELEASE")
		if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := waitForRemoteCreateHelperFiles([]string{releasePath}, 5*time.Second); err != nil {
			t.Fatal(err)
		}
		_, err = CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
			Repo: record.Repo, Title: "PR", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch,
			Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
		}, reader, lease, func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			file, openErr := os.OpenFile(os.Getenv("ISSUEOPS_CREATE_HELPER_COUNT"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if openErr != nil {
				return port.IssueProviderCreatePullRequestResult{}, openErr
			}
			_, writeErr := file.WriteString("x")
			closeErr := file.Close()
			if writeErr != nil {
				return port.IssueProviderCreatePullRequestResult{}, writeErr
			}
			if closeErr != nil {
				return port.IssueProviderCreatePullRequestResult{}, closeErr
			}
			return port.IssueProviderCreatePullRequestResult{URL: "https://github.com/acme/repo/pull/16"}, nil
		})
		outcome, outcomeErr := remoteCreateHelperOutcome(err)
		if outcomeErr != nil {
			t.Fatal(outcomeErr)
		}
		fmt.Print(outcome)
		return
	}
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	countPath := filepath.Join(t.TempDir(), "provider-count")
	barrierDir := t.TempDir()
	releasePath := filepath.Join(barrierDir, "release")
	readyPaths := []string{filepath.Join(barrierDir, "ready-0"), filepath.Join(barrierDir, "ready-1")}
	commands := make([]*exec.Cmd, 2)
	for i := range commands {
		commands[i] = exec.Command(os.Args[0], "-test.run=^TestCreateRemotePullRequestTwoProcessesInvokeProviderOnce$")
		commands[i].Env = append(os.Environ(), "ISSUEOPS_CREATE_HELPER_STATE="+stateRoot, "ISSUEOPS_CREATE_HELPER_ID="+record.ID, "ISSUEOPS_CREATE_HELPER_COUNT="+countPath, "ISSUEOPS_CREATE_HELPER_READY="+readyPaths[i], "ISSUEOPS_CREATE_HELPER_RELEASE="+releasePath)
	}
	outputs := make([]bytes.Buffer, len(commands))
	for i, command := range commands {
		command.Stdout = &outputs[i]
		command.Stderr = &outputs[i]
		if err := command.Start(); err != nil {
			t.Fatalf("start helper %d: %v", i, err)
		}
	}
	if err := waitForRemoteCreateHelperFiles(readyPaths, 5*time.Second); err != nil {
		for _, command := range commands {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
		t.Fatal(err)
	}
	if err := os.WriteFile(releasePath, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	created, blocked := 0, 0
	waitErrors := make([]string, 0, len(commands))
	for i, command := range commands {
		if err := command.Wait(); err != nil {
			waitErrors = append(waitErrors, fmt.Sprintf("helper %d: %v\n%s", i, err, outputs[i].String()))
		}
		created += strings.Count(outputs[i].String(), "created")
		blocked += strings.Count(outputs[i].String(), "blocked")
	}
	if len(waitErrors) != 0 {
		t.Fatalf("wait helpers failed:\n%s", strings.Join(waitErrors, "\n"))
	}
	if created != 1 || blocked != 1 {
		t.Fatalf("two-process create outcomes created=%d blocked=%d outputs=%q", created, blocked, []string{outputs[0].String(), outputs[1].String()})
	}
	count, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(count) != "x" {
		t.Fatalf("two-process provider invocation count bytes = %q", count)
	}
}

func TestRemoteCreateClaimEnvelopeRejectsContradictoryAuthority(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.Phase = IssueOpsPhasePR
	claimed, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err = ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(claimed))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*IssueOpsRecord){
		func(r *IssueOpsRecord) { r.RemoteArtifact = &IssueOpsRemoteArtifactVerification{} },
		func(r *IssueOpsRecord) { r.RemoteCreateClaim.Provider = "gitlab" },
		func(r *IssueOpsRecord) { r.RemoteCreateClaim.Base = "wrong" },
		func(r *IssueOpsRecord) { r.RemoteCreateClaim.FinalHead = strings.Repeat("b", 40) },
		func(r *IssueOpsRecord) { r.ExecutionHandoff.PublishReceipt = nil },
		func(r *IssueOpsRecord) {
			r.RemoteCreateClaim.Labels = make([]string, 129)
			for i := range r.RemoteCreateClaim.Labels {
				r.RemoteCreateClaim.Labels[i] = fmt.Sprintf("label-%d", i)
			}
		},
		func(r *IssueOpsRecord) { r.RemoteCreateClaim.Assignees = []string{strings.Repeat("a", 4097)} },
	} {
		corrupt := claimed
		claimCopy := *claimed.RemoteCreateClaim
		corrupt.RemoteCreateClaim = &claimCopy
		mutate(&corrupt)
		if _, err := WriteIssueOps(t.TempDir(), corrupt); err == nil {
			t.Fatal("contradictory remote create claim envelope was accepted")
		}
	}
}

func TestCreateRemotePullRequestClearsExactClaimWhenPostClaimPublicationRevalidationFails(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{
		localHead:  finalHead,
		remoteHead: finalHead,
		pushTargets: []string{
			"https://github.com/acme/repo.git",
			"https://github.com/acme/other.git",
		},
	}
	lease := handoffDispatchFake(record)
	lease.terminals = nil
	providerCalls := 0
	_, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, Title: "title", Body: "body", HeadBranch: record.Branch,
		BaseBranch: record.BranchPrepare.BaseBranch, Labels: []string{" bug ", "bug"},
		Assignees: []string{" octocat ", "octocat"}, Draft: true, Confirm: true,
	}, reader, lease, func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		return port.IssueProviderCreatePullRequestResult{}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "git push target must match linked issue project authority") {
		t.Fatalf("post-claim publication revalidation error = %v", err)
	}
	if providerCalls != 0 {
		t.Fatalf("provider calls after post-claim validation failure = %d", providerCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.RemoteCreateClaim != nil {
		t.Fatalf("exact reserved claim was not cleared: %#v", persisted.RemoteCreateClaim)
	}
}

func TestCreateRemotePullRequestProviderRequestEqualsCompleteDurableClaimProjection(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: "https://github.com/acme/repo.git"}
	lease := handoffDispatchFake(record)
	lease.terminals = nil
	raw := port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, ProjectKey: "attacker.example/wrong/project", Title: " durable title ", Body: " \n durable body \t",
		HeadBranch: " " + record.Branch + " ", BaseBranch: " " + record.BranchPrepare.BaseBranch + " ",
		Labels: []string{" bug ", "bug", ""}, Assignees: []string{" octocat ", "octocat"},
		Draft: true, ExpectedHeadSHA: strings.Repeat("0", 40), Confirm: true,
	}
	var captured port.IssueProviderCreatePullRequestRequest
	expectedBodySum := sha256.Sum256([]byte("durable body"))
	result, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", raw, reader, lease, func(req port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		captured = req
		claimed, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if claimed.RemoteCreateClaim == nil || claimed.RemoteCreateClaim.Body != "durable body" || claimed.RemoteCreateClaim.BodySHA256 != hex.EncodeToString(expectedBodySum[:]) {
			t.Fatalf("claim body authority = %#v", claimed.RemoteCreateClaim)
		}
		return port.IssueProviderCreatePullRequestResult{URL: "https://github.com/acme/repo/pull/16"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := raw
	want.Title = "durable title"
	want.Body = "durable body"
	want.ProjectKey = "github.com/acme/repo"
	want.HeadBranch = record.Branch
	want.BaseBranch = record.BranchPrepare.BaseBranch
	want.Labels = []string{"bug"}
	want.Assignees = []string{"octocat"}
	want.Draft = true
	want.ExpectedHeadSHA = finalHead
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("provider request = %#v, want complete claim projection %#v", captured, want)
	}
	if result.URL != "https://github.com/acme/repo/pull/16" {
		t.Fatalf("result = %#v", result)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.RemoteArtifact == nil || persisted.RemoteCreateClaim != nil {
		t.Fatalf("ordered cross-root create did not finalize: artifact=%#v claim=%#v", persisted.RemoteArtifact, persisted.RemoteCreateClaim)
	}
}

func TestRemoteCreateReconcileProviderRequestEqualsCompleteDurableClaimProjection(t *testing.T) {
	record := IssueOpsRecord{
		Repo: "/workspace/repo",
		RemoteCreateClaim: &IssueOpsRemoteCreateClaim{
			ProjectKey: "gitlab.example:8443/group/nested/repo", Head: "issue/16", Base: "main",
			FinalHead: strings.Repeat("a", 40), Title: "canonical title", BodySHA256: strings.Repeat("b", 64),
			Labels: []string{"bug", "security"}, Assignees: []string{"alice", "bob"}, Draft: true,
		},
	}
	want := port.IssueProviderReconcilePullRequestRequest{
		Repo: record.Repo, ProjectKey: "gitlab.example:8443/group/nested/repo", HeadBranch: "issue/16", BaseBranch: "main",
		ExpectedHeadSHA: strings.Repeat("a", 40), Title: "canonical title", BodySHA256: strings.Repeat("b", 64),
		Labels: []string{"bug", "security"}, Assignees: []string{"alice", "bob"}, Draft: true,
	}
	got, err := ProjectIssueOpsRemoteCreateClaimForProviderReconcile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("provider reconcile request = %#v, want complete durable claim projection %#v", got, want)
	}
	record.RemoteCreateClaim.Labels[0] = "mutated"
	record.RemoteCreateClaim.Assignees[0] = "mutated"
	if got.Labels[0] != "bug" || got.Assignees[0] != "alice" {
		t.Fatalf("provider reconcile request aliases mutable claim slices: %#v", got)
	}
}

func TestCreateRemotePullRequestBlankTitleFailsBeforeClaimAndProvider(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	providerCalls := 0
	_, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, Title: " \t ", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch,
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
	}, &publicationRefFake{}, handoffDispatchFake(record), func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		providerCalls++
		return port.IssueProviderCreatePullRequestResult{}, nil
	})
	if err == nil || providerCalls != 0 {
		t.Fatalf("blank title error=%v providerCalls=%d", err, providerCalls)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("blank supervised title mutated durable state")
	}
}

func TestCreateRemotePullRequestRejectsSecretLikeTitleAndBodyBeforeClaim(t *testing.T) {
	for _, tc := range []struct {
		name  string
		title string
		body  string
	}{
		{name: "title", title: "api_key=opaque-fixture-title", body: "body"},
		{name: "body", title: "title", body: "body api_key=opaque-fixture-body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			providerCalls := 0
			_, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
				Repo: record.Repo, Title: tc.title, Body: tc.body, HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch,
				Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
			}, &publicationRefFake{}, handoffDispatchFake(record), func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
				providerCalls++
				return port.IssueProviderCreatePullRequestResult{}, nil
			})
			if err == nil || providerCalls != 0 {
				t.Fatalf("secret-like supervised input error=%v providerCalls=%d", err, providerCalls)
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("secret-like supervised input mutated durable state")
			}
			if strings.Contains(string(after), "opaque-fixture") {
				t.Fatal("synthetic opaque marker reached durable state")
			}
		})
	}
}

func TestRemoteCreateClaimEnvelopeRejectsSecretLikeTitleAndBody(t *testing.T) {
	for _, field := range []string{"title", "body"} {
		t.Run(field, func(t *testing.T) {
			stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
			claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
			if err != nil {
				t.Fatal(err)
			}
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if field == "title" {
				claimed.RemoteCreateClaim.Title = "api_key=opaque-fixture-title"
			} else {
				claimed.RemoteCreateClaim.Body = "body api_key=opaque-fixture-body"
				bodySum := sha256.Sum256([]byte(claimed.RemoteCreateClaim.Body))
				claimed.RemoteCreateClaim.BodySHA256 = hex.EncodeToString(bodySum[:])
			}
			if _, err := WriteIssueOps(stateRoot, claimed); err == nil {
				t.Fatal("secret-like remote create claim envelope was accepted")
			}
			after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			if !reflect.DeepEqual(before, after) || strings.Contains(string(after), "opaque-fixture") {
				t.Fatal("rejected secret-like claim changed durable state")
			}
		})
	}
}

func TestRemoteCreateClaimEnvelopeRejectsNonHexAndControlClaimIDs(t *testing.T) {
	for _, claimID := range []string{"claim_gggggggggggggggggggggggggggggggg", "claim_0000000000000000000000000000000\n"} {
		stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
		claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
		if err != nil {
			t.Fatal(err)
		}
		before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
		claimed.RemoteCreateClaim.ClaimID = claimID
		if _, err := WriteIssueOps(stateRoot, claimed); err == nil {
			t.Fatalf("invalid claim id %q accepted", claimID)
		}
		after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("invalid claim id rewrote durable bytes")
		}
	}
}

func TestCreateRemotePullRequestPreservesSecretLikeLegacyInputBytes(t *testing.T) {
	stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
	record.ExecutionHandoff = nil
	record.RemoteCreateClaim = nil
	record, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	want := port.IssueProviderCreatePullRequestRequest{
		Repo: record.Repo, Title: " api_key=opaque-fixture-title ", Body: " body api_key=opaque-fixture-body ",
		HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch, Labels: []string{"bug"}, Assignees: []string{"@me"}, Confirm: true,
	}
	var captured port.IssueProviderCreatePullRequestRequest
	_, err = CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", want, nil, nil, func(got port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
		captured = got
		return port.IssueProviderCreatePullRequestResult{URL: "https://github.com/acme/repo/pull/16"}, nil
	})
	if err != nil || !reflect.DeepEqual(captured, want) {
		t.Fatalf("legacy request=%#v want=%#v err=%v", captured, want, err)
	}
	after := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("legacy secret-like request changed durable state")
	}
}

func TestRemoteCreateReconcileCanonicalSetsIgnoreOrderButNotMembership(t *testing.T) {
	if !sameCanonicalRemoteCreateSet([]string{" security ", "bug"}, []string{"bug", "security"}) {
		t.Fatal("canonical set comparison remained order-sensitive")
	}
	if sameCanonicalRemoteCreateSet([]string{"bug", "security"}, []string{"bug", "other"}) {
		t.Fatal("canonical set comparison ignored membership drift")
	}
}

func TestCreateRemotePullRequestReportsClearAndFinalizeTransitionFailures(t *testing.T) {
	t.Run("pre-invocation clear failure is combined", func(t *testing.T) {
		stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
		finalHead := record.ExecutionHandoff.Result.FinalHead
		reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: publicationTargetForProvider("github")}
		lease := handoffDispatchFake(record)
		lease.terminals = nil
		_, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
			Repo: record.Repo, Title: "title", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch, Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
		}, reader, lease, func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			claimed, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				return port.IssueProviderCreatePullRequestResult{}, readErr
			}
			if markErr := MarkIssueOpsRemoteCreateUnknown(context.Background(), stateRoot, claimed, ""); markErr != nil {
				return port.IssueProviderCreatePullRequestResult{}, markErr
			}
			return port.IssueProviderCreatePullRequestResult{}, &port.IssueProviderCreateError{Invoked: false, Err: context.Canceled}
		})
		if err == nil || !strings.Contains(err.Error(), "durable state transition also failed") {
			t.Fatalf("clear transition error = %v", err)
		}
	})

	t.Run("provider success plus finalize failure keeps reconciliation diagnostic", func(t *testing.T) {
		stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
		finalHead := record.ExecutionHandoff.Result.FinalHead
		reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead, pushTarget: publicationTargetForProvider("github")}
		lease := handoffDispatchFake(record)
		lease.terminals = nil
		reader.remoteHeadHook = func(call int) {
			if call != 2 {
				return
			}
			claimed, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr == nil {
				_ = MarkIssueOpsRemoteCreateUnknown(context.Background(), stateRoot, claimed, "")
			}
		}
		result, err := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
			Repo: record.Repo, Title: "title", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch, Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
		}, reader, lease, func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
			return port.IssueProviderCreatePullRequestResult{URL: "https://github.com/acme/repo/pull/16"}, nil
		})
		if err == nil || result.URL == "" || !strings.Contains(err.Error(), "known URL") || !strings.Contains(err.Error(), "reconciliation") || !strings.Contains(err.Error(), "must not be retried") {
			t.Fatalf("finalize transition result=%#v err=%v", result, err)
		}
		persisted, readErr := ReadIssueOps(stateRoot, record.ID)
		if readErr != nil || persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.KnownURL != result.URL || persisted.RemoteArtifact != nil {
			t.Fatalf("finalize failure durable URL = %#v readErr=%v", persisted, readErr)
		}
	})
}

func TestRemoteCreateReconcileAuthorityZeroOneManyAndAmbiguity(t *testing.T) {
	newClaim := func(t *testing.T) (string, IssueOpsRecord, IssueOpsRemoteCreateCandidate) {
		t.Helper()
		stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
		claimed, err := ClaimIssueOpsRemoteCreate(context.Background(), stateRoot, remoteCreateClaimRequest(record))
		if err != nil {
			t.Fatal(err)
		}
		claim := claimed.RemoteCreateClaim
		candidate := IssueOpsRemoteCreateCandidate{
			URL: "https://github.com/acme/repo/pull/16", Provider: claim.Provider, Kind: claim.Kind,
			ProjectKey: claim.ProjectKey, SourceProjectKey: claim.ProjectKey, Head: claim.Head, Base: claim.Base, FinalHead: claim.FinalHead, Title: claim.Title, BodySHA256: claim.BodySHA256,
			Labels: append([]string(nil), claim.Labels...), Assignees: append([]string(nil), claim.Assignees...), Draft: claim.Draft,
		}
		return stateRoot, claimed, candidate
	}
	request := func(record IssueOpsRecord) IssueOpsRemoteCreateReconcileRequest {
		session := record.ExecutionHandoff.CoordinatorSession
		return IssueOpsRemoteCreateReconcileRequest{ID: record.ID, ClaimID: record.RemoteCreateClaim.ClaimID, CoordinatorRecipient: record.ExecutionHandoff.CoordinatorMailboxHandle, Confirm: true, Host: session.Host, SessionID: session.SessionID, AgentID: session.AgentID, SourceCWD: record.Repo}
	}
	publicationDeps := func(record IssueOpsRecord) (*publicationRefFake, *dispatchOrcaFake) {
		reader := &publicationRefFake{localHead: record.ExecutionHandoff.Result.FinalHead, remoteHead: record.ExecutionHandoff.Result.FinalHead, pushTarget: publicationTargetForProvider(record.BranchPrepare.Provider)}
		lease := handoffDispatchFake(record)
		lease.terminals = nil
		return reader, lease
	}

	t.Run("copied mailbox from worker session and source cwd is rejected", func(t *testing.T) {
		stateRoot, claimed, candidate := newClaim(t)
		reader, lease := publicationDeps(claimed)
		req := request(claimed)
		req.SessionID = "worker-session"
		req.SourceCWD = claimed.ExecutionHandoff.WorkerRoot
		_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, req, reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "native session") {
			t.Fatalf("copied coordinator mailbox authority = %v, want native-session rejection", err)
		}
	})

	t.Run("source project must equal the published project", func(t *testing.T) {
		stateRoot, claimed, candidate := newClaim(t)
		reader, lease := publicationDeps(claimed)
		candidate.SourceProjectKey = "github.com/fork/repo"
		_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "source project") {
			t.Fatalf("fork-origin candidate = %v, want source-project rejection", err)
		}
	})

	t.Run("title and rendered body hash must equal the durable claim", func(t *testing.T) {
		for _, field := range []string{"title", "body"} {
			t.Run(field, func(t *testing.T) {
				stateRoot, claimed, candidate := newClaim(t)
				reader, lease := publicationDeps(claimed)
				if field == "title" {
					candidate.Title = "other"
				} else {
					candidate.BodySHA256 = strings.Repeat("0", 64)
				}
				_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
					return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
				})
				if err == nil {
					t.Fatalf("%s drift was accepted", field)
				}
			})
		}
	})

	t.Run("exactly one verified candidate finalizes", func(t *testing.T) {
		stateRoot, claimed, candidate := newClaim(t)
		reader, lease := publicationDeps(claimed)
		got, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
		})
		if err != nil || got.RemoteArtifact == nil || got.RemoteCreateClaim != nil {
			t.Fatalf("one-candidate reconcile = %#v, %v", got, err)
		}
	})

	t.Run("authoritative zero cannot clear while provider create is in flight", func(t *testing.T) {
		stateRoot, record := acceptedPublishedRemoteCreateRecord(t, "github")
		reader, lease := publicationDeps(record)
		providerStarted := make(chan struct{})
		releaseProvider := make(chan struct{})
		createDone := make(chan error, 1)
		go func() {
			_, createErr := CreateIssueOpsRemotePullRequest(context.Background(), stateRoot, record.ID, "github", port.IssueProviderCreatePullRequestRequest{
				Repo: record.Repo, Title: "title", Body: "body", HeadBranch: record.Branch, BaseBranch: record.BranchPrepare.BaseBranch,
				Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, Confirm: true,
			}, reader, lease, func(port.IssueProviderCreatePullRequestRequest) (port.IssueProviderCreatePullRequestResult, error) {
				close(providerStarted)
				<-releaseProvider
				return port.IssueProviderCreatePullRequestResult{URL: "https://github.com/acme/repo/pull/16"}, nil
			})
			createDone <- createErr
		}()
		<-providerStarted
		claimed, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		req := request(claimed)
		req.ApproveZeroClear = true
		probeStarted := make(chan struct{})
		reconcileDone := make(chan error, 1)
		go func() {
			_, reconcileErr := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, req, reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
				close(probeStarted)
				return IssueOpsRemoteCreateProbeResult{AuthoritativeZero: true}, nil
			})
			reconcileDone <- reconcileErr
		}()
		select {
		case <-probeStarted:
			t.Fatal("reconcile probed and could clear a claim while provider create was in flight")
		case <-time.After(100 * time.Millisecond):
		}
		close(releaseProvider)
		if err := <-createDone; err != nil {
			t.Fatalf("provider create: %v", err)
		}
		if err := <-reconcileDone; err == nil || !strings.Contains(err.Error(), "claim") {
			t.Fatalf("post-create reconcile error = %v, want missing exact claim", err)
		}
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil || persisted.RemoteArtifact == nil || persisted.RemoteCreateClaim != nil {
			t.Fatalf("concurrent reconcile changed successful create state: %#v, %v", persisted, err)
		}
	})

	t.Run("finalize failure preserves candidate URL and no-retry diagnostic", func(t *testing.T) {
		stateRoot, claimed, candidate := newClaim(t)
		reader, lease := publicationDeps(claimed)
		original := finalizeIssueOpsRemoteCreateClaimForReconcile
		finalizeIssueOpsRemoteCreateClaimForReconcile = func(context.Context, string, IssueOpsRecord, IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
			return IssueOpsRecord{}, errors.New("forced reconcile finalize failure")
		}
		defer func() { finalizeIssueOpsRemoteCreateClaimForReconcile = original }()
		_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
		})
		if err == nil || strings.Contains(err.Error(), candidate.URL) || !strings.Contains(err.Error(), "needs reconciliation") || !strings.Contains(err.Error(), "not retried") || len(err.Error()) > 512 {
			t.Fatalf("reconcile finalize diagnostic = %v", err)
		}
		persisted, readErr := ReadIssueOps(stateRoot, claimed.ID)
		if readErr != nil || persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.State != "unknown" || persisted.RemoteCreateClaim.KnownURL != candidate.URL || persisted.RemoteArtifact != nil {
			t.Fatalf("reconcile finalize durable state=%#v readErr=%v", persisted, readErr)
		}
	})

	t.Run("finalize and unknown transition failures are combined", func(t *testing.T) {
		stateRoot, claimed, candidate := newClaim(t)
		reader, lease := publicationDeps(claimed)
		original := finalizeIssueOpsRemoteCreateClaimForReconcile
		finalizeIssueOpsRemoteCreateClaimForReconcile = func(ctx context.Context, root string, expected IssueOpsRecord, _ IssueOpsRemoteArtifactVerificationRequest) (IssueOpsRecord, error) {
			if err := MarkIssueOpsRemoteCreateUnknown(ctx, root, expected, ""); err != nil {
				return IssueOpsRecord{}, err
			}
			return IssueOpsRecord{}, errors.New("forced reconcile finalize failure")
		}
		defer func() { finalizeIssueOpsRemoteCreateClaimForReconcile = original }()
		_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "forced reconcile finalize failure") || !strings.Contains(err.Error(), "durable state transition also failed") {
			t.Fatalf("combined reconcile transition error = %v", err)
		}
	})

	t.Run("provider and transition diagnostics are bounded and redacted", func(t *testing.T) {
		secret := "api_key=opaque-marker"
		oversized := strings.Repeat("x", 128*1024) + secret
		err := combineRemoteCreateTransitionError("remote create failed", errors.New(oversized), errors.New("transition "+oversized))
		if strings.Contains(err.Error(), secret) || len(err.Error()) > 1024 {
			t.Fatalf("remote create combined diagnostic is not bounded/redacted: len=%d", len(err.Error()))
		}
	})

	t.Run("post-probe claim swap cannot adopt stale candidate", func(t *testing.T) {
		stateRoot, claimed, candidate := newClaim(t)
		reader, lease := publicationDeps(claimed)
		newClaimID := "claim_11111111111111111111111111111111"
		reader.remoteHeadHook = func(call int) {
			if call != 1 {
				return
			}
			current, readErr := ReadIssueOps(stateRoot, claimed.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			current.RemoteCreateClaim.ClaimID = newClaimID
			if _, writeErr := WriteIssueOps(stateRoot, current); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
		_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
		})
		if err == nil || !strings.Contains(err.Error(), "claim identity changed") {
			t.Fatalf("post-probe claim swap error = %v", err)
		}
		persisted, readErr := ReadIssueOps(stateRoot, claimed.ID)
		if readErr != nil || persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.ClaimID != newClaimID || persisted.RemoteCreateClaim.KnownURL != "" || persisted.RemoteArtifact != nil {
			t.Fatalf("stale candidate touched replacement claim: %#v readErr=%v", persisted, readErr)
		}
	})

	t.Run("zero requires explicit approval and authoritative proof", func(t *testing.T) {
		stateRoot, claimed, _ := newClaim(t)
		reader, lease := publicationDeps(claimed)
		_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{AuthoritativeZero: true}, nil
		})
		if err == nil {
			t.Fatal("zero candidates cleared without explicit approval")
		}
		persisted, _ := ReadIssueOps(stateRoot, claimed.ID)
		if persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.State != "unknown" {
			t.Fatalf("unapproved zero did not retain unknown: %#v", persisted.RemoteCreateClaim)
		}
		req := request(persisted)
		req.ApproveZeroClear = true
		reader, lease = publicationDeps(persisted)
		got, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, req, reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
			return IssueOpsRemoteCreateProbeResult{AuthoritativeZero: true}, nil
		})
		if err != nil || got.RemoteCreateClaim != nil || got.RemoteArtifact != nil {
			t.Fatalf("approved authoritative zero = %#v, %v", got, err)
		}
	})

	t.Run("multiple and transport ambiguity retain unknown", func(t *testing.T) {
		for _, ambiguity := range []string{"multiple", "transport"} {
			t.Run(ambiguity, func(t *testing.T) {
				stateRoot, claimed, candidate := newClaim(t)
				reader, lease := publicationDeps(claimed)
				_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
					if ambiguity == "transport" {
						return IssueOpsRemoteCreateProbeResult{}, context.DeadlineExceeded
					}
					return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate, candidate}}, nil
				})
				if err == nil {
					t.Fatal("ambiguous reconcile succeeded")
				}
				persisted, _ := ReadIssueOps(stateRoot, claimed.ID)
				if persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.State != "unknown" || persisted.RemoteArtifact != nil {
					t.Fatalf("ambiguity did not retain unknown claim: %#v", persisted)
				}
			})
		}
	})

	for _, drift := range []string{"fingerprint", "force-push"} {
		t.Run(drift+" drift before finalize retains unknown", func(t *testing.T) {
			stateRoot, claimed, candidate := newClaim(t)
			reader, lease := publicationDeps(claimed)
			if drift == "fingerprint" {
				reader.pushTargets = []string{publicationTargetForProvider("github"), "https://github.com/acme/other.git"}
			} else {
				reader.remoteHeads = []string{claimed.RemoteCreateClaim.FinalHead, strings.Repeat("b", 40)}
			}
			probeCalls := 0
			_, err := ReconcileIssueOpsRemoteCreate(context.Background(), stateRoot, request(claimed), reader, lease, func(context.Context, IssueOpsRecord) (IssueOpsRemoteCreateProbeResult, error) {
				probeCalls++
				return IssueOpsRemoteCreateProbeResult{Candidates: []IssueOpsRemoteCreateCandidate{candidate}}, nil
			})
			if err == nil || probeCalls != 1 {
				t.Fatalf("%s drift reconcile error=%v probeCalls=%d", drift, err, probeCalls)
			}
			persisted, readErr := ReadIssueOps(stateRoot, claimed.ID)
			if readErr != nil || persisted.RemoteCreateClaim == nil || persisted.RemoteCreateClaim.State != "unknown" || persisted.RemoteArtifact != nil {
				t.Fatalf("%s drift durable state=%#v readErr=%v", drift, persisted, readErr)
			}
		})
	}
}
