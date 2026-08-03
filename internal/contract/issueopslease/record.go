package issueopslease

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	statecontract "agent-harness/internal/contract/state"
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
	PlanPath                string          `json:"plan_path,omitempty"`
	WorktreePath            string          `json:"worktree_path,omitempty"`
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
	SourceMisdirectWarnings int             `json:"source_misdirect_warnings,omitempty"`
	CleanupFinishFailure    json.RawMessage `json:"cleanup_finish_failure,omitempty"`
	ImplementationReview    json.RawMessage `json:"implementation_review,omitempty"`
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
	Mode           string          `json:"mode"`
	Workspace      Workspace       `json:"workspace"`
	Lease          Lease           `json:"lease"`
	Orca           *OrcaBinding    `json:"orca,omitempty"`
	Pending        *ExternalIntent `json:"pending,omitempty"`
	Completion     *Completion     `json:"completion,omitempty"`
	Failure        *FailureDetail  `json:"failure,omitempty"`
	SyncBaseEvents []SyncBaseEvent `json:"sync_base_events,omitempty"`
}

type OrcaBinding = stableV1OrcaBinding
type ExternalIntent = stableV1ExternalIntent
type Completion = stableV1Completion
type FailureDetail = stableV1Failure
type SyncBaseEvent = stableV1SyncBaseEvent

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
	if err := validateSidecars(*execution); err != nil {
		return err
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
	if actor.Host != "codex" && actor.Host != "claude" {
		return fmt.Errorf("native actor host must be codex or claude")
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
	if execution.Completion != nil && (!validHexDigest(execution.Completion.FinalHead, 40, 64) || strings.TrimSpace(execution.Completion.TuringReportPath) == "" || len(execution.Completion.Verification) == 0 || strings.TrimSpace(execution.Completion.RemoteArtifactURL) == "" || strings.TrimSpace(execution.Completion.CompletedAt) == "") {
		return fmt.Errorf("execution completion is incomplete")
	}
	if execution.Failure != nil && (execution.Failure.Code == "" || execution.Failure.At == "" || len(execution.Failure.Message) > 4096) {
		return fmt.Errorf("execution failure is invalid")
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
