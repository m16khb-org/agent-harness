package contract

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const SchemaVersion = 1

var ErrMalformedSchema = errors.New("malformed issueops schema")

type UnsupportedSchemaError struct {
	Version int
}

func (e UnsupportedSchemaError) Error() string {
	return fmt.Sprintf("unsupported issueops schema_version %d", e.Version)
}

type Record struct {
	OK                      bool            `json:"ok"`
	SchemaVersion           int             `json:"schema_version"`
	ID                      string          `json:"id"`
	Repo                    json.RawMessage `json:"repo"`
	Branch                  json.RawMessage `json:"branch,omitempty"`
	Phase                   json.RawMessage `json:"phase"`
	Intent                  json.RawMessage `json:"intent,omitempty"`
	DesignReview            json.RawMessage `json:"design_review,omitempty"`
	DomainReview            json.RawMessage `json:"domain_review,omitempty"`
	IssueURL                json.RawMessage `json:"issue_url,omitempty"`
	PlanPath                json.RawMessage `json:"plan_path,omitempty"`
	WorktreePath            json.RawMessage `json:"worktree_path,omitempty"`
	IssueLinks              json.RawMessage `json:"issue_links,omitempty"`
	BranchPrepare           json.RawMessage `json:"branch_prepare,omitempty"`
	RemoteArtifact          json.RawMessage `json:"remote_artifact,omitempty"`
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
	SourceMisdirectWarnings json.RawMessage `json:"source_misdirect_warnings,omitempty"`
	CleanupFinishFailure    json.RawMessage `json:"cleanup_finish_failure,omitempty"`
	ImplementationReview    json.RawMessage `json:"implementation_review,omitempty"`
	RoutingTrace            json.RawMessage `json:"routing_trace,omitempty"`
	AISlopCleanAt           json.RawMessage `json:"ai_slop_clean_at,omitempty"`
	AISlopCleanHead         json.RawMessage `json:"ai_slop_clean_head,omitempty"`
	AISlopCleanFingerprint  json.RawMessage `json:"ai_slop_clean_fingerprint,omitempty"`
	AISlopCleanCategories   json.RawMessage `json:"ai_slop_clean_categories,omitempty"`
	AISlopCleanVerification json.RawMessage `json:"ai_slop_clean_verification,omitempty"`
	PhaseLedger             json.RawMessage `json:"phase_ledger,omitempty"`
	CreatedAt               json.RawMessage `json:"created_at"`
	UpdatedAt               json.RawMessage `json:"updated_at"`
}

type Execution struct {
	Mode           string          `json:"mode"`
	Workspace      Workspace       `json:"workspace"`
	Lease          Lease           `json:"lease"`
	Orca           json.RawMessage `json:"orca,omitempty"`
	Pending        json.RawMessage `json:"pending,omitempty"`
	Completion     json.RawMessage `json:"completion,omitempty"`
	Failure        json.RawMessage `json:"failure,omitempty"`
	SyncBaseEvents json.RawMessage `json:"sync_base_events,omitempty"`
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

type orcaBinding struct {
	RuntimeID  string `json:"runtime_id"`
	RepoID     string `json:"repo_id"`
	WorktreeID string `json:"worktree_id"`
	OwnerHost  string `json:"owner_host"`
	OwnerModel string `json:"owner_model"`
	TaskID     string `json:"task_id"`
	DispatchID string `json:"dispatch_id"`
}

type externalIntent struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Marker      string `json:"marker"`
	StartedAt   string `json:"started_at"`
}

type executionCompletion struct {
	FinalHead         string   `json:"final_head"`
	TuringReportPath  string   `json:"turing_report_path"`
	Verification      []string `json:"verification"`
	RemoteArtifactURL string   `json:"remote_artifact_url"`
	CompletedAt       string   `json:"completed_at"`
}

type executionFailure struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	At      string `json:"at"`
}

type syncBaseEvent struct {
	Mode          string `json:"mode"`
	BaseBranch    string `json:"base_branch"`
	BaseOID       string `json:"base_oid"`
	MergeCommit   string `json:"merge_commit"`
	ConflictFiles int    `json:"conflict_files"`
	Actor         string `json:"actor"`
	At            string `json:"at"`
}

func Decode(id string, data []byte) (Record, error) {
	var header struct {
		SchemaVersion int    `json:"schema_version"`
		ID            string `json:"id"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Record{}, schemaDecodeError(err)
	}
	if header.SchemaVersion != SchemaVersion {
		return Record{}, UnsupportedSchemaError{Version: header.SchemaVersion}
	}
	if header.ID != id {
		return Record{}, fmt.Errorf("issueops id mismatch: record has %q", header.ID)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Record{}, schemaDecodeError(err)
	}
	for _, name := range []string{"execution_handoff", "execution_workspace", "ownership", "remote_create_claim"} {
		raw := bytes.TrimSpace(fields[name])
		if len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
			return Record{}, fmt.Errorf("legacy execution authority %s is forbidden", name)
		}
	}
	var shape stableV1Record
	if err := json.Unmarshal(data, &shape); err != nil {
		return Record{}, schemaDecodeError(err)
	}
	canonical, err := json.Marshal(shape)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(canonical, &record); err != nil {
		return Record{}, schemaDecodeError(err)
	}
	record.OK = true
	if err := validateRecord(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func schemaDecodeError(err error) error {
	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrMalformedSchema, err)
}

func Encode(record Record) ([]byte, error) {
	record.OK = true
	if record.SchemaVersion == 0 {
		record.SchemaVersion = SchemaVersion
	}
	if record.SchemaVersion != SchemaVersion {
		return nil, UnsupportedSchemaError{Version: record.SchemaVersion}
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
	workspace := execution.Workspace
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
	if execution.Mode == "direct" && workspace.Driver != "git" {
		return fmt.Errorf("direct execution workspace driver must be git")
	}
	if execution.Mode == "orca" && workspace.Driver != "orca" {
		return fmt.Errorf("Orca execution workspace driver must be orca")
	}
	if err := validateExecutionSidecars(*execution); err != nil {
		return err
	}
	lease := execution.Lease
	if lease.Generation == 0 {
		return fmt.Errorf("lease generation must start at 1")
	}
	switch lease.Status {
	case "active":
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" || lease.ClaimedAt == "" {
			return fmt.Errorf("active lease requires one holder and no token hash")
		}
		if err := validateActor(*lease.Holder); err != nil {
			return err
		}
	case "revoking":
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("%s lease requires a holder and no token hash", lease.Status)
		}
		if err := validateActor(*lease.Holder); err != nil {
			return err
		}
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

func validateExecutionSidecars(execution Execution) error {
	var orca orcaBinding
	hasOrca, err := decodeOptional(execution.Orca, &orca)
	if err != nil {
		return fmt.Errorf("Orca binding is malformed: %w", err)
	}
	if execution.Mode == "direct" && hasOrca {
		return fmt.Errorf("direct execution must not contain an Orca binding")
	}
	if hasOrca {
		for name, value := range map[string]string{
			"runtime_id": orca.RuntimeID, "repo_id": orca.RepoID, "worktree_id": orca.WorktreeID,
			"owner_host": orca.OwnerHost, "owner_model": orca.OwnerModel,
			"task_id": orca.TaskID, "dispatch_id": orca.DispatchID,
		} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("Orca binding %s is required", name)
			}
		}
		if orca.OwnerHost != "codex" && orca.OwnerHost != "claude" {
			return fmt.Errorf("Orca owner_host must be codex or claude")
		}
	}

	var pending externalIntent
	hasPending, err := decodeOptional(execution.Pending, &pending)
	if err != nil {
		return fmt.Errorf("pending external intent is malformed: %w", err)
	}
	if hasPending && (pending.OperationID == "" || pending.Kind == "" || pending.Marker == "" || pending.StartedAt == "") {
		return fmt.Errorf("pending external intent is incomplete")
	}

	var completion executionCompletion
	hasCompletion, err := decodeOptional(execution.Completion, &completion)
	if err != nil {
		return fmt.Errorf("execution completion is malformed: %w", err)
	}
	if hasCompletion {
		if !validHexDigest(completion.FinalHead, 40, 64) ||
			strings.TrimSpace(completion.TuringReportPath) == "" ||
			len(completion.Verification) == 0 ||
			strings.TrimSpace(completion.RemoteArtifactURL) == "" ||
			strings.TrimSpace(completion.CompletedAt) == "" {
			return fmt.Errorf("execution completion is incomplete")
		}
		for _, evidence := range completion.Verification {
			if strings.TrimSpace(evidence) == "" {
				return fmt.Errorf("execution completion verification must be nonempty")
			}
		}
	}

	var failure executionFailure
	hasFailure, err := decodeOptional(execution.Failure, &failure)
	if err != nil {
		return fmt.Errorf("execution failure is malformed: %w", err)
	}
	if hasFailure && (failure.Code == "" || failure.At == "" || len(failure.Message) > 4096) {
		return fmt.Errorf("execution failure is invalid")
	}

	if len(bytes.TrimSpace(execution.SyncBaseEvents)) > 0 && !bytes.Equal(bytes.TrimSpace(execution.SyncBaseEvents), []byte("null")) {
		var events []syncBaseEvent
		if err := json.Unmarshal(execution.SyncBaseEvents, &events); err != nil {
			return fmt.Errorf("execution sync-base events are malformed: %w", err)
		}
		for _, event := range events {
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
	}
	return nil
}

func decodeOptional(raw json.RawMessage, destination any) (bool, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false, nil
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return false, err
	}
	return true, nil
}

func validHexDigest(value string, sizes ...int) bool {
	validSize := false
	for _, size := range sizes {
		if len(value) == size {
			validSize = true
			break
		}
	}
	if !validSize {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateActor(actor Actor) error {
	if actor.Host != "codex" && actor.Host != "claude" {
		return fmt.Errorf("native actor host must be codex or claude")
	}
	if strings.TrimSpace(actor.SessionID) == "" || actor.SessionProcess == nil ||
		actor.SessionProcess.PID <= 0 || strings.TrimSpace(actor.SessionProcess.StartedAt) == "" ||
		strings.TrimSpace(actor.SessionProcess.Executable) == "" {
		return fmt.Errorf("native actor receipt is incomplete")
	}
	return nil
}
