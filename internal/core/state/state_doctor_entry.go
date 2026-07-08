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
// records live as database rows, so the only files the harness owns here are
// the sqlite databases (and their sidecars), the hook failure log, and legacy
// leftovers from the pre-sqlite layout (*.json records and lock files), which
// are ignored rather than flagged.
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
	case "projects", "daemon", "worker", "workpool", "issueops", "issueops-benchmarks", "audit":
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
	// Legacy pre-sqlite layout leftovers: JSON record files and advisory lock
	// files. They are inert after the fresh-start migration and not worth a
	// warning per file.
	if strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".state-lock") || strings.HasSuffix(name, ".lock") {
		return true
	}
	switch name {
	case HookFailureLogFile, "hook-metrics.jsonl", storeMaintainSentinel:
		return true
	default:
		return false
	}
}
