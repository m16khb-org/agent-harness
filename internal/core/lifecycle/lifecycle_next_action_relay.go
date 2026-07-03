package lifecycle

import (
	"os"
	"path/filepath"

	"agent-harness/internal/core/lifecycle/fingerprint"
	"agent-harness/internal/core/repopath"
)

func ClearStopNextActionRelayIfPresent(repoRoot string) StopNextActionRelayResult {
	path, err := stopNextActionRelayPath(repoRoot)
	if err != nil {
		return ClearStopNextActionRelay(repoRoot)
	}
	result := StopNextActionRelayResult{OK: true, Path: path}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			result.Reason = "no_next_action_relay"
			return result
		}
		return ClearStopNextActionRelay(repoRoot)
	}
	return ClearStopNextActionRelay(repoRoot)
}

func stopNextActionRelayPath(repoRoot string) (string, error) {
	root, err := repopath.NormalizeRoot(repoRoot)
	if err != nil {
		return "", err
	}
	repoID := fingerprint.RepoID(fingerprint.ForRoot(root))
	return filepath.Join(StateDir(), "projects", repoID, stopNextActionRelayFile), nil
}
