package issueops

import (
	"sync"
	"testing"
)

func TestRemoteCreateClaimAllowsOnlyOneConcurrentCallerAndFinalizes(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	finalHead := record.ExecutionHandoff.Result.FinalHead
	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan IssueOpsRecord, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			defer wg.Done()
			got, err := ClaimIssueOpsRemoteCreate(stateRoot, record.ID, "github", record.Branch, "main", finalHead)
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
	if err := MarkIssueOpsRemoteCreateUnknown(stateRoot, claimed); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimIssueOpsRemoteCreate(stateRoot, record.ID, "github", record.Branch, "main", finalHead); err == nil {
		t.Fatal("unknown claim allowed retry")
	}
}

func TestRemoteCreateClaimClearsOnlyPreInvocationAndFinalizesArtifact(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	record.Phase = IssueOpsPhasePR
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	claimed, err := ClaimIssueOpsRemoteCreate(stateRoot, record.ID, "github", record.Branch, "main", record.ExecutionHandoff.Result.FinalHead)
	if err != nil {
		t.Fatal(err)
	}
	if err := ClearIssueOpsRemoteCreateClaim(stateRoot, claimed); err != nil {
		t.Fatal(err)
	}
	claimed, err = ClaimIssueOpsRemoteCreate(stateRoot, record.ID, "github", record.Branch, "main", record.ExecutionHandoff.Result.FinalHead)
	if err != nil {
		t.Fatal(err)
	}
	final, err := FinalizeIssueOpsRemoteCreateClaim(stateRoot, claimed, IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/16", Labels: []string{"bug"}, Assignees: []string{"octocat"}, TargetBranch: "main"})
	if err != nil || final.RemoteArtifact == nil || final.RemoteCreateClaim != nil {
		t.Fatalf("final=%#v err=%v", final, err)
	}
}
