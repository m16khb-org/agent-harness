package handoff

import (
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
)

var fullCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

const (
	MaxPriorAttempts      = 16
	MaxPriorAttemptsBytes = 256 * 1024
)

// ValidateEnvelope rejects corrupted or future supervised-handoff state. An
// absent execution_handoff is the only legacy/inline representation.
func ValidateEnvelope(record model.IssueOpsRecord) error {
	if record.ExecutionHandoff == nil {
		return nil
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
	if h.Orca != nil && !canonicalOrcaIdentity(h.Orca) {
		return fmt.Errorf("execution handoff Orca identity is not canonical")
	}
	if err := validatePriorAttempts(record, h); err != nil {
		return err
	}

	knownDisposition := h.ClosedDisposition == DispositionAccepted || h.ClosedDisposition == DispositionWorkerFailed || h.ClosedDisposition == DispositionCancelled
	if h.PendingOperation != nil && h.CleanupOnly != nil {
		return fmt.Errorf("execution handoff cannot have both a pending operation and cleanup-only evidence")
	}
	switch h.State {
	case StateCoordinatorPreparing:
		if h.ClosedDisposition != "" || h.CleanupOnly != nil || h.WorkerSession != nil || h.Result != nil || h.Failure != nil || h.DeliveryMode != "" || h.DispatchedAt != "" || h.ClaimedAt != "" || h.LastHeartbeatAt != "" || h.CompletedAt != "" || h.AcceptedAt != "" {
			return fmt.Errorf("coordinator_preparing handoff contains state-incompatible fields")
		}
	case StateRecoveryRequired:
		if h.ClosedDisposition != "" || (h.PendingOperation == nil) == (h.CleanupOnly == nil) || !validFailure(h.Failure) || h.WorkerSession != nil || h.Result != nil || h.DeliveryMode != "" || h.DispatchedAt != "" || h.ClaimedAt != "" || h.LastHeartbeatAt != "" || h.CompletedAt != "" || h.AcceptedAt != "" {
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
		if h.Failure == nil || h.Failure.Code != "worktree_cleanup_only" || h.Orca != nil && (h.Orca.WorktreeID != "" || h.Orca.WorktreePath != "") {
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
	for _, value := range []string{o.RuntimeID, o.RepoID, o.BaseRef, o.WorktreeID, o.WorktreeInstanceID, o.WorktreePath, o.WorkerPTYID, o.WorkerMailboxHandle, o.TaskID, o.DispatchID} {
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
	if h.Failure != nil {
		checks = append(checks,
			boundedHandoffString{"failure code", h.Failure.Code, 128},
			boundedHandoffString{"failure message", h.Failure.Message, 8192},
			boundedHandoffString{"failure timestamp", h.Failure.At, 128},
		)
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
			}{"worker mailbox handle", o.WorkerMailboxHandle, 1024},
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

func knownOperation(kind string) bool {
	switch kind {
	case OperationWorktreeCreate, OperationTerminalCreate, OperationTaskCreate, OperationDispatch:
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
	if len(h.Result.ChangedFiles) == 0 || !validChangedFiles(h.Result.ChangedFiles) || len(h.Result.Verification) == 0 || len(h.Result.CleanupReceipts) == 0 {
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
	return h.Failure == nil || canonicalTimestamp(h.Failure.At)
}

func canonicalOrcaIdentity(identity *model.IssueOpsOrcaIdentity) bool {
	if identity == nil {
		return true
	}
	for _, value := range []string{identity.RuntimeID, identity.RepoID, identity.BaseRef, identity.WorktreeID, identity.WorktreeInstanceID, identity.WorktreePath, identity.WorkerPTYID, identity.WorkerMailboxHandle, identity.TaskID, identity.DispatchID} {
		if value != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}
