package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/core/policy"
)

var fullCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var remoteCreateClaimIDPattern = regexp.MustCompile(`^claim_[0-9a-f]{32}$`)

const (
	MaxPriorAttempts      = 16
	MaxPriorAttemptsBytes = 256 * 1024

	ProviderIssueLinkGitLabUnavailable = "gitlab_native_unavailable"
	ProviderIssueLinkGitLabExact       = "gitlab_native_exact"
)

// ValidateEnvelope routes each durable protocol through an explicit validator;
// schema migration never reinterprets a protocol-v1 handoff as ownership v2.
func ValidateEnvelope(record model.IssueOpsRecord) error {
	if err := validateExecutionWorkspace(record); err != nil {
		return err
	}
	if record.ExecutionHandoff == nil {
		if record.RemoteCreateClaim != nil {
			return fmt.Errorf("remote create claim requires supervised execution handoff authority")
		}
		return nil
	}
	switch record.ExecutionHandoff.ProtocolVersion {
	case ProtocolVersion:
		return validateV1Envelope(record)
	case OwnershipTransferProtocolVersion:
		return validateOwnershipEnvelope(record)
	default:
		return fmt.Errorf("unsupported execution handoff protocol_version %d", record.ExecutionHandoff.ProtocolVersion)
	}
}

// validateV1Envelope is deliberately the prior validator body, kept intact as
// the compatibility contract for all existing handoff records.
func validateV1Envelope(record model.IssueOpsRecord) error {
	if record.ExecutionHandoff == nil {
		if record.RemoteCreateClaim != nil {
			return fmt.Errorf("remote create claim requires supervised execution handoff authority")
		}
		return nil
	}
	if record.RemoteArtifact != nil && record.RemoteCreateClaim != nil {
		return fmt.Errorf("remote create claim and remote artifact are mutually exclusive")
	}
	h := record.ExecutionHandoff
	if err := validateHandoffExternalStringBounds(h); err != nil {
		return err
	}
	if h.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported execution handoff protocol_version %d", h.ProtocolVersion)
	}
	if h.Driver != "orca" {
		return fmt.Errorf("execution handoff driver must be orca")
	}
	if _, err := NormalizeAgent(h.Agent); err != nil || h.Agent != strings.TrimSpace(strings.ToLower(h.Agent)) {
		return fmt.Errorf("execution handoff agent must be canonical codex, claude, or gjc")
	}
	if h.Attempt < 1 || !canonicalNonSpace(h.OwnershipEpoch) {
		return fmt.Errorf("execution handoff requires a positive attempt and ownership epoch")
	}
	if h.CoordinatorRoot == "" || h.WorkerRoot == "" || !filepath.IsAbs(record.Repo) || !filepath.IsAbs(h.CoordinatorRoot) || !filepath.IsAbs(h.WorkerRoot) || h.CoordinatorRoot != strings.TrimSpace(h.CoordinatorRoot) || h.WorkerRoot != strings.TrimSpace(h.WorkerRoot) {
		return fmt.Errorf("execution handoff requires coordinator and worker roots")
	}
	canonicalRepo := filepath.Clean(record.Repo)
	if record.Repo != canonicalRepo || h.CoordinatorRoot != canonicalRepo {
		return fmt.Errorf("execution handoff coordinator root must match the IssueOps repository")
	}
	expectedWorkerRoot := filepath.Join(canonicalRepo+".worktrees", strings.ReplaceAll(record.Branch, "/", "-"))
	if h.WorkerRoot != expectedWorkerRoot || existingPathIsSymlink(h.WorkerRoot) || existingPathIsSymlink(filepath.Dir(h.WorkerRoot)) {
		return fmt.Errorf("execution handoff worker root must be the exact canonical flat worktree path")
	}
	if !fullCommitPattern.MatchString(h.AttemptBaseHead) {
		return fmt.Errorf("execution handoff attempt base head must be a full lowercase commit SHA")
	}
	sealedContext := h.ContextVersion != 0 || h.ContextSHA256 != "" || h.ContextSourceSHA256 != ""
	if sealedContext && (h.ContextVersion != ContextVersion || !sha256Pattern.MatchString(h.ContextSHA256) || !sha256Pattern.MatchString(h.ContextSourceSHA256) || h.ContextOptions == nil) {
		return fmt.Errorf("sealed execution handoff context is incomplete")
	}
	if !sealedContext && h.ContextOptions != nil && h.State != StateCoordinatorPreparing && h.State != StateRecoveryRequired {
		return fmt.Errorf("unsealed context options are permitted only while preparing or recovering")
	}
	if h.ContextOptions != nil {
		canonical := CanonicalContextOptions(ContextOptionsFromModel(*h.ContextOptions))
		if !contextOptionsEqual(canonical, *h.ContextOptions) || validateContextOptionBounds(*h.ContextOptions) != nil {
			return fmt.Errorf("execution handoff context options are not canonical or bounded")
		}
	}
	if h.Failure != nil && !validFailure(h.Failure) {
		return fmt.Errorf("execution handoff failure evidence is not canonical")
	}
	if h.PublicationRecovery != nil && (!validFailure(h.PublicationRecovery) || h.State != StateClosed || h.ClosedDisposition != DispositionAccepted || h.PublicationRecovery.Code != "publication_inventory_ambiguous" && h.PublicationRecovery.Code != "publication_writer_conflict") {
		return fmt.Errorf("publication recovery evidence is not canonical accepted authority")
	}
	if h.Orca != nil && !canonicalOrcaIdentity(h.Orca) {
		return fmt.Errorf("execution handoff Orca identity is not canonical")
	}
	if h.Orca != nil && (h.Orca.DispatchID == "") != (h.Orca.WorkerMailboxHandle == "") {
		return fmt.Errorf("execution handoff Orca dispatch id and worker mailbox must be paired")
	}
	if h.Orca != nil && h.Orca.ProviderIssueLinkStatus != "" && h.Orca.ProviderIssueLinkStatus != ProviderIssueLinkGitLabUnavailable && h.Orca.ProviderIssueLinkStatus != ProviderIssueLinkGitLabExact {
		return fmt.Errorf("execution handoff provider issue link status is unknown")
	}
	gitLabProvider := record.BranchPrepare != nil && strings.EqualFold(strings.TrimSpace(record.BranchPrepare.Provider), "gitlab")
	if h.Orca != nil && h.Orca.ProviderIssueLinkStatus != "" && !gitLabProvider {
		return fmt.Errorf("execution handoff GitLab provider issue link status requires GitLab authority")
	}
	if h.Orca != nil && h.Orca.WorktreeID != "" && gitLabProvider && h.Orca.ProviderIssueLinkStatus == "" {
		return fmt.Errorf("provisioned GitLab handoff requires a provider issue link status")
	}
	if h.Orca != nil && (h.Orca.WorkerTabID == "") != (h.Orca.WorkerLeafID == "") {
		return fmt.Errorf("execution handoff stable terminal tab/leaf identity is incomplete")
	}
	if h.CoordinatorSession != nil && (strings.TrimSpace(h.CoordinatorMailboxHandle) == "" || !validWorkerSession(h.CoordinatorSession)) {
		return fmt.Errorf("execution handoff coordinator native session is not canonical")
	}
	if err := validateWorkerDoneProjection(h); err != nil {
		return err
	}
	projectionAuthority := h.State == StateSubmitted ||
		h.State == StateRecoveryRequired && h.Cancellation != nil ||
		h.State == StateClosed && (h.ClosedDisposition == DispositionAccepted || h.ClosedDisposition == DispositionCancelled)
	if h.WorkerDoneProjection != nil && !projectionAuthority {
		return fmt.Errorf("worker_done projection requires submitted, cancelling, accepted, or cancelled authority")
	}
	if err := validatePublishReceipt(record, h); err != nil {
		return err
	}
	if err := validateRemoteCreateClaim(record, h); err != nil {
		return err
	}
	if err := validateCleanup(h); err != nil {
		return err
	}
	if err := validatePriorAttempts(record, h); err != nil {
		return err
	}

	knownDisposition := h.ClosedDisposition == DispositionAccepted || h.ClosedDisposition == DispositionWorkerFailed || h.ClosedDisposition == DispositionCancelled
	if h.PendingOperation != nil && h.CleanupOnly != nil {
		return fmt.Errorf("execution handoff cannot have both a pending operation and cleanup-only evidence")
	}
	if h.State != StateRecoveryRequired && h.Cancellation != nil {
		return fmt.Errorf("execution handoff cancellation tombstone requires recovery_required")
	}
	switch h.State {
	case StateCoordinatorPreparing:
		if h.ClosedDisposition != "" || h.CleanupOnly != nil || h.Cancellation != nil || h.WorkerSession != nil || h.Result != nil || h.Failure != nil || h.DeliveryMode != "" || h.DispatchedAt != "" || h.ClaimedAt != "" || h.LastHeartbeatAt != "" || h.CompletedAt != "" || h.AcceptedAt != "" {
			return fmt.Errorf("coordinator_preparing handoff contains state-incompatible fields")
		}
	case StateRecoveryRequired:
		if h.Cancellation != nil {
			if h.ClosedDisposition != "" || !validFailure(h.Failure) || h.Failure.Code != "cancellation_requested" || !validCancellation(h.Cancellation) {
				return fmt.Errorf("recovery_required cancellation tombstone is incomplete")
			}
		} else if h.ClosedDisposition != "" || (h.PendingOperation == nil) == (h.CleanupOnly == nil) || !validFailure(h.Failure) || h.WorkerSession != nil || h.Result != nil || h.DeliveryMode != "" || h.DispatchedAt != "" || h.ClaimedAt != "" || h.LastHeartbeatAt != "" || h.CompletedAt != "" || h.AcceptedAt != "" {
			return fmt.Errorf("recovery_required handoff requires exactly one pending operation or cleanup-only artifact")
		}
	case StateDispatched:
		if h.ClosedDisposition != "" || h.PendingOperation != nil || h.CleanupOnly != nil || !sealedContext || !completeDispatchedOrcaIdentity(h) || h.WorkerSession != nil || h.Result != nil || h.Failure != nil || h.ClaimedAt != "" || h.LastHeartbeatAt != "" || h.CompletedAt != "" || h.AcceptedAt != "" {
			return fmt.Errorf("dispatched handoff cannot have a disposition or pending operation")
		}
	case StateClaimed:
		if h.ClosedDisposition != "" || h.PendingOperation != nil || h.CleanupOnly != nil || !sealedContext || !completeDispatchedOrcaIdentity(h) || !validWorkerSession(h.WorkerSession) || h.Result != nil || h.Failure != nil || h.CompletedAt != "" || h.AcceptedAt != "" {
			return fmt.Errorf("claimed handoff requires a worker session and no disposition or pending operation")
		}
	case StateSubmitted:
		if h.ClosedDisposition != "" || h.PendingOperation != nil || h.CleanupOnly != nil || !sealedContext || !completeDispatchedOrcaIdentity(h) || !validWorkerSession(h.WorkerSession) || !validCompletedResult(h) || h.Failure != nil || h.AcceptedAt != "" {
			return fmt.Errorf("submitted handoff requires a completed worker result and no disposition or pending operation")
		}
	case StateClosed:
		if !knownDisposition || h.PendingOperation != nil {
			return fmt.Errorf("closed handoff requires a known disposition and no pending operation")
		}
		switch h.ClosedDisposition {
		case DispositionAccepted:
			if h.CleanupOnly != nil || !sealedContext || !completeDispatchedOrcaIdentity(h) || !validWorkerSession(h.WorkerSession) || !validCompletedResult(h) || h.Failure != nil {
				return fmt.Errorf("closed accepted handoff requires a completed result")
			}
		case DispositionWorkerFailed:
			if h.CleanupOnly != nil || !sealedContext || !completeDispatchedOrcaIdentity(h) || !validWorkerSession(h.WorkerSession) || !validFailedResult(h) || h.Failure != nil {
				return fmt.Errorf("closed worker_failed handoff requires a failed result")
			}
		case DispositionCancelled:
			if h.AcceptedAt != "" || h.WorkerSession != nil && !validWorkerSession(h.WorkerSession) || h.Result != nil && !validCompletedResult(h) {
				return fmt.Errorf("closed cancelled handoff contains state-incompatible fields")
			}
			if h.WorkerSession != nil || h.Result != nil || h.DeliveryMode != "" {
				if !sealedContext || !completeDispatchedOrcaIdentity(h) {
					return fmt.Errorf("closed cancelled dispatched evidence is incomplete")
				}
			}
		}
	default:
		return fmt.Errorf("unknown execution handoff state")
	}
	if h.PendingOperation != nil && !knownOperation(h.PendingOperation.Kind) {
		return fmt.Errorf("unknown execution handoff pending operation")
	}
	if h.PendingOperation != nil && (h.PendingOperation.StartedAt == "" || !canonicalTimestamp(h.PendingOperation.StartedAt)) {
		return fmt.Errorf("execution handoff pending operation requires a canonical timestamp")
	}
	if h.PendingOperation != nil {
		if h.PendingOperation.Kind == OperationDispatch {
			if !canonicalNonSpace(h.PendingOperation.ExpectedAssigneeHandle) || h.PendingOperation.DeliveryMode != "inject" {
				return fmt.Errorf("dispatch pending operation requires an exact assignee and inject delivery")
			}
		} else if h.PendingOperation.ExpectedAssigneeHandle != "" || h.PendingOperation.DeliveryMode != "" {
			return fmt.Errorf("non-dispatch pending operation cannot contain dispatch delivery identity")
		}
		for kind, values := range map[string][]string{
			"worktree": h.PendingOperation.BaselineWorktreeIDs,
			"task":     h.PendingOperation.BaselineTaskIDs,
			"terminal": h.PendingOperation.BaselinePTYIDs,
		} {
			canonical, err := CanonicalBaselineIDs(kind, values)
			if err != nil || !reflect.DeepEqual(canonical, values) {
				return fmt.Errorf("invalid %s pending baseline", kind)
			}
		}
	}
	if h.Orca != nil {
		canonical, err := CanonicalBaselineIDs("terminal", h.Orca.TerminalBaselinePTYIDs)
		if err != nil || !reflect.DeepEqual(canonical, h.Orca.TerminalBaselinePTYIDs) {
			return fmt.Errorf("invalid persisted terminal baseline")
		}
	}
	if h.Result != nil && h.Result.Outcome != OutcomeCompleted && h.Result.Outcome != OutcomeFailed {
		return fmt.Errorf("unknown execution handoff result outcome")
	}
	if handoffRequiresDispatchedIdentity(h) {
		if record.WorktreePath != h.WorkerRoot || h.Orca == nil || h.Orca.WorktreePath != h.WorkerRoot {
			return fmt.Errorf("dispatched execution handoff worktree paths must exactly match the canonical worker root")
		}
	}
	if h.CleanupOnly != nil {
		artifact := h.CleanupOnly
		if artifact.Kind != "worktree" || artifact.ID == "" || artifact.ID != strings.TrimSpace(artifact.ID) || artifact.InstanceID == "" || artifact.Path == "" || artifact.Reason == "" || artifact.Reason != redact(artifact.Reason) {
			return fmt.Errorf("invalid cleanup-only worktree evidence")
		}
		expectedFailureCode := "worktree_cleanup_only"
		if h.Cancellation != nil {
			expectedFailureCode = "cancellation_requested"
		} else if h.State == StateClosed && h.ClosedDisposition == DispositionCancelled {
			expectedFailureCode = "cancellation_finalized"
		}
		if h.Failure == nil || h.Failure.Code != expectedFailureCode || h.Orca != nil && (h.Orca.WorktreeID != "" || h.Orca.WorktreePath != "") {
			return fmt.Errorf("cleanup-only worktree evidence requires an explicit cleanup-only failure")
		}
	}
	if !validOptionalTimestamps(h) {
		return fmt.Errorf("execution handoff timestamps must be canonical RFC3339 values")
	}
	return nil
}

func validatePriorAttempts(record model.IssueOpsRecord, current *model.IssueOpsExecutionHandoff) error {
	if len(current.PriorAttempts) > MaxPriorAttempts {
		return fmt.Errorf("execution handoff prior attempts exceed %d entries", MaxPriorAttempts)
	}
	encodedHistory, err := json.Marshal(current.PriorAttempts)
	if err != nil {
		return fmt.Errorf("encode execution handoff prior attempts: %w", err)
	}
	if len(encodedHistory) > MaxPriorAttemptsBytes {
		return fmt.Errorf("execution handoff prior attempts exceed %d bytes", MaxPriorAttemptsBytes)
	}
	previousAttempt := 0
	for _, prior := range current.PriorAttempts {
		if prior.Attempt <= previousAttempt || prior.Attempt >= current.Attempt {
			return fmt.Errorf("execution handoff prior attempts are not strictly ordered before the current attempt")
		}
		if prior.State != StateClosed || prior.ClosedDisposition != DispositionWorkerFailed && prior.ClosedDisposition != DispositionCancelled {
			return fmt.Errorf("execution handoff prior attempt is not a retryable terminal attempt")
		}
		encoded, err := json.Marshal(prior)
		if err != nil {
			return fmt.Errorf("encode execution handoff prior attempt: %w", err)
		}
		var attempt model.IssueOpsExecutionHandoff
		if err := json.Unmarshal(encoded, &attempt); err != nil {
			return fmt.Errorf("decode execution handoff prior attempt: %w", err)
		}
		priorRecord := record
		priorRecord.ExecutionHandoff = &attempt
		if err := ValidateEnvelope(priorRecord); err != nil {
			return fmt.Errorf("invalid execution handoff prior attempt: %w", err)
		}
		previousAttempt = prior.Attempt
	}
	return nil
}

// SnapshotPriorAttempt copies one terminal attempt into the deliberately
// nonrecursive audit DTO. PriorAttempts is absent from the destination type.
func SnapshotPriorAttempt(current *model.IssueOpsExecutionHandoff) (model.IssueOpsExecutionHandoffPriorAttempt, error) {
	if current == nil {
		return model.IssueOpsExecutionHandoffPriorAttempt{}, fmt.Errorf("execution handoff is required")
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		return model.IssueOpsExecutionHandoffPriorAttempt{}, fmt.Errorf("encode execution handoff prior attempt: %w", err)
	}
	var snapshot model.IssueOpsExecutionHandoffPriorAttempt
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return model.IssueOpsExecutionHandoffPriorAttempt{}, fmt.Errorf("decode execution handoff prior attempt: %w", err)
	}
	return snapshot, nil
}

func contextOptionsEqual(left, right model.IssueOpsExecutionHandoffContextOptions) bool {
	return stringListEqual(left.CriteriaIDs, right.CriteriaIDs) && stringListEqual(left.RequiredDocs, right.RequiredDocs) && stringListEqual(left.RequiredSkills, right.RequiredSkills) &&
		left.WorkerScope == right.WorkerScope && stringListEqual(left.VerificationCommands, right.VerificationCommands) && left.HeartbeatCadence == right.HeartbeatCadence &&
		stringListEqual(left.StopConditions, right.StopConditions) && left.ResultFormat == right.ResultFormat && left.AllowCodexHookTrustBypass == right.AllowCodexHookTrustBypass
}

func stringListEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// NormalizeAgent provides one host identity for probe, persistence, and
// terminal creation. Empty input keeps the public Orca default of Codex.
func NormalizeAgent(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = "codex"
	}
	switch value {
	case "codex", "claude", "gjc":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported handoff agent; expected codex, claude, or gjc")
	}
}

func handoffRequiresDispatchedIdentity(h *model.IssueOpsExecutionHandoff) bool {
	if h == nil {
		return false
	}
	switch h.State {
	case StateDispatched, StateClaimed, StateSubmitted:
		return true
	case StateClosed:
		return h.ClosedDisposition == DispositionAccepted || h.ClosedDisposition == DispositionWorkerFailed
	default:
		return false
	}
}

func completeDispatchedOrcaIdentity(h *model.IssueOpsExecutionHandoff) bool {
	if h == nil || h.Orca == nil || h.DeliveryMode != "inject" {
		return false
	}
	o := h.Orca
	for _, value := range []string{o.RuntimeID, o.RepoID, o.BaseRef, o.WorktreeID, o.WorktreeInstanceID, o.WorktreePath, o.WorkerPTYID, o.WorkerTerminalHandle, o.WorkerMailboxHandle, o.TaskID, o.DispatchID} {
		if value == "" || value != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}

func validateHandoffExternalStringBounds(h *model.IssueOpsExecutionHandoff) error {
	checks := []struct {
		name  string
		value string
		max   int
	}{
		{name: "state", value: h.State, max: 64},
		{name: "closed disposition", value: h.ClosedDisposition, max: 64},
		{name: "driver", value: h.Driver, max: 32},
		{name: "agent", value: h.Agent, max: 32},
		{name: "delivery mode", value: h.DeliveryMode, max: 32},
		{name: "ownership epoch", value: h.OwnershipEpoch, max: 512},
		{name: "coordinator root", value: h.CoordinatorRoot, max: 4096},
		{name: "coordinator mailbox handle", value: h.CoordinatorMailboxHandle, max: 256},
		{name: "worker root", value: h.WorkerRoot, max: 4096},
		{name: "attempt base head", value: h.AttemptBaseHead, max: 128},
		{name: "context sha256", value: h.ContextSHA256, max: 64},
		{name: "context source sha256", value: h.ContextSourceSHA256, max: 64},
		{name: "prepared timestamp", value: h.PreparedAt, max: 128},
		{name: "provisioned timestamp", value: h.ProvisionedAt, max: 128},
		{name: "dispatched timestamp", value: h.DispatchedAt, max: 128},
		{name: "claimed timestamp", value: h.ClaimedAt, max: 128},
		{name: "heartbeat timestamp", value: h.LastHeartbeatAt, max: 128},
		{name: "completed timestamp", value: h.CompletedAt, max: 128},
		{name: "accepted timestamp", value: h.AcceptedAt, max: 128},
		{name: "updated timestamp", value: h.UpdatedAt, max: 128},
	}
	if h.WorkerSession != nil {
		checks = append(checks,
			boundedHandoffString{"worker host", h.WorkerSession.Host, 32},
			boundedHandoffString{"worker session id", h.WorkerSession.SessionID, 1024},
			boundedHandoffString{"worker agent id", h.WorkerSession.AgentID, 1024},
		)
	}
	if h.CoordinatorSession != nil {
		checks = append(checks,
			boundedHandoffString{"coordinator host", h.CoordinatorSession.Host, 32},
			boundedHandoffString{"coordinator session id", h.CoordinatorSession.SessionID, 1024},
			boundedHandoffString{"coordinator agent id", h.CoordinatorSession.AgentID, 1024},
		)
	}
	if h.PendingOperation != nil {
		checks = append(checks,
			boundedHandoffString{"pending operation", h.PendingOperation.Kind, 64},
			boundedHandoffString{"pending started timestamp", h.PendingOperation.StartedAt, 128},
			boundedHandoffString{"pending expected assignee", h.PendingOperation.ExpectedAssigneeHandle, MaxExternalIDBytes},
			boundedHandoffString{"pending delivery mode", h.PendingOperation.DeliveryMode, 32},
		)
	}
	if h.Result != nil {
		checks = append(checks,
			boundedHandoffString{"result outcome", h.Result.Outcome, 32},
			boundedHandoffString{"result final head", h.Result.FinalHead, 128},
			boundedHandoffString{"Turing report path", h.Result.TuringReportPath, 4096},
			boundedHandoffString{"evidence digest", h.Result.EvidenceDigest, 8192},
			boundedHandoffString{"result task id", h.Result.TaskID, MaxExternalIDBytes},
			boundedHandoffString{"result dispatch id", h.Result.DispatchID, MaxExternalIDBytes},
		)
	}
	if h.WorkerDoneProjection != nil {
		p := h.WorkerDoneProjection
		checks = append(checks,
			boundedHandoffString{"worker_done projection state", p.State, 32},
			boundedHandoffString{"worker_done diagnostic code", p.DiagnosticCode, 128},
			boundedHandoffString{"worker_done payload sha256", p.PayloadSHA256, 64},
			boundedHandoffString{"worker_done from handle", p.FromHandle, 256},
			boundedHandoffString{"worker_done to handle", p.ToHandle, 256},
			boundedHandoffString{"worker_done subject", p.Subject, 256},
			boundedHandoffString{"worker_done body", p.Body, 4096},
			boundedHandoffString{"worker_done task id", p.TaskID, MaxExternalIDBytes},
			boundedHandoffString{"worker_done dispatch id", p.DispatchID, MaxExternalIDBytes},
			boundedHandoffString{"worker_done final head", p.FinalHead, 128},
			boundedHandoffString{"worker_done report path", p.ReportPath, 4096},
			boundedHandoffString{"worker_done host identity", p.HostIdentity, 4096},
			boundedHandoffString{"worker_done message id", p.MessageID, MaxExternalIDBytes},
			boundedHandoffString{"worker_done started timestamp", p.StartedAt, 128},
			boundedHandoffString{"worker_done completed timestamp", p.CompletedAt, 128},
		)
		if len(p.ChangedFiles) > 512 {
			return fmt.Errorf("worker_done changed files exceed 512 entries")
		}
		for _, path := range p.ChangedFiles {
			checks = append(checks, boundedHandoffString{"worker_done changed file", path, 4096})
		}
	}
	if h.Failure != nil {
		checks = append(checks,
			boundedHandoffString{"failure code", h.Failure.Code, 128},
			boundedHandoffString{"failure message", h.Failure.Message, 8192},
			boundedHandoffString{"failure timestamp", h.Failure.At, 128},
		)
	}
	if h.Cancellation != nil {
		checks = append(checks,
			boundedHandoffString{"cancellation requested timestamp", h.Cancellation.RequestedAt, 128},
			boundedHandoffString{"cancellation reason", h.Cancellation.Reason, 4096},
		)
	}
	if h.PublishReceipt != nil {
		p := h.PublishReceipt
		checks = append(checks,
			boundedHandoffString{"publish provider", p.Provider, 32},
			boundedHandoffString{"publish project key", p.ProjectKey, 4096},
			boundedHandoffString{"publish remote", p.Remote, 256},
			boundedHandoffString{"publish push target fingerprint", p.PushTargetSHA256, 64},
			boundedHandoffString{"publish branch", p.Branch, 1024},
			boundedHandoffString{"publish base", p.Base, 1024},
			boundedHandoffString{"publish remote ref", p.RemoteRef, 2048},
			boundedHandoffString{"publish final head", p.FinalHead, 128},
			boundedHandoffString{"publish verified timestamp", p.VerifiedAt, 128},
		)
	}
	if h.PublicationRecovery != nil {
		checks = append(checks,
			boundedHandoffString{"publication recovery code", h.PublicationRecovery.Code, 128},
			boundedHandoffString{"publication recovery message", h.PublicationRecovery.Message, 4096},
			boundedHandoffString{"publication recovery timestamp", h.PublicationRecovery.At, 128},
		)
	}
	if h.Cleanup != nil {
		checks = append(checks,
			boundedHandoffString{"cleanup disposition", h.Cleanup.Disposition, 32},
			boundedHandoffString{"cleanup reason", h.Cleanup.Reason, 4096},
			boundedHandoffString{"cleanup approved timestamp", h.Cleanup.ApprovedAt, 128},
		)
		if h.ProtocolVersion != OwnershipTransferProtocolVersion && len(h.Cleanup.Receipts) > 3 {
			return fmt.Errorf("cleanup receipts exceed 3 entries")
		}
		for _, receipt := range h.Cleanup.Receipts {
			checks = append(checks,
				boundedHandoffString{"cleanup receipt step", receipt.Step, 64},
				boundedHandoffString{"cleanup receipt task id", receipt.TaskID, MaxExternalIDBytes},
				boundedHandoffString{"cleanup receipt dispatch id", receipt.DispatchID, MaxExternalIDBytes},
				boundedHandoffString{"cleanup receipt terminal handle", receipt.TerminalHandle, MaxExternalIDBytes},
				boundedHandoffString{"cleanup receipt PTY id", receipt.PTYID, MaxExternalIDBytes},
				boundedHandoffString{"cleanup receipt worktree id", receipt.WorktreeID, MaxWorktreeBaselineIDBytes},
				boundedHandoffString{"cleanup receipt worktree instance id", receipt.WorktreeInstanceID, MaxExternalIDBytes},
				boundedHandoffString{"cleanup receipt recorded timestamp", receipt.RecordedAt, 128},
			)
		}
	}
	if h.Orca != nil {
		o := h.Orca
		checks = append(checks,
			struct {
				name, value string
				max         int
			}{"runtime id", o.RuntimeID, 1024},
			struct {
				name, value string
				max         int
			}{"repo id", o.RepoID, 4096},
			struct {
				name, value string
				max         int
			}{"base ref", o.BaseRef, 4096},
			struct {
				name, value string
				max         int
			}{"provider issue link status", o.ProviderIssueLinkStatus, 64},
			struct {
				name, value string
				max         int
			}{"worktree id", o.WorktreeID, 4096},
			struct {
				name, value string
				max         int
			}{"worktree instance id", o.WorktreeInstanceID, 1024},
			struct {
				name, value string
				max         int
			}{"worktree path", o.WorktreePath, 4096},
			struct {
				name, value string
				max         int
			}{"worker pty id", o.WorkerPTYID, 1024},
			struct {
				name, value string
				max         int
			}{"worker terminal handle", o.WorkerTerminalHandle, 1024},
			struct {
				name, value string
				max         int
			}{"worker mailbox handle", o.WorkerMailboxHandle, 1024},
			struct {
				name, value string
				max         int
			}{"worker tab id", o.WorkerTabID, 1024},
			struct {
				name, value string
				max         int
			}{"worker leaf id", o.WorkerLeafID, 1024},
			struct {
				name, value string
				max         int
			}{"task id", o.TaskID, 1024},
			struct {
				name, value string
				max         int
			}{"dispatch id", o.DispatchID, 1024},
		)
	}
	if h.CleanupOnly != nil {
		checks = append(checks,
			struct {
				name, value string
				max         int
			}{"cleanup worktree id", h.CleanupOnly.ID, MaxWorktreeBaselineIDBytes},
			struct {
				name, value string
				max         int
			}{"cleanup worktree instance id", h.CleanupOnly.InstanceID, MaxExternalIDBytes},
			struct {
				name, value string
				max         int
			}{"cleanup worktree path", h.CleanupOnly.Path, 4096},
			boundedHandoffString{"cleanup kind", h.CleanupOnly.Kind, 32},
			boundedHandoffString{"cleanup reason", h.CleanupOnly.Reason, 4096},
		)
	}
	for _, check := range checks {
		if len(check.value) > check.max {
			return fmt.Errorf("execution handoff %s exceeds %d bytes", check.name, check.max)
		}
	}
	return nil
}

type boundedHandoffString struct {
	name  string
	value string
	max   int
}

func validWorkerSession(session *model.IssueOpsHostSessionIdentity) bool {
	if session == nil || !canonicalSupportedHost(session.Host) || !canonicalNonSpace(session.SessionID) {
		return false
	}
	return session.AgentID == "" || canonicalNonSpace(session.AgentID)
}

func validCancellation(cancellation *model.IssueOpsExecutionHandoffCancellation) bool {
	return cancellation != nil && canonicalTimestamp(cancellation.RequestedAt) && cancellation.Reason != "" && cancellation.Reason == strings.TrimSpace(cancellation.Reason) && cancellation.Reason == redact(cancellation.Reason)
}

func validatePublishReceipt(record model.IssueOpsRecord, h *model.IssueOpsExecutionHandoff) error {
	p := h.PublishReceipt
	if p == nil {
		return nil
	}
	if h.State != StateClosed || h.ClosedDisposition != DispositionAccepted || h.Result == nil || h.Orca == nil || record.BranchPrepare == nil {
		return fmt.Errorf("publish receipt requires closed accepted authority")
	}
	provider := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
	branch := strings.TrimSpace(record.Branch)
	prefix, suffix := "refs/remotes/", "/"+branch
	baseRef := strings.TrimSpace(h.Orca.BaseRef)
	if provider != "github" && provider != "gitlab" || branch == "" || !strings.HasPrefix(baseRef, prefix) || !strings.HasSuffix(baseRef, suffix) {
		return fmt.Errorf("publish receipt provider or branch authority is invalid")
	}
	remoteName := strings.TrimSuffix(strings.TrimPrefix(baseRef, prefix), suffix)
	projectKey := remote.ProjectKey(record.IssueURL, provider, "issue")
	if remoteName == "" || strings.ContainsAny(remoteName, " \t\r\n") || projectKey == "" || p.Provider != provider || p.ProjectKey != projectKey || p.Remote != remoteName || !sha256Pattern.MatchString(p.PushTargetSHA256) || p.Branch != branch || p.Base != strings.TrimSpace(record.BranchPrepare.BaseBranch) || p.RemoteRef != "refs/heads/"+branch || p.FinalHead != h.Result.FinalHead || !fullCommitPattern.MatchString(p.FinalHead) || !canonicalTimestamp(p.VerifiedAt) {
		return fmt.Errorf("publish receipt does not match accepted provider, branch, ref, and final head")
	}
	return nil
}

func validateRemoteCreateClaim(record model.IssueOpsRecord, h *model.IssueOpsExecutionHandoff) error {
	c := record.RemoteCreateClaim
	if c == nil {
		return nil
	}
	if record.Phase != model.IssueOpsPhasePR || record.RemoteArtifact != nil || h.State != StateClosed || h.ClosedDisposition != DispositionAccepted || h.Result == nil || h.PublishReceipt == nil || record.BranchPrepare == nil {
		return fmt.Errorf("remote create claim requires supervised closed accepted pr authority with no artifact")
	}
	provider := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	}
	projectKey := remote.ProjectKey(record.IssueURL, provider, "issue")
	p := h.PublishReceipt
	bodySum := sha256.Sum256([]byte(c.Body))
	if !remoteCreateClaimIDPattern.MatchString(c.ClaimID) || c.Provider != provider || c.Kind != kind || c.ProjectKey == "" || c.ProjectKey != projectKey ||
		c.Remote != p.Remote || c.RemoteRef != p.RemoteRef || c.PushTargetSHA256 != p.PushTargetSHA256 || !sha256Pattern.MatchString(c.PushTargetSHA256) || c.Head != record.Branch || c.Base != record.BranchPrepare.BaseBranch || c.FinalHead != h.Result.FinalHead ||
		p.Provider != c.Provider || p.ProjectKey != c.ProjectKey || p.Branch != c.Head || p.Base != c.Base || p.FinalHead != c.FinalHead || c.Title == "" || c.Title != strings.TrimSpace(c.Title) || len(c.Title) > 1024 || len(c.Body) > 1<<20 ||
		policy.RedactFreeform(c.Title) != c.Title || policy.RedactFreeform(c.Body) != c.Body || c.BodySHA256 != hex.EncodeToString(bodySum[:]) || !c.Draft || !canonicalTimestamp(c.ClaimedAt) {
		return fmt.Errorf("remote create claim does not match exact publish receipt and accepted authority")
	}
	if c.State != "pending" && c.State != "unknown" || c.State == "pending" && c.InvocationState != "reserved" || c.State == "unknown" && c.InvocationState != "unknown" {
		return fmt.Errorf("remote create claim state is not canonical")
	}
	if validateBoundedStringList(c.Labels, 128, 4096, 128*1024, true) != nil || validateBoundedStringList(c.Assignees, 128, 4096, 128*1024, true) != nil ||
		!reflect.DeepEqual(c.Labels, remote.CleanValues(c.Labels)) || len(c.Labels) == 0 || !reflect.DeepEqual(c.Assignees, remote.CleanValues(c.Assignees)) || len(c.Assignees) == 0 || remote.InvalidAssignee(c.Assignees) != "" {
		return fmt.Errorf("remote create claim labels or assignees are not canonical")
	}
	if c.KnownURL != "" {
		if c.State != "unknown" || remote.ValidateArtifactURL(c.KnownURL, c.Provider, c.Kind) != nil || remote.ValidateArtifactMatchesIssue(record.IssueURL, c.KnownURL, c.Provider, c.Kind) != nil {
			return fmt.Errorf("remote create claim known URL is not canonical authority")
		}
	}
	return nil
}

func validateCleanup(h *model.IssueOpsExecutionHandoff) error {
	cleanup := h.Cleanup
	if cleanup == nil {
		return nil
	}
	if h.State != StateClosed || h.ClosedDisposition != DispositionWorkerFailed && h.ClosedDisposition != DispositionCancelled || h.Orca == nil || h.PublishReceipt != nil {
		return fmt.Errorf("cleanup approval requires closed worker_failed or cancelled authority")
	}
	if cleanup.Disposition != "retry" && cleanup.Disposition != "remove" || cleanup.Reason == "" || cleanup.Reason != strings.TrimSpace(cleanup.Reason) || cleanup.Reason != redact(cleanup.Reason) || !canonicalTimestamp(cleanup.ApprovedAt) {
		return fmt.Errorf("cleanup approval disposition, reason, or timestamp is invalid")
	}
	expected := []string{"task_terminal", "terminal_quiescent", "worktree_removed"}
	if cleanup.Disposition == "retry" && len(cleanup.Receipts) > 2 || cleanup.Disposition == "remove" && len(cleanup.Receipts) > 3 {
		return fmt.Errorf("cleanup receipt count exceeds the approved disposition")
	}
	for i, receipt := range cleanup.Receipts {
		if receipt.Step != expected[i] || !canonicalTimestamp(receipt.RecordedAt) {
			return fmt.Errorf("cleanup receipts are out of order or have invalid timestamps")
		}
		switch receipt.Step {
		case "task_terminal":
			if tasklessPreDispatchCancellation(h) {
				if receipt.TaskID != "" || receipt.DispatchID != "" || receipt.TerminalHandle != "" || receipt.PTYID != "" || receipt.WorktreeID != "" || receipt.WorktreeInstanceID != "" {
					return fmt.Errorf("taskless pre-dispatch cleanup receipt contains external identity")
				}
				continue
			}
			if receipt.TaskID != h.Orca.TaskID || receipt.DispatchID != h.Orca.DispatchID || receipt.TaskID == "" || receipt.DispatchID == "" || receipt.TerminalHandle != "" || receipt.PTYID != "" || receipt.WorktreeID != "" || receipt.WorktreeInstanceID != "" {
				return fmt.Errorf("task cleanup receipt does not match exact task and dispatch identity")
			}
		case "terminal_quiescent":
			if terminallessPreDispatchCancellation(h) {
				if receipt.TerminalHandle != "" || receipt.PTYID != "" || receipt.WorktreeID != "" || receipt.TaskID != "" || receipt.DispatchID != "" || receipt.WorktreeInstanceID != "" {
					return fmt.Errorf("terminalless pre-dispatch cleanup receipt contains external identity")
				}
				continue
			}
			if receipt.TerminalHandle != h.Orca.WorkerTerminalHandle || receipt.PTYID != h.Orca.WorkerPTYID || receipt.WorktreeID != h.Orca.WorktreeID || receipt.TerminalHandle == "" || receipt.PTYID == "" || receipt.WorktreeID == "" || receipt.TaskID != "" || receipt.DispatchID != "" || receipt.WorktreeInstanceID != "" {
				return fmt.Errorf("terminal cleanup receipt does not match exact terminal and worktree identity")
			}
		case "worktree_removed":
			if receipt.WorktreeID != h.Orca.WorktreeID || receipt.WorktreeInstanceID != h.Orca.WorktreeInstanceID || receipt.WorktreeID == "" || receipt.WorktreeInstanceID == "" || receipt.TaskID != "" || receipt.DispatchID != "" || receipt.TerminalHandle != "" || receipt.PTYID != "" {
				return fmt.Errorf("worktree cleanup receipt does not match exact worktree identity")
			}
		}
	}
	return nil
}

func tasklessPreDispatchCancellation(h *model.IssueOpsExecutionHandoff) bool {
	return h != nil && h.Orca != nil && h.ClosedDisposition == DispositionCancelled && h.DeliveryMode == "" && h.WorkerSession == nil && h.Result == nil && h.Orca.TaskID == "" && h.Orca.DispatchID == ""
}

func terminallessPreDispatchCancellation(h *model.IssueOpsExecutionHandoff) bool {
	return tasklessPreDispatchCancellation(h) && h.Orca.WorkerPTYID == "" && h.Orca.WorkerTerminalHandle == "" && h.Orca.WorkerMailboxHandle == "" && h.Orca.WorkerTabID == "" && h.Orca.WorkerLeafID == ""
}

func knownOperation(kind string) bool {
	switch kind {
	case OperationWorktreeCreate, OperationTerminalCreate, OperationTaskCreate, OperationDispatch, OperationRuntimeRefresh, OperationLeaseAttestation:
		return true
	default:
		return false
	}
}

func canonicalNonSpace(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && strings.IndexFunc(value, unicode.IsSpace) < 0
}

func canonicalSupportedHost(value string) bool {
	if !canonicalNonSpace(value) {
		return false
	}
	canonical, err := NormalizeAgent(value)
	return err == nil && canonical == value
}

func existingPathIsSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func validateContextOptionBounds(options model.IssueOpsExecutionHandoffContextOptions) error {
	for _, values := range [][]string{options.CriteriaIDs, options.RequiredDocs, options.RequiredSkills, options.VerificationCommands, options.StopConditions} {
		if err := validateBoundedStringList(values, 128, 4096, 128*1024, true); err != nil {
			return err
		}
	}
	for _, value := range []string{options.WorkerScope, options.HeartbeatCadence, options.ResultFormat} {
		if len(value) > 4096 || value != redact(value) {
			return fmt.Errorf("context option is not bounded and redacted")
		}
	}
	return nil
}

func validateBoundedStringList(values []string, maxCount, maxItem, maxTotal int, trimCanonical bool) error {
	if len(values) > maxCount {
		return fmt.Errorf("list exceeds bounded item count")
	}
	total := 0
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" || len(value) > maxItem || trimCanonical && value != strings.TrimSpace(value) || value != redact(value) {
			return fmt.Errorf("list contains a noncanonical item")
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("list contains a duplicate item")
		}
		seen[value] = struct{}{}
		total += len(value)
		if total > maxTotal {
			return fmt.Errorf("list exceeds bounded total bytes")
		}
	}
	return nil
}

func validFailure(failure *model.IssueOpsExecutionHandoffFailure) bool {
	return failure != nil && canonicalNonSpace(failure.Code) && (failure.Message == "" || failure.Message == redact(failure.Message)) && canonicalTimestamp(failure.At)
}

func validCompletedResult(h *model.IssueOpsExecutionHandoff) bool {
	if h == nil || h.Result == nil || h.Result.Outcome != OutcomeCompleted || !fullCommitPattern.MatchString(h.Result.FinalHead) || !safeRelativeHandoffPath(h.Result.TuringReportPath) {
		return false
	}
	if !validChangedFiles(h.Result.ChangedFiles) || len(h.Result.Verification) == 0 || len(h.Result.CleanupReceipts) == 0 {
		return false
	}
	return validResultIdentityAndEvidence(h)
}

func validFailedResult(h *model.IssueOpsExecutionHandoff) bool {
	if h == nil || h.Result == nil || h.Result.Outcome != OutcomeFailed {
		return false
	}
	if h.Result.FinalHead != "" && !fullCommitPattern.MatchString(h.Result.FinalHead) || !validChangedFiles(h.Result.ChangedFiles) {
		return false
	}
	if h.Result.TuringReportPath != "" && !safeRelativeHandoffPath(h.Result.TuringReportPath) {
		return false
	}
	return validResultIdentityAndEvidence(h)
}

func validResultIdentityAndEvidence(h *model.IssueOpsExecutionHandoff) bool {
	result := h.Result
	if h.Orca == nil || result.TaskID == "" || result.DispatchID == "" || result.TaskID != h.Orca.TaskID || result.DispatchID != h.Orca.DispatchID {
		return false
	}
	if result.EvidenceDigest != redact(result.EvidenceDigest) || validateBoundedStringList(result.Verification, 128, 4096, 128*1024, true) != nil || validateBoundedStringList(result.CleanupReceipts, 128, 4096, 128*1024, true) != nil {
		return false
	}
	return reflect.DeepEqual(cleanResultList(result.Verification), result.Verification) && reflect.DeepEqual(cleanResultList(result.CleanupReceipts), result.CleanupReceipts)
}

func validChangedFiles(values []string) bool {
	if len(values) > 512 {
		return false
	}
	seen := map[string]struct{}{}
	total := 0
	for _, value := range values {
		if !safeRelativeHandoffPath(value) || len(value) > 4096 {
			return false
		}
		if _, ok := seen[value]; ok {
			return false
		}
		seen[value] = struct{}{}
		total += len(value)
		if total > 256*1024 {
			return false
		}
	}
	return true
}

func safeRelativeHandoffPath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.ContainsRune(value, 0) || filepath.ToSlash(filepath.Clean(value)) != value {
		return false
	}
	return value != "." && value != ".." && !strings.HasPrefix(value, "../")
}

func canonicalTimestamp(value string) bool {
	if value == "" {
		return true
	}
	if value != strings.TrimSpace(value) || len(value) > 128 {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validOptionalTimestamps(h *model.IssueOpsExecutionHandoff) bool {
	for _, value := range []string{h.PreparedAt, h.ProvisionedAt, h.DispatchedAt, h.ClaimedAt, h.LastHeartbeatAt, h.CompletedAt, h.AcceptedAt, h.UpdatedAt} {
		if !canonicalTimestamp(value) {
			return false
		}
	}
	if h.Failure != nil && !canonicalTimestamp(h.Failure.At) {
		return false
	}
	if h.WorkerDoneProjection != nil {
		if !canonicalTimestamp(h.WorkerDoneProjection.StartedAt) || !canonicalTimestamp(h.WorkerDoneProjection.CompletedAt) {
			return false
		}
	}
	return true
}

func validateWorkerDoneProjection(h *model.IssueOpsExecutionHandoff) error {
	p := h.WorkerDoneProjection
	if p == nil {
		return nil
	}
	if p.StartedAt == "" {
		return fmt.Errorf("worker_done projection started_at is required")
	}
	if p.Attempt != h.Attempt || p.OwnershipEpoch == "" || p.OwnershipEpoch != h.OwnershipEpoch || p.DiagnosticCode == "" || p.DiagnosticCode != strings.TrimSpace(p.DiagnosticCode) {
		return fmt.Errorf("worker_done projection attempt identity or diagnostic is invalid")
	}
	if p.PayloadSHA256 == "" {
		if p.State != "failed" || p.Invoked || p.CompletedAt == "" || p.FromHandle != "" || p.ToHandle != "" || p.Subject != "" || p.Body != "" || p.TaskID != "" || p.DispatchID != "" || p.FinalHead != "" || len(p.ChangedFiles) != 0 || p.ReportPath != "" || p.HostIdentity != "" || p.MessageID != "" || p.MessageSequence != 0 {
			return fmt.Errorf("worker_done precondition diagnostic contains mutation evidence")
		}
		return nil
	}
	if !sha256Pattern.MatchString(p.PayloadSHA256) || h.Result == nil || h.Orca == nil || h.WorkerSession == nil {
		return fmt.Errorf("worker_done projection payload evidence is incomplete")
	}
	hostIdentity := h.WorkerSession.Host + "/" + h.WorkerSession.SessionID
	if h.WorkerSession.AgentID != "" {
		hostIdentity += "/" + h.WorkerSession.AgentID
	}
	expectedReport := filepath.Clean(filepath.Join(h.WorkerRoot, filepath.FromSlash(h.Result.TuringReportPath)))
	if p.FromHandle != h.Orca.WorkerMailboxHandle || p.ToHandle != h.CoordinatorMailboxHandle || p.TaskID != h.Result.TaskID || p.TaskID != h.Orca.TaskID || p.DispatchID != h.Result.DispatchID || p.DispatchID != h.Orca.DispatchID || p.FinalHead != h.Result.FinalHead || !reflect.DeepEqual(p.ChangedFiles, h.Result.ChangedFiles) || p.ReportPath != expectedReport || p.HostIdentity != hostIdentity {
		return fmt.Errorf("worker_done projection does not match durable handoff evidence")
	}
	payload, err := json.Marshal(struct {
		FromHandle   string   `json:"from_handle"`
		ToHandle     string   `json:"to_handle"`
		Subject      string   `json:"subject"`
		Body         string   `json:"body"`
		TaskID       string   `json:"task_id"`
		DispatchID   string   `json:"dispatch_id"`
		ChangedFiles []string `json:"changed_files"`
		ReportPath   string   `json:"report_path"`
	}{p.FromHandle, p.ToHandle, p.Subject, p.Body, p.TaskID, p.DispatchID, p.ChangedFiles, p.ReportPath})
	if err != nil {
		return fmt.Errorf("encode worker_done projection payload: %w", err)
	}
	sum := sha256.Sum256(payload)
	if p.PayloadSHA256 != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("worker_done projection payload digest mismatch")
	}
	switch p.State {
	case "intent":
		if p.Invoked || p.DiagnosticCode != "intent_persisted" || p.CompletedAt != "" || p.MessageID != "" || p.MessageSequence != 0 {
			return fmt.Errorf("worker_done projection intent contains outcome evidence")
		}
	case "sent":
		if !p.Invoked || p.DiagnosticCode != "sent" || p.CompletedAt == "" || !canonicalNonSpace(p.MessageID) || p.MessageSequence <= 0 {
			return fmt.Errorf("worker_done sent projection evidence is incomplete")
		}
	case "failed":
		if p.DiagnosticCode == "intent_persisted" || p.DiagnosticCode == "sent" || p.CompletedAt == "" || p.MessageID != "" || p.MessageSequence != 0 {
			return fmt.Errorf("worker_done failed projection evidence is invalid")
		}
	default:
		return fmt.Errorf("unknown worker_done projection state")
	}
	return nil
}

func canonicalOrcaIdentity(identity *model.IssueOpsOrcaIdentity) bool {
	if identity == nil {
		return true
	}
	for _, value := range []string{identity.RuntimeID, identity.RepoID, identity.BaseRef, identity.ProviderIssueLinkStatus, identity.WorktreeID, identity.WorktreeInstanceID, identity.WorktreePath, identity.WorkerPTYID, identity.WorkerTerminalHandle, identity.WorkerMailboxHandle, identity.WorkerTabID, identity.WorkerLeafID, identity.TaskID, identity.DispatchID} {
		if value != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}
