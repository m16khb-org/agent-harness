package issueops

import (
	"fmt"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
)

// IssueOpsActor identifies the native host session that is authorized to make
// a durable mutation for one IssueOps cycle. CWD is deliberately part of the
// request identity: a valid session in the source checkout is not authority
// for the isolated workspace, and vice versa.
type IssueOpsActor struct {
	Host      string `json:"host"`
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id,omitempty"`
	CWD       string `json:"cwd"`
}

func (actor IssueOpsActor) session() (model.IssueOpsHostSessionIdentity, error) {
	if strings.TrimSpace(actor.Host) == "" {
		return model.IssueOpsHostSessionIdentity{}, fmt.Errorf("native host is required")
	}
	host, err := handoff.NormalizeAgent(actor.Host)
	if err != nil {
		return model.IssueOpsHostSessionIdentity{}, fmt.Errorf("native host identity: %w", err)
	}
	if strings.TrimSpace(actor.SessionID) == "" {
		return model.IssueOpsHostSessionIdentity{}, fmt.Errorf("native session id is required")
	}
	return model.IssueOpsHostSessionIdentity{Host: host, SessionID: strings.TrimSpace(actor.SessionID), AgentID: strings.TrimSpace(actor.AgentID)}, nil
}

func validateWorkspacePreparationActor(record IssueOpsRecord, selectedAgent string, actor IssueOpsActor) (model.IssueOpsHostSessionIdentity, error) {
	session, err := actor.session()
	if err != nil {
		return model.IssueOpsHostSessionIdentity{}, fmt.Errorf("Orca worktree preparation requires %w", err)
	}
	if session.Host != selectedAgent {
		return model.IssueOpsHostSessionIdentity{}, fmt.Errorf("Orca worktree preparation host must match the selected agent")
	}
	if !filepath.IsAbs(strings.TrimSpace(actor.CWD)) || filepath.Clean(actor.CWD) != filepath.Clean(record.Repo) {
		return model.IssueOpsHostSessionIdentity{}, fmt.Errorf("Orca worktree preparation requires the exact source checkout cwd")
	}
	return session, nil
}

func validateReadyWorkspacePreparationActor(record IssueOpsRecord, actor IssueOpsActor) error {
	workspace := record.ExecutionWorkspace
	if workspace == nil {
		return nil
	}
	if record.ExecutionHandoff != nil || workspace.State != "ready" {
		return fmt.Errorf("workspace preparation requires a ready execution workspace without an ownership handoff")
	}
	session, err := actor.session()
	if err != nil {
		return fmt.Errorf("workspace preparation requires %w", err)
	}
	if workspace.PreparationSession == nil || *workspace.PreparationSession != session {
		return fmt.Errorf("workspace preparation requires the exact sealed native preparation session")
	}
	if !filepath.IsAbs(strings.TrimSpace(actor.CWD)) || filepath.Clean(actor.CWD) != filepath.Clean(workspace.WorkerRoot) {
		return fmt.Errorf("workspace preparation requires the canonical ready workspace cwd")
	}
	return nil
}

func validateWorkspacePreparationMutation(record IssueOpsRecord, actor *IssueOpsActor) error {
	if record.ExecutionWorkspace == nil {
		return nil
	}
	if actor == nil {
		return fmt.Errorf("workspace preparation requires a native actor; use the actor-aware recorder")
	}
	return validateReadyWorkspacePreparationActor(record, *actor)
}

// validatePostTransferMutation keeps current-contract durable writes bound to the
// owner even when callers bypass lifecycle hooks through a direct CLI or MCP
// request. Legacy cycles retain their existing actor-optional behavior.
func validatePostTransferMutation(record IssueOpsRecord, actor *IssueOpsActor) error {
	h := record.ExecutionHandoff
	if h == nil {
		return nil
	}
	if h.State != handoff.StateOwnerActive {
		return fmt.Errorf("ownership transfer post-transfer mutation requires owner_active state")
	}
	if actor == nil {
		return fmt.Errorf("ownership transfer post-transfer mutation requires a native owner actor")
	}
	session, err := actor.session()
	if err != nil {
		return fmt.Errorf("ownership transfer post-transfer mutation requires %w", err)
	}
	if h.OwnerSession == nil || *h.OwnerSession != session {
		return fmt.Errorf("ownership transfer post-transfer mutation requires the exact native owner session")
	}
	if !filepath.IsAbs(strings.TrimSpace(actor.CWD)) || filepath.Clean(actor.CWD) != filepath.Clean(h.WorkerRoot) {
		return fmt.Errorf("ownership transfer post-transfer mutation requires the canonical worker root cwd")
	}
	return nil
}

// ValidateReadyWorkspacePreparationActor is the pre-side-effect guard for
// adapters that must inspect or prepare dependencies before they persist their
// result. It intentionally has the same source-root behavior as the
// durable recorders: only an execution workspace requires the sealed actor.
func ValidateReadyWorkspacePreparationActor(record IssueOpsRecord, actor IssueOpsActor) error {
	if record.ExecutionWorkspace == nil {
		return nil
	}
	return validateReadyWorkspacePreparationActor(record, actor)
}
