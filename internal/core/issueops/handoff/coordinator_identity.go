package handoff

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/model"
)

// CoordinatorIdentityMatches treats the mailbox handle as routing only. The
// native host session and the source checkout are the durable authority.
func CoordinatorIdentityMatches(record model.IssueOpsRecord, native model.IssueOpsHostSessionIdentity, cwd string) bool {
	h := record.ExecutionHandoff
	if h == nil || h.CoordinatorSession == nil {
		return false
	}
	sealed := h.CoordinatorSession
	if strings.TrimSpace(native.Host) == "" || !strings.EqualFold(strings.TrimSpace(native.Host), strings.TrimSpace(sealed.Host)) ||
		strings.TrimSpace(native.SessionID) == "" || strings.TrimSpace(native.SessionID) != strings.TrimSpace(sealed.SessionID) ||
		strings.TrimSpace(native.AgentID) != strings.TrimSpace(sealed.AgentID) {
		return false
	}
	return cleanCoordinatorPath(cwd) != "" && cleanCoordinatorPath(cwd) == cleanCoordinatorPath(record.Repo) && cleanCoordinatorPath(h.CoordinatorRoot) == cleanCoordinatorPath(record.Repo)
}

// LegacyCoordinatorIdentityCanBeSealed authorizes the one explicit recovery
// transition for a genuine schema-v5 accepted publication record, which
// predates the durable native coordinator identity. The source checkout and
// current native event are the authority; the mailbox remains routing only.
func LegacyCoordinatorIdentityCanBeSealed(record model.IssueOpsRecord, native model.IssueOpsHostSessionIdentity, cwd string) bool {
	h := record.ExecutionHandoff
	if record.SchemaVersion != 5 || h == nil || h.CoordinatorSession != nil || h.State != StateClosed || h.ClosedDisposition != DispositionAccepted || h.PublishReceipt == nil || strings.TrimSpace(h.CoordinatorMailboxHandle) == "" {
		return false
	}
	if strings.TrimSpace(native.Host) == "" || strings.TrimSpace(native.SessionID) == "" {
		return false
	}
	repo := cleanCoordinatorPath(record.Repo)
	return repo != "" && cleanCoordinatorPath(cwd) == repo && cleanCoordinatorPath(h.CoordinatorRoot) == repo
}

func cleanCoordinatorPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	clean, err := filepath.Abs(value)
	if err != nil {
		return ""
	}
	return filepath.Clean(clean)
}
