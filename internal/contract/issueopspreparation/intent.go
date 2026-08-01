// Package issueopspreparation defines the stable execution-preparation
// contract shared by prepare, resume, and reconcile.
package issueopspreparation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

const (
	PurposePrepare = "prepare"
	PurposeResume  = "resume"

	InvocationNotInvoked  = "not_invoked_proven"
	InvocationUnknown     = "unknown"
	MaxInvocationAttempts = 2

	markerPrefix = "agent-harness issueops-v1"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type LaunchIdentity struct {
	PromptPath          string `json:"prompt_path"`
	PromptSHA256        string `json:"prompt_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
}

type IntentStage string

const (
	IntentStageWorktree IntentStage = "worktree_create"
	IntentStageTerminal IntentStage = "terminal_create"
	IntentStageRun      IntentStage = "run_create"
	IntentStageRunBind  IntentStage = "run_bind"
	IntentStageTask     IntentStage = "task_create"
	IntentStageDispatch IntentStage = "dispatch"
)

type WorkspaceRequest struct {
	LifecycleID    string `json:"lifecycle_id"`
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseBranch     string `json:"base_branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Confirm        bool   `json:"confirm,omitempty"`
}

type WorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Driver         string `json:"driver"`
	Exists         bool   `json:"exists,omitempty"`
}

type ProbeRequest struct {
	Repo     string `json:"repo"`
	Host     string `json:"host"`
	Model    string `json:"model"`
	Effort   string `json:"effort,omitempty"`
	Provider string `json:"provider,omitempty"`
	Issue    int    `json:"issue,omitempty"`
	Marker   string `json:"marker"`
}

type OrcaWorkspaceReceipt struct {
	Workspace          WorkspaceReceipt `json:"workspace"`
	RuntimeID          string           `json:"runtime_id"`
	RepoID             string           `json:"repo_id"`
	WorktreeID         string           `json:"worktree_id"`
	WorktreeInstanceID string           `json:"worktree_instance_id,omitempty"`
}

// Intent is the only persisted JSON shape for Orca prepare and resume work.
type Intent struct {
	SchemaVersion      int                        `json:"schema_version"`
	Purpose            string                     `json:"purpose,omitempty"`
	OperationID        string                     `json:"operation_id"`
	LifecycleID        string                     `json:"lifecycle_id"`
	Generation         uint64                     `json:"generation"`
	Stage              IntentStage                `json:"stage"`
	Marker             string                     `json:"marker"`
	StartedAt          string                     `json:"started_at"`
	InvocationState    string                     `json:"invocation_state"`
	InvocationAttempts int                        `json:"invocation_attempts"`
	Workspace          WorkspaceRequest           `json:"workspace"`
	Probe              ProbeRequest               `json:"probe"`
	Prepared           *OrcaWorkspaceReceipt      `json:"prepared,omitempty"`
	Launch             *LaunchIdentity            `json:"launch,omitempty"`
	IssueBodySHA256    string                     `json:"issue_body_sha256"`
	ClaimTokenSHA256   string                     `json:"claim_token_sha256,omitempty"`
	TerminalPTYID      string                     `json:"terminal_pty_id,omitempty"`
	RunID              string                     `json:"run_id,omitempty"`
	RunBound           bool                       `json:"run_bound,omitempty"`
	TaskID             string                     `json:"task_id,omitempty"`
	PriorBinding       *leasecontract.OrcaBinding `json:"prior_binding,omitempty"`
	ResumeLease        *leasecontract.Lease       `json:"resume_lease,omitempty"`
}

type IssueIdentity struct {
	Provider string
	Issue    int
}

type IntentCodec struct{}

type IntentError struct {
	Code   string
	Detail string
}

func (e *IntentError) Error() string {
	if strings.TrimSpace(e.Detail) == "" {
		return e.Code
	}
	return e.Code + ": " + strings.TrimSpace(e.Detail)
}

func (e *IntentError) IssueOpsErrorFields() map[string]any {
	return map[string]any{"code": e.Code}
}

func (IntentCodec) Decode(operationID string, raw []byte) (Intent, error) {
	intent, err := decodeShape(raw)
	if err != nil {
		return Intent{}, err
	}
	if err := validateIntent(intent, operationID); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

func (IntentCodec) DecodeShape(operationID string, raw []byte) (Intent, error) {
	intent, err := decodeShape(raw)
	if err != nil {
		return Intent{}, err
	}
	if intent.OperationID != operationID {
		return Intent{}, fmt.Errorf("Orca external intent payload is invalid")
	}
	return intent, nil
}

func (IntentCodec) Validate(intent Intent, operationID string) error {
	return validateIntent(intent, operationID)
}

func (IntentCodec) ValidateShape(intent Intent, operationID string) error {
	return validateShape(intent, operationID)
}

func (IntentCodec) Encode(intent Intent) ([]byte, error) {
	if err := validateIntent(intent, intent.OperationID); err != nil {
		return nil, err
	}
	return json.Marshal(intent)
}

func (IntentCodec) Seal(intent Intent, issue IssueIdentity) (Intent, error) {
	if intent.LifecycleID == "" || !validProvider(issue.Provider) || issue.Issue <= 0 {
		return Intent{}, contractError("intent_identity_mismatch", "Orca intent issue identity is invalid")
	}
	intent.Probe.Provider = strings.ToLower(strings.TrimSpace(issue.Provider))
	intent.Probe.Issue = issue.Issue
	marker, err := renderMarker(MarkerIdentity{
		Purpose: normalizedPurpose(intent), LifecycleID: intent.LifecycleID,
		Generation: intent.Generation, OperationID: intent.OperationID,
		Provider: intent.Probe.Provider, Issue: intent.Probe.Issue,
	})
	if err != nil {
		return Intent{}, err
	}
	intent.Marker = marker
	intent.Probe.Marker = marker
	if err := validateIntent(intent, intent.OperationID); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

// Canonicalize returns identical bytes for canonical intents. An exact legacy
// marker may be upgraded only before an external invocation is possible.
func (codec IntentCodec) Canonicalize(record leasecontract.Record, raw []byte) (Intent, []byte, bool, error) {
	intent, err := decodeShape(raw)
	if err != nil {
		return Intent{}, nil, false, unsafeLegacy(err.Error())
	}
	if err := validateIntent(intent, intent.OperationID); err == nil {
		if err := validateRecordAuthority(record, intent); err != nil {
			return Intent{}, nil, false, unsafeLegacy(err.Error())
		}
		return intent, append([]byte(nil), raw...), false, nil
	}
	if intent.InvocationState != InvocationNotInvoked {
		return Intent{}, nil, false, unsafeLegacy("legacy Orca intent invocation was not proven absent")
	}
	legacy, err := parseLegacyMarker(intent.Marker)
	if err != nil {
		return Intent{}, nil, false, unsafeLegacy("Orca intent marker is neither canonical nor exact legacy")
	}
	if legacy.Purpose != normalizedPurpose(intent) || legacy.LifecycleID != intent.LifecycleID ||
		legacy.Generation != intent.Generation || legacy.OperationID != intent.OperationID {
		return Intent{}, nil, false, unsafeLegacy("legacy Orca intent identity does not match current pending authority")
	}
	if err := validateRecordAuthority(record, intent); err != nil {
		return Intent{}, nil, false, unsafeLegacy(err.Error())
	}
	if record.Execution.Failure != nil && record.Execution.Failure.OperationID != intent.OperationID {
		return Intent{}, nil, false, unsafeLegacy("legacy Orca intent failure receipt belongs to another operation")
	}
	issue, err := issueIdentity(record)
	if err != nil {
		return Intent{}, nil, false, unsafeLegacy(err.Error())
	}
	if intent.Probe.Provider != issue.Provider || intent.Probe.Issue != issue.Issue {
		return Intent{}, nil, false, unsafeLegacy("legacy Orca probe identity does not match the verified issue")
	}
	sealed, err := codec.Seal(intent, issue)
	if err != nil {
		return Intent{}, nil, false, unsafeLegacy(err.Error())
	}
	adjusted := record
	execution := *record.Execution
	pending := *record.Execution.Pending
	execution.Pending = &pending
	adjusted.Execution = &execution
	adjusted.Execution.Pending.Marker = sealed.Marker
	if err := validateRecordAuthority(adjusted, sealed); err != nil {
		return Intent{}, nil, false, unsafeLegacy(err.Error())
	}
	encoded, err := codec.Encode(sealed)
	if err != nil {
		return Intent{}, nil, false, err
	}
	return sealed, encoded, !bytes.Equal(encoded, raw), nil
}

func decodeShape(raw []byte) (Intent, error) {
	var intent Intent
	if err := json.Unmarshal(raw, &intent); err != nil {
		return Intent{}, fmt.Errorf("decode Orca external intent payload: %w", err)
	}
	if err := validateShape(intent, intent.OperationID); err != nil {
		return Intent{}, err
	}
	return intent, nil
}

func validateIntent(intent Intent, operationID string) error {
	if err := validateShape(intent, operationID); err != nil {
		return err
	}
	identity, err := parseMarker(intent.Marker)
	if err != nil {
		return err
	}
	if identity.Purpose != normalizedPurpose(intent) || identity.LifecycleID != intent.LifecycleID ||
		identity.Generation != intent.Generation || identity.OperationID != intent.OperationID ||
		identity.Provider != intent.Probe.Provider || identity.Issue != intent.Probe.Issue {
		return contractError("intent_identity_mismatch", "Orca intent marker does not match the sealed payload identity")
	}
	return nil
}

func validateShape(intent Intent, operationID string) error {
	purpose := normalizedPurpose(intent)
	if intent.SchemaVersion != leasecontract.SchemaVersion || intent.OperationID != operationID ||
		intent.LifecycleID == "" || intent.Generation == 0 || intent.Marker == "" || intent.StartedAt == "" ||
		intent.Workspace.LifecycleID != intent.LifecycleID || intent.Probe.Marker != intent.Marker ||
		!samePath(intent.Probe.Repo, intent.Workspace.SourceRoot) || strings.TrimSpace(intent.Probe.Model) == "" ||
		(intent.Probe.Host != "codex" && intent.Probe.Host != "claude") ||
		(intent.InvocationState != InvocationNotInvoked && intent.InvocationState != InvocationUnknown) ||
		intent.InvocationAttempts < 0 || intent.InvocationAttempts > MaxInvocationAttempts || !sha256Pattern.MatchString(intent.IssueBodySHA256) {
		return fmt.Errorf("Orca external intent payload is invalid")
	}
	switch purpose {
	case PurposePrepare:
		if intent.Generation != 1 || intent.PriorBinding != nil || intent.ResumeLease != nil {
			return fmt.Errorf("Orca prepare intent payload is invalid")
		}
	case PurposeResume:
		if intent.Stage == IntentStageWorktree || intent.PriorBinding == nil || intent.ResumeLease == nil ||
			intent.ResumeLease.Generation != intent.Generation || intent.ResumeLease.Status != "claimable" ||
			intent.ResumeLease.Holder != nil || intent.ResumeLease.ClaimTokenSHA256 != intent.ClaimTokenSHA256 ||
			!sha256Pattern.MatchString(intent.ResumeLease.ClaimTokenSHA256) {
			return fmt.Errorf("Orca resume intent payload is invalid")
		}
	default:
		return fmt.Errorf("unsupported Orca external intent purpose %q", intent.Purpose)
	}
	switch intent.Stage {
	case IntentStageWorktree:
		if intent.Prepared != nil || intent.Launch != nil || intent.ClaimTokenSHA256 != "" || intent.TerminalPTYID != "" || intent.RunID != "" || intent.RunBound || intent.TaskID != "" {
			return fmt.Errorf("Orca worktree intent payload contains later-stage receipts")
		}
	case IntentStageTerminal, IntentStageRun, IntentStageRunBind, IntentStageTask, IntentStageDispatch:
		if intent.Prepared == nil || intent.Launch == nil || !sha256Pattern.MatchString(intent.ClaimTokenSHA256) ||
			!sha256Pattern.MatchString(intent.Launch.PromptSHA256) || !sha256Pattern.MatchString(intent.Launch.ContextPacketSHA256) ||
			strings.TrimSpace(intent.Launch.PromptPath) == "" || strings.TrimSpace(intent.Launch.ContextPacketPath) == "" ||
			validateWorkspaceReceipt(intent.Workspace, *intent.Prepared) != nil {
			return fmt.Errorf("Orca owner intent payload is missing sealed launch receipts")
		}
		if intent.Stage == IntentStageTerminal && (intent.TerminalPTYID != "" || intent.RunID != "" || intent.RunBound || intent.TaskID != "") {
			return fmt.Errorf("Orca terminal intent payload contains later-stage receipts")
		}
		if intent.Stage == IntentStageRun && (intent.TerminalPTYID == "" || intent.RunID != "" || intent.RunBound || intent.TaskID != "") {
			return fmt.Errorf("Orca Run intent payload is incomplete")
		}
		if intent.Stage == IntentStageRunBind && (intent.TerminalPTYID == "" || intent.RunID == "" || intent.RunBound || intent.TaskID != "") {
			return fmt.Errorf("Orca Run bind intent payload is incomplete")
		}
		if intent.Stage == IntentStageTask && (intent.TerminalPTYID == "" || intent.RunID == "" || !intent.RunBound || intent.TaskID != "") {
			return fmt.Errorf("Orca task intent payload is incomplete")
		}
		if intent.Stage == IntentStageDispatch && (intent.TerminalPTYID == "" || intent.RunID == "" || !intent.RunBound || intent.TaskID == "") {
			return fmt.Errorf("Orca dispatch intent payload is incomplete")
		}
	default:
		return fmt.Errorf("unsupported Orca external intent stage %q", intent.Stage)
	}
	return nil
}

func validateWorkspaceReceipt(request WorkspaceRequest, receipt OrcaWorkspaceReceipt) error {
	workspace := receipt.Workspace
	if !samePath(workspace.SourceRoot, request.SourceRoot) || !samePath(workspace.Root, request.Root) ||
		workspace.Branch != request.Branch || workspace.BaseHead != request.BaseHead ||
		!sameOptionalPath(workspace.ParentWorktree, request.ParentWorktree) || workspace.Driver != "orca" ||
		strings.TrimSpace(receipt.RuntimeID) == "" || strings.TrimSpace(receipt.RepoID) == "" || strings.TrimSpace(receipt.WorktreeID) == "" {
		return fmt.Errorf("Orca workspace receipt does not match the sealed request")
	}
	return nil
}

func validateRecordAuthority(record leasecontract.Record, intent Intent) error {
	if record.ID != intent.LifecycleID || record.Execution == nil || record.Execution.Pending == nil ||
		record.Execution.Pending.OperationID != intent.OperationID || record.Execution.Pending.Marker != intent.Marker ||
		record.Execution.Pending.Kind != pendingKind(intent.Stage) || record.Execution.Lease.Generation != intent.Generation {
		return fmt.Errorf("Orca intent authority changed before CAS")
	}
	switch normalizedPurpose(intent) {
	case PurposePrepare:
		if record.Execution.Lease.Status != "released" || record.Execution.Orca != nil {
			return fmt.Errorf("Orca prepare intent authority changed before CAS")
		}
	case PurposeResume:
		if intent.ResumeLease == nil || intent.PriorBinding == nil ||
			!leasesEqual(record.Execution.Lease, *intent.ResumeLease) || !bindingsEqual(record.Execution.Orca, intent.PriorBinding) {
			return fmt.Errorf("Orca resume intent authority changed before CAS")
		}
	default:
		return fmt.Errorf("unsupported Orca intent purpose")
	}
	return nil
}

func issueIdentity(record leasecontract.Record) (IssueIdentity, error) {
	var prepared struct {
		Provider     string `json:"provider"`
		IssueURL     string `json:"issue_url"`
		LinkVerified bool   `json:"link_verified"`
	}
	if len(record.BranchPrepare) == 0 || json.Unmarshal(record.BranchPrepare, &prepared) != nil || !prepared.LinkVerified {
		return IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires verified branch issue identity")
	}
	provider := strings.ToLower(strings.TrimSpace(prepared.Provider))
	if !validProvider(provider) || strings.TrimSpace(prepared.IssueURL) == "" || strings.TrimSpace(record.IssueURL) != strings.TrimSpace(prepared.IssueURL) {
		return IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent issue URL does not match the verified branch identity")
	}
	parsed, err := url.Parse(prepared.IssueURL)
	if err != nil || parsed.Hostname() == "" || !providerHostMatches(provider, parsed.Hostname()) {
		return IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent provider does not match the verified issue URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || (parts[len(parts)-2] != "issues" && parts[len(parts)-2] != "work_items") {
		return IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires a positive issue number")
	}
	issue, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || issue <= 0 {
		return IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires a positive issue number")
	}
	return IssueIdentity{Provider: provider, Issue: issue}, nil
}

type MarkerIdentity struct {
	Purpose, LifecycleID, OperationID, Provider string
	Generation                                  uint64
	Issue                                       int
}

func (codec IntentCodec) RenderMarker(identity MarkerIdentity) (string, error) {
	return renderMarker(identity)
}

func (codec IntentCodec) ParseMarker(marker string) (MarkerIdentity, error) {
	return parseMarker(marker)
}

func (codec IntentCodec) ParseLegacyMarker(marker string) (MarkerIdentity, error) {
	return parseLegacyMarker(marker)
}

func renderMarker(identity MarkerIdentity) (string, error) {
	if (identity.Purpose != PurposePrepare && identity.Purpose != PurposeResume) ||
		!validToken(identity.LifecycleID) || !validToken(identity.OperationID) || !validProvider(identity.Provider) || identity.Issue <= 0 ||
		(identity.Purpose == PurposePrepare && identity.Generation != 1) || (identity.Purpose == PurposeResume && identity.Generation == 0) {
		return "", contractError("intent_marker_invalid", "Orca intent marker identity is invalid")
	}
	fields := []string{markerPrefix}
	if identity.Purpose == PurposeResume {
		fields = append(fields, "resume")
	}
	fields = append(fields, "lifecycle="+identity.LifecycleID)
	if identity.Purpose == PurposeResume {
		fields = append(fields, "generation="+strconv.FormatUint(identity.Generation, 10))
	}
	fields = append(fields, "operation="+identity.OperationID, "provider="+identity.Provider, "issue="+strconv.Itoa(identity.Issue))
	return strings.Join(fields, " "), nil
}

func parseMarker(marker string) (MarkerIdentity, error) {
	fields := strings.Fields(marker)
	identity := MarkerIdentity{}
	var offset int
	switch {
	case len(fields) == 6 && fields[0] == "agent-harness" && fields[1] == "issueops-v1":
		identity.Purpose, identity.Generation, offset = PurposePrepare, 1, 2
	case len(fields) == 8 && fields[0] == "agent-harness" && fields[1] == "issueops-v1" && fields[2] == "resume":
		identity.Purpose, offset = PurposeResume, 3
	default:
		return MarkerIdentity{}, invalidMarker()
	}
	var err error
	if identity.LifecycleID, err = markerField(fields[offset], "lifecycle"); err != nil {
		return MarkerIdentity{}, err
	}
	if identity.Purpose == PurposeResume {
		raw, fieldErr := markerField(fields[offset+1], "generation")
		if fieldErr != nil {
			return MarkerIdentity{}, fieldErr
		}
		identity.Generation, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return MarkerIdentity{}, invalidMarker()
		}
		offset++
	}
	if identity.OperationID, err = markerField(fields[offset+1], "operation"); err != nil {
		return MarkerIdentity{}, err
	}
	if identity.Provider, err = markerField(fields[offset+2], "provider"); err != nil {
		return MarkerIdentity{}, err
	}
	rawIssue, err := markerField(fields[offset+3], "issue")
	if err != nil {
		return MarkerIdentity{}, err
	}
	identity.Issue, err = strconv.Atoi(rawIssue)
	if err != nil {
		return MarkerIdentity{}, invalidMarker()
	}
	rendered, err := renderMarker(identity)
	if err != nil || rendered != marker {
		return MarkerIdentity{}, invalidMarker()
	}
	return identity, nil
}

func parseLegacyMarker(marker string) (MarkerIdentity, error) {
	fields := strings.Fields(marker)
	identity := MarkerIdentity{}
	switch {
	case len(fields) == 4 && fields[0] == "agent-harness" && fields[1] == "issueops-v1":
		identity.Purpose, identity.Generation = PurposePrepare, 1
		identity.LifecycleID = strings.TrimPrefix(fields[2], "lifecycle=")
		identity.OperationID = strings.TrimPrefix(fields[3], "operation=")
	case len(fields) == 6 && fields[0] == "agent-harness" && fields[1] == "issueops-v1" && fields[2] == "resume":
		identity.Purpose = PurposeResume
		identity.LifecycleID = strings.TrimPrefix(fields[3], "lifecycle=")
		raw := strings.TrimPrefix(fields[4], "generation=")
		var err error
		identity.Generation, err = strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return MarkerIdentity{}, invalidMarker()
		}
		identity.OperationID = strings.TrimPrefix(fields[5], "operation=")
	default:
		return MarkerIdentity{}, invalidMarker()
	}
	if !validToken(identity.LifecycleID) || !validToken(identity.OperationID) {
		return MarkerIdentity{}, invalidMarker()
	}
	return identity, nil
}

func markerField(field, name string) (string, error) {
	prefix := name + "="
	if !strings.HasPrefix(field, prefix) || !validToken(strings.TrimPrefix(field, prefix)) {
		return "", invalidMarker()
	}
	return strings.TrimPrefix(field, prefix), nil
}

func normalizedPurpose(intent Intent) string {
	if strings.TrimSpace(intent.Purpose) == "" {
		return PurposePrepare
	}
	return strings.TrimSpace(intent.Purpose)
}

func pendingKind(stage IntentStage) string {
	switch stage {
	case IntentStageWorktree:
		return "worktree_create"
	case IntentStageTerminal, IntentStageRun, IntentStageRunBind, IntentStageTask:
		return "owner_launch"
	case IntentStageDispatch:
		return "dispatch"
	default:
		return ""
	}
}

func samePath(left, right string) bool {
	a, err := filepath.Abs(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(a); resolveErr == nil {
		a = resolved
	}
	b, err := filepath.Abs(strings.TrimSpace(right))
	if err != nil {
		return false
	}
	if resolved, resolveErr := filepath.EvalSymlinks(b); resolveErr == nil {
		b = resolved
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func sameOptionalPath(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" || right == "" {
		return left == right
	}
	return samePath(left, right)
}

func leasesEqual(left, right leasecontract.Lease) bool {
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	return bytes.Equal(leftRaw, rightRaw)
}

func bindingsEqual(left, right *leasecontract.OrcaBinding) bool {
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	return bytes.Equal(leftRaw, rightRaw)
}

func validToken(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && !strings.ContainsRune(value, '=') &&
		!strings.ContainsFunc(value, unicode.IsSpace)
}
func validProvider(value string) bool { return value == "github" || value == "gitlab" }
func providerHostMatches(provider, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return (provider == "github" && host == "github.com") || (provider == "gitlab" && (host == "gitlab.com" || strings.Contains(host, "gitlab")))
}
func contractError(code, detail string) error { return &IntentError{Code: code, Detail: detail} }
func invalidMarker() error {
	return contractError("intent_marker_invalid", "Orca intent marker is not canonical")
}
func unsafeLegacy(detail string) error { return contractError("legacy_intent_upgrade_unsafe", detail) }
