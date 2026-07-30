package harnessapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestHarnessAppClaimWiring(t *testing.T) {
	_, err := issueOpsClaimHandler(context.Background(), t.TempDir(), issueops.ExecutionClaimRequest{ID: "io-claim-wiring"}, issueops.ExecutionClaimDependencies{})
	if err == nil || !strings.Contains(err.Error(), "issueops record io-claim-wiring not found") {
		t.Fatalf("claim wiring error=%v", err)
	}
}

func TestIssueOpsClaimProviderNameUsesBranchPrepareAuthority(t *testing.T) {
	got, err := issueOpsClaimProviderName(issueops.IssueOpsRecord{
		IssueURL:      "https://code.company.example/group/agent-harness/-/issues/197",
		BranchPrepare: &issueops.IssueOpsBranchPrepare{Provider: "gitlab"},
	})
	if err != nil || got != "gitlab" {
		t.Fatalf("provider=%q err=%v", got, err)
	}
}

func TestIssueOpsClaimProviderNameRejectsURLInferenceWithoutBranchAuthority(t *testing.T) {
	_, err := issueOpsClaimProviderName(issueops.IssueOpsRecord{})
	if err == nil || !strings.Contains(err.Error(), "linked issue provider is unavailable") {
		t.Fatalf("URL inference must be rejected: %v", err)
	}
}

func TestIssueOpsClaimHandlerUsesResolvedSnapshotReader(t *testing.T) {
	stateRoot, record, token, issueDigest, packetDigest := seedOrcaClaimSnapshot(t)
	reads := 0
	result, err := issueOpsClaimHandler(context.Background(), stateRoot, issueops.ExecutionClaimRequest{
		ID: record.ID, Generation: 1, Actor: claimWiringActor(t), CWD: record.Execution.Workspace.Root,
		TokenFile: token, IssueBodySHA256: issueDigest, ContextPacketSHA256: packetDigest,
	}, issueops.ExecutionClaimDependencies{ReadIssue: func(_ context.Context, providerName string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		reads++
		if providerName != "gitlab" || request.URL != record.IssueURL {
			t.Fatalf("snapshot request provider=%q url=%q", providerName, request.URL)
		}
		return port.ExecutionIssueSnapshot{URL: request.URL, Body: claimWiringIssueBody()}, nil
	},
	})
	if err != nil {
		t.Fatalf("claim with resolved snapshot reader: %v", err)
	}
	if !result.OK || result.Execution.Lease.Status != model.LeaseStatusActive || reads != 1 {
		t.Fatalf("claim result=%+v resolved_reads=%d", result, reads)
	}
}

func seedOrcaClaimSnapshot(t *testing.T) (string, issueops.IssueOpsRecord, string, string, string) {
	t.Helper()
	stateRoot := t.TempDir()
	source := filepath.Join(t.TempDir(), "source")
	worktree := filepath.Join(t.TempDir(), "worktree")
	const branch = "192-snapshot-claim"
	claimWiringGit(t, "", "init", "-q", "-b", "main", source)
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# snapshot fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	claimWiringGit(t, source, "add", "README.md")
	claimWiringGit(t, source, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: snapshot fixture")
	claimWiringGit(t, source, "worktree", "add", "-q", "-b", branch, worktree, "main")
	baseHead := strings.TrimSpace(claimWiringGit(t, worktree, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: source, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueops.IssueOpsPhaseImplement
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	record.BranchPrepare = &issueops.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true}
	record.Execution = &model.Execution{
		Mode:      model.ExecutionModeOrca,
		Workspace: model.Workspace{SourceRoot: source, Root: worktree, Branch: record.Branch, BaseHead: baseHead, Driver: "orca", LinkedAt: "2026-07-30T09:00:00Z"},
		Lease:     model.WriteLease{Generation: 1, Status: model.LeaseStatusClaimable},
		Orca:      &model.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1, OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch"},
	}
	token := "snapshot-claim-token"
	tokenDigest := claimWiringSHA256(token)
	record.Execution.Lease.ClaimTokenSHA256 = tokenDigest
	key := claimWiringSHA256(record.ID)[:16]
	tokenPath := filepath.Join(worktree, ".agent-harness", "state", "issueops-v1", key, "lease-1.token")
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	issueDigest := claimWiringSHA256(claimWiringIssueBody())
	packetPath := issueops.SealedOwnerContextPacketPath(record)
	packet := map[string]any{
		"schema_version": 1, "lifecycle_id": record.ID, "mode": "orca", "source_root": source, "worktree_root": worktree,
		"branch": record.Branch, "base_head": record.Execution.Workspace.BaseHead, "lease_generation": uint64(1), "claim_token_file": tokenPath,
		"issue": map[string]string{"url": record.IssueURL, "body": claimWiringIssueBody(), "body_sha256": issueDigest}, "artifact_manifest": map[string]string{},
	}
	packetBytes, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, packetBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, tokenPath, issueDigest, claimWiringSHA256(string(packetBytes))
}

func claimWiringActor(t *testing.T) model.NativeActor {
	t.Helper()
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return model.NativeActor{Host: "codex", SessionID: "claim-wiring", SessionProcess: &receipt, ProcessAncestry: []model.NativeProcessReceipt{receipt}}
}

func claimWiringIssueBody() string {
	return "## acceptance criteria\n\n- [ ] AC-09: resolved snapshot reader\n\n## verification\n\n```bash\ngo test ./cmd/harness/harnessapp -run Snapshot -count=1\n```\n"
}

func claimWiringSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func claimWiringGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
