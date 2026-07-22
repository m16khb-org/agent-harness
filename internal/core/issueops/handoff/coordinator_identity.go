package handoff

import (
	"path/filepath"
	"strings"

	"agent-harness/internal/core/issueops/model"
)

// CoordinatorIdentityMatches treats the mailbox handle as routing only. The
// native host session and the source checkout are the durable authority.
func CoordinatorIdentityMatches(record model.IssueOpsRecord, native model.IssueOpsHostSessionIdentity, cwd string) bool {
	h := model.CurrentExecutionHandoff(record)
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
