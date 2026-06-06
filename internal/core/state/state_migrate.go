package state

import (
	"sort"
)

func StateMigrate(confirm bool) (StateMigrateResult, error) {
	dir := StateDir()
	result := StateMigrateResult{
		OK:            false,
		StateDir:      dir,
		FromSchema:    0,
		ToSchema:      StateCurrentSchemaVersion,
		Confirm:       confirm,
		DryRun:        !confirm,
		CandidateKeys: []string{},
		Candidates:    []StateListEntry{},
		MigratedKeys:  []string{},
		SkippedKeys:   []string{},
		Skipped:       []StateListEntry{},
		Issues:        []StateDoctorIssue{},
	}
	doctor, err := StateDoctor()
	if err != nil {
		return result, err
	}
	for _, issue := range doctor.Issues {
		if issue.Code != "legacy_schema" {
			result.Issues = append(result.Issues, issue)
		}
	}
	for _, entry := range doctor.Valid {
		read, err := StateRead(entry.Key)
		if err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     statePath(dir, entry.Key),
				Key:      entry.Key,
				Severity: "error",
				Code:     "read_error",
				Message:  err.Error(),
			})
			continue
		}
		if read.Record.SchemaVersion != 0 {
			result.Skipped = append(result.Skipped, entry)
			result.SkippedKeys = append(result.SkippedKeys, entry.Key)
			continue
		}
		result.CandidateKeys = append(result.CandidateKeys, entry.Key)
		result.Candidates = append(result.Candidates, entry)
		if !confirm {
			continue
		}
		migrated := read.Record
		migrated.SchemaVersion = StateCurrentSchemaVersion
		if _, err := writeStateRecord(dir, migrated.Key, migrated); err != nil {
			result.Issues = append(result.Issues, StateDoctorIssue{
				Path:     statePath(dir, migrated.Key),
				Key:      migrated.Key,
				Severity: "error",
				Code:     "write_error",
				Message:  err.Error(),
			})
			continue
		}
		result.MigratedKeys = append(result.MigratedKeys, migrated.Key)
	}
	sort.Strings(result.CandidateKeys)
	sort.Strings(result.MigratedKeys)
	sort.Strings(result.SkippedKeys)
	sort.Slice(result.Candidates, func(i, j int) bool { return result.Candidates[i].Key < result.Candidates[j].Key })
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].Key < result.Skipped[j].Key })
	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Path != result.Issues[j].Path {
			return result.Issues[i].Path < result.Issues[j].Path
		}
		if result.Issues[i].Code != result.Issues[j].Code {
			return result.Issues[i].Code < result.Issues[j].Code
		}
		return result.Issues[i].Message < result.Issues[j].Message
	})
	result.OK = true
	return result, nil
}
