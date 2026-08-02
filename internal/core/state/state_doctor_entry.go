package state

import (
	"os"
	"path/filepath"
	"strings"
)

const HookFailureLogFile = "hook-failures.jsonl"

type stateDoctorEntryInspection struct {
	Issues []StateDoctorIssue
}

// inspectStateDoctorEntry flags foreign files in the state directory. State
// records live as database rows, so only current sqlite stores and current
// harness-owned auxiliary state are accepted here.
func inspectStateDoctorEntry(dir string, entry os.DirEntry) stateDoctorEntryInspection {
	name := entry.Name()
	path := filepath.Join(dir, name)
	if entry.IsDir() {
		if isHarnessOwnedStateDirectory(name) {
			return stateDoctorEntryInspection{}
		}
		return stateDoctorEntryInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Severity: "warning",
			Code:     "unexpected_directory",
			Message:  "state directory contains an unexpected subdirectory",
		}}}
	}
	if isHarnessOwnedStateFile(name) {
		return stateDoctorEntryInspection{}
	}
	return stateDoctorEntryInspection{Issues: []StateDoctorIssue{{
		Path:     path,
		Severity: "warning",
		Code:     "unexpected_file",
		Message:  "state directory contains an unexpected file",
	}}}
}

func isHarnessOwnedStateDirectory(name string) bool {
	switch name {
	case "projects", "daemon", "worker", "loop", "issueops-benchmarks", "issueops_v1", "native-activation", "audit":
		return true
	default:
		return false
	}
}

func isHarnessOwnedStateFile(name string) bool {
	// The sqlite store and its WAL/SHM/journal sidecars.
	for _, base := range []string{"harness.db", "harness.lock.db"} {
		if name == base || strings.HasPrefix(name, base+"-") {
			return true
		}
	}
	switch name {
	case HookFailureLogFile, "hook-metrics.jsonl", storeMaintainSentinel:
		return true
	default:
		return false
	}
}
