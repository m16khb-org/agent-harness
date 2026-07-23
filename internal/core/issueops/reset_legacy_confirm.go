package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/sqlstore"
)

type resetLegacyProcess struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
	Command    string `json:"command,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Source     string `json:"source,omitempty"`
}

type resetLegacyBinaryIdentity struct {
	Version    string `json:"version"`
	Executable string `json:"executable"`
	SHA256     string `json:"sha256"`
	Mode       uint32 `json:"mode"`
	Size       int64  `json:"size"`
	Device     uint64 `json:"device"`
	Inode      uint64 `json:"inode"`
}

type resetLegacyDeps struct {
	Now               func() time.Time
	ActiveBinary      func() (resetLegacyBinaryIdentity, error)
	LiveProcesses     func(string) ([]resetLegacyProcess, error)
	RequireActivation func(*sqlstore.DB, string, int, resetLegacyBinaryIdentity) error
	Orca              *LegacyResetOrcaDependencies
	AfterStep         func(string) error
}

type resetLegacyJournal struct {
	SchemaVersion int                `json:"schema_version"`
	TargetSchema  int                `json:"target_schema"`
	StateRoot     string             `json:"state_root"`
	LegacyRoot    string             `json:"legacy_root"`
	RootDevice    uint64             `json:"root_device"`
	RootInode     uint64             `json:"root_inode"`
	Fingerprint   string             `json:"fingerprint"`
	Stage         string             `json:"stage"`
	Rows          []legacyResetRow   `json:"rows"`
	Entries       []legacyResetEntry `json:"entries,omitempty"`
	Orca          []LegacyOrcaTask   `json:"orca,omitempty"`
	FileCursor    int                `json:"file_cursor,omitempty"`
	TargetRows    int                `json:"target_rows"`
	TargetFiles   int                `json:"target_files"`
	BinaryVersion string             `json:"binary_version"`
	BinaryPath    string             `json:"binary_path"`
	BinarySHA256  string             `json:"binary_sha256"`
}

// ConfirmLegacyReset performs the exact, fingerprint-CAS reset. The caller
// must have obtained expectedFingerprint from PreviewLegacyReset.
func ConfirmLegacyReset(stateDir string, targetSchema int, expectedFingerprint string) (LegacyResetResult, error) {
	return confirmLegacyReset(stateDir, targetSchema, expectedFingerprint, defaultResetLegacyDeps())
}

func ConfirmLegacyResetWithOrca(ctx context.Context, stateDir string, targetSchema int, expectedFingerprint string, orcaDeps LegacyResetOrcaDependencies) (LegacyResetResult, error) {
	deps := defaultResetLegacyDeps()
	deps.Orca = &orcaDeps
	return confirmLegacyResetContext(ctx, stateDir, targetSchema, expectedFingerprint, deps)
}

func confirmLegacyReset(stateDir string, targetSchema int, expectedFingerprint string, deps resetLegacyDeps) (LegacyResetResult, error) {
	return confirmLegacyResetContext(context.Background(), stateDir, targetSchema, expectedFingerprint, deps)
}

func confirmLegacyResetContext(ctx context.Context, stateDir string, targetSchema int, expectedFingerprint string, deps resetLegacyDeps) (LegacyResetResult, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, targetSchema)
	if err != nil {
		return LegacyResetResult{}, err
	}
	expectedFingerprint = strings.TrimSpace(expectedFingerprint)
	if len(expectedFingerprint) != 64 {
		return LegacyResetResult{}, fmt.Errorf("issueops legacy reset requires the exact 64-character --expected-fingerprint")
	}
	if deps.Now == nil || deps.ActiveBinary == nil || deps.LiveProcesses == nil || deps.RequireActivation == nil {
		return LegacyResetResult{}, fmt.Errorf("issueops legacy reset dependencies are incomplete")
	}
	if err := requireNoLiveHarnessProcesses(stateRoot, deps); err != nil {
		return LegacyResetResult{}, err
	}

	controlRoot := filepath.Join(stateRoot, issueOpsResetDirectory)
	control, err := sqlstore.Open(controlRoot)
	if err != nil {
		return LegacyResetResult{}, err
	}
	var result LegacyResetResult
	err = control.WithSpan(ctx, func(spanCtx context.Context) error {
		activeBinary, err := deps.ActiveBinary()
		if err != nil {
			return fmt.Errorf("verify activated issueops v1 binary: %w", err)
		}
		if err := validateResetLegacyBinaryIdentity(activeBinary); err != nil {
			return fmt.Errorf("verify activated issueops v1 binary: %w", err)
		}
		if err := deps.RequireActivation(control, stateRoot, targetSchema, activeBinary); err != nil {
			return err
		}
		if err := requireNoLiveHarnessProcesses(stateRoot, deps); err != nil {
			return err
		}
		if receipt, ok, err := readLegacyResetReceipt(control); err != nil {
			return err
		} else if ok {
			if receipt.TargetSchema != targetSchema || receipt.StateRoot != stateRoot || receipt.Fingerprint != expectedFingerprint {
				return fmt.Errorf("completed issueops reset receipt does not match the requested state root, target schema, and fingerprint")
			}
			if _, err := os.Lstat(filepath.Join(stateRoot, issueOpsLegacyDirectory)); err == nil {
				return fmt.Errorf("legacy IssueOps state reappeared after the completed reset receipt; run reset-legacy preview again")
			} else if !os.IsNotExist(err) {
				return err
			}
			result = receipt
			return nil
		}

		journal, ok, err := readLegacyResetJournal(control)
		if err != nil {
			return err
		}
		if ok {
			if err := validateLegacyResetJournal(journal, stateRoot, targetSchema, expectedFingerprint); err != nil {
				return err
			}
			if journal.BinaryPath != activeBinary.Executable || journal.BinarySHA256 != activeBinary.SHA256 {
				return fmt.Errorf("in-progress issueops reset is bound to a different staged binary")
			}
		} else {
			manifest, exists, err := buildLegacyResetManifest(stateRoot, targetSchema)
			if err != nil {
				return err
			}
			preview := projectLegacyResetPreview(manifest, exists, targetSchema)
			if !exists {
				return fmt.Errorf("legacy IssueOps state is absent and no matching reset receipt exists")
			}
			if manifest.Fingerprint != expectedFingerprint {
				return fmt.Errorf("stale issueops legacy reset fingerprint: expected %s, current %s", expectedFingerprint, manifest.Fingerprint)
			}
			if len(preview.Blockers) != 0 {
				return fmt.Errorf("issueops legacy reset blocked: %s", strings.Join(preview.Blockers, "; "))
			}
			journal = resetLegacyJournal{
				SchemaVersion: 1, TargetSchema: targetSchema, StateRoot: stateRoot, LegacyRoot: manifest.LegacyRoot,
				RootDevice: manifest.RootDevice, RootInode: manifest.RootInode,
				Fingerprint: manifest.Fingerprint, Stage: "prepared", Rows: stripLegacyResetRowData(manifest.Rows),
				Entries: orderLegacyResetDeletionEntries(manifest.Entries), Orca: sortedLegacyResetOrcaTasks(manifest.Orca),
				TargetRows: preview.RowCount, TargetFiles: preview.FileCount,
				BinaryVersion: activeBinary.Version, BinaryPath: activeBinary.Executable, BinarySHA256: activeBinary.SHA256,
			}
			if err := writeLegacyResetJournal(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStep(deps, "journal_written"); err != nil {
				return err
			}
		}
		if err := requireNoLiveHarnessProcesses(stateRoot, deps); err != nil {
			return err
		}
		if err := requireLegacyResetOrcaQuiescence(spanCtx, journal.Orca, deps.Orca); err != nil {
			return err
		}

		if journal.Stage == "prepared" {
			if err := deleteLegacyResetRows(spanCtx, journal, targetSchema); err != nil {
				return err
			}
			journal.Stage = "rows_deleted"
			if err := writeLegacyResetJournal(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStep(deps, "rows_deleted"); err != nil {
				return err
			}
		}

		if journal.Stage == "rows_deleted" {
			if err := sqlstore.CloseRoot(journal.LegacyRoot); err != nil {
				return fmt.Errorf("close legacy issueops store: %w", err)
			}
			if err := verifyLegacyResetRootIdentity(journal); err != nil {
				return err
			}
			entries, err := refreshLegacyResetSealedEntries(journal)
			if err != nil {
				return err
			}
			journal.Entries = entries
			journal.FileCursor = 0
			journal.Stage = "files"
			if err := writeLegacyResetJournal(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStep(deps, "files_snapshot"); err != nil {
				return err
			}
		}

		if journal.Stage == "files" {
			if err := deleteLegacyResetEntries(control, &journal, deps); err != nil {
				return err
			}
			journal.Stage = "files_deleted"
			if err := writeLegacyResetJournal(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStep(deps, "files_deleted"); err != nil {
				return err
			}
		}

		if journal.Stage == "files_deleted" {
			if _, err := os.Lstat(journal.LegacyRoot); !os.IsNotExist(err) {
				if err == nil {
					return fmt.Errorf("legacy IssueOps root remains after exact deletion")
				}
				return err
			}
			if err := initializeIssueOpsSchema(spanCtx, stateRoot); err != nil {
				return err
			}
			journal.Stage = "schema_initialized"
			if err := writeLegacyResetJournal(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStep(deps, "schema_initialized"); err != nil {
				return err
			}
		}

		if journal.Stage != "schema_initialized" {
			return fmt.Errorf("unsupported issueops legacy reset journal stage %q", journal.Stage)
		}
		result = LegacyResetResult{
			OK: true, SchemaVersion: 1, TargetSchema: targetSchema, StateRoot: stateRoot,
			Fingerprint: journal.Fingerprint, DeletedRows: journal.TargetRows, DeletedFiles: journal.TargetFiles,
			BinaryVersion: journal.BinaryVersion, CompletedAt: deps.Now().UTC().Format(time.RFC3339Nano), Completed: true,
		}
		if err := writeLegacyResetReceipt(control, result); err != nil {
			return err
		}
		return resetLegacyAfterStep(deps, "completed")
	})
	if err != nil {
		return LegacyResetResult{}, err
	}
	return result, nil
}

func sortedLegacyResetOrcaTasks(tasks map[string]LegacyOrcaTask) []LegacyOrcaTask {
	result := make([]LegacyOrcaTask, 0, len(tasks))
	for _, task := range tasks {
		task.Reconciled = false
		task.NextCommand = ""
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool {
		return legacyResetOrcaAuthorityKey(result[i]) < legacyResetOrcaAuthorityKey(result[j])
	})
	return result
}

func requireLegacyResetOrcaQuiescence(ctx context.Context, tasks []LegacyOrcaTask, deps *LegacyResetOrcaDependencies) error {
	if len(tasks) == 0 {
		return nil
	}
	if deps == nil {
		return fmt.Errorf("issueops legacy reset requires fresh Orca inventory before confirmation")
	}
	for _, task := range tasks {
		if _, err := observeLegacyResetOrcaQuiescence(ctx, task, *deps); err != nil {
			return fmt.Errorf("issueops legacy reset Orca authority %s is not quiescent: %w", task.TaskID, err)
		}
	}
	return nil
}

func deleteLegacyResetRows(ctx context.Context, journal resetLegacyJournal, targetSchema int) error {
	manifest, exists, err := buildLegacyResetManifest(journal.StateRoot, targetSchema)
	if err != nil {
		return err
	}
	if !exists {
		if len(journal.Rows) == 0 {
			return nil
		}
		return fmt.Errorf("legacy IssueOps root disappeared after reset journal creation")
	}
	if manifest.Fingerprint != journal.Fingerprint {
		absent, err := legacyResetRowsAbsent(journal)
		if err != nil {
			return err
		}
		if absent {
			return nil
		}
		return fmt.Errorf("stale issueops legacy reset fingerprint after journal creation: expected %s, current %s", journal.Fingerprint, manifest.Fingerprint)
	}
	if len(journal.Rows) == 0 {
		return nil
	}
	db, err := sqlstore.Open(journal.LegacyRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(context.Context) error {
		mutations := make([]sqlstore.Mutation, 0, len(journal.Rows))
		for _, row := range journal.Rows {
			if row.Store != "" || (row.Bucket != "issueops" && row.Bucket != "session") {
				return fmt.Errorf("reset journal contains an unsupported legacy row target %q/%q", row.Store, row.Bucket)
			}
			data, ok, err := db.Get(row.Bucket, row.ID)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("legacy reset row %s/%s disappeared before atomic deletion", row.Bucket, row.ID)
			}
			if !legacyResetRowMatches(row, data) {
				return fmt.Errorf("legacy reset row %s/%s changed before atomic deletion", row.Bucket, row.ID)
			}
			mutations = append(mutations, sqlstore.Mutation{Bucket: row.Bucket, ID: row.ID, Delete: true})
		}
		return db.Apply(ctx, mutations)
	})
}

func legacyResetRowsAbsent(journal resetLegacyJournal) (bool, error) {
	for _, row := range journal.Rows {
		if row.Store != "" {
			return false, fmt.Errorf("reset journal contains unsupported nested SQLite rows")
		}
		_, ok, err := sqlstore.GetExisting(journal.LegacyRoot, row.Bucket, row.ID)
		if isLegacyResetMissing(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}
	return true, nil
}

func legacyResetRowMatches(expected legacyResetRow, data []byte) bool {
	sum := sha256Bytes(data)
	return len(data) == expected.Size && sum == expected.SHA256
}

func sha256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func refreshLegacyResetSealedEntries(journal resetLegacyJournal) ([]legacyResetEntry, error) {
	sealed := make(map[string]legacyResetEntry, len(journal.Entries))
	for _, entry := range journal.Entries {
		if entry.Path == "" || !entry.Known {
			return nil, fmt.Errorf("reset journal contains an invalid sealed file target")
		}
		if _, exists := sealed[entry.Path]; exists {
			return nil, fmt.Errorf("reset journal contains duplicate sealed target %q", entry.Path)
		}
		sealed[entry.Path] = entry
	}
	refreshed := make(map[string]legacyResetEntry, len(sealed))
	err := filepath.WalkDir(journal.LegacyRoot, func(path string, dirEntry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == journal.LegacyRoot {
			return nil
		}
		rel, err := filepath.Rel(journal.LegacyRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		expected, ok := sealed[rel]
		if !ok {
			return fmt.Errorf("legacy reset entry %q was not in the sealed preview manifest", rel)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		current, blocker, err := inspectLegacyEntry(path, rel, info)
		if err != nil {
			return err
		}
		if blocker != "" || !current.Known || expected.Path != current.Path || expected.Kind != current.Kind || expected.Mode != current.Mode || expected.Device != current.Device || expected.Inode != current.Inode {
			return fmt.Errorf("legacy reset sealed target %q changed identity after row deletion", rel)
		}
		if current.Kind != "directory" && !legacySQLiteFilePattern.MatchString(filepath.Base(rel)) && !sameLegacyResetEntry(expected, current) {
			return fmt.Errorf("legacy reset sealed non-SQLite target %q changed after preview", rel)
		}
		refreshed[rel] = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	entries := make([]legacyResetEntry, 0, len(journal.Entries))
	for _, entry := range journal.Entries {
		if current, ok := refreshed[entry.Path]; ok {
			entries = append(entries, current)
		} else {
			entries = append(entries, entry)
		}
	}
	return orderLegacyResetDeletionEntries(entries), nil
}

func orderLegacyResetDeletionEntries(entries []legacyResetEntry) []legacyResetEntry {
	ordered := append([]legacyResetEntry(nil), entries...)
	sort.Slice(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.Kind != right.Kind {
			return left.Kind == "file"
		}
		leftDepth, rightDepth := strings.Count(left.Path, "/"), strings.Count(right.Path, "/")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return left.Path < right.Path
	})
	return ordered
}

func deleteLegacyResetEntries(control *sqlstore.DB, journal *resetLegacyJournal, deps resetLegacyDeps) error {
	for journal.FileCursor < len(journal.Entries) {
		if err := verifyNoUnsealedLegacyResetEntries(*journal); err != nil {
			return err
		}
		entry := journal.Entries[journal.FileCursor]
		path, err := safeLegacyResetTarget(journal.LegacyRoot, entry.Path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			journal.FileCursor++
			if err := writeLegacyResetJournal(control, *journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStep(deps, fmt.Sprintf("file_deleted:%d", journal.FileCursor)); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		current, blocker, err := inspectLegacyEntry(path, entry.Path, info)
		if err != nil {
			return err
		}
		if blocker != "" || !sameLegacyResetEntry(entry, current) {
			return fmt.Errorf("legacy reset target %q changed after deletion snapshot", entry.Path)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete exact legacy reset target %q: %w", entry.Path, err)
		}
		journal.FileCursor++
		if err := writeLegacyResetJournal(control, *journal); err != nil {
			return err
		}
		if err := resetLegacyAfterStep(deps, fmt.Sprintf("file_deleted:%d", journal.FileCursor)); err != nil {
			return err
		}
	}
	info, err := os.Lstat(journal.LegacyRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	device, inode, _ := fileIdentity(info)
	if !info.IsDir() || device != journal.RootDevice || inode != journal.RootInode {
		return fmt.Errorf("legacy reset root identity changed before final removal")
	}
	if err := os.Remove(journal.LegacyRoot); err != nil {
		return fmt.Errorf("remove empty legacy issueops root: %w", err)
	}
	return nil
}

func verifyNoUnsealedLegacyResetEntries(journal resetLegacyJournal) error {
	sealed := make(map[string]struct{}, len(journal.Entries))
	for _, entry := range journal.Entries {
		sealed[entry.Path] = struct{}{}
	}
	return filepath.WalkDir(journal.LegacyRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == journal.LegacyRoot {
			return nil
		}
		rel, err := filepath.Rel(journal.LegacyRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := sealed[rel]; !ok {
			return fmt.Errorf("legacy reset entry %q was not in the sealed preview manifest", rel)
		}
		return nil
	})
}

func sameLegacyResetEntry(expected, current legacyResetEntry) bool {
	if expected.Path != current.Path || expected.Kind != current.Kind || expected.Mode != current.Mode || expected.Device != current.Device || expected.Inode != current.Inode || !current.Known {
		return false
	}
	if expected.Kind == "directory" {
		return true
	}
	return expected.Size == current.Size && expected.SHA256 == current.SHA256
}

func safeLegacyResetTarget(root, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe legacy reset target %q", rel)
	}
	target := filepath.Join(root, clean)
	relCheck, err := filepath.Rel(root, target)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("legacy reset target escapes root: %q", rel)
	}
	return target, nil
}

func initializeIssueOpsSchema(ctx context.Context, stateRoot string) error {
	schemaRoot := filepath.Join(stateRoot, issueOpsDirectory)
	db, err := sqlstore.Open(schemaRoot)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(context.Context) error {
		expected := []byte(`{"schema_version":1}`)
		current, ok, err := db.Get(issueOpsMetaBucket, issueOpsSchemaMarkerID)
		if err != nil {
			return err
		}
		if ok {
			if string(current) != string(expected) {
				return fmt.Errorf("issueops v1 schema marker is not canonical")
			}
			return nil
		}
		return db.Put(issueOpsMetaBucket, issueOpsSchemaMarkerID, expected)
	})
}

func stripLegacyResetRowData(rows []legacyResetRow) []legacyResetRow {
	result := make([]legacyResetRow, len(rows))
	for i, row := range rows {
		row.Data = nil
		result[i] = row
	}
	return result
}

func readLegacyResetJournal(db *sqlstore.DB) (resetLegacyJournal, bool, error) {
	data, ok, err := db.Get(issueOpsResetBucket, issueOpsResetJournalID)
	if err != nil || !ok {
		return resetLegacyJournal{}, ok, err
	}
	var journal resetLegacyJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return resetLegacyJournal{}, false, fmt.Errorf("decode issueops reset journal: %w", err)
	}
	return journal, true, nil
}

func writeLegacyResetJournal(db *sqlstore.DB, journal resetLegacyJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return db.Put(issueOpsResetBucket, issueOpsResetJournalID, data)
}

func validateLegacyResetJournal(journal resetLegacyJournal, stateRoot string, targetSchema int, fingerprint string) error {
	if journal.SchemaVersion != 1 || journal.TargetSchema != targetSchema || journal.StateRoot != stateRoot || journal.LegacyRoot != filepath.Join(stateRoot, issueOpsLegacyDirectory) || journal.Fingerprint != fingerprint || journal.RootDevice == 0 || journal.RootInode == 0 {
		return fmt.Errorf("in-progress issueops reset journal does not match the requested state root, target schema, and fingerprint")
	}
	if journal.Stage == "prepared" && len(journal.Entries) == 0 && journal.TargetFiles != 0 {
		return fmt.Errorf("in-progress issueops reset journal is missing its sealed preview file manifest")
	}
	return nil
}

func verifyLegacyResetRootIdentity(journal resetLegacyJournal) error {
	info, err := os.Lstat(journal.LegacyRoot)
	if err != nil {
		return fmt.Errorf("verify legacy reset root identity: %w", err)
	}
	device, inode, ok := fileIdentity(info)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || device != journal.RootDevice || inode != journal.RootInode {
		return fmt.Errorf("legacy reset root identity changed after the deletion journal was sealed")
	}
	return nil
}

func readLegacyResetReceipt(db *sqlstore.DB) (LegacyResetResult, bool, error) {
	data, ok, err := db.Get(issueOpsResetBucket, issueOpsResetReceiptID)
	if err != nil || !ok {
		return LegacyResetResult{}, ok, err
	}
	var receipt LegacyResetResult
	if err := json.Unmarshal(data, &receipt); err != nil {
		return LegacyResetResult{}, false, fmt.Errorf("decode issueops reset receipt: %w", err)
	}
	return receipt, true, nil
}

func writeLegacyResetReceipt(db *sqlstore.DB, result LegacyResetResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	mutations := []sqlstore.Mutation{
		{Bucket: issueOpsResetBucket, ID: issueOpsResetReceiptID, Data: data},
		{Bucket: issueOpsResetBucket, ID: issueOpsResetJournalID, Delete: true},
	}
	ids, err := db.List(issueOpsResetBucket)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if strings.HasPrefix(id, legacyResetRemoteReceiptPrefix) || strings.HasPrefix(id, legacyResetOrcaReceiptPrefix) || strings.HasPrefix(id, legacyResetCycleReceiptPrefix) {
			mutations = append(mutations, sqlstore.Mutation{Bucket: issueOpsResetBucket, ID: id, Delete: true})
		}
	}
	return db.Apply(context.Background(), mutations)
}

func resetLegacyAfterStep(deps resetLegacyDeps, step string) error {
	if deps.AfterStep == nil {
		return nil
	}
	return deps.AfterStep(step)
}

func requireNoLiveHarnessProcesses(stateRoot string, deps resetLegacyDeps) error {
	processes, err := deps.LiveProcesses(stateRoot)
	if err != nil {
		return fmt.Errorf("enumerate live harness processes: %w", err)
	}
	if len(processes) == 0 {
		return nil
	}
	identities := make([]string, 0, len(processes))
	for _, process := range processes {
		identities = append(identities, fmt.Sprintf("pid=%d started_at=%s executable=%s kind=%s source=%s", process.PID, process.StartedAt, process.Executable, process.Kind, process.Source))
	}
	return fmt.Errorf("issueops legacy reset blocked by live harness processes: %s; next: stop the registered daemon and exact listed PID/start identities, then rerun issueops reset-legacy --target-schema 1 --status", strings.Join(identities, "; "))
}

func defaultResetLegacyDeps() resetLegacyDeps {
	return resetLegacyDeps{
		Now:               time.Now,
		ActiveBinary:      activeResetLegacyBinaryIdentity,
		LiveProcesses:     liveHarnessProcesses,
		RequireActivation: requireLegacyResetActivation,
	}
}

func resetLegacyBinaryVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && strings.TrimSpace(info.Main.Version) != "" {
		return strings.TrimSpace(info.Main.Version)
	}
	return "devel"
}

func activeResetLegacyBinaryIdentity() (resetLegacyBinaryIdentity, error) {
	executable, err := os.Executable()
	if err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	return resetLegacyBinaryIdentityFromPath(executable, resetLegacyBinaryVersion())
}

func validateResetLegacyBinaryIdentity(identity resetLegacyBinaryIdentity) error {
	if strings.TrimSpace(identity.Version) == "" {
		return fmt.Errorf("binary version is empty")
	}
	if !filepath.IsAbs(identity.Executable) || filepath.Clean(identity.Executable) != identity.Executable {
		return fmt.Errorf("binary executable must be an absolute canonical path")
	}
	decoded, err := hex.DecodeString(identity.SHA256)
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("binary SHA-256 is invalid")
	}
	if identity.Mode == 0 || identity.Size <= 0 || identity.Device == 0 || identity.Inode == 0 {
		return fmt.Errorf("binary physical identity is incomplete")
	}
	return nil
}
