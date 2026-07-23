package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	issueremote "agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

const (
	legacyResetRemoteReceiptPrefix = "remote-"
	legacyResetOrcaReceiptPrefix   = "orca-"
	legacyResetCycleReceiptPrefix  = "cycle-"
)

type LegacyResetRemoteReconcileRequest struct {
	TargetSchema        int    `json:"target_schema"`
	ExpectedFingerprint string `json:"expected_fingerprint"`
	LifecycleID         string `json:"lifecycle_id"`
	ClaimID             string `json:"claim_id"`
	Confirm             bool   `json:"confirm"`
}

type LegacyResetRemoteDependencies struct {
	Reconcile func(string, port.IssueProviderReconcilePullRequestRequest) (port.IssueProviderReconcilePullRequestResult, error)
	Verify    func(model.IssueOpsRemoteArtifactVerificationRequest) error
	Now       func() time.Time
}

type LegacyResetRemoteReconcileResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	TargetSchema  int    `json:"target_schema"`
	Fingerprint   string `json:"fingerprint"`
	LifecycleID   string `json:"lifecycle_id"`
	ClaimID       string `json:"claim_id"`
	ArtifactURL   string `json:"artifact_url"`
	VerifiedAt    string `json:"verified_at"`
	Reconciled    bool   `json:"reconciled"`
	NextCommand   string `json:"next_command"`
}

type LegacyResetOrcaReconcileRequest struct {
	TargetSchema        int    `json:"target_schema"`
	ExpectedFingerprint string `json:"expected_fingerprint"`
	LifecycleID         string `json:"lifecycle_id"`
	RuntimeID           string `json:"runtime_id,omitempty"`
	TaskID              string `json:"task_id,omitempty"`
	DispatchID          string `json:"dispatch_id,omitempty"`
	Confirm             bool   `json:"confirm"`
}

type LegacyResetOrcaDependencies struct {
	Status       func(context.Context) (port.OrcaStatus, error)
	ListTasks    func(context.Context) ([]port.OrcaTask, error)
	ShowDispatch func(context.Context, string) (port.OrcaDispatch, error)
	Now          func() time.Time
}

type LegacyResetOrcaReconcileResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	TargetSchema  int    `json:"target_schema"`
	Fingerprint   string `json:"fingerprint"`
	LifecycleID   string `json:"lifecycle_id"`
	RuntimeID     string `json:"runtime_id,omitempty"`
	TaskID        string `json:"task_id,omitempty"`
	DispatchID    string `json:"dispatch_id,omitempty"`
	VerifiedAt    string `json:"verified_at"`
	Reconciled    bool   `json:"reconciled"`
	NextCommand   string `json:"next_command"`
}

type LegacyResetDrainCycleRequest struct {
	TargetSchema        int    `json:"target_schema"`
	ExpectedFingerprint string `json:"expected_fingerprint"`
	LifecycleID         string `json:"lifecycle_id"`
	Confirm             bool   `json:"confirm"`
}

type LegacyResetDrainCycleResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	TargetSchema  int    `json:"target_schema"`
	Fingerprint   string `json:"fingerprint"`
	LifecycleID   string `json:"lifecycle_id"`
	Drained       bool   `json:"drained"`
	NextCommand   string `json:"next_command"`
}

type legacyResetDrainReceipt struct {
	SchemaVersion     int    `json:"schema_version"`
	Kind              string `json:"kind"`
	Fingerprint       string `json:"fingerprint"`
	LifecycleID       string `json:"lifecycle_id"`
	ClaimID           string `json:"claim_id,omitempty"`
	ArtifactSHA256    string `json:"artifact_sha256,omitempty"`
	CandidateSHA256   string `json:"candidate_sha256,omitempty"`
	AuthoritySHA256   string `json:"authority_sha256,omitempty"`
	ObservationSHA256 string `json:"observation_sha256,omitempty"`
	VerifiedAt        string `json:"verified_at"`
}

type legacyResetOrcaObservation struct {
	Outcome  string             `json:"outcome"`
	Status   port.OrcaStatus    `json:"status"`
	Task     *port.OrcaTask     `json:"task,omitempty"`
	Dispatch *port.OrcaDispatch `json:"dispatch,omitempty"`
}

// ReconcileLegacyRemoteClaim performs a read-only provider inventory call.
// It never invokes provider Create and records a fingerprint-bound receipt only
// when exactly one candidate matches and passes live verification.
func ReconcileLegacyRemoteClaim(ctx context.Context, stateDir string, req LegacyResetRemoteReconcileRequest, deps LegacyResetRemoteDependencies) (LegacyResetRemoteReconcileResult, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, req.TargetSchema)
	if err != nil {
		return LegacyResetRemoteReconcileResult{}, err
	}
	req.ExpectedFingerprint = strings.TrimSpace(req.ExpectedFingerprint)
	req.LifecycleID = strings.TrimSpace(req.LifecycleID)
	req.ClaimID = strings.TrimSpace(req.ClaimID)
	if !req.Confirm || len(req.ExpectedFingerprint) != 64 || req.LifecycleID == "" || req.ClaimID == "" {
		return LegacyResetRemoteReconcileResult{}, fmt.Errorf("legacy remote reconciliation requires lifecycle id, claim id, exact fingerprint, and confirm")
	}
	manifest, record, claim, err := legacyResetClaimAuthority(stateRoot, req)
	if err != nil {
		return LegacyResetRemoteReconcileResult{}, err
	}
	receiptKey := legacyResetRemoteReceiptKey(req.LifecycleID, req.ClaimID)
	if _, ok := manifest.DrainedClaims[req.LifecycleID+"\x00"+req.ClaimID]; ok {
		return legacyResetRemoteReconcileResult(req, "", "", true), nil
	}
	if deps.Reconcile == nil || deps.Verify == nil {
		return LegacyResetRemoteReconcileResult{}, fmt.Errorf("legacy remote reconciliation dependencies are unavailable")
	}
	providerRequest := port.IssueProviderReconcilePullRequestRequest{
		Repo: record.Repo, ProjectKey: claim.ProjectKey, HeadBranch: claim.Head, BaseBranch: claim.Base,
		ExpectedHeadSHA: claim.FinalHead, Title: claim.Title, BodySHA256: claim.BodySHA256,
		Labels: append([]string(nil), claim.Labels...), Assignees: append([]string(nil), claim.Assignees...), Draft: claim.Draft,
	}
	inventory, err := deps.Reconcile(strings.ToLower(strings.TrimSpace(claim.Provider)), providerRequest)
	if err != nil {
		return LegacyResetRemoteReconcileResult{}, fmt.Errorf("legacy remote reconciliation transport is ambiguous; claim retained: %w", err)
	}
	if len(inventory.Candidates) != 1 {
		return LegacyResetRemoteReconcileResult{}, fmt.Errorf("legacy remote reconciliation requires exactly one live candidate; found %d and retained the claim", len(inventory.Candidates))
	}
	candidate := inventory.Candidates[0]
	if err := validateLegacyResetRemoteCandidate(record, claim, candidate); err != nil {
		return LegacyResetRemoteReconcileResult{}, err
	}
	if err := deps.Verify(model.IssueOpsRemoteArtifactVerificationRequest{
		Provider: claim.Provider, Kind: claim.Kind, URL: candidate.URL,
		Labels: append([]string(nil), claim.Labels...), Assignees: append([]string(nil), claim.Assignees...), TargetBranch: claim.Base,
	}); err != nil {
		return LegacyResetRemoteReconcileResult{}, fmt.Errorf("legacy remote candidate live verification failed; claim retained: %w", err)
	}
	current, _, _, err := legacyResetClaimAuthority(stateRoot, req)
	if err != nil {
		return LegacyResetRemoteReconcileResult{}, err
	}
	if current.Fingerprint != manifest.Fingerprint {
		return LegacyResetRemoteReconcileResult{}, fmt.Errorf("legacy reset fingerprint changed during remote reconciliation; receipt was not recorded")
	}
	verifiedAt := legacyResetDrainNow(deps.Now)
	candidateData, err := json.Marshal(candidate)
	if err != nil {
		return LegacyResetRemoteReconcileResult{}, err
	}
	receipt := legacyResetDrainReceipt{
		SchemaVersion: 1, Kind: "remote", Fingerprint: manifest.Fingerprint,
		LifecycleID: req.LifecycleID, ClaimID: req.ClaimID,
		ArtifactSHA256: sha256Text(candidate.URL), CandidateSHA256: sha256Bytes(candidateData), VerifiedAt: verifiedAt,
	}
	if err := writeLegacyResetDrainReceipt(stateRoot, receiptKey, receipt); err != nil {
		return LegacyResetRemoteReconcileResult{}, err
	}
	return legacyResetRemoteReconcileResult(req, candidate.URL, verifiedAt, true), nil
}

// ReconcileLegacyOrcaTask records quiescence only from a complete, read-only
// Orca inventory. Transport ambiguity, duplicate identity, or any live exact
// task/dispatch retains the legacy authority and blocks reset.
func ReconcileLegacyOrcaTask(ctx context.Context, stateDir string, req LegacyResetOrcaReconcileRequest, deps LegacyResetOrcaDependencies) (LegacyResetOrcaReconcileResult, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, req.TargetSchema)
	if err != nil {
		return LegacyResetOrcaReconcileResult{}, err
	}
	req.ExpectedFingerprint = strings.TrimSpace(req.ExpectedFingerprint)
	req.LifecycleID = strings.TrimSpace(req.LifecycleID)
	req.RuntimeID = strings.TrimSpace(req.RuntimeID)
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.DispatchID = strings.TrimSpace(req.DispatchID)
	if !req.Confirm || len(req.ExpectedFingerprint) != 64 || req.LifecycleID == "" || (req.RuntimeID == "" && req.TaskID == "" && req.DispatchID == "") {
		return LegacyResetOrcaReconcileResult{}, fmt.Errorf("legacy Orca reconciliation requires exact authority identity, fingerprint, and confirm")
	}
	manifest, task, authorityKey, err := legacyResetOrcaAuthority(stateRoot, req)
	if err != nil {
		return LegacyResetOrcaReconcileResult{}, err
	}
	observation, err := observeLegacyResetOrcaQuiescence(ctx, task, deps)
	if err != nil {
		return LegacyResetOrcaReconcileResult{}, err
	}
	verifiedAt := legacyResetDrainNow(deps.Now)
	if _, drained := manifest.DrainedOrca[authorityKey]; drained {
		return legacyResetOrcaReconcileResult(req, verifiedAt, true), nil
	}
	current, currentTask, currentKey, err := legacyResetOrcaAuthority(stateRoot, req)
	if err != nil {
		return LegacyResetOrcaReconcileResult{}, err
	}
	if current.Fingerprint != manifest.Fingerprint || currentKey != authorityKey || legacyResetOrcaAuthorityKey(currentTask) != authorityKey {
		return LegacyResetOrcaReconcileResult{}, fmt.Errorf("legacy reset fingerprint changed during Orca reconciliation; receipt was not recorded")
	}
	observationData, err := json.Marshal(observation)
	if err != nil {
		return LegacyResetOrcaReconcileResult{}, err
	}
	receipt := legacyResetDrainReceipt{
		SchemaVersion: 1, Kind: "orca", Fingerprint: manifest.Fingerprint, LifecycleID: task.LifecycleID,
		AuthoritySHA256: sha256Text(authorityKey), ObservationSHA256: sha256Bytes(observationData), VerifiedAt: verifiedAt,
	}
	if err := writeLegacyResetDrainReceipt(stateRoot, legacyResetOrcaReceiptKey(task), receipt); err != nil {
		return LegacyResetOrcaReconcileResult{}, err
	}
	return legacyResetOrcaReconcileResult(req, verifiedAt, true), nil
}

func observeLegacyResetOrcaQuiescence(ctx context.Context, task LegacyOrcaTask, deps LegacyResetOrcaDependencies) (legacyResetOrcaObservation, error) {
	if deps.Status == nil || deps.ListTasks == nil || deps.ShowDispatch == nil {
		return legacyResetOrcaObservation{}, fmt.Errorf("legacy Orca reconciliation dependencies are unavailable")
	}
	status, err := deps.Status(ctx)
	if err != nil {
		return legacyResetOrcaObservation{}, fmt.Errorf("legacy Orca runtime inventory is ambiguous; authority retained: %w", err)
	}
	observation := legacyResetOrcaObservation{Status: status}
	currentRuntime := strings.TrimSpace(status.RuntimeID)
	if currentRuntime == "" || !status.RuntimeReachable || strings.ToLower(strings.TrimSpace(status.RuntimeState)) != "ready" || strings.ToLower(strings.TrimSpace(status.GraphState)) != "ready" {
		return observation, fmt.Errorf("legacy Orca runtime inventory is not authoritatively ready; authority retained")
	}
	if task.TaskID == "" {
		return observation, fmt.Errorf("legacy Orca authority has no task id; authority retained")
	}
	tasks, err := deps.ListTasks(ctx)
	if err != nil {
		return observation, fmt.Errorf("legacy Orca task inventory is ambiguous; authority retained: %w", err)
	}
	candidates := legacyResetOrcaTaskCandidates(tasks, task.TaskID)
	if len(candidates) > 1 {
		return observation, fmt.Errorf("legacy Orca reconciliation requires at most one exact task; found %d and retained authority", len(candidates))
	}
	if len(candidates) == 1 {
		candidate := candidates[0]
		observation.Task = &candidate
		if strings.TrimSpace(candidate.RuntimeID) != currentRuntime {
			return observation, fmt.Errorf("legacy Orca task runtime identity changed; authority retained")
		}
		if !legacyResetOrcaTerminalTaskStatus(candidate.Status) {
			return observation, fmt.Errorf("legacy Orca task %s is still live with status %s", task.TaskID, candidate.Status)
		}
		observation.Outcome = "task_terminal"
	} else {
		observation.Outcome = "task_absent"
	}
	return observeLegacyResetOrcaDispatch(ctx, task, currentRuntime, observation, deps.ShowDispatch)
}

func observeLegacyResetOrcaDispatch(ctx context.Context, task LegacyOrcaTask, currentRuntime string, observation legacyResetOrcaObservation, show func(context.Context, string) (port.OrcaDispatch, error)) (legacyResetOrcaObservation, error) {
	dispatch, err := show(ctx, task.TaskID)
	if err != nil {
		var orcaErr *port.OrcaError
		if errors.As(err, &orcaErr) && orcaErr.Code == "not_found" {
			observation.Outcome += "_dispatch_absent"
			return observation, nil
		}
		return observation, fmt.Errorf("legacy Orca dispatch inventory is ambiguous; authority retained: %w", err)
	}
	observation.Dispatch = &dispatch
	if dispatch.TaskID != task.TaskID || (task.DispatchID != "" && dispatch.ID != task.DispatchID) || strings.TrimSpace(dispatch.RuntimeID) != currentRuntime {
		return observation, fmt.Errorf("legacy Orca dispatch identity changed; authority retained")
	}
	if !legacyResetOrcaTerminalDispatchStatus(dispatch.Status) {
		return observation, fmt.Errorf("legacy Orca dispatch %s is still live with status %s", dispatch.ID, dispatch.Status)
	}
	observation.Outcome += "_dispatch_terminal"
	return observation, nil
}

func DrainLegacyCycle(stateDir string, req LegacyResetDrainCycleRequest) (LegacyResetDrainCycleResult, error) {
	return drainLegacyCycle(context.Background(), stateDir, req, nil)
}

func DrainLegacyCycleWithOrca(ctx context.Context, stateDir string, req LegacyResetDrainCycleRequest, deps LegacyResetOrcaDependencies) (LegacyResetDrainCycleResult, error) {
	return drainLegacyCycle(ctx, stateDir, req, &deps)
}

func drainLegacyCycle(ctx context.Context, stateDir string, req LegacyResetDrainCycleRequest, orcaDeps *LegacyResetOrcaDependencies) (LegacyResetDrainCycleResult, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, req.TargetSchema)
	if err != nil {
		return LegacyResetDrainCycleResult{}, err
	}
	req.ExpectedFingerprint = strings.TrimSpace(req.ExpectedFingerprint)
	req.LifecycleID = strings.TrimSpace(req.LifecycleID)
	if !req.Confirm || len(req.ExpectedFingerprint) != 64 || req.LifecycleID == "" {
		return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle drain requires lifecycle id, exact fingerprint, and confirm")
	}
	manifest, exists, err := buildLegacyResetManifest(stateRoot, req.TargetSchema)
	if err != nil {
		return LegacyResetDrainCycleResult{}, err
	}
	if !exists || manifest.Fingerprint != req.ExpectedFingerprint {
		return LegacyResetDrainCycleResult{}, fmt.Errorf("stale legacy reset fingerprint for cycle drain")
	}
	if len(manifest.Blockers) != 0 {
		return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle drain blocked by unknown state: %s", strings.Join(manifest.Blockers, "; "))
	}
	if _, active := manifest.Active[req.LifecycleID]; !active {
		return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle %s is not active", req.LifecycleID)
	}
	_, alreadyDrained := manifest.DrainedCycles[req.LifecycleID]
	for key, task := range manifest.Orca {
		if task.LifecycleID == req.LifecycleID {
			if _, reconciled := manifest.DrainedOrca[key]; !reconciled {
				return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle %s still has an unreconciled Orca task", req.LifecycleID)
			}
			if orcaDeps == nil {
				return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle %s requires fresh Orca inventory before drain", req.LifecycleID)
			}
			if _, err := observeLegacyResetOrcaQuiescence(ctx, task, *orcaDeps); err != nil {
				return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle %s Orca authority is not quiescent: %w", req.LifecycleID, err)
			}
		}
	}
	for key, claim := range manifest.Claims {
		if claim.LifecycleID != req.LifecycleID {
			continue
		}
		if _, reconciled := manifest.DrainedClaims[key]; !reconciled {
			return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy cycle %s still has unresolved remote claim %s", req.LifecycleID, claim.ClaimID)
		}
	}
	current, exists, err := buildLegacyResetManifest(stateRoot, req.TargetSchema)
	if err != nil {
		return LegacyResetDrainCycleResult{}, err
	}
	if !exists || current.Fingerprint != manifest.Fingerprint {
		return LegacyResetDrainCycleResult{}, fmt.Errorf("legacy reset fingerprint changed during cycle drain; receipt was not recorded")
	}
	if alreadyDrained {
		return legacyResetDrainCycleResult(req, true), nil
	}
	receipt := legacyResetDrainReceipt{
		SchemaVersion: 1, Kind: "cycle", Fingerprint: manifest.Fingerprint,
		LifecycleID: req.LifecycleID, VerifiedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeLegacyResetDrainReceipt(stateRoot, legacyResetCycleReceiptKey(req.LifecycleID), receipt); err != nil {
		return LegacyResetDrainCycleResult{}, err
	}
	return legacyResetDrainCycleResult(req, true), nil
}

func legacyResetOrcaAuthority(stateRoot string, req LegacyResetOrcaReconcileRequest) (legacyResetManifest, LegacyOrcaTask, string, error) {
	manifest, exists, err := buildLegacyResetManifest(stateRoot, req.TargetSchema)
	if err != nil {
		return manifest, LegacyOrcaTask{}, "", err
	}
	if !exists || manifest.Fingerprint != req.ExpectedFingerprint {
		return manifest, LegacyOrcaTask{}, "", fmt.Errorf("stale legacy reset fingerprint for Orca reconciliation")
	}
	if len(manifest.Blockers) != 0 {
		return manifest, LegacyOrcaTask{}, "", fmt.Errorf("legacy Orca reconciliation blocked by unknown state: %s", strings.Join(manifest.Blockers, "; "))
	}
	matches := make([]string, 0, 1)
	for key, task := range manifest.Orca {
		if task.LifecycleID == req.LifecycleID && task.RuntimeID == req.RuntimeID && task.TaskID == req.TaskID && task.DispatchID == req.DispatchID {
			matches = append(matches, key)
		}
	}
	if len(matches) != 1 {
		return manifest, LegacyOrcaTask{}, "", fmt.Errorf("legacy Orca authority identity matched %d records", len(matches))
	}
	key := matches[0]
	return manifest, manifest.Orca[key], key, nil
}

func legacyResetOrcaTaskCandidates(tasks []port.OrcaTask, taskID string) []port.OrcaTask {
	result := make([]port.OrcaTask, 0, 1)
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == taskID {
			result = append(result, task)
		}
	}
	return result
}

func legacyResetOrcaTerminalTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed":
		return true
	default:
		return false
	}
}

func legacyResetOrcaTerminalDispatchStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "circuit_broken":
		return true
	default:
		return false
	}
}

func legacyResetOrcaReconcileCommand(task LegacyOrcaTask, fingerprint string) string {
	command := fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --reconcile-orca --id %s", task.LifecycleID)
	if task.RuntimeID != "" {
		command += " --runtime-id " + task.RuntimeID
	}
	if task.TaskID != "" {
		command += " --task-id " + task.TaskID
	}
	if task.DispatchID != "" {
		command += " --dispatch-id " + task.DispatchID
	}
	return command + " --expected-fingerprint " + fingerprint + " --confirm --json"
}

func legacyResetClaimAuthority(stateRoot string, req LegacyResetRemoteReconcileRequest) (legacyResetManifest, legacyRecordHeader, *legacyRemoteCreateClaimAuthority, error) {
	manifest, exists, err := buildLegacyResetManifest(stateRoot, req.TargetSchema)
	if err != nil {
		return manifest, legacyRecordHeader{}, nil, err
	}
	if !exists || manifest.Fingerprint != req.ExpectedFingerprint {
		return manifest, legacyRecordHeader{}, nil, fmt.Errorf("stale legacy reset fingerprint for remote reconciliation")
	}
	if len(manifest.Blockers) != 0 {
		return manifest, legacyRecordHeader{}, nil, fmt.Errorf("legacy remote reconciliation blocked by unknown state: %s", strings.Join(manifest.Blockers, "; "))
	}
	record, err := legacyResetRecordAuthority(manifest, req.LifecycleID)
	if err != nil {
		return manifest, record, nil, err
	}
	claim := record.RemoteCreateClaim
	if claim == nil || strings.TrimSpace(claim.ClaimID) != req.ClaimID {
		return manifest, record, nil, fmt.Errorf("legacy remote claim identity does not match %s/%s", req.LifecycleID, req.ClaimID)
	}
	return manifest, record, claim, nil
}

func legacyResetRecordAuthority(manifest legacyResetManifest, lifecycleID string) (legacyRecordHeader, error) {
	for _, row := range manifest.Rows {
		if row.Store == "" && row.Bucket == "issueops" && row.ID == lifecycleID {
			var record legacyRecordHeader
			if err := json.Unmarshal(row.Data, &record); err != nil {
				return record, err
			}
			return record, nil
		}
	}
	path := filepath.Join(manifest.LegacyRoot, lifecycleID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return legacyRecordHeader{}, fmt.Errorf("legacy cycle %s was not found", lifecycleID)
		}
		return legacyRecordHeader{}, err
	}
	var record legacyRecordHeader
	if err := json.Unmarshal(raw, &record); err != nil {
		return record, err
	}
	return record, nil
}

func validateLegacyResetRemoteCandidate(record legacyRecordHeader, claim *legacyRemoteCreateClaimAuthority, candidate port.IssueProviderReconcilePullRequestCandidate) error {
	if candidate.ProjectKey != claim.ProjectKey || candidate.SourceProjectKey != claim.ProjectKey || candidate.HeadBranch != claim.Head || candidate.BaseBranch != claim.Base ||
		candidate.HeadSHA != claim.FinalHead || candidate.Title != claim.Title || candidate.BodySHA256 != claim.BodySHA256 || candidate.Draft != claim.Draft ||
		!sameCanonicalRemoteSet(candidate.Labels, claim.Labels) || !sameCanonicalRemoteSet(candidate.Assignees, claim.Assignees) {
		return fmt.Errorf("legacy remote reconciliation candidate does not match the exact durable claim")
	}
	if strings.TrimSpace(claim.KnownURL) != "" && candidate.URL != strings.TrimSpace(claim.KnownURL) {
		return fmt.Errorf("legacy remote reconciliation candidate URL differs from the durable known URL")
	}
	if err := issueremote.ValidateArtifactURL(candidate.URL, claim.Provider, claim.Kind); err != nil {
		return fmt.Errorf("legacy remote reconciliation candidate URL is not canonical: %w", err)
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("legacy remote reconciliation requires the durable issue URL")
	}
	if err := issueremote.ValidateArtifactMatchesIssue(record.IssueURL, candidate.URL, claim.Provider, claim.Kind); err != nil {
		return fmt.Errorf("legacy remote reconciliation candidate project differs from the durable issue: %w", err)
	}
	return nil
}

func writeLegacyResetDrainReceipt(stateRoot, id string, receipt legacyResetDrainReceipt) error {
	control, err := sqlstore.Open(filepath.Join(stateRoot, issueOpsResetDirectory))
	if err != nil {
		return err
	}
	return control.WithSpan(context.Background(), func(context.Context) error {
		current, ok, err := control.Get(issueOpsResetBucket, id)
		if err != nil {
			return err
		}
		data, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if ok {
			if string(current) == string(data) {
				return nil
			}
			return fmt.Errorf("legacy reset drain receipt identity changed")
		}
		return control.Apply(context.Background(), []sqlstore.Mutation{{Bucket: issueOpsResetBucket, ID: id, Data: data, RequireAbsent: true}})
	})
}

func loadLegacyResetDrainReceipts(manifest *legacyResetManifest) error {
	rows, err := sqlstore.GetAllExisting(filepath.Join(manifest.StateRoot, issueOpsResetDirectory), issueOpsResetBucket)
	if err != nil {
		if isLegacyResetMissing(err) {
			return nil
		}
		return err
	}
	for _, row := range rows {
		if !strings.HasPrefix(row.ID, legacyResetRemoteReceiptPrefix) && !strings.HasPrefix(row.ID, legacyResetOrcaReceiptPrefix) && !strings.HasPrefix(row.ID, legacyResetCycleReceiptPrefix) {
			continue
		}
		var receipt legacyResetDrainReceipt
		if err := json.Unmarshal(row.Data, &receipt); err != nil {
			return fmt.Errorf("decode legacy reset drain receipt %s: %w", row.ID, err)
		}
		if receipt.SchemaVersion != 1 || receipt.Fingerprint != manifest.Fingerprint || strings.TrimSpace(receipt.VerifiedAt) == "" {
			continue
		}
		switch receipt.Kind {
		case "remote":
			if row.ID != legacyResetRemoteReceiptKey(receipt.LifecycleID, receipt.ClaimID) || receipt.ClaimID == "" || receipt.ArtifactSHA256 == "" || receipt.CandidateSHA256 == "" {
				return fmt.Errorf("legacy reset remote drain receipt %s is malformed", row.ID)
			}
			manifest.DrainedClaims[receipt.LifecycleID+"\x00"+receipt.ClaimID] = struct{}{}
		case "orca":
			matched := ""
			for key, task := range manifest.Orca {
				if row.ID == legacyResetOrcaReceiptKey(task) && receipt.LifecycleID == task.LifecycleID && receipt.AuthoritySHA256 == sha256Text(key) {
					if matched != "" {
						return fmt.Errorf("legacy reset Orca drain receipt %s is ambiguous", row.ID)
					}
					matched = key
				}
			}
			if matched == "" || receipt.ObservationSHA256 == "" {
				return fmt.Errorf("legacy reset Orca drain receipt %s is malformed", row.ID)
			}
			manifest.DrainedOrca[matched] = struct{}{}
		case "cycle":
			if row.ID != legacyResetCycleReceiptKey(receipt.LifecycleID) || receipt.LifecycleID == "" {
				return fmt.Errorf("legacy reset cycle drain receipt %s is malformed", row.ID)
			}
			manifest.DrainedCycles[receipt.LifecycleID] = struct{}{}
		default:
			return fmt.Errorf("legacy reset drain receipt %s has unknown kind %q", row.ID, receipt.Kind)
		}
	}
	return nil
}

func legacyResetRemoteReceiptKey(lifecycleID, claimID string) string {
	return legacyResetRemoteReceiptPrefix + sha256Text(lifecycleID+"\x00"+claimID)
}

func legacyResetOrcaAuthorityKey(task LegacyOrcaTask) string {
	return task.LifecycleID + "\x00" + task.RuntimeID + "\x00" + task.TaskID + "\x00" + task.DispatchID
}

func legacyResetOrcaReceiptKey(task LegacyOrcaTask) string {
	return legacyResetOrcaReceiptPrefix + sha256Text(legacyResetOrcaAuthorityKey(task))
}

func legacyResetCycleReceiptKey(lifecycleID string) string {
	return legacyResetCycleReceiptPrefix + sha256Text(lifecycleID)
}

func sha256Text(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func legacyResetDrainNow(now func() time.Time) string {
	if now == nil {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return now().UTC().Format(time.RFC3339Nano)
}

func legacyResetRemoteReconcileResult(req LegacyResetRemoteReconcileRequest, url, verifiedAt string, reconciled bool) LegacyResetRemoteReconcileResult {
	return LegacyResetRemoteReconcileResult{
		OK: true, SchemaVersion: 1, TargetSchema: req.TargetSchema, Fingerprint: req.ExpectedFingerprint,
		LifecycleID: req.LifecycleID, ClaimID: req.ClaimID, ArtifactURL: url, VerifiedAt: verifiedAt, Reconciled: reconciled,
		NextCommand: fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --drain-cycle --id %s --expected-fingerprint %s --confirm --json", req.LifecycleID, req.ExpectedFingerprint),
	}
}

func legacyResetOrcaReconcileResult(req LegacyResetOrcaReconcileRequest, verifiedAt string, reconciled bool) LegacyResetOrcaReconcileResult {
	return LegacyResetOrcaReconcileResult{
		OK: true, SchemaVersion: 1, TargetSchema: req.TargetSchema, Fingerprint: req.ExpectedFingerprint,
		LifecycleID: req.LifecycleID, RuntimeID: req.RuntimeID, TaskID: req.TaskID, DispatchID: req.DispatchID,
		VerifiedAt: verifiedAt, Reconciled: reconciled,
		NextCommand: fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --drain-cycle --id %s --expected-fingerprint %s --confirm --json", req.LifecycleID, req.ExpectedFingerprint),
	}
}

func legacyResetDrainCycleResult(req LegacyResetDrainCycleRequest, drained bool) LegacyResetDrainCycleResult {
	return LegacyResetDrainCycleResult{
		OK: true, SchemaVersion: 1, TargetSchema: req.TargetSchema, Fingerprint: req.ExpectedFingerprint,
		LifecycleID: req.LifecycleID, Drained: drained,
		NextCommand: fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --preview --json"),
	}
}
