package issueopslease

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
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
	fixture := newSealedClaimContext(t, "https://github.com/example/issueops/issues/197", issueBody, []byte("sealed plan\n"))
	if _, err := fixture.preflight.Preflight(context.Background(), fixture.request); err != nil {
		t.Fatalf("sealed artifact preflight: %v", err)
	}
	if err := os.WriteFile(fixture.artifactPath, []byte("changed plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.preflight.Preflight(context.Background(), fixture.request); err == nil || !strings.Contains(err.Error(), "artifact plan digest mismatch") {
		t.Fatalf("artifact drift error=%v", err)
	}
}

func TestClaimContextPreflightAcceptsSealed98163ByteArtifact(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-04: claim the sealed plan\n"
	fixture := newSealedClaimContext(t, "https://github.com/example/issueops/issues/237", issueBody, make([]byte, 98_163))
	if _, err := fixture.preflight.Preflight(context.Background(), fixture.request); err != nil {
		t.Fatalf("98,163-byte sealed artifact preflight: %v", err)
	}
}

type sealedClaimContext struct {
	preflight    *ClaimContextPreflight
	request      leaseapp.ClaimPreflightRequest
	artifactPath string
}

func newSealedClaimContext(t *testing.T, issueURL, issueBody string, artifact []byte) sealedClaimContext {
	t.Helper()
	record := claimableRecord(t, leasecontract.Actor{}, "token")
	record.Execution.Mode = "orca"
	record.Execution.Workspace.SourceRoot = filepath.Dir(record.Execution.Workspace.Root)
	record.Execution.Workspace.Driver = "orca"
	record.IssueURL = issueURL
	artifactPath := filepath.Join(record.Execution.Workspace.Root, ".issueops", "artifact", "plan.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	issueDigest := claimDigest([]byte(issueBody))
	packet := claimContextPacket{
		SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Mode: "orca",
		SourceRoot: record.Execution.Workspace.SourceRoot, WorktreeRoot: record.Execution.Workspace.Root,
		Branch: record.Execution.Workspace.Branch, BaseHead: record.Execution.Workspace.BaseHead,
		LeaseGeneration:  record.Execution.Lease.Generation,
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
	return sealedClaimContext{preflight: preflight, request: request, artifactPath: artifactPath}
}

func TestReadClaimOwnerArtifactRejectsUnsafeFiles(t *testing.T) {
	tests := map[string]struct {
		prepare func(t *testing.T, path string)
		want    string
	}{
		"larger than one MiB": {
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, make([]byte, 1<<20+1), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "owner artifact must be a private bounded regular file",
		},
		"symlink": {
			prepare: func(t *testing.T, path string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(path), "target.md")
				if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			want: "owner artifact path contains a missing entry or symlink",
		},
		"non-private": {
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("public"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "owner artifact must be a private bounded regular file",
		},
		"non-regular": {
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: "owner artifact must be a private bounded regular file",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "artifact.md")
			test.prepare(t, path)
			if _, err := readClaimOwnerArtifact(root, path); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("readClaimOwnerArtifact() error = %v, want containing %q", err, test.want)
			}
		})
	}
}
