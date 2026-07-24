package issueops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/port"
)

func TestStageIssueOpsArtifactRejections(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: t.TempDir(), Branch: "82-artifact"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "notes", []byte("x")); err == nil {
		t.Fatal("unknown artifact name must be rejected")
	}
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "plan", make([]byte, executionOwnerArtifactLimit+1)); err == nil {
		t.Fatal("oversized artifact must be rejected")
	}
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte("token: ghp_"+strings.Repeat("a", 36))); err == nil {
		t.Fatal("secret-like artifact must be rejected, not scrubbed")
	}
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte("계획 본문")); err != nil {
		t.Fatalf("valid staging must pass: %v", err)
	}
	names, err := StagedIssueOpsArtifactNames(stateRoot, record.ID)
	if err != nil || len(names) != 1 || names[0] != "plan" {
		t.Fatalf("staged names must round-trip: %v %v", names, err)
	}

	// prepare 이후(Execution 존재) 스테이징은 조용한 no-op이 아니라 명시 실패.
	worktree := filepath.Join(t.TempDir(), "82-artifact")
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.Execution = &Execution{
			Mode:      "direct",
			Workspace: Workspace{SourceRoot: rec.Repo, Root: worktree, Branch: "82-artifact", BaseHead: "deadbeef", Driver: "git", LinkedAt: "2026-07-24T00:00:00Z"},
			Lease:     WriteLease{Generation: 1, Status: "released"},
		}
	})
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "spec", []byte("늦은 스펙")); err == nil || !strings.Contains(err.Error(), "before execution prepare") {
		t.Fatalf("post-prepare staging must fail loudly: %v", err)
	}
}

// AC-02: stage→prepare materialize(0o600)→manifest 봉인→claim 검증(drift), 빈
// manifest 하위 호환.
func TestExecutionOrcaPrepareSealsStagedArtifactManifest(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "plan", []byte("계획 본문")); err != nil {
		t.Fatal(err)
	}
	if _, err := StageIssueOpsArtifact(stateRoot, record.ID, "turing-loop", []byte("AC 루프")); err != nil {
		t.Fatal(err)
	}
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}
	got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "codex",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}

	root := got.Execution.Workspace.Root
	planPath := filepath.Join(root, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
	info, err := os.Lstat(planPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("materialized artifact must be a 0600 regular file: %v %v", info, err)
	}

	packetBytes, err := os.ReadFile(got.ContextPacketPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		ArtifactManifest map[string]string `json:"artifact_manifest"`
	}
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatal(err)
	}
	if len(packet.ArtifactManifest) != 2 || packet.ArtifactManifest["plan"] == "" || packet.ArtifactManifest["turing-loop"] == "" {
		t.Fatalf("packet must seal the artifact manifest: %+v", packet.ArtifactManifest)
	}

	// claim 검증: 봉인 그대로면 통과, 파일 변조 시 drift 거부.
	rec, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	issueDigest := got.IssueBodySHA256
	packetDigest := got.ContextPacketSHA256
	if err := validateExecutionClaimPacket(rec, issueDigest, packetDigest); err != nil {
		t.Fatalf("untampered artifacts must pass claim validation: %v", err)
	}
	if err := os.Chmod(planPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, []byte("변조된 계획"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateExecutionClaimPacket(rec, issueDigest, packetDigest); err == nil || !strings.Contains(err.Error(), "artifact plan digest mismatch") {
		t.Fatalf("tampered artifact must be rejected as drift: %v", err)
	}
}

func TestExecutionOrcaPrepareWithoutStagingSealsEmptyManifest(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	fake := &executionOrcaFake{probe: port.ExecutionOrcaProbeResult{Available: true, Ready: true}}
	fake.prepare = func(workspace port.ExecutionWorkspaceRequest, _ port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaWorkspaceReceipt, error) {
		if err := os.MkdirAll(workspace.Root, 0o755); err != nil {
			return port.ExecutionOrcaWorkspaceReceipt{}, err
		}
		return executionOrcaWorkspaceReceipt(workspace), nil
	}
	got, err := PrepareExecution(context.Background(), stateRoot, ExecutionPrepareRequest{
		ID: record.ID, Mode: "orca", CWD: record.Repo, Confirm: true,
		Actor: executionActor("codex", "coordinator"), OwnerHost: "codex",
	}, ExecutionPrepareDependencies{Orca: fake, ReadIssue: executionIssueSnapshotReader})
	if err != nil {
		t.Fatal(err)
	}
	packetBytes, err := os.ReadFile(got.ContextPacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(packetBytes), "artifact_manifest") {
		t.Fatalf("empty manifest must stay omitted for backward compatibility: %s", packetBytes)
	}
}
