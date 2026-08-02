package issueopslease

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
)

func TestClaimContextPreflight(t *testing.T) {
	record := claimableRecord(t, leasecontract.Actor{}, "token")
	store := newClaimStore(t, record)
	validator, err := NewClaimContextPreflight(store, nil).Preflight(context.Background(), leaseapp.ClaimPreflightRequest{ID: record.ID, Generation: record.Execution.Lease.Generation})
	if err != nil {
		t.Fatalf("direct preflight: %v", err)
	}
	if err := validator(leaseapp.Record{ID: record.ID, Stable: record}); err != nil {
		t.Fatalf("direct local validator: %v", err)
	}
}

func TestClaimContextPreflightRejectsSealedArtifactDrift(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: packet artifact\n"
	record := claimableRecord(t, leasecontract.Actor{}, "token")
	record.Execution.Mode = "orca"
	record.Execution.Workspace.SourceRoot = filepath.Dir(record.Execution.Workspace.Root)
	record.Execution.Workspace.Driver = "orca"
	record.IssueURL = "https://github.com/example/agent-harness/issues/197"
	artifactPath := filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "artifact", "plan.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	artifact := []byte("sealed plan\n")
	if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	issueDigest := claimDigest([]byte(issueBody))
	packet := claimContextPacket{
		SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Mode: "orca",
		SourceRoot: record.Execution.Workspace.SourceRoot, WorktreeRoot: record.Execution.Workspace.Root,
		Branch: record.Execution.Workspace.Branch, BaseHead: record.Execution.Workspace.BaseHead,
		LeaseGeneration: record.Execution.Lease.Generation, ClaimTokenFile: claimTokenPath(record),
		Issue:            claimPacketIssue{URL: record.IssueURL, Body: issueBody, BodySHA256: issueDigest},
		ArtifactManifest: map[string]string{"plan": claimDigest(artifact)},
	}
	packetBytes, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	packetPath := claimContextPacketPath(record)
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, packetBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	store := newClaimStore(t, record)
	preflight := NewClaimContextPreflight(store, func(_ context.Context, repo, issueURL string) (IssueSnapshot, error) {
		if repo != record.Repo || issueURL != record.IssueURL {
			t.Fatalf("remote issue request repo=%q url=%q", repo, issueURL)
		}
		return IssueSnapshot{URL: record.IssueURL, Body: issueBody}, nil
	})
	request := leaseapp.ClaimPreflightRequest{ID: record.ID, Generation: record.Execution.Lease.Generation, IssueBodySHA256: issueDigest, ContextPacketSHA256: claimDigest(packetBytes)}
	if _, err := preflight.Preflight(context.Background(), request); err != nil {
		t.Fatalf("sealed artifact preflight: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("changed plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := preflight.Preflight(context.Background(), request); err == nil || !strings.Contains(err.Error(), "artifact plan digest mismatch") {
		t.Fatalf("artifact drift error=%v", err)
	}
}
