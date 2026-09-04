package issueopsapp

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

	"issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestIssueOpsAppClaimWiring(t *testing.T) {
	_, err := issueOpsClaimHandler(context.Background(), t.TempDir(), issueops.ExecutionClaimRequest{ID: "io-claim-wiring"}, issueops.ExecutionClaimDependencies{})
	if err == nil || !strings.Contains(err.Error(), "issueops record io-claim-wiring not found") {
		t.Fatalf("claim wiring error=%v", err)
	}
}

func TestIssueOpsClaimProviderNameUsesBranchPrepareAuthority(t *testing.T) {
	got, err := issueOpsClaimProviderName(issueopscontract.IssueOpsRecord{
		IssueURL:      "https://code.company.example/group/issueops/-/issues/197",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{Provider: "gitlab"},
	})
	if err != nil || got != "gitlab" {
		t.Fatalf("provider=%q err=%v", got, err)
	}
}

func TestIssueOpsClaimProviderNameRejectsURLInferenceWithoutBranchAuthority(t *testing.T) {
	_, err := issueOpsClaimProviderName(issueopscontract.IssueOpsRecord{})
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
	if !result.OK || result.Execution.Lease.Status != issueopscontract.LeaseStatusActive || reads != 1 {
		t.Fatalf("claim result=%+v resolved_reads=%d", result, reads)
	}
}

func seedOrcaClaimSnapshot(t *testing.T) (string, issueopscontract.IssueOpsRecord, string, string, string) {
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
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: source, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueops.IssueOpsPhaseImplement
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{Provider: "gitlab", IssueURL: record.IssueURL, Branch: record.Branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true}
	const plan = "# Snapshot owner plan\n"
	if _, err := stageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte(plan)); err != nil {
		t.Fatal(err)
	}
	seedPlannerGates(t, stateRoot, record.ID)
	record.WorktreePath = worktree
	record.PlanPath = filepath.Join(worktree, "plans", "linked.md")
	if err := os.MkdirAll(filepath.Dir(record.PlanPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.PlanPath, []byte(plan), 0o600); err != nil {
		t.Fatal(err)
	}
	sealedPlanPath := filepath.Join(worktree, filepath.FromSlash(issueops.IssueOpsArtifactDir), "plan.md")
	if err := os.MkdirAll(filepath.Dir(sealedPlanPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sealedPlanPath, []byte(plan), 0o600); err != nil {
		t.Fatal(err)
	}
	record.Execution = &issueopscontract.Execution{
		Mode:      issueopscontract.ExecutionModeOrca,
		Workspace: issueopscontract.Workspace{SourceRoot: source, Root: worktree, Branch: record.Branch, BaseHead: baseHead, Driver: "orca", LinkedAt: "2026-07-30T09:00:00Z"},
		Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusClaimable},
		Orca:      &issueopscontract.OrcaBinding{RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", LeaseGeneration: 1, OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch"},
	}
	token := "snapshot-claim-token"
	tokenDigest := claimWiringSHA256(token)
	record.Execution.Lease.ClaimTokenSHA256 = tokenDigest
	key := claimWiringSHA256(record.ID)[:16]
	tokenPath := filepath.Join(worktree, ".issueops", "state", "issueops-v1", key, "lease-1.token")
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
		"issue": map[string]string{"url": record.IssueURL, "body": claimWiringIssueBody(), "body_sha256": issueDigest}, "artifact_manifest": map[string]string{"plan": claimWiringSHA256(plan)},
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
	packetDigest := claimWiringSHA256(string(packetBytes))
	record.Execution.Orca.ArtifactIdentityVersion = issueopscontract.OrcaArtifactIdentityVersion
	record.Execution.Orca.IssueBodySHA256 = issueDigest
	record.Execution.Orca.ContextPacketSHA256 = packetDigest
	record.Execution.Orca.OwnerPromptSHA256 = strings.Repeat("d", 64)
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	return stateRoot, record, tokenPath, issueDigest, packetDigest
}

func claimWiringActor(t *testing.T) issueopscontract.NativeActor {
	t.Helper()
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return issueopscontract.NativeActor{Host: "codex", SessionID: "claim-wiring", SessionProcess: &receipt, ProcessAncestry: []issueopscontract.NativeProcessReceipt{receipt}}
}

func claimWiringIssueBody() string {
	return "## acceptance criteria\n\n- [ ] AC-09: resolved snapshot reader\n\n## verification\n\n```bash\ngo test ./cmd/issueops/issueopsapp -run Snapshot -count=1\n```\n"
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
