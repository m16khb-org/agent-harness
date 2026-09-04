package orphancleanup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
	corehealth "issueops/internal/domain/operationalhealth"
)

// Dependencies keep the control-plane policy in core while the CLI supplies
// the read-only operational inventory and provider-specific merge verifier.
type Dependencies struct {
	Collect      func(context.Context, string) (corehealth.Snapshot, error)
	VerifyMerged func(model.IssueOpsRemoteArtifactVerification) error
}

func Preview(ctx context.Context, request Request, deps Dependencies) (Result, error) {
	request, err := normalizeRequest(request)
	if err != nil {
		return Result{}, err
	}
	result := resultForRequest(request)
	if deps.Collect == nil {
		return blocked(result, "inventory_read", "operational inventory reader is unavailable"), nil
	}
	snapshot, err := deps.Collect(ctx, request.RepoRoot)
	if err != nil {
		return blocked(result, "inventory_read", "operational inventory could not be refreshed"), nil
	}
	result.InventoryRefreshed = true
	inspectInventory(&result, request, snapshot)
	inspectWorktreeCleanliness(&result, request.WorktreePath)
	verifyMergedArtifact(&result, request.Artifact, deps.VerifyMerged)
	finish(&result)
	return result, nil
}

func Apply(ctx context.Context, request Request, apply ApplyRequest, deps Dependencies) (Result, error) {
	if !apply.Confirm {
		return Result{}, fmt.Errorf("orphan cleanup apply requires --confirm")
	}
	if strings.TrimSpace(apply.Fingerprint) == "" {
		return Result{}, fmt.Errorf("orphan cleanup apply requires --fingerprint from a ready preview")
	}
	preview, err := Preview(ctx, request, deps)
	if err != nil {
		return preview, err
	}
	preview.Preview = false
	preview.Confirmed = true
	if !preview.Ready {
		return preview, fmt.Errorf("orphan cleanup apply is blocked: %s", strings.Join(preview.Missing, ", "))
	}
	if strings.TrimSpace(apply.Fingerprint) != preview.Fingerprint {
		return preview, fmt.Errorf("stale preview fingerprint: rerun orphan cleanup preview before apply")
	}
	if code, _, stderr := GitCmd(preview.RepoRoot, "worktree", "remove", preview.WorktreePath); code != 0 {
		return preview, fmt.Errorf("remove confirmed local worktree: %s", boundedGitFailure(stderr))
	}
	preview.LocalWorktreeRemoved = true
	if code, _, stderr := GitCmd(preview.RepoRoot, "update-ref", "-d", "refs/heads/"+preview.Branch, preview.HeadSHA); code != 0 {
		return preview, fmt.Errorf("remove confirmed local branch with preview HEAD CAS: %s", boundedGitFailure(stderr))
	}
	preview.LocalBranchRemoved = true
	preview.Applied = true
	return preview, nil
}

func normalizeRequest(request Request) (Request, error) {
	var err error
	request.ID = strings.TrimSpace(request.ID)
	request.Branch = strings.TrimSpace(request.Branch)
	request.RepoRoot, err = canonicalPath(request.RepoRoot)
	if err != nil {
		return Request{}, fmt.Errorf("orphan cleanup repo root: %w", err)
	}
	request.WorktreePath, err = canonicalPath(request.WorktreePath)
	if err != nil {
		return Request{}, fmt.Errorf("orphan cleanup worktree path: %w", err)
	}
	request.Artifact.Provider = strings.ToLower(strings.TrimSpace(request.Artifact.Provider))
	request.Artifact.Kind = strings.ToLower(strings.TrimSpace(request.Artifact.Kind))
	request.Artifact.URL = strings.TrimSpace(request.Artifact.URL)
	if request.ID == "" {
		return Request{}, fmt.Errorf("orphan cleanup id is required")
	}
	if request.Branch == "" {
		return Request{}, fmt.Errorf("orphan cleanup branch is required")
	}
	if request.Artifact.Provider == "" || request.Artifact.Kind == "" || request.Artifact.URL == "" {
		return Request{}, fmt.Errorf("orphan cleanup provider, artifact kind, and artifact URL are required")
	}
	return request, nil
}

func canonicalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func resultForRequest(request Request) Result {
	return Result{
		OK:                   true,
		Preview:              true,
		ID:                   request.ID,
		RepoRoot:             request.RepoRoot,
		WorktreePath:         request.WorktreePath,
		Branch:               request.Branch,
		Provider:             request.Artifact.Provider,
		ArtifactKind:         request.Artifact.Kind,
		RemoteArtifactURL:    request.Artifact.URL,
		RecordAbsent:         true,
		RemoteBranchDeletion: "remote branch is intentionally untouched; deletion requires separate explicit approval",
		Missing:              []string{},
		Warnings:             []string{},
	}
}

func inspectInventory(result *Result, request Request, snapshot corehealth.Snapshot) {
	if !samePath(snapshot.RepoRoot, request.RepoRoot) {
		missing(result, "repo_root_match")
		return
	}
	if len(snapshot.InventoryProblems) > 0 {
		missing(result, "inventory_complete")
	}
	canonicalCount := 0
	for _, worktree := range snapshot.GitWorktrees {
		if worktree.Canonical && samePath(worktree.Path, request.RepoRoot) {
			canonicalCount++
		}
		if !samePath(worktree.Path, request.WorktreePath) {
			continue
		}
		result.TargetWorktreeCount++
		result.TargetCanonical = result.TargetCanonical || worktree.Canonical
		if result.HeadSHA == "" {
			result.HeadSHA = strings.TrimSpace(worktree.Head)
		}
		if strings.TrimSpace(worktree.Branch) != request.Branch {
			// branch_match: `missing`은 충족되지 않은 요구의 목록이므로 요구형으로
			// 적는다. cleanup status가 같은 조건에 쓰는 이름과 같아진다(#185).
			missing(result, "branch_match")
		}
	}
	if canonicalCount != 1 {
		missing(result, "canonical_repo_root")
	}
	if result.TargetWorktreeCount != 1 {
		missing(result, "target_worktree_count")
	}
	if result.TargetCanonical {
		missing(result, "canonical_worktree")
	}
	if !validGitOID(result.HeadSHA) {
		missing(result, "worktree_head")
	} else {
		result.RecoveryHead = result.HeadSHA
		result.RecoveryPath = filepath.Join(filepath.Dir(request.WorktreePath), filepath.Base(request.WorktreePath)+"-recovery-"+result.HeadSHA[:12])
	}
	matchingLocalRefs := 0
	for _, ref := range snapshot.LocalRefs {
		if strings.TrimSpace(ref.Location) != "local" || strings.TrimSpace(ref.Branch) != request.Branch {
			continue
		}
		matchingLocalRefs++
		result.LocalBranchOID = strings.TrimSpace(ref.OID)
	}
	if matchingLocalRefs != 1 || result.LocalBranchOID == "" || result.LocalBranchOID != result.HeadSHA {
		missing(result, "local_branch_head")
	}
	for _, cycle := range snapshot.Cycles {
		if strings.TrimSpace(cycle.ID) == request.ID {
			result.RecordAbsent = false
			missing(result, "record_present")
		}
		ownsTargetBranch := samePath(cycle.Repo, request.RepoRoot) && strings.TrimSpace(cycle.Branch) == request.Branch
		if !samePath(cycle.WorktreePath, request.WorktreePath) && !ownsTargetBranch {
			continue
		}
		result.RecordAbsent = false
		missing(result, "target_record_present")
		authority := corehealth.EvaluateCycleAuthority(cycle, corehealth.Options{})
		if authority == corehealth.AuthorityLive || authority == corehealth.AuthorityPreserved {
			missing(result, "target_lifecycle_owner")
		} else if authority == corehealth.AuthorityUnknown {
			missing(result, "target_lifecycle_authority_unknown")
		}
	}
	for _, index := range snapshot.LeaseHolderIndexes {
		if strings.TrimSpace(index.LifecycleID) == request.ID {
			missing(result, "target_lease_authority")
		}
	}
	for _, worktree := range snapshot.OrcaWorktrees {
		if samePath(worktree.Path, request.WorktreePath) {
			missing(result, "orca_worktree_authority")
		}
	}
}

func inspectWorktreeCleanliness(result *Result, worktree string) {
	code, out, _ := GitCmd(worktree, "status", "--porcelain=v1")
	if code != 0 {
		missing(result, "worktree_git_status")
		return
	}
	result.TargetClean = strings.TrimSpace(out) == ""
	if !result.TargetClean {
		// worktree_clean: 요구형 극성. `worktree_git_status`는 관측 실패라 별개
		// 축이며 그대로 둔다(#154가 세운 구분).
		missing(result, "worktree_clean")
	}
}

func verifyMergedArtifact(result *Result, artifact model.IssueOpsRemoteArtifactVerification, verify func(model.IssueOpsRemoteArtifactVerification) error) {
	if verify == nil || verify(artifact) != nil {
		missing(result, "remote_artifact_merged")
		result.Warnings = append(result.Warnings, "remote merge evidence could not be verified through the configured provider")
		return
	}
	result.RemoteMerged = true
}

func finish(result *Result) {
	result.Missing = uniqueSorted(result.Missing)
	result.Warnings = uniqueSorted(result.Warnings)
	result.Ready = len(result.Missing) == 0
	if result.Ready {
		result.Fingerprint = fingerprint(*result)
	}
}

func blocked(result Result, missingCode, warning string) Result {
	missing(&result, missingCode)
	if strings.TrimSpace(warning) != "" {
		result.Warnings = append(result.Warnings, warning)
	}
	finish(&result)
	return result
}

func missing(result *Result, code string) {
	code = strings.TrimSpace(code)
	if code != "" {
		result.Missing = append(result.Missing, code)
	}
}

func fingerprint(result Result) string {
	payload := struct {
		ID                string `json:"id"`
		RepoRoot          string `json:"repo_root"`
		WorktreePath      string `json:"worktree_path"`
		Branch            string `json:"branch"`
		Provider          string `json:"provider"`
		ArtifactKind      string `json:"artifact_kind"`
		RemoteArtifactURL string `json:"remote_artifact_url"`
		HeadSHA           string `json:"head_sha"`
		LocalBranchOID    string `json:"local_branch_oid"`
	}{
		ID: result.ID, RepoRoot: result.RepoRoot, WorktreePath: result.WorktreePath, Branch: result.Branch,
		Provider: result.Provider, ArtifactKind: result.ArtifactKind, RemoteArtifactURL: result.RemoteArtifactURL,
		HeadSHA: result.HeadSHA, LocalBranchOID: result.LocalBranchOID,
	}
	encoded, _ := json.Marshal(payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func validGitOID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, runeValue := range value {
		if !((runeValue >= '0' && runeValue <= '9') || (runeValue >= 'a' && runeValue <= 'f') || (runeValue >= 'A' && runeValue <= 'F')) {
			return false
		}
	}
	return true
}

func samePath(left, right string) bool {
	left, leftErr := canonicalPath(left)
	right, rightErr := canonicalPath(right)
	return leftErr == nil && rightErr == nil && left == right
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func boundedGitFailure(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return "git returned a non-zero exit"
	}
	if len(stderr) > 512 {
		return stderr[:512]
	}
	return stderr
}
