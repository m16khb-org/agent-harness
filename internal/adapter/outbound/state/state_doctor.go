package state

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"issueops/internal/adapter/outbound/sqlstore"
	stateapplication "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
)

func StateDoctor() (statecontract.StateDoctorResult, error) {
	dir := StateDir()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return failedDoctorResult(dir), err
	}
	entrySnapshots := make([]stateapplication.DoctorEntry, 0, len(entries))
	for _, entry := range entries {
		entrySnapshots = append(entrySnapshots, stateapplication.DoctorEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(dir, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}
	rows, err := sqlstore.GetAllExisting(dir, stateBucket)
	if errors.Is(err, fs.ErrNotExist) {
		rows = nil
	} else if err != nil {
		return failedDoctorResult(dir), err
	}
	rowSnapshots := make([]stateapplication.DoctorRow, 0, len(rows))
	for _, row := range rows {
		rowSnapshots = append(rowSnapshots, stateapplication.DoctorRow{
			Key:  row.ID,
			Path: statePath(dir, row.ID),
			Data: row.Data,
		})
	}
	return stateapplication.Doctor(dir, entrySnapshots, rowSnapshots), nil
}

func failedDoctorResult(dir string) statecontract.StateDoctorResult {
	return statecontract.StateDoctorResult{
		OK:        false,
		Healthy:   false,
		StateDir:  dir,
		ValidKeys: []string{},
		Valid:     []statecontract.StateListEntry{},
		Issues:    []statecontract.StateDoctorIssue{},
	}
}
