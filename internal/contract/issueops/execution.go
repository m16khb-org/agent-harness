package issueops

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	IssueOpsSchemaVersion       = 1
	OrcaArtifactIdentityVersion = 1
)

type ExecutionMode string

const (
	ExecutionModeDirect ExecutionMode = "direct"
	ExecutionModeOrca   ExecutionMode = "orca"
)

type LeaseStatus string

const (
	LeaseStatusClaimable LeaseStatus = "claimable"
	LeaseStatusActive    LeaseStatus = "active"
	LeaseStatusRevoking  LeaseStatus = "revoking"
	LeaseStatusReleased  LeaseStatus = "released"
)

type Execution struct {
	Mode               ExecutionMode                `json:"mode"`
	Selection          *ExecutionSelection          `json:"selection,omitempty"`
	Workspace          Workspace                    `json:"workspace"`
	Lease              WriteLease                   `json:"lease"`
	Orca               *OrcaBinding                 `json:"orca,omitempty"`
	Pending            *ExternalIntent              `json:"pending,omitempty"`
	Completion         *ExecutionCompletion         `json:"completion,omitempty"`
	CompletionHistory  []ExecutionCompletionHistory `json:"completion_history,omitempty"`
	Failure            *ExecutionFailure            `json:"failure,omitempty"`
	SyncBaseResolution *ExecutionSyncBaseResolution `json:"sync_base_resolution,omitempty"`
	// SyncBaseEvents는 completion 이후 base 재동기화(merge+push)의 durable
	// 감사 기록이다. append-only이며 Completion.FinalHead는 불변으로 남는다
	// — 완결 시점 증거를 보존하고, PR head는 provider가 관측하며, merge OID는
	// 이 이벤트가 담당한다(설계 v2 brooks F9). 기존 레코드는 nil이므로
	// 스키마는 additive다.
	SyncBaseEvents []ExecutionSyncBaseEvent `json:"sync_base_events,omitempty"`
}

type ExecutionSelection struct {
	RequestedMode        string `json:"requested_mode"`
	ResolvedMode         string `json:"resolved_mode"`
	ProbeAttempted       bool   `json:"probe_attempted"`
	ProbeAvailable       bool   `json:"probe_available"`
	ProbeReady           bool   `json:"probe_ready"`
	ProbeCode            string `json:"probe_code,omitempty"`
	FallbackCode         string `json:"fallback_code,omitempty"`
	ReadinessFingerprint string `json:"readiness_fingerprint"`
	SelectedAt           string `json:"selected_at"`
	ExplicitDirectReason string `json:"explicit_direct_reason,omitempty"`
}

type Workspace struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	LinkedAt       string `json:"linked_at"`
}

type WriteLease struct {
	Generation        uint64       `json:"generation"`
	Status            LeaseStatus  `json:"status"`
	Holder            *NativeActor `json:"holder,omitempty"`
	ClaimTokenSHA256  string       `json:"claim_token_sha256,omitempty"`
	ClaimedAt         string       `json:"claimed_at,omitempty"`
	ReleasedAt        string       `json:"released_at,omitempty"`
	ReplacedAt        string       `json:"replaced_at,omitempty"`
	ReplacementReason string       `json:"replacement_reason,omitempty"`
}

type NativeActor struct {
	Host           string                `json:"host"`
	SessionID      string                `json:"session_id"`
	AgentID        string                `json:"agent_id,omitempty"`
	SessionProcess *NativeProcessReceipt `json:"session_process,omitempty"`
	// ProcessAncestry is populated by a first-party adapter from the local OS
	// process tree. It is never accepted from JSON or persisted.
	ProcessAncestry []NativeProcessReceipt `json:"-"`
}

type NativeProcessReceipt struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}

type OrcaBinding struct {
	RuntimeID               string `json:"runtime_id"`
	RepoID                  string `json:"repo_id"`
	WorktreeID              string `json:"worktree_id"`
	RunID                   string `json:"run_id,omitempty"`
	WorktreeInstanceID      string `json:"worktree_instance_id,omitempty"`
	LeaseGeneration         uint64 `json:"lease_generation,omitempty"`
	ArtifactIdentityVersion uint64 `json:"artifact_identity_version,omitempty"`
	IssueBodySHA256         string `json:"issue_body_sha256,omitempty"`
	ContextPacketSHA256     string `json:"context_packet_sha256,omitempty"`
	OwnerPromptSHA256       string `json:"owner_prompt_sha256,omitempty"`
	OwnerHost               string `json:"owner_host"`
	OwnerModel              string `json:"owner_model"`
	OwnerEffort             string `json:"owner_effort,omitempty"`
	TaskID                  string `json:"task_id"`
	DispatchID              string `json:"dispatch_id"`
	TerminalPTYID           string `json:"terminal_pty_id,omitempty"`
}

type ExternalIntent struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Marker      string `json:"marker"`
	StartedAt   string `json:"started_at"`
}

type ExecutionCompletion struct {
	Generation        uint64   `json:"generation,omitempty"`
	FinalHead         string   `json:"final_head"`
	TuringReportPath  string   `json:"turing_report_path"`
	Verification      []string `json:"verification"`
	RemoteArtifactURL string   `json:"remote_artifact_url"`
	CompletedAt       string   `json:"completed_at"`
}

type ExecutionCompletionHistory struct {
	Generation uint64              `json:"generation"`
	Completion ExecutionCompletion `json:"completion"`
	Reason     string              `json:"reason"`
	ReopenedAt string              `json:"reopened_at"`
}

type ExecutionFailure struct {
	OperationID string `json:"operation_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
	At          string `json:"at"`
}

const (
	ExecutionSyncBaseEventApply    = "apply"
	ExecutionSyncBaseEventFinalize = "finalize"
)

// ExecutionSyncBaseEvent는 성공한 sync-base 변형 1회의 증거다. abort는
// 되돌림이므로 이벤트를 남기지 않는다(설계 v2 — apply/finalize 성공 시에만 append).
type ExecutionSyncBaseEvent struct {
	Mode          string `json:"mode"` // apply | finalize
	BaseBranch    string `json:"base_branch"`
	BaseOID       string `json:"base_oid"`
	MergeCommit   string `json:"merge_commit"`
	ConflictFiles int    `json:"conflict_files"`
	Actor         string `json:"actor"`
	At            string `json:"at"`
}

// ExecutionSyncBaseResolution seals temporary conflict-resolution authority
// without reopening the released completion as a general write lease.
type ExecutionSyncBaseResolution struct {
	Generation           uint64      `json:"generation"`
	CompletionGeneration uint64      `json:"completion_generation"`
	BaseOID              string      `json:"base_oid"`
	Actor                NativeActor `json:"actor"`
	ConflictFiles        []string    `json:"conflict_files"`
	StartedAt            string      `json:"started_at"`
}

func ValidateExecution(execution Execution) error {
	if execution.Mode != ExecutionModeDirect && execution.Mode != ExecutionModeOrca {
		return fmt.Errorf("execution mode must be direct or orca")
	}
	if err := validateWorkspace(execution.Workspace, execution.Mode); err != nil {
		return err
	}
	if err := validateWriteLease(execution.Lease); err != nil {
		return err
	}
	if execution.Selection != nil {
		if err := validateExecutionSelection(*execution.Selection, execution.Mode); err != nil {
			return err
		}
	}
	if execution.Mode == ExecutionModeDirect && execution.Orca != nil {
		return fmt.Errorf("direct execution must not contain an Orca binding")
	}
	if execution.Orca != nil {
		if err := validateOrcaBinding(*execution.Orca); err != nil {
			return err
		}
		if execution.Orca.LeaseGeneration > execution.Lease.Generation {
			return fmt.Errorf("Orca binding lease_generation exceeds the lease generation")
		}
	}
	if execution.Pending != nil {
		if execution.Pending.OperationID == "" || execution.Pending.Kind == "" || execution.Pending.Marker == "" || execution.Pending.StartedAt == "" {
			return fmt.Errorf("pending external intent is incomplete")
		}
	}
	if execution.Completion != nil {
		if err := validateExecutionCompletion(*execution.Completion); err != nil {
			return err
		}
		if execution.Completion.Generation > execution.Lease.Generation {
			return fmt.Errorf("execution completion generation exceeds the lease generation")
		}
	}
	for _, entry := range execution.CompletionHistory {
		if entry.Generation == 0 || strings.TrimSpace(entry.Reason) == "" || strings.TrimSpace(entry.ReopenedAt) == "" {
			return fmt.Errorf("execution completion history entry is incomplete")
		}
		if entry.Generation >= execution.Lease.Generation {
			return fmt.Errorf("execution completion history generation must precede the lease generation")
		}
		if err := validateExecutionCompletion(entry.Completion); err != nil {
			return fmt.Errorf("execution completion history: %w", err)
		}
		if entry.Completion.Generation != 0 && entry.Completion.Generation != entry.Generation {
			return fmt.Errorf("execution completion history generation conflicts with its completion")
		}
	}
	if execution.Failure != nil {
		if execution.Failure.Code == "" || execution.Failure.At == "" || len(execution.Failure.Message) > 4096 {
			return fmt.Errorf("execution failure is invalid")
		}
	}
	if execution.SyncBaseResolution != nil {
		if err := validateExecutionSyncBaseResolution(execution, *execution.SyncBaseResolution); err != nil {
			return err
		}
	}
	for _, event := range execution.SyncBaseEvents {
		if err := validateExecutionSyncBaseEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func validateExecutionSyncBaseResolution(execution Execution, resolution ExecutionSyncBaseResolution) error {
	if execution.Lease.Status != LeaseStatusReleased || execution.Completion == nil ||
		resolution.Generation == 0 || resolution.Generation != execution.Lease.Generation ||
		resolution.CompletionGeneration == 0 || resolution.CompletionGeneration != execution.Completion.Generation {
		return fmt.Errorf("execution sync-base resolution must bind the released current completion")
	}
	if !validCommitSHA(resolution.BaseOID) || strings.TrimSpace(resolution.StartedAt) == "" || len(resolution.ConflictFiles) == 0 {
		return fmt.Errorf("execution sync-base resolution is incomplete")
	}
	if err := ValidateNativeActor(resolution.Actor); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, path := range resolution.ConflictFiles {
		clean := filepath.Clean(path)
		if path == "" || path != clean || filepath.IsAbs(path) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || seen[clean] {
			return fmt.Errorf("execution sync-base resolution conflict path is invalid")
		}
		seen[clean] = true
	}
	return nil
}

func validateExecutionCompletion(completion ExecutionCompletion) error {
	if !validCommitSHA(completion.FinalHead) || strings.TrimSpace(completion.TuringReportPath) == "" ||
		len(completion.Verification) == 0 || strings.TrimSpace(completion.RemoteArtifactURL) == "" || strings.TrimSpace(completion.CompletedAt) == "" {
		return fmt.Errorf("execution completion is incomplete")
	}
	for _, evidence := range completion.Verification {
		if strings.TrimSpace(evidence) == "" {
			return fmt.Errorf("execution completion verification must be nonempty")
		}
	}
	return nil
}

func validateExecutionSelection(selection ExecutionSelection, mode ExecutionMode) error {
	if selection.RequestedMode != "auto" && selection.RequestedMode != "direct" && selection.RequestedMode != "orca" {
		return fmt.Errorf("selection requested_mode must be auto, direct, or orca")
	}
	if selection.ResolvedMode != string(mode) {
		return fmt.Errorf("selection resolved_mode must equal execution mode")
	}
	if selection.ProbeAvailable && !selection.ProbeAttempted {
		return fmt.Errorf("selection probe_available requires probe_attempted")
	}
	if selection.ProbeReady && !selection.ProbeAvailable {
		return fmt.Errorf("selection probe_ready requires probe_available")
	}
	if !selection.ProbeAttempted && strings.TrimSpace(selection.ProbeCode) != "" {
		return fmt.Errorf("unattempted selection probe must not contain probe_code")
	}
	if selection.RequestedMode != "direct" && !selection.ProbeAttempted {
		return fmt.Errorf("auto and Orca selections require a readiness probe")
	}
	if (selection.RequestedMode == "direct" && mode != ExecutionModeDirect) ||
		(selection.RequestedMode == "orca" && mode != ExecutionModeOrca) {
		return fmt.Errorf("explicit selection mode must equal execution mode")
	}
	if mode == ExecutionModeOrca && !selection.ProbeReady {
		return fmt.Errorf("Orca selection requires a ready probe")
	}
	if selection.RequestedMode == "auto" && mode == ExecutionModeDirect {
		probeCode := strings.TrimSpace(selection.ProbeCode)
		fallbackCode := strings.TrimSpace(selection.FallbackCode)
		if selection.ProbeReady || fallbackCode == "" || fallbackCode != probeCode || fallbackCode != selection.FallbackCode {
			return fmt.Errorf("auto direct selection requires the exact probe failure fallback_code")
		}
	}
	if mode == ExecutionModeOrca && strings.TrimSpace(selection.FallbackCode) != "" {
		return fmt.Errorf("Orca selection must not contain fallback_code")
	}
	if selection.RequestedMode == "direct" {
		if selection.ProbeAttempted || strings.TrimSpace(selection.ExplicitDirectReason) == "" {
			return fmt.Errorf("explicit direct selection requires a reason and no probe")
		}
		if selection.FallbackCode != "" {
			return fmt.Errorf("explicit direct selection must not contain fallback_code")
		}
	} else if strings.TrimSpace(selection.ExplicitDirectReason) != "" {
		return fmt.Errorf("non-direct selection must not contain explicit_direct_reason")
	}
	if !validSHA256(selection.ReadinessFingerprint) || strings.TrimSpace(selection.SelectedAt) == "" {
		return fmt.Errorf("selection receipt requires fingerprint and selected_at")
	}
	return nil
}

func validateExecutionSyncBaseEvent(event ExecutionSyncBaseEvent) error {
	if event.Mode != ExecutionSyncBaseEventApply && event.Mode != ExecutionSyncBaseEventFinalize {
		return fmt.Errorf("execution sync-base event mode must be apply or finalize")
	}
	if !validCommitSHA(event.BaseOID) || !validCommitSHA(event.MergeCommit) {
		return fmt.Errorf("execution sync-base event requires full base and merge commit OIDs")
	}
	if strings.TrimSpace(event.BaseBranch) == "" || strings.TrimSpace(event.Actor) == "" || strings.TrimSpace(event.At) == "" {
		return fmt.Errorf("execution sync-base event is incomplete")
	}
	if event.ConflictFiles < 0 {
		return fmt.Errorf("execution sync-base event conflict count must not be negative")
	}
	return nil
}

func validateWorkspace(workspace Workspace, mode ExecutionMode) error {
	for name, value := range map[string]string{
		"source_root": workspace.SourceRoot,
		"root":        workspace.Root,
		"branch":      workspace.Branch,
		"base_head":   workspace.BaseHead,
		"driver":      workspace.Driver,
		"linked_at":   workspace.LinkedAt,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("workspace %s is required", name)
		}
	}
	if workspace.SourceRoot == workspace.Root {
		return fmt.Errorf("canonical worktree must be isolated from source_root")
	}
	if mode == ExecutionModeDirect && workspace.Driver != "git" {
		return fmt.Errorf("direct execution workspace driver must be git")
	}
	if mode == ExecutionModeOrca && workspace.Driver != "orca" {
		return fmt.Errorf("Orca execution workspace driver must be orca")
	}
	return nil
}

func validateWriteLease(lease WriteLease) error {
	if lease.Generation == 0 {
		return fmt.Errorf("lease generation must start at 1")
	}
	switch lease.Status {
	case LeaseStatusClaimable:
		if lease.Holder != nil || !validSHA256(lease.ClaimTokenSHA256) {
			return fmt.Errorf("claimable lease requires no holder and one token hash")
		}
	case LeaseStatusActive:
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" || lease.ClaimedAt == "" {
			return fmt.Errorf("active lease requires one holder and no token hash")
		}
		if err := ValidateNativeActor(*lease.Holder); err != nil {
			return err
		}
	case LeaseStatusRevoking:
		// 이전 holder는 quiescence 진단에만 남고 이 상태에서는 writer가 아니다.
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("revoking lease requires the fenced holder and no token hash")
		}
		if err := ValidateNativeActor(*lease.Holder); err != nil {
			return err
		}
	case LeaseStatusReleased:
		if lease.Holder != nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("released lease must not retain a holder or token hash")
		}
	default:
		return fmt.Errorf("unsupported lease status %q", lease.Status)
	}
	return nil
}

func ValidateNativeActor(actor NativeActor) error {
	if actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" {
		return fmt.Errorf("native actor host must be codex, claude, or omo")
	}
	if strings.TrimSpace(actor.SessionID) == "" {
		return fmt.Errorf("native actor session_id is required")
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" || actor.SessionProcess.Executable == "" {
		return fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	return nil
}

func validateOrcaBinding(binding OrcaBinding) error {
	for name, value := range map[string]string{
		"runtime_id":  binding.RuntimeID,
		"repo_id":     binding.RepoID,
		"worktree_id": binding.WorktreeID,
		"owner_host":  binding.OwnerHost,
		"owner_model": binding.OwnerModel,
		"task_id":     binding.TaskID,
		"dispatch_id": binding.DispatchID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Orca binding %s is required", name)
		}
	}
	if binding.OwnerHost != "codex" && binding.OwnerHost != "claude" {
		return fmt.Errorf("Orca owner_host must be codex or claude")
	}
	if binding.RunID != "" && (binding.RunID != strings.TrimSpace(binding.RunID) || len(binding.RunID) > 1024) {
		return fmt.Errorf("Orca binding run_id must be one canonical explicit Run identity")
	}
	digests := []string{binding.IssueBodySHA256, binding.ContextPacketSHA256, binding.OwnerPromptSHA256}
	present := 0
	for _, digest := range digests {
		if digest != "" {
			present++
		}
	}
	if present != 0 && present != len(digests) {
		return fmt.Errorf("Orca binding requires a complete sealed artifact identity")
	}
	switch binding.ArtifactIdentityVersion {
	case 0:
		if present != 0 {
			return fmt.Errorf("Orca binding sealed artifact identity requires artifact identity version")
		}
	case OrcaArtifactIdentityVersion:
		if present != len(digests) {
			return fmt.Errorf("Orca binding artifact identity version requires a complete sealed artifact identity")
		}
	default:
		return fmt.Errorf("unsupported Orca artifact identity version %d", binding.ArtifactIdentityVersion)
	}
	if present == len(digests) {
		for _, digest := range digests {
			if !validSHA256(digest) {
				return fmt.Errorf("Orca binding sealed artifact identity must contain SHA-256 digests")
			}
		}
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validCommitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
