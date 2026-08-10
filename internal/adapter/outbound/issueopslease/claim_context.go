package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/port"
)

type IssueSnapshot struct {
	URL  string
	Body string
}

type IssueSnapshotReader func(context.Context, string, string) (IssueSnapshot, error)

type ClaimContextPreflight struct {
	store     port.TransactionalRecordStore
	readIssue IssueSnapshotReader
}

func NewClaimContextPreflight(store port.TransactionalRecordStore, readIssue IssueSnapshotReader) *ClaimContextPreflight {
	return &ClaimContextPreflight{store: store, readIssue: readIssue}
}

func (p *ClaimContextPreflight) Preflight(ctx context.Context, request leaseapp.ClaimPreflightRequest) (leaseapp.RecordValidator, error) {
	if p == nil || p.store == nil {
		return nil, fmt.Errorf("claim context store is required")
	}
	data, ok, err := p.store.Get(recordBucket, request.ID)
	if err != nil {
		return nil, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return nil, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", request.ID))
	}
	record, err := decodeLeaseRecord(request.ID, data)
	if err != nil {
		return nil, err
	}
	if record.Execution == nil || record.Execution.Mode != "orca" || request.Generation == 0 || request.Generation != record.Execution.Lease.Generation {
		return func(leaseapp.Record) error { return nil }, nil
	}
	issueDigest := strings.ToLower(strings.TrimSpace(request.IssueBodySHA256))
	packetDigest := strings.ToLower(strings.TrimSpace(request.ContextPacketSHA256))
	if !claimSHA256(issueDigest) || !claimSHA256(packetDigest) {
		return nil, fmt.Errorf("Orca claim requires sealed issue and context packet digests")
	}
	if err := validateClaimPacket(record, issueDigest, packetDigest); err != nil {
		return nil, err
	}
	if p.readIssue == nil {
		return nil, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
	}
	snapshot, err := p.readIssue(ctx, record.Repo, record.IssueURL)
	if err != nil {
		return nil, fmt.Errorf("read remote issue before claim: %w", err)
	}
	if strings.TrimSpace(snapshot.URL) != strings.TrimSpace(record.IssueURL) {
		return nil, fmt.Errorf("remote issue snapshot url does not match the linked issue: observed=%s expected=%s", strings.TrimSpace(snapshot.URL), strings.TrimSpace(record.IssueURL))
	}
	if observed := claimDigest([]byte(snapshot.Body)); observed != issueDigest {
		return nil, fmt.Errorf("remote issue body digest drifted from the sealed owner context: expected=%s observed=%s; reseal with `agent-harness issueops execution replace --reseed` after confirming the revision is intended", issueDigest, observed)
	}
	return func(current leaseapp.Record) error {
		return validateClaimPacket(current.Stable, issueDigest, packetDigest)
	}, nil
}

type claimContextPacket struct {
	SchemaVersion    int               `json:"schema_version"`
	LifecycleID      string            `json:"lifecycle_id"`
	Mode             string            `json:"mode"`
	SourceRoot       string            `json:"source_root"`
	WorktreeRoot     string            `json:"worktree_root"`
	Branch           string            `json:"branch"`
	BaseHead         string            `json:"base_head"`
	LeaseGeneration  uint64            `json:"lease_generation"`
	Issue            claimPacketIssue  `json:"issue"`
	ArtifactManifest map[string]string `json:"artifact_manifest"`
}

type claimPacketIssue struct {
	URL        string `json:"url"`
	Body       string `json:"body"`
	BodySHA256 string `json:"body_sha256"`
}

func validateClaimPacket(record leasecontract.Record, issueDigest, packetDigest string) error {
	if record.Execution == nil || record.Execution.Mode != "orca" || record.Execution.Lease.Generation == 0 {
		return fmt.Errorf("sealed owner context no longer matches an Orca execution generation")
	}
	packetPath := claimContextPacketPath(record)
	data, err := readClaimOwnerArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if observed := claimDigest(data); observed != packetDigest {
		return fmt.Errorf("sealed context packet digest mismatch: expected=%s observed=%s path=%s", packetDigest, observed, packetPath)
	}
	var packet claimContextPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	execution := record.Execution
	if packet.SchemaVersion != leasecontract.SchemaVersion || packet.LifecycleID != record.ID || packet.Mode != execution.Mode ||
		!(FilesystemPathMatcher{}).Matches(packet.SourceRoot, execution.Workspace.SourceRoot) ||
		!(FilesystemPathMatcher{}).Matches(packet.WorktreeRoot, execution.Workspace.Root) ||
		packet.Branch != execution.Workspace.Branch || packet.BaseHead != execution.Workspace.BaseHead ||
		packet.LeaseGeneration != execution.Lease.Generation || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch: packet_generation=%d expected_generation=%d", packet.LeaseGeneration, execution.Lease.Generation)
	}
	if packet.Issue.BodySHA256 != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch: expected=%s observed=%s", issueDigest, packet.Issue.BodySHA256)
	}
	if observed := claimDigest([]byte(packet.Issue.Body)); observed != issueDigest {
		return fmt.Errorf("sealed context packet issue body does not hash to its sealed digest: expected=%s observed=%s", issueDigest, observed)
	}
	for name, digest := range packet.ArtifactManifest {
		path := filepath.Join(execution.Workspace.Root, ".agent-harness", "artifact", name+".md")
		artifact, err := readClaimOwnerArtifact(execution.Workspace.Root, path)
		if err != nil {
			return fmt.Errorf("read sealed artifact %s: %w", name, err)
		}
		if claimDigest(artifact) != digest {
			return fmt.Errorf("sealed artifact %s digest mismatch", name)
		}
	}
	return nil
}

func claimContextPacketPath(record leasecontract.Record) string {
	key := claimTokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, fmt.Sprintf("generation-%d", record.Execution.Lease.Generation), "context.json")
}

func readClaimOwnerArtifact(root, path string) ([]byte, error) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("owner artifact must be inside the canonical worktree")
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	current := root
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("owner artifact path contains a missing entry or symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("owner artifact ancestor is not a directory")
		}
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > leasecontract.OwnerArtifactMaxBytes) {
			return nil, fmt.Errorf("owner artifact must be a private bounded regular file")
		}
	}
	return os.ReadFile(path)
}

func claimDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func claimSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
