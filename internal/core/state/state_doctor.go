package state

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		result.OK = true
		result.Healthy = true
		return result, nil
	}
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(dir, name)
		inspected, checked := inspectStateDoctorEntry(path, entry)
		result.Issues = append(result.Issues, inspected.Issues...)
		if !checked {
			continue
		}
		result.Checked++
		key := strings.TrimSuffix(name, ".json")
		recordResult := inspectStateDoctorRecord(path, key)
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
