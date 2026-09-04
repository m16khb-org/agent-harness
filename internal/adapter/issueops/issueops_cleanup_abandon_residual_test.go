package issueops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/contract/issueops"
)

func TestCleanupAbandonPreviewInventoriesRecordBackedWorktree(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-abandon")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"))

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err != nil {
		t.Fatalf("clean record-backed residue must be previewable: %v missing=%v", err, result.Missing)
	}
	if !result.Preview || result.Fingerprint == "" || !result.WorktreePresent || !result.BranchPresent {
		t.Fatalf("preview must seal both local targets without mutation: %+v", result)
	}
	if !result.WorktreeCanonical || !result.WorktreeClean || result.WorktreeHead != head || result.BranchOID != head {
		t.Fatalf("preview inventory mismatch: %+v", result)
	}
	wantPlan := []string{
		"local_worktree:" + fixture.worktree,
		"local_branch:293-cleanup-abandon@" + head,
		"record:" + fixture.record.ID,
	}
	if strings.Join(result.RemovalPlan, "\n") != strings.Join(wantPlan, "\n") {
		t.Fatalf("removal plan mismatch\nwant=%q\n got=%q", wantPlan, result.RemovalPlan)
	}
	if result.RemoteBranchDeletion != "not_planned" {
		t.Fatalf("remote branch deletion must stay out of scope: %+v", result)
	}
	if got := strings.TrimSpace(preflight.GitOut(fixture.worktree, "rev-parse", "HEAD")); got != head {
		t.Fatalf("preview mutated worktree HEAD: want=%s got=%s", head, got)
	}
	if got := strings.TrimSpace(preflight.GitOut(fixture.record.Repo, "rev-parse", "refs/heads/293-cleanup-abandon")); got != head {
		t.Fatalf("preview mutated local branch: want=%s got=%s", head, got)
	}
}

func TestCleanupAbandonApplyRemovesLocalTargetsBeforeRecord(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-apply")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}

	remoteCalls := 0
	git := func(dir string, args ...string) (int, string) {
		if len(args) > 0 && (args[0] == "fetch" || args[0] == "push" || args[0] == "ls-remote" || args[0] == "remote") {
			remoteCalls++
		}
		code, stdout, stderr := preflight.GitCmd(dir, args...)
		if code != 0 {
			return code, stderr
		}
		return code, stdout
	}
	deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.RecordDeleted {
		t.Fatalf("apply must delete the record after local cleanup: %+v", applied)
	}
	if _, err := os.Stat(fixture.worktree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("apply must remove the local worktree first: %v", err)
	}
	if code, _, _ := preflight.GitCmd(fixture.record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/293-cleanup-apply"); code == 0 {
		t.Fatal("apply must remove the exact local branch")
	}
	if _, err := ReadIssueOps(stateRoot, fixture.record.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("apply must remove the lifecycle record last: %v", err)
	}
	if remoteCalls != 0 || applied.RemoteBranchDeletion != "not_planned" {
		t.Fatalf("cleanup abandon must not inspect or mutate a remote branch: calls=%d result=%+v", remoteCalls, applied)
	}
}

func TestCleanupAbandonBranchFailurePreservesRetryInventory(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-partial")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"))
	git := func(dir string, args ...string) (int, string) {
		if len(args) > 0 && args[0] == "update-ref" {
			return 1, "injected branch delete failure"
		}
		code, stdout, stderr := preflight.GitCmd(dir, args...)
		if code != 0 {
			return code, stderr
		}
		return code, stdout
	}
	deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git}

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps)
	if err == nil || result.FailedStep != "branch_delete" {
		t.Fatalf("branch failure must be surfaced after worktree removal: err=%v result=%+v", err, result)
	}
	kept, readErr := ReadIssueOps(stateRoot, fixture.record.ID)
	if readErr != nil {
		t.Fatalf("record must survive partial local cleanup: %v", readErr)
	}
	failure := kept.CleanupAbandonFailure
	if failure == nil || failure.Step != "branch_delete" || failure.Fingerprint != preview.Fingerprint ||
		failure.RecordSHA == "" || failure.InventorySHA256 == "" ||
		failure.WorktreePath != fixture.worktree || failure.Branch != "293-cleanup-partial" ||
		failure.WorktreeHead != head || failure.BranchOID != head {
		t.Fatalf("retry inventory must seal the original local targets: %+v", failure)
	}
	if _, statErr := os.Stat(fixture.worktree); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("worktree removal should remain observable after branch failure: %v", statErr)
	}
	if code, _, _ := preflight.GitCmd(fixture.record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/293-cleanup-partial"); code != 0 {
		t.Fatal("failed branch deletion must leave the local branch present")
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(record *issueops.IssueOpsRecord) {
		record.CleanupAbandonFailure.Fingerprint = strings.Repeat("b", 64)
	})
	forged, forgedErr := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if forgedErr == nil || !containsString(forged.Missing, "cleanup_failure_inventory") {
		t.Fatalf("a substituted receipt fingerprint must fail closed: err=%v result=%+v", forgedErr, forged)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(record *issueops.IssueOpsRecord) {
		record.CleanupAbandonFailure.Fingerprint = preview.Fingerprint
	})
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(record *issueops.IssueOpsRecord) {
		record.CleanupAbandonFailure.BranchOID = "different-oid"
	})
	// receipt의 OID가 관측과 어긋나면 계속 막는다. 예전에는 이것이
	// local_residue_pair로 보고됐지만, #433이 비대칭 자체를 허용하면서
	// 이제 cleanup_failure_inventory가 잡는다 — receipt와 관측이 불일치한다는
	// 더 정확한 진단이다. 막는다는 사실은 그대로다.
	mismatch, mismatchErr := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if mismatchErr == nil || !containsString(mismatch.Missing, "cleanup_failure_inventory") {
		t.Fatalf("branch-only retry must match the sealed OID: err=%v result=%+v", mismatchErr, mismatch)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(record *issueops.IssueOpsRecord) {
		record.CleanupAbandonFailure.BranchOID = head
	})

	retryPreview, retryErr := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if retryErr != nil {
		t.Fatalf("sealed branch-only partial state must be retryable: %v missing=%v", retryErr, retryPreview.Missing)
	}
	wantRetryPlan := []string{
		"local_branch:293-cleanup-partial@" + head,
		"record:" + fixture.record.ID,
	}
	if strings.Join(retryPreview.RemovalPlan, "\n") != strings.Join(wantRetryPlan, "\n") {
		t.Fatalf("partial retry plan mismatch: want=%q got=%q", wantRetryPlan, retryPreview.RemovalPlan)
	}
	retried, retryErr := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, retryPreview.Fingerprint), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if retryErr != nil || !retried.BranchDeleted || !retried.RecordDeleted {
		t.Fatalf("partial retry must finish branch and record cleanup: err=%v result=%+v", retryErr, retried)
	}
}

func TestCleanupAbandonApplyingReceiptForOriginallyAbsentExecutionIsRetryable(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-originally-absent")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"))
	originalRecordSHA := cleanupAbandonRecordSHA(fixture.record)
	if code, _, stderr := preflight.GitCmd(fixture.record.Repo, "worktree", "remove", fixture.worktree); code != 0 {
		t.Fatalf("remove absent fixture worktree: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(fixture.record.Repo, "update-ref", "-d", "refs/heads/293-cleanup-originally-absent", head); code != 0 {
		t.Fatalf("remove absent fixture branch: %s", stderr)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(current *issueops.IssueOpsRecord) {
		current.CleanupAbandonFailure = &issueops.IssueOpsCleanupAbandonFailure{
			Step: "applying", Fingerprint: strings.Repeat("a", 64),
			RecordSHA:    originalRecordSHA,
			WorktreePath: fixture.worktree, Branch: current.Branch,
			At: "2026-08-04T00:00:00Z",
		}
		current.CleanupAbandonFailure.InventorySHA256 = cleanupAbandonFailureSeal(*current, current.CleanupAbandonFailure)
	})

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err != nil {
		t.Fatalf("an armed originally-absent execution must be retryable: %v missing=%v", err, preview.Missing)
	}
	if strings.Join(preview.RemovalPlan, "\n") != "record:"+fixture.record.ID {
		t.Fatalf("originally-absent retry must only remove the record: %+v", preview.RemovalPlan)
	}
}

func TestSwitchExecutionModeRejectsCleanupAbandonFence(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	if code, _, stderr := preflight.GitCmd(record.Repo, "update-ref", "-d", "refs/remotes/origin/"+record.Branch); code != 0 {
		t.Fatalf("remove remote branch fixture: %s", stderr)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(current *issueops.IssueOpsRecord) {
		current.Execution = issueOpsExecutionForTest(current.Repo, filepath.Join(t.TempDir(), "absent-worktree"), current.Branch)
		current.Execution.Lease = issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased}
		current.CleanupAbandonFailure = &issueops.IssueOpsCleanupAbandonFailure{
			Step: "applying", Fingerprint: strings.Repeat("a", 64), At: "2026-08-04T00:00:00Z",
		}
	})

	preview, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: string(issueops.ExecutionModeOrca),
	}, ExecutionSwitchModeDependencies{})
	if err != nil {
		t.Fatalf("switch-mode preview: %v", err)
	}
	result, err := SwitchExecutionMode(context.Background(), stateRoot, ExecutionSwitchModeRequest{
		ID: record.ID, Mode: string(issueops.ExecutionModeOrca), Apply: true, Confirm: true, Fingerprint: preview.Fingerprint,
	}, ExecutionSwitchModeDependencies{})
	if err == nil || !strings.Contains(err.Error(), "cleanup abandon apply is in progress") {
		t.Fatalf("switch-mode must not erase an applying cleanup fence: err=%v result=%+v", err, result)
	}
	kept, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || kept.Execution == nil || kept.CleanupAbandonFailure == nil {
		t.Fatalf("rejected switch-mode must preserve the fenced record: err=%v record=%+v", readErr, kept)
	}
}

func TestCleanupAbandonRecordDeletePartialStateIsRetryable(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-record-partial")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"))
	originalRecordSHA := cleanupAbandonRecordSHA(fixture.record)
	if code, _, stderr := preflight.GitCmd(fixture.record.Repo, "worktree", "remove", fixture.worktree); code != 0 {
		t.Fatalf("remove partial worktree: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(fixture.record.Repo, "update-ref", "-d", "refs/heads/293-cleanup-record-partial", head); code != 0 {
		t.Fatalf("remove partial branch: %s", stderr)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(current *issueops.IssueOpsRecord) {
		current.CleanupAbandonFailure = &issueops.IssueOpsCleanupAbandonFailure{
			Step: "record_delete", Message: "injected record delete failure",
			Fingerprint: strings.Repeat("a", 64), Branch: current.Branch,
			At: "2026-08-04T00:00:00Z",
		}
	})

	invalid, invalidErr := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if invalidErr == nil || !containsString(invalid.Missing, "cleanup_failure_inventory") {
		t.Fatalf("incomplete record-delete inventory must fail closed: err=%v result=%+v", invalidErr, invalid)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(current *issueops.IssueOpsRecord) {
		current.CleanupAbandonFailure.WorktreePath = fixture.worktree
		current.CleanupAbandonFailure.Branch = "293-cleanup-record-partial"
		current.CleanupAbandonFailure.WorktreeHead = head
		current.CleanupAbandonFailure.BranchOID = head
		current.CleanupAbandonFailure.RecordSHA = originalRecordSHA
		current.CleanupAbandonFailure.InventorySHA256 = cleanupAbandonFailureSeal(*current, current.CleanupAbandonFailure)
	})
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err != nil {
		t.Fatalf("both-absent record-delete partial state must be retryable: %v missing=%v", err, preview.Missing)
	}
	wantPlan := []string{"record:" + fixture.record.ID}
	if strings.Join(preview.RemovalPlan, "\n") != strings.Join(wantPlan, "\n") {
		t.Fatalf("record-only retry plan mismatch: want=%q got=%q", wantPlan, preview.RemovalPlan)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err != nil || !applied.RecordDeleted || applied.WorktreeRemoved || applied.BranchDeleted {
		t.Fatalf("record-only retry must finish without local Git deletion: err=%v result=%+v", err, applied)
	}
}

func TestCleanupAbandonPersistsFenceBeforeGitMutation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-fence")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	fenceObserved := false
	git := func(dir string, args ...string) (int, string) {
		if !fenceObserved && len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
			fenceObserved = true
			current, err := ReadIssueOps(stateRoot, fixture.record.ID)
			if err != nil {
				t.Fatalf("read pre-mutation fence: %v", err)
			}
			if current.CleanupAbandonFailure == nil || current.CleanupAbandonFailure.Step != "applying" {
				t.Fatalf("cleanup attempt must be durable before Git mutation: %+v", current.CleanupAbandonFailure)
			}
			if current.Execution.Lease.Status != issueops.LeaseStatusReleased || current.Execution.Lease.ClaimTokenSHA256 != "" {
				t.Fatalf("pre-mutation fence must make the lease unclaimable: %+v", current.Execution.Lease)
			}
			if err := withIssueOpsLock(context.Background(), stateRoot, fixture.record.ID, func(context.Context) error {
				t.Fatal("ordinary lifecycle writer entered an armed cleanup span")
				return nil
			}); err == nil || !strings.Contains(err.Error(), "cleanup abandon apply is in progress") {
				t.Fatalf("ordinary lifecycle writer must observe the cleanup fence: %v", err)
			}
		}
		code, stdout, stderr := preflight.GitCmd(dir, args...)
		if code != 0 {
			return code, stderr
		}
		return code, stdout
	}
	deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps); err != nil {
		t.Fatal(err)
	}
	if !fenceObserved {
		t.Fatal("expected worktree mutation was not observed")
	}
}

func TestCleanupAbandonAuthorityCASRunsBeforeGitMutation(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-cas")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	armed := false
	mutated := false
	destructiveCalls := 0
	git := func(dir string, args ...string) (int, string) {
		if (len(args) > 1 && args[0] == "worktree" && args[1] == "remove") || (len(args) > 0 && args[0] == "update-ref") {
			destructiveCalls++
		}
		if armed && !mutated && len(args) > 1 && args[0] == "rev-parse" && args[1] == "--verify" {
			mutated = true
			mutateFinishRecord(t, stateRoot, fixture.record.ID, func(record *issueops.IssueOpsRecord) {
				record.Phase = IssueOpsPhasePlan
			})
		}
		code, stdout, stderr := preflight.GitCmd(dir, args...)
		if code != 0 {
			return code, stderr
		}
		return code, stdout
	}
	deps := CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	armed = true
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps)
	if err == nil || !strings.Contains(err.Error(), "authority changed before local cleanup CAS") {
		t.Fatalf("concurrent lifecycle mutation must fail the pre-Git CAS: err=%v result=%+v", err, result)
	}
	if destructiveCalls != 0 {
		t.Fatalf("authority drift must cause zero Git mutations, got %d", destructiveCalls)
	}
	if _, err := os.Stat(fixture.worktree); err != nil {
		t.Fatalf("worktree must survive authority drift: %v", err)
	}
	if _, err := ReadIssueOps(stateRoot, fixture.record.ID); err != nil {
		t.Fatalf("record must survive authority drift: %v", err)
	}
}

func TestCleanupAbandonRejectsBranchCheckedOutInAnotherWorktree(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-elsewhere")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(fixture.record.Repo, "worktree", "remove", fixture.worktree); code != 0 {
		t.Fatalf("remove original test worktree: %s", stderr)
	}
	other := issueOpsWorktreePathForTest(fixture.record.Repo, "other-293-cleanup-elsewhere")
	if code, _, stderr := preflight.GitCmd(fixture.record.Repo, "worktree", "add", "-q", other, "293-cleanup-elsewhere"); code != 0 {
		t.Fatalf("check branch out elsewhere: %s", stderr)
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err == nil || !containsString(result.Missing, "branch_checked_out_elsewhere") {
		t.Fatalf("another worktree owning the branch must fail closed: err=%v missing=%v", err, result.Missing)
	}
}

func TestCleanupAbandonRejectsDirtyWorktree(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-dirty")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.worktree, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err == nil || !containsString(result.Missing, "worktree_clean") || result.Fingerprint != "" {
		t.Fatalf("dirty worktree must not receive an applicable fingerprint: err=%v result=%+v", err, result)
	}
	if _, err := os.Stat(fixture.worktree); err != nil {
		t.Fatalf("dirty worktree must survive preview: %v", err)
	}
}

func TestCleanupAbandonApplyRejectsCleanHeadDrift(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "293-cleanup-head-drift")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.worktree, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(fixture.worktree, "add", "README.md"); code != 0 {
		t.Fatalf("git add drift: %s", stderr)
	}
	if code, _, stderr := preflight.GitCmd(fixture.worktree, "commit", "-q", "-m", "drift"); code != 0 {
		t.Fatalf("git commit drift: %s", stderr)
	}

	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), CleanupAbandonDeps{Processes: quietCleanupProcesses()})
	if err == nil || !strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("clean HEAD/OID drift must invalidate the preview: err=%v result=%+v", err, result)
	}
	if _, err := os.Stat(fixture.worktree); err != nil {
		t.Fatalf("drifted worktree must survive stale apply: %v", err)
	}
	if _, err := ReadIssueOps(stateRoot, fixture.record.ID); err != nil {
		t.Fatalf("record must survive stale apply: %v", err)
	}
}

// 실측(io-2fb20438b925, 2026-08-26): execution prepare 없이 진행된 사이클이 MR
// 머지·이슈 종료까지 끝난 뒤 link된 워크트리·브랜치가 남아 있으면 세 typed
// 경로가 모두 막혔다 — finish는 기록된 artifact를, orphan은 record 부재를,
// abandon은 execution 소유 근거(local_residue_execution)를 요구했다. record가
// link한 워크트리(record.WorktreePath)는 격리 경로·브랜치 일치가 검증된 record
// 소유 잔여물이므로, 같은 canonical/branch/head/clean 검사와 fingerprint·confirm
// 아래 abandon이 정리할 수 있어야 한다.
func TestCleanupAbandonAcceptsRecordLinkedResidueWithoutExecution(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "state")
	fixture := newClaimableExecutionFixture(t, stateRoot, "2799-linked-no-execution")
	if err := os.Remove(fixture.tokenPath); err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, stateRoot, fixture.record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = nil
		rec.WorktreePath = fixture.worktree
	})
	head := strings.TrimSpace(preflight.GitOut(fixture.worktree, "rev-parse", "HEAD"))
	deps := CleanupAbandonDeps{Processes: quietCleanupProcesses()}

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, false, ""), deps)
	if err != nil {
		t.Fatalf("record-linked residue without execution must be previewable: %v missing=%v", err, preview.Missing)
	}
	if preview.Fingerprint == "" || !preview.WorktreePresent || !preview.BranchPresent {
		t.Fatalf("preview must seal both local targets: %+v", preview)
	}
	wantPlan := []string{
		"local_worktree:" + fixture.worktree,
		"local_branch:2799-linked-no-execution@" + head,
		"record:" + fixture.record.ID,
	}
	if strings.Join(preview.RemovalPlan, "\n") != strings.Join(wantPlan, "\n") {
		t.Fatalf("removal plan mismatch\nwant=%q\n got=%q", wantPlan, preview.RemovalPlan)
	}

	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(fixture.record.ID, true, preview.Fingerprint), deps)
	if err != nil || !applied.RecordDeleted {
		t.Fatalf("apply must clean the linked residue and delete the record: err=%v result=%+v", err, applied)
	}
	if _, err := os.Stat(fixture.worktree); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("apply must remove the linked worktree: %v", err)
	}
	if code, _, _ := preflight.GitCmd(fixture.record.Repo, "show-ref", "--verify", "--quiet", "refs/heads/2799-linked-no-execution"); code == 0 {
		t.Fatal("apply must remove the linked local branch")
	}
	if _, err := ReadIssueOps(stateRoot, fixture.record.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("apply must remove the record last: %v", err)
	}
}
