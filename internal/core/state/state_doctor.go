package state

import (
	"errors"
	"io/fs"
	"os"
	"sort"

	"agent-harness/internal/core/sqlstore"
)

func StateDoctor() (StateDoctorResult, error) {
	dir := StateDir()
	result := StateDoctorResult{
		OK:        false,
		Healthy:   false,
		StateDir:  dir,
		ValidKeys: []string{},
		Valid:     []StateListEntry{},
		Issues:    []StateDoctorIssue{},
	}
	// Inspect directory entries first: the state dir may carry foreign files
	// that do not belong to the harness. Records themselves are database rows.
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return result, err
	}
	for _, entry := range entries {
		inspected := inspectStateDoctorEntry(dir, entry)
		result.Issues = append(result.Issues, inspected.Issues...)
	}
	rows, err := sqlstore.GetAllExisting(dir, stateBucket)
	if errors.Is(err, fs.ErrNotExist) {
		rows = nil
	} else if err != nil {
		return result, err
	}
	for _, row := range rows {
		result.Checked++
		recordResult := inspectStateDoctorRecord(statePath(dir, row.ID), row.ID, row.Data)
		result.Issues = append(result.Issues, recordResult.Issues...)
		if !recordResult.Valid {
			continue
		}
		result.Valid = append(result.Valid, StateListEntry{
			Key:           recordResult.Record.Key,
			UpdatedAt:     recordResult.Record.UpdatedAt,
			Bytes:         recordResult.Record.Bytes,
			SchemaVersion: recordResult.Record.SchemaVersion,
		})
		result.ValidKeys = append(result.ValidKeys, recordResult.Record.Key)
	}
	sort.Strings(result.ValidKeys)
	sort.Slice(result.Valid, func(i, j int) bool { return result.Valid[i].Key < result.Valid[j].Key })
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
	result.Healthy = len(result.Issues) == 0
	return result, nil
}
