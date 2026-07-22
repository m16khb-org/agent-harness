package handoff

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"agent-harness/internal/core/issueops/model"
)

var fullCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

const (
	ProviderIssueLinkGitLabUnavailable = "gitlab_native_unavailable"
	ProviderIssueLinkGitLabExact       = "gitlab_native_exact"
)

// ValidateEnvelope enforces the single current ownership handoff contract.
func ValidateEnvelope(record model.IssueOpsRecord) error {
	if record.SchemaVersion >= 9 && (record.CycleState != "" || record.Ownership != nil) {
		return ValidateOwnershipLedger(record)
	}
	if err := validateExecutionWorkspace(record); err != nil {
		return err
	}
	if record.ExecutionHandoff == nil {
		if record.RemoteCreateClaim != nil {
			return fmt.Errorf("remote create claim requires handoff authority")
		}
		return nil
	}
	return validateOwnershipEnvelope(record)
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

func validateHandoffExternalStringBounds(h *model.IssueOpsExecutionHandoff) error {
	checks := []struct {
		name  string
		value string
		max   int
	}{
		{"state", h.State, 64}, {"closed disposition", h.ClosedDisposition, 64},
		{"driver", h.Driver, 32}, {"agent", h.Agent, 32}, {"delivery mode", h.DeliveryMode, 32},
		{"ownership epoch", h.OwnershipEpoch, 512}, {"workspace epoch", h.WorkspaceEpoch, 512},
		{"coordinator root", h.CoordinatorRoot, 4096}, {"coordinator mailbox handle", h.CoordinatorMailboxHandle, 256},
		{"worker root", h.WorkerRoot, 4096}, {"attempt base head", h.AttemptBaseHead, 128},
		{"context sha256", h.ContextSHA256, 64}, {"context source sha256", h.ContextSourceSHA256, 64},
		{"prepared timestamp", h.PreparedAt, 128}, {"provisioned timestamp", h.ProvisionedAt, 128},
		{"dispatched timestamp", h.DispatchedAt, 128}, {"claimed timestamp", h.ClaimedAt, 128},
		{"heartbeat timestamp", h.LastHeartbeatAt, 128}, {"completed timestamp", h.CompletedAt, 128}, {"updated timestamp", h.UpdatedAt, 128},
	}
	for _, session := range []*model.IssueOpsHostSessionIdentity{h.CoordinatorSession, h.OwnerSession} {
		if session != nil {
			checks = append(checks, struct {
				name  string
				value string
				max   int
			}{"session id", session.SessionID, 1024}, struct {
				name  string
				value string
				max   int
			}{"agent id", session.AgentID, 1024})
		}
	}
	if h.Failure != nil {
		checks = append(checks, struct {
			name  string
			value string
			max   int
		}{"failure code", h.Failure.Code, 128}, struct {
			name  string
			value string
			max   int
		}{"failure message", h.Failure.Message, 8192})
	}
	if h.Completion != nil {
		checks = append(checks, struct {
			name  string
			value string
			max   int
		}{"completion head", h.Completion.FinalHead, 128}, struct {
			name  string
			value string
			max   int
		}{"completion report", h.Completion.TuringReport, 4096})
	}
	for _, check := range checks {
		if len(check.value) > check.max {
			return fmt.Errorf("execution handoff %s exceeds %d bytes", check.name, check.max)
		}
	}
	return nil
}

func validWorkerSession(session *model.IssueOpsHostSessionIdentity) bool {
	if session == nil || !canonicalSupportedHost(session.Host) || !canonicalNonSpace(session.SessionID) {
		return false
	}
	return session.AgentID == "" || canonicalNonSpace(session.AgentID)
}

func canonicalNonSpace(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && strings.IndexFunc(value, unicode.IsSpace) < 0
}

func canonicalSupportedHost(value string) bool {
	canonical, err := NormalizeAgent(value)
	return err == nil && canonical == value
}

func validFailure(failure *model.IssueOpsExecutionHandoffFailure) bool {
	return failure != nil && canonicalNonSpace(failure.Code) && failure.Message == redact(failure.Message) && canonicalTimestamp(failure.At)
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
	for _, value := range []string{h.PreparedAt, h.ProvisionedAt, h.DispatchedAt, h.ClaimedAt, h.LastHeartbeatAt, h.CompletedAt, h.UpdatedAt} {
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
	for _, value := range []string{identity.RuntimeID, identity.RepoID, identity.BaseRef, identity.ProviderIssueLinkStatus, identity.WorktreeID, identity.WorktreeInstanceID, identity.WorktreePath, identity.WorkerPTYID, identity.WorkerTerminalHandle, identity.WorkerMailboxHandle, identity.WorkerTabID, identity.WorkerLeafID, identity.TaskID, identity.DispatchID} {
		if value != strings.TrimSpace(value) {
			return false
		}
	}
	return true
}
