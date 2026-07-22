package model

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const IssueOpsSchemaVersion = 1

type ExecutionModeV1 string

const (
	ExecutionModeDirect ExecutionModeV1 = "direct"
	ExecutionModeOrca   ExecutionModeV1 = "orca"
)

type LeaseStatusV1 string

const (
	LeaseStatusClaimable LeaseStatusV1 = "claimable"
	LeaseStatusActive    LeaseStatusV1 = "active"
	LeaseStatusRevoking  LeaseStatusV1 = "revoking"
	LeaseStatusReleased  LeaseStatusV1 = "released"
)

type ExecutionV1 struct {
	Mode       ExecutionModeV1        `json:"mode"`
	Workspace  WorkspaceV1            `json:"workspace"`
	Lease      WriteLeaseV1           `json:"lease"`
	Orca       *OrcaBindingV1         `json:"orca,omitempty"`
	Pending    *ExternalIntentV1      `json:"pending,omitempty"`
	Completion *ExecutionCompletionV1 `json:"completion,omitempty"`
	Failure    *ExecutionFailureV1    `json:"failure,omitempty"`
}

type WorkspaceV1 struct {
	SourceRoot string `json:"source_root"`
	Root       string `json:"root"`
	Branch     string `json:"branch"`
	BaseHead   string `json:"base_head"`
	Driver     string `json:"driver"`
	LinkedAt   string `json:"linked_at"`
}

type WriteLeaseV1 struct {
	Generation        uint64         `json:"generation"`
	Status            LeaseStatusV1  `json:"status"`
	Holder            *NativeActorV1 `json:"holder,omitempty"`
	ClaimTokenSHA256  string         `json:"claim_token_sha256,omitempty"`
	ClaimedAt         string         `json:"claimed_at,omitempty"`
	ReleasedAt        string         `json:"released_at,omitempty"`
	ReplacedAt        string         `json:"replaced_at,omitempty"`
	ReplacementReason string         `json:"replacement_reason,omitempty"`
}

type NativeActorV1 struct {
	Host           string                  `json:"host"`
	SessionID      string                  `json:"session_id"`
	AgentID        string                  `json:"agent_id,omitempty"`
	SessionProcess *NativeProcessReceiptV1 `json:"session_process,omitempty"`
	// ProcessAncestry is populated by a first-party adapter from the local OS
	// process tree. It is never accepted from JSON or persisted.
	ProcessAncestry []NativeProcessReceiptV1 `json:"-"`
}

type NativeProcessReceiptV1 struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
}

type OrcaBindingV1 struct {
	RuntimeID          string `json:"runtime_id"`
	RepoID             string `json:"repo_id"`
	WorktreeID         string `json:"worktree_id"`
	WorktreeInstanceID string `json:"worktree_instance_id,omitempty"`
	OwnerHost          string `json:"owner_host"`
	OwnerModel         string `json:"owner_model"`
	OwnerEffort        string `json:"owner_effort,omitempty"`
	TaskID             string `json:"task_id"`
	DispatchID         string `json:"dispatch_id"`
	TerminalPTYID      string `json:"terminal_pty_id,omitempty"`
}

type ExternalIntentV1 struct {
	OperationID string `json:"operation_id"`
	Kind        string `json:"kind"`
	Marker      string `json:"marker"`
	StartedAt   string `json:"started_at"`
}

type ExecutionCompletionV1 struct {
	FinalHead         string   `json:"final_head"`
	TuringReportPath  string   `json:"turing_report_path"`
	Verification      []string `json:"verification"`
	RemoteArtifactURL string   `json:"remote_artifact_url"`
	CompletedAt       string   `json:"completed_at"`
}

type ExecutionFailureV1 struct {
	OperationID string `json:"operation_id,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message,omitempty"`
	At          string `json:"at"`
}

func ValidateExecutionV1(execution ExecutionV1) error {
	if execution.Mode != ExecutionModeDirect && execution.Mode != ExecutionModeOrca {
		return fmt.Errorf("execution mode must be direct or orca")
	}
	if err := validateWorkspaceV1(execution.Workspace, execution.Mode); err != nil {
		return err
	}
	if err := validateWriteLeaseV1(execution.Lease); err != nil {
		return err
	}
	if execution.Mode == ExecutionModeDirect && execution.Orca != nil {
		return fmt.Errorf("direct execution must not contain an Orca binding")
	}
	if execution.Orca != nil {
		if err := validateOrcaBindingV1(*execution.Orca); err != nil {
			return err
		}
	}
	if execution.Pending != nil {
		if execution.Pending.OperationID == "" || execution.Pending.Kind == "" || execution.Pending.Marker == "" || execution.Pending.StartedAt == "" {
			return fmt.Errorf("pending external intent is incomplete")
		}
	}
	if execution.Completion != nil {
		if !validCommitSHA(execution.Completion.FinalHead) || strings.TrimSpace(execution.Completion.TuringReportPath) == "" ||
			len(execution.Completion.Verification) == 0 || strings.TrimSpace(execution.Completion.RemoteArtifactURL) == "" || strings.TrimSpace(execution.Completion.CompletedAt) == "" {
			return fmt.Errorf("execution completion is incomplete")
		}
		for _, evidence := range execution.Completion.Verification {
			if strings.TrimSpace(evidence) == "" {
				return fmt.Errorf("execution completion verification must be nonempty")
			}
		}
	}
	if execution.Failure != nil {
		if execution.Failure.Code == "" || execution.Failure.At == "" || len(execution.Failure.Message) > 4096 {
			return fmt.Errorf("execution failure is invalid")
		}
	}
	return nil
}

func validateWorkspaceV1(workspace WorkspaceV1, mode ExecutionModeV1) error {
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

func validateWriteLeaseV1(lease WriteLeaseV1) error {
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
		if err := ValidateNativeActorV1(*lease.Holder); err != nil {
			return err
		}
	case LeaseStatusRevoking:
		// 이전 holder는 quiescence 진단에만 남고 이 상태에서는 writer가 아니다.
		if lease.Holder == nil || lease.ClaimTokenSHA256 != "" {
			return fmt.Errorf("revoking lease requires the fenced holder and no token hash")
		}
		if err := ValidateNativeActorV1(*lease.Holder); err != nil {
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

func ValidateNativeActorV1(actor NativeActorV1) error {
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

func validateOrcaBindingV1(binding OrcaBindingV1) error {
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
