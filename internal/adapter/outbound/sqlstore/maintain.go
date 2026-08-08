package sqlstore

import (
	"fmt"
	"os"
	"path/filepath"

	statecontract "agent-harness/internal/contract/state"
)

// Maintain runs the store's periodic maintenance: it truncates the data
// database's WAL via PRAGMA wal_checkpoint(TRUNCATE) and re-asserts 0600 on
// known store files and sidecars. It is safe to run concurrently with readers
// and writers; when a writer holds the WAL busy the checkpoint is skipped for
// this pass (Checkpointed=false) instead of failing.
func (d *DB) Maintain() (statecontract.StoreMaintainResult, error) {
	result := statecontract.StoreMaintainResult{Dir: d.dir}
	fixed, err := repairPrivateSQLiteFiles(d.dir)
	if err != nil {
		return result, fmt.Errorf("sqlstore maintain permissions %s: %w", d.dir, err)
	}
	result.PermissionsFixed = fixed
	walPath := filepath.Join(d.dir, dataDBFile+"-wal")
	if info, err := os.Stat(walPath); err == nil {
		result.WALBytesBefore = info.Size()
	}
	var busy, logFrames, checkpointed int
	if err := d.data.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return result, fmt.Errorf("sqlstore maintain checkpoint %s: %w", d.dir, err)
	}
	result.Checkpointed = busy == 0
	if info, err := os.Stat(walPath); err == nil {
		result.WALBytesAfter = info.Size()
	}
	return result, nil
}
