package state

import (
	"os"
	"strings"
)

const HookFailureLogFile = "hook-failures.jsonl"

type stateDoctorEntryInspection struct {
	Issues []StateDoctorIssue
}

func inspectStateDoctorEntry(path string, entry os.DirEntry) (stateDoctorEntryInspection, bool) {
	name := entry.Name()
	if entry.IsDir() {
		if isHarnessOwnedStateDirectory(name) {
			return stateDoctorEntryInspection{}, false
		}
		return stateDoctorEntryInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Severity: "warning",
			Code:     "unexpected_directory",
			Message:  "state directory contains an unexpected subdirectory",
		}}}, false
	}
	if !strings.HasSuffix(name, ".json") {
		if isHarnessOwnedStateFile(name) {
			return stateDoctorEntryInspection{}, false
		}
		return stateDoctorEntryInspection{Issues: []StateDoctorIssue{{
			Path:     path,
			Severity: "warning",
			Code:     "unexpected_file",
			Message:  "state directory contains a non-json file",
		}}}, false
	}
	return stateDoctorEntryInspection{}, true
}

func isHarnessOwnedStateDirectory(name string) bool {
	switch name {
	case "projects", "daemon", "worker", "issueops", "issueops-benchmarks", "audit":
		return true
	default:
		return false
	}
}

func isHarnessOwnedStateFile(name string) bool {
	if strings.HasSuffix(name, ".state-lock") {
		return true
	}
	switch name {
	case HookFailureLogFile:
		return true
	default:
		return false
	}
}
