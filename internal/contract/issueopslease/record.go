package issueopslease

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	statecontract "issueops/internal/contract/state"
)

const (
	SchemaVersion               = 1
	OrcaArtifactIdentityVersion = 1
)

// Record은 release가 변경하지 않는 v1 sidecar를 canonical DTO로 보존한다.
type Record struct {
	OK                      bool            `json:"ok"`
	SchemaVersion           int             `json:"schema_version"`
	ID                      string          `json:"id"`
	Repo                    string          `json:"repo"`
	Branch                  string          `json:"branch,omitempty"`
	Phase                   string          `json:"phase"`
	Intent                  json.RawMessage `json:"intent,omitempty"`
	DesignReview            json.RawMessage `json:"design_review,omitempty"`
	DomainReview            json.RawMessage `json:"domain_review,omitempty"`
	IssueURL                string          `json:"issue_url,omitempty"`
	IssueCreateIntent       json.RawMessage `json:"issue_create_intent,omitempty"`
	PlanPath                string          `json:"plan_path,omitempty"`
	WorktreePath            string          `json:"worktree_path,omitempty"`
	IssueLinks              json.RawMessage `json:"issue_links,omitempty"`
	BranchPrepare           json.RawMessage `json:"branch_prepare,omitempty"`
	RemoteArtifact          json.RawMessage `json:"remote_artifact,omitempty"`
	BodySyncs               json.RawMessage `json:"body_syncs,omitempty"`
	Decisions               json.RawMessage `json:"decisions,omitempty"`
	PlanPrep                json.RawMessage `json:"plan_prep,omitempty"`
	CompatibilityReview     json.RawMessage `json:"compatibility_review,omitempty"`
	DevilsAdvocateReview    json.RawMessage `json:"devils_advocate_review,omitempty"`
	Feedback                json.RawMessage `json:"feedback,omitempty"`
	RegressEvents           json.RawMessage `json:"regress_events,omitempty"`
	Delegation              json.RawMessage `json:"delegation,omitempty"`
	ChildCycles             json.RawMessage `json:"child_cycles,omitempty"`
	Execution               *Execution      `json:"execution,omitempty"`
	RemoteCompletion        json.RawMessage `json:"remote_completion,omitempty"`
	SourceMisdirectWarnings int             `json:"source_misdirect_warnings,omitempty"`
	CleanupFinishFailure    json.RawMessage `json:"cleanup_finish_failure,omitempty"`
	CleanupAbandonFailure   json.RawMessage `json:"cleanup_abandon_failure,omitempty"`
	ImplementationReview    json.RawMessage `json:"implementation_review,omitempty"`
	ProjectDocsReview       json.RawMessage `json:"project_docs_review,omitempty"`
	SchemaEvidence          json.RawMessage `json:"schema_evidence,omitempty"`
	RoutingTrace            json.RawMessage `json:"routing_trace,omitempty"`
	AISlopCleanAt           string          `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead         string          `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint  string          `json:"ai_slop_clean_fingerprint,omitempty"`
	AISlopCleanCategories   json.RawMessage `json:"ai_slop_clean_categories,omitempty"`
	AISlopCleanVerification json.RawMessage `json:"ai_slop_clean_verification,omitempty"`
	PhaseLedger             json.RawMessage `json:"phase_ledger,omitempty"`
	CreatedAt               string          `json:"created_at"`
	UpdatedAt               string          `json:"updated_at"`
}

type Execution struct {
	Mode               string                   `json:"mode"`
	Selection          *Selection               `json:"selection,omitempty"`
	Workspace          Workspace                `json:"workspace"`
	Lease              Lease                    `json:"lease"`
	Orca               *OrcaBinding             `json:"orca,omitempty"`
	Pending            *ExternalIntent          `json:"pending,omitempty"`
	Completion         *Completion              `json:"completion,omitempty"`
	CompletionHistory  []CompletionHistoryEntry `json:"completion_history,omitempty"`
	Failure            *FailureDetail           `json:"failure,omitempty"`
	SyncBaseResolution *SyncBaseResolution      `json:"sync_base_resolution,omitempty"`
	SyncBaseEvents     []SyncBaseEvent          `json:"sync_base_events,omitempty"`
}

type Selection struct {
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

type OrcaBinding = stableV1OrcaBinding
type ExternalIntent = stableV1ExternalIntent
type Completion = stableV1Completion
type CompletionHistoryEntry = stableV1CompletionHistory
type FailureDetail = stableV1Failure
type SyncBaseResolution = stableV1SyncBaseResolution
type SyncBaseEvent = stableV1SyncBaseEvent

type Workspace struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	LinkedAt       string `json:"linked_at"`
	ArtifactDir    string `json:"artifact_dir,omitempty"`
}
type Lease struct {
	Generation        uint64 `json:"generation"`
	Status            string `json:"status"`
	Holder            *Actor `json:"holder,omitempty"`
	ClaimTokenSHA256  string `json:"claim_token_sha256,omitempty"`
	ClaimedAt         string `json:"claimed_at,omitempty"`
	ReleasedAt        string `json:"released_at,omitempty"`
	ReplacedAt        string `json:"replaced_at,omitempty"`
	ReplacementReason string `json:"replacement_reason,omitempty"`
}
type Actor struct {
	Host           string          `json:"host"`
	SessionID      string          `json:"session_id"`
	AgentID        string          `json:"agent_id,omitempty"`
	SessionProcess *ProcessReceipt `json:"session_process,omitempty"`
}
type ProcessReceipt struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}

func Decode(id string, data []byte) (Record, error) {
	var shape stableV1Record
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&shape); err != nil {
		return Record{}, statecontract.Invalid("")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Record{}, statecontract.Invalid("")
	}
	if shape.SchemaVersion != SchemaVersion || shape.ID != id {
		return Record{}, statecontract.Invalid("")
	}
	canonical, err := json.Marshal(shape)
	if err != nil {
		return Record{}, statecontract.Invalid("")
	}
	var record Record
	if err := json.Unmarshal(canonical, &record); err != nil {
		return Record{}, statecontract.Invalid("")
	}
	record.OK = true
	if err := validateRecord(record); err != nil {
		return Record{}, statecontract.Invalid("")
	}
	return record, nil
}

func Encode(record Record) ([]byte, error) {
	record.OK = true
	if record.SchemaVersion != SchemaVersion {
		return nil, statecontract.Invalid("")
	}
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	return json.MarshalIndent(record, "", "  ")
}

func validateRecord(record Record) error {
	if record.Execution == nil {
		return nil
	}
	execution := record.Execution
	if execution.Mode != "direct" && execution.Mode != "orca" {
		return fmt.Errorf("execution mode must be direct or orca")
	}
	for name, value := range map[string]string{"source_root": execution.Workspace.SourceRoot, "root": execution.Workspace.Root, "branch": execution.Workspace.Branch, "base_head": execution.Workspace.BaseHead, "driver": execution.Workspace.Driver, "linked_at": execution.Workspace.LinkedAt} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("workspace %s is required", name)
		}
	}
	if execution.Workspace.SourceRoot == execution.Workspace.Root {
		return fmt.Errorf("canonical worktree must be isolated from source_root")
	}
	if execution.Mode == "direct" && execution.Workspace.Driver != "git" {
		return fmt.Errorf("direct execution workspace driver must be git")
	}
	if execution.Mode == "orca" && execution.Workspace.Driver != "orca" {
		return fmt.Errorf("Orca execution workspace driver must be orca")
	}
	if err := validateLease(execution.Lease); err != nil {
		return err
	}
	if execution.Selection != nil {
		if err := validateSelection(*execution.Selection, execution.Mode); err != nil {
			return err
		}
	}
	if err := validateSidecars(*execution); err != nil {
		return err
	}
	return nil
}

func validateSelection(selection Selection, mode string) error {
	if selection.RequestedMode != "auto" && selection.RequestedMode != "direct" && selection.RequestedMode != "orca" {
		return fmt.Errorf("selection requested_mode must be auto, direct, or orca")
	}
	if selection.ResolvedMode != mode {
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
	if (selection.RequestedMode == "direct" && mode != "direct") ||
		(selection.RequestedMode == "orca" && mode != "orca") {
		return fmt.Errorf("explicit selection mode must equal execution mode")
	}
	if mode == "orca" && !selection.ProbeReady {
		return fmt.Errorf("Orca selection requires a ready probe")
	}
	if selection.RequestedMode == "auto" && mode == "direct" {
		probeCode := strings.TrimSpace(selection.ProbeCode)
		fallbackCode := strings.TrimSpace(selection.FallbackCode)
		if selection.ProbeReady || fallbackCode == "" || fallbackCode != probeCode || fallbackCode != selection.FallbackCode {
			return fmt.Errorf("auto direct selection requires the exact probe failure fallback_code")
		}
	}
	if mode == "orca" && strings.TrimSpace(selection.FallbackCode) != "" {
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
	if !validHexDigest(selection.ReadinessFingerprint, 64) || strings.TrimSpace(selection.SelectedAt) == "" {
		return fmt.Errorf("selection receipt requires fingerprint and selected_at")
	}
	return nil
}

func validateLease(lease Lease) error {
	if lease.Generation == 0 {
		return fmt.Errorf("lease generation must start at 1")
	}
	switch lease.Status {
	case "active":
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" || lease.ClaimedAt == "" {
			return fmt.Errorf("active lease requires one holder and no token hash")
		}
		return validateActor(*lease.Holder)
	case "revoking":
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("revoking lease requires a holder and no token hash")
		}
		return validateActor(*lease.Holder)
	case "claimable":
		if lease.Holder != nil || !validHexDigest(lease.ClaimTokenSHA256, 64) {
			return fmt.Errorf("claimable lease requires no holder and one token hash")
		}
	case "released":
		if lease.Holder != nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("released lease must not retain a holder or token hash")
		}
	default:
		return fmt.Errorf("unsupported lease status %q", lease.Status)
	}
	return nil
}

func validateActor(actor Actor) error {
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

func validateSidecars(execution Execution) error {
	if execution.Mode == "direct" && execution.Orca != nil {
		return fmt.Errorf("direct execution must not contain an Orca binding")
	}
	if execution.Orca != nil {
		for name, value := range map[string]string{"runtime_id": execution.Orca.RuntimeID, "repo_id": execution.Orca.RepoID, "worktree_id": execution.Orca.WorktreeID, "owner_host": execution.Orca.OwnerHost, "owner_model": execution.Orca.OwnerModel, "task_id": execution.Orca.TaskID, "dispatch_id": execution.Orca.DispatchID} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("Orca binding %s is required", name)
			}
		}
		digests := []string{execution.Orca.IssueBodySHA256, execution.Orca.ContextPacketSHA256, execution.Orca.OwnerPromptSHA256}
		present := 0
		for _, digest := range digests {
			if digest != "" {
				present++
			}
		}
		if present != 0 && present != len(digests) {
			return fmt.Errorf("Orca binding requires a complete sealed artifact identity")
		}
		switch execution.Orca.ArtifactIdentityVersion {
		case 0:
			if present != 0 {
				return fmt.Errorf("Orca binding sealed artifact identity requires artifact identity version")
			}
		case OrcaArtifactIdentityVersion:
			if present != len(digests) {
				return fmt.Errorf("Orca binding artifact identity version requires a complete sealed artifact identity")
			}
		default:
			return fmt.Errorf("unsupported Orca artifact identity version %d", execution.Orca.ArtifactIdentityVersion)
		}
		if present == len(digests) {
			for _, digest := range digests {
				if !validHexDigest(digest, 64) {
					return fmt.Errorf("Orca binding sealed artifact identity must contain SHA-256 digests")
				}
			}
		}
	}
	if execution.Pending != nil && (execution.Pending.OperationID == "" || execution.Pending.Kind == "" || execution.Pending.Marker == "" || execution.Pending.StartedAt == "") {
		return fmt.Errorf("pending external intent is incomplete")
	}
	if execution.Completion != nil {
		if err := validateCompletion(*execution.Completion); err != nil {
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
		if err := validateCompletion(entry.Completion); err != nil {
			return fmt.Errorf("execution completion history: %w", err)
		}
		if entry.Completion.Generation != 0 && entry.Completion.Generation != entry.Generation {
			return fmt.Errorf("execution completion history generation conflicts with its completion")
		}
	}
	if execution.Failure != nil && (execution.Failure.Code == "" || execution.Failure.At == "" || len(execution.Failure.Message) > 4096) {
		return fmt.Errorf("execution failure is invalid")
	}
	if execution.SyncBaseResolution != nil {
		resolution := execution.SyncBaseResolution
		if execution.Lease.Status != "released" || execution.Completion == nil ||
			resolution.Generation == 0 || resolution.Generation != execution.Lease.Generation ||
			resolution.CompletionGeneration == 0 || resolution.CompletionGeneration != execution.Completion.Generation ||
			!validHexDigest(resolution.BaseOID, 40, 64) || strings.TrimSpace(resolution.StartedAt) == "" || len(resolution.ConflictFiles) == 0 {
			return fmt.Errorf("execution sync-base resolution is invalid")
		}
		if err := validateActor(resolution.Actor); err != nil {
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
	}
	for _, event := range execution.SyncBaseEvents {
		if event.Mode != "apply" && event.Mode != "finalize" {
			return fmt.Errorf("execution sync-base event mode must be apply or finalize")
		}
		if !validHexDigest(event.BaseOID, 40, 64) || !validHexDigest(event.MergeCommit, 40, 64) {
			return fmt.Errorf("execution sync-base event requires full base and merge commit OIDs")
		}
		if strings.TrimSpace(event.BaseBranch) == "" || strings.TrimSpace(event.Actor) == "" || strings.TrimSpace(event.At) == "" {
			return fmt.Errorf("execution sync-base event is incomplete")
		}
		if event.ConflictFiles < 0 {
			return fmt.Errorf("execution sync-base event conflict count must not be negative")
		}
	}
	return nil
}

func validateCompletion(completion Completion) error {
	if !validHexDigest(completion.FinalHead, 40, 64) || strings.TrimSpace(completion.VerificationReportPath) == "" || len(completion.Verification) == 0 || strings.TrimSpace(completion.RemoteArtifactURL) == "" || strings.TrimSpace(completion.CompletedAt) == "" {
		return fmt.Errorf("execution completion is incomplete")
	}
	for _, evidence := range completion.Verification {
		if strings.TrimSpace(evidence) == "" {
			return fmt.Errorf("execution completion verification must be nonempty")
		}
	}
	return nil
}
func validHexDigest(value string, sizes ...int) bool {
	valid := false
	for _, size := range sizes {
		if len(value) == size {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
