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

type resetLegacyV1Process struct {
	PID        int    `json:"pid"`
	StartedAt  string `json:"started_at"`
	Executable string `json:"executable"`
	Command    string `json:"command,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Source     string `json:"source,omitempty"`
}

type resetLegacyV1BinaryIdentity struct {
	Version    string `json:"version"`
	Executable string `json:"executable"`
	SHA256     string `json:"sha256"`
	Mode       uint32 `json:"mode"`
	Size       int64  `json:"size"`
	Device     uint64 `json:"device"`
	Inode      uint64 `json:"inode"`
}

type resetLegacyV1Deps struct {
	Now               func() time.Time
	ActiveBinary      func() (resetLegacyV1BinaryIdentity, error)
	LiveProcesses     func(string) ([]resetLegacyV1Process, error)
	RequireActivation func(*sqlstore.DB, string, int, resetLegacyV1BinaryIdentity) error
	Orca              *LegacyResetOrcaDependenciesV1
	AfterStep         func(string) error
}

type resetLegacyV1Journal struct {
	SchemaVersion int                  `json:"schema_version"`
	TargetSchema  int                  `json:"target_schema"`
	StateRoot     string               `json:"state_root"`
	LegacyRoot    string               `json:"legacy_root"`
	RootDevice    uint64               `json:"root_device"`
	RootInode     uint64               `json:"root_inode"`
	Fingerprint   string               `json:"fingerprint"`
	Stage         string               `json:"stage"`
	Rows          []legacyResetRowV1   `json:"rows"`
	Entries       []legacyResetEntryV1 `json:"entries,omitempty"`
	Orca          []LegacyOrcaTaskV1   `json:"orca,omitempty"`
	FileCursor    int                  `json:"file_cursor,omitempty"`
	TargetRows    int                  `json:"target_rows"`
	TargetFiles   int                  `json:"target_files"`
	BinaryVersion string               `json:"binary_version"`
	BinaryPath    string               `json:"binary_path"`
	BinarySHA256  string               `json:"binary_sha256"`
}

// ConfirmLegacyResetV1 performs the exact, fingerprint-CAS reset. The caller
// must have obtained expectedFingerprint from PreviewLegacyResetV1.
func ConfirmLegacyResetV1(stateDir string, targetSchema int, expectedFingerprint string) (LegacyResetResultV1, error) {
	return confirmLegacyResetV1(stateDir, targetSchema, expectedFingerprint, defaultResetLegacyV1Deps())
}

func ConfirmLegacyResetWithOrcaV1(ctx context.Context, stateDir string, targetSchema int, expectedFingerprint string, orcaDeps LegacyResetOrcaDependenciesV1) (LegacyResetResultV1, error) {
	deps := defaultResetLegacyV1Deps()
	deps.Orca = &orcaDeps
	return confirmLegacyResetV1Context(ctx, stateDir, targetSchema, expectedFingerprint, deps)
}

func confirmLegacyResetV1(stateDir string, targetSchema int, expectedFingerprint string, deps resetLegacyV1Deps) (LegacyResetResultV1, error) {
	return confirmLegacyResetV1Context(context.Background(), stateDir, targetSchema, expectedFingerprint, deps)
}

func confirmLegacyResetV1Context(ctx context.Context, stateDir string, targetSchema int, expectedFingerprint string, deps resetLegacyV1Deps) (LegacyResetResultV1, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, targetSchema)
	if err != nil {
		return LegacyResetResultV1{}, err
	}
	expectedFingerprint = strings.TrimSpace(expectedFingerprint)
	if len(expectedFingerprint) != 64 {
		return LegacyResetResultV1{}, fmt.Errorf("issueops legacy reset requires the exact 64-character --expected-fingerprint")
	}
	if deps.Now == nil || deps.ActiveBinary == nil || deps.LiveProcesses == nil || deps.RequireActivation == nil {
		return LegacyResetResultV1{}, fmt.Errorf("issueops legacy reset dependencies are incomplete")
	}
	if err := requireNoLiveHarnessProcessesV1(stateRoot, deps); err != nil {
		return LegacyResetResultV1{}, err
	}

	controlRoot := filepath.Join(stateRoot, issueOpsResetV1Directory)
	control, err := sqlstore.Open(controlRoot)
	if err != nil {
		return LegacyResetResultV1{}, err
	}
	var result LegacyResetResultV1
	err = control.WithSpan(ctx, func(spanCtx context.Context) error {
		activeBinary, err := deps.ActiveBinary()
		if err != nil {
			return fmt.Errorf("verify activated issueops v1 binary: %w", err)
		}
		if err := validateResetLegacyBinaryIdentityV1(activeBinary); err != nil {
			return fmt.Errorf("verify activated issueops v1 binary: %w", err)
		}
		if err := deps.RequireActivation(control, stateRoot, targetSchema, activeBinary); err != nil {
			return err
		}
		if err := requireNoLiveHarnessProcessesV1(stateRoot, deps); err != nil {
			return err
		}
		if receipt, ok, err := readLegacyResetReceiptV1(control); err != nil {
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

		journal, ok, err := readLegacyResetJournalV1(control)
		if err != nil {
			return err
		}
		if ok {
			if err := validateLegacyResetJournalV1(journal, stateRoot, targetSchema, expectedFingerprint); err != nil {
				return err
			}
			if journal.BinaryPath != activeBinary.Executable || journal.BinarySHA256 != activeBinary.SHA256 {
				return fmt.Errorf("in-progress issueops reset is bound to a different staged binary")
			}
		} else {
			manifest, exists, err := buildLegacyResetManifestV1(stateRoot, targetSchema)
			if err != nil {
				return err
			}
			preview := projectLegacyResetPreviewV1(manifest, exists, targetSchema)
			if !exists {
				return fmt.Errorf("legacy IssueOps state is absent and no matching reset receipt exists")
			}
			if manifest.Fingerprint != expectedFingerprint {
				return fmt.Errorf("stale issueops legacy reset fingerprint: expected %s, current %s", expectedFingerprint, manifest.Fingerprint)
			}
			if len(preview.Blockers) != 0 {
				return fmt.Errorf("issueops legacy reset blocked: %s", strings.Join(preview.Blockers, "; "))
			}
			journal = resetLegacyV1Journal{
				SchemaVersion: 1, TargetSchema: targetSchema, StateRoot: stateRoot, LegacyRoot: manifest.LegacyRoot,
				RootDevice: manifest.RootDevice, RootInode: manifest.RootInode,
				Fingerprint: manifest.Fingerprint, Stage: "prepared", Rows: stripLegacyResetRowDataV1(manifest.Rows),
				Entries: orderLegacyResetDeletionEntriesV1(manifest.Entries), Orca: sortedLegacyResetOrcaTasksV1(manifest.Orca),
				TargetRows: preview.RowCount, TargetFiles: preview.FileCount,
				BinaryVersion: activeBinary.Version, BinaryPath: activeBinary.Executable, BinarySHA256: activeBinary.SHA256,
			}
			if err := writeLegacyResetJournalV1(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStepV1(deps, "journal_written"); err != nil {
				return err
			}
		}
		if err := requireNoLiveHarnessProcessesV1(stateRoot, deps); err != nil {
			return err
		}
		if err := requireLegacyResetOrcaQuiescenceV1(spanCtx, journal.Orca, deps.Orca); err != nil {
			return err
		}

		if journal.Stage == "prepared" {
			if err := deleteLegacyResetRowsV1(spanCtx, journal, targetSchema); err != nil {
				return err
			}
			journal.Stage = "rows_deleted"
			if err := writeLegacyResetJournalV1(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStepV1(deps, "rows_deleted"); err != nil {
				return err
			}
		}

		if journal.Stage == "rows_deleted" {
			if err := sqlstore.CloseRoot(journal.LegacyRoot); err != nil {
				return fmt.Errorf("close legacy issueops store: %w", err)
			}
			if err := verifyLegacyResetRootIdentityV1(journal); err != nil {
				return err
			}
			entries, err := refreshLegacyResetSealedEntriesV1(journal)
			if err != nil {
				return err
			}
			journal.Entries = entries
			journal.FileCursor = 0
			journal.Stage = "files"
			if err := writeLegacyResetJournalV1(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStepV1(deps, "files_snapshot"); err != nil {
				return err
			}
		}

		if journal.Stage == "files" {
			if err := deleteLegacyResetEntriesV1(control, &journal, deps); err != nil {
				return err
			}
			journal.Stage = "files_deleted"
			if err := writeLegacyResetJournalV1(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStepV1(deps, "files_deleted"); err != nil {
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
			if err := initializeIssueOpsSchemaV1(spanCtx, stateRoot); err != nil {
				return err
			}
			journal.Stage = "schema_initialized"
			if err := writeLegacyResetJournalV1(control, journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStepV1(deps, "schema_initialized"); err != nil {
				return err
			}
		}

		if journal.Stage != "schema_initialized" {
			return fmt.Errorf("unsupported issueops legacy reset journal stage %q", journal.Stage)
		}
		result = LegacyResetResultV1{
			OK: true, SchemaVersion: 1, TargetSchema: targetSchema, StateRoot: stateRoot,
			Fingerprint: journal.Fingerprint, DeletedRows: journal.TargetRows, DeletedFiles: journal.TargetFiles,
			BinaryVersion: journal.BinaryVersion, CompletedAt: deps.Now().UTC().Format(time.RFC3339Nano), Completed: true,
		}
		if err := writeLegacyResetReceiptV1(control, result); err != nil {
			return err
		}
		return resetLegacyAfterStepV1(deps, "completed")
	})
	if err != nil {
		return LegacyResetResultV1{}, err
	}
	return result, nil
}

func sortedLegacyResetOrcaTasksV1(tasks map[string]LegacyOrcaTaskV1) []LegacyOrcaTaskV1 {
	result := make([]LegacyOrcaTaskV1, 0, len(tasks))
	for _, task := range tasks {
		task.Reconciled = false
		task.NextCommand = ""
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool {
		return legacyResetOrcaAuthorityKeyV1(result[i]) < legacyResetOrcaAuthorityKeyV1(result[j])
	})
	return result
}

func requireLegacyResetOrcaQuiescenceV1(ctx context.Context, tasks []LegacyOrcaTaskV1, deps *LegacyResetOrcaDependenciesV1) error {
	if len(tasks) == 0 {
		return nil
	}
	if deps == nil {
		return fmt.Errorf("issueops legacy reset requires fresh Orca inventory before confirmation")
	}
	for _, task := range tasks {
		if _, err := observeLegacyResetOrcaQuiescenceV1(ctx, task, *deps); err != nil {
			return fmt.Errorf("issueops legacy reset Orca authority %s is not quiescent: %w", task.TaskID, err)
		}
	}
	return nil
}

func deleteLegacyResetRowsV1(ctx context.Context, journal resetLegacyV1Journal, targetSchema int) error {
	manifest, exists, err := buildLegacyResetManifestV1(journal.StateRoot, targetSchema)
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
		absent, err := legacyResetRowsAbsentV1(journal)
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
			if !legacyResetRowMatchesV1(row, data) {
				return fmt.Errorf("legacy reset row %s/%s changed before atomic deletion", row.Bucket, row.ID)
			}
			mutations = append(mutations, sqlstore.Mutation{Bucket: row.Bucket, ID: row.ID, Delete: true})
		}
		return db.Apply(ctx, mutations)
	})
}

func legacyResetRowsAbsentV1(journal resetLegacyV1Journal) (bool, error) {
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

func legacyResetRowMatchesV1(expected legacyResetRowV1, data []byte) bool {
	sum := sha256BytesV1(data)
	return len(data) == expected.Size && sum == expected.SHA256
}

func sha256BytesV1(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func refreshLegacyResetSealedEntriesV1(journal resetLegacyV1Journal) ([]legacyResetEntryV1, error) {
	sealed := make(map[string]legacyResetEntryV1, len(journal.Entries))
	for _, entry := range journal.Entries {
		if entry.Path == "" || !entry.Known {
			return nil, fmt.Errorf("reset journal contains an invalid sealed file target")
		}
		if _, exists := sealed[entry.Path]; exists {
			return nil, fmt.Errorf("reset journal contains duplicate sealed target %q", entry.Path)
		}
		sealed[entry.Path] = entry
	}
	refreshed := make(map[string]legacyResetEntryV1, len(sealed))
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
		current, blocker, err := inspectLegacyEntryV1(path, rel, info)
		if err != nil {
			return err
		}
		if blocker != "" || !current.Known || expected.Path != current.Path || expected.Kind != current.Kind || expected.Mode != current.Mode || expected.Device != current.Device || expected.Inode != current.Inode {
			return fmt.Errorf("legacy reset sealed target %q changed identity after row deletion", rel)
		}
		if current.Kind != "directory" && !legacySQLiteFilePattern.MatchString(filepath.Base(rel)) && !sameLegacyResetEntryV1(expected, current) {
			return fmt.Errorf("legacy reset sealed non-SQLite target %q changed after preview", rel)
		}
		refreshed[rel] = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	entries := make([]legacyResetEntryV1, 0, len(journal.Entries))
	for _, entry := range journal.Entries {
		if current, ok := refreshed[entry.Path]; ok {
			entries = append(entries, current)
		} else {
			entries = append(entries, entry)
		}
	}
	return orderLegacyResetDeletionEntriesV1(entries), nil
}

func orderLegacyResetDeletionEntriesV1(entries []legacyResetEntryV1) []legacyResetEntryV1 {
	ordered := append([]legacyResetEntryV1(nil), entries...)
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

func deleteLegacyResetEntriesV1(control *sqlstore.DB, journal *resetLegacyV1Journal, deps resetLegacyV1Deps) error {
	for journal.FileCursor < len(journal.Entries) {
		if err := verifyNoUnsealedLegacyResetEntriesV1(*journal); err != nil {
			return err
		}
		entry := journal.Entries[journal.FileCursor]
		path, err := safeLegacyResetTargetV1(journal.LegacyRoot, entry.Path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			journal.FileCursor++
			if err := writeLegacyResetJournalV1(control, *journal); err != nil {
				return err
			}
			if err := resetLegacyAfterStepV1(deps, fmt.Sprintf("file_deleted:%d", journal.FileCursor)); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		current, blocker, err := inspectLegacyEntryV1(path, entry.Path, info)
		if err != nil {
			return err
		}
		if blocker != "" || !sameLegacyResetEntryV1(entry, current) {
			return fmt.Errorf("legacy reset target %q changed after deletion snapshot", entry.Path)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete exact legacy reset target %q: %w", entry.Path, err)
		}
		journal.FileCursor++
		if err := writeLegacyResetJournalV1(control, *journal); err != nil {
			return err
		}
		if err := resetLegacyAfterStepV1(deps, fmt.Sprintf("file_deleted:%d", journal.FileCursor)); err != nil {
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
	device, inode, _ := fileIdentityV1(info)
	if !info.IsDir() || device != journal.RootDevice || inode != journal.RootInode {
		return fmt.Errorf("legacy reset root identity changed before final removal")
	}
	if err := os.Remove(journal.LegacyRoot); err != nil {
		return fmt.Errorf("remove empty legacy issueops root: %w", err)
	}
	return nil
}

func verifyNoUnsealedLegacyResetEntriesV1(journal resetLegacyV1Journal) error {
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

func sameLegacyResetEntryV1(expected, current legacyResetEntryV1) bool {
	if expected.Path != current.Path || expected.Kind != current.Kind || expected.Mode != current.Mode || expected.Device != current.Device || expected.Inode != current.Inode || !current.Known {
		return false
	}
	if expected.Kind == "directory" {
		return true
	}
	return expected.Size == current.Size && expected.SHA256 == current.SHA256
}

func safeLegacyResetTargetV1(root, rel string) (string, error) {
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

func initializeIssueOpsSchemaV1(ctx context.Context, stateRoot string) error {
	v1Root := filepath.Join(stateRoot, issueOpsV1Directory)
	db, err := sqlstore.Open(v1Root)
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, func(context.Context) error {
		expected := []byte(`{"schema_version":1}`)
		current, ok, err := db.Get(issueOpsMetaV1Bucket, issueOpsSchemaMarkerV1ID)
		if err != nil {
			return err
		}
		if ok {
			if string(current) != string(expected) {
				return fmt.Errorf("issueops v1 schema marker is not canonical")
			}
			return nil
		}
		return db.Put(issueOpsMetaV1Bucket, issueOpsSchemaMarkerV1ID, expected)
	})
}

func stripLegacyResetRowDataV1(rows []legacyResetRowV1) []legacyResetRowV1 {
	result := make([]legacyResetRowV1, len(rows))
	for i, row := range rows {
		row.Data = nil
		result[i] = row
	}
	return result
}

func readLegacyResetJournalV1(db *sqlstore.DB) (resetLegacyV1Journal, bool, error) {
	data, ok, err := db.Get(issueOpsResetV1Bucket, issueOpsResetV1JournalID)
	if err != nil || !ok {
		return resetLegacyV1Journal{}, ok, err
	}
	var journal resetLegacyV1Journal
	if err := json.Unmarshal(data, &journal); err != nil {
		return resetLegacyV1Journal{}, false, fmt.Errorf("decode issueops reset journal: %w", err)
	}
	return journal, true, nil
}

func writeLegacyResetJournalV1(db *sqlstore.DB, journal resetLegacyV1Journal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return db.Put(issueOpsResetV1Bucket, issueOpsResetV1JournalID, data)
}

func validateLegacyResetJournalV1(journal resetLegacyV1Journal, stateRoot string, targetSchema int, fingerprint string) error {
	if journal.SchemaVersion != 1 || journal.TargetSchema != targetSchema || journal.StateRoot != stateRoot || journal.LegacyRoot != filepath.Join(stateRoot, issueOpsLegacyDirectory) || journal.Fingerprint != fingerprint || journal.RootDevice == 0 || journal.RootInode == 0 {
		return fmt.Errorf("in-progress issueops reset journal does not match the requested state root, target schema, and fingerprint")
	}
	if journal.Stage == "prepared" && len(journal.Entries) == 0 && journal.TargetFiles != 0 {
		return fmt.Errorf("in-progress issueops reset journal is missing its sealed preview file manifest")
	}
	return nil
}

func verifyLegacyResetRootIdentityV1(journal resetLegacyV1Journal) error {
	info, err := os.Lstat(journal.LegacyRoot)
	if err != nil {
		return fmt.Errorf("verify legacy reset root identity: %w", err)
	}
	device, inode, ok := fileIdentityV1(info)
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || device != journal.RootDevice || inode != journal.RootInode {
		return fmt.Errorf("legacy reset root identity changed after the deletion journal was sealed")
	}
	return nil
}

func readLegacyResetReceiptV1(db *sqlstore.DB) (LegacyResetResultV1, bool, error) {
	data, ok, err := db.Get(issueOpsResetV1Bucket, issueOpsResetV1ReceiptID)
	if err != nil || !ok {
		return LegacyResetResultV1{}, ok, err
	}
	var receipt LegacyResetResultV1
	if err := json.Unmarshal(data, &receipt); err != nil {
		return LegacyResetResultV1{}, false, fmt.Errorf("decode issueops reset receipt: %w", err)
	}
	return receipt, true, nil
}

func writeLegacyResetReceiptV1(db *sqlstore.DB, result LegacyResetResultV1) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	mutations := []sqlstore.Mutation{
		{Bucket: issueOpsResetV1Bucket, ID: issueOpsResetV1ReceiptID, Data: data},
		{Bucket: issueOpsResetV1Bucket, ID: issueOpsResetV1JournalID, Delete: true},
	}
	ids, err := db.List(issueOpsResetV1Bucket)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if strings.HasPrefix(id, legacyResetRemoteReceiptPrefixV1) || strings.HasPrefix(id, legacyResetOrcaReceiptPrefixV1) || strings.HasPrefix(id, legacyResetCycleReceiptPrefixV1) {
			mutations = append(mutations, sqlstore.Mutation{Bucket: issueOpsResetV1Bucket, ID: id, Delete: true})
		}
	}
	return db.Apply(context.Background(), mutations)
}

func resetLegacyAfterStepV1(deps resetLegacyV1Deps, step string) error {
	if deps.AfterStep == nil {
		return nil
	}
	return deps.AfterStep(step)
}

func requireNoLiveHarnessProcessesV1(stateRoot string, deps resetLegacyV1Deps) error {
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

func defaultResetLegacyV1Deps() resetLegacyV1Deps {
	return resetLegacyV1Deps{
		Now:               time.Now,
		ActiveBinary:      activeResetLegacyBinaryIdentityV1,
		LiveProcesses:     liveHarnessProcessesV1,
		RequireActivation: requireLegacyResetActivationV1,
	}
}

func resetLegacyBinaryVersionV1() string {
	if info, ok := debug.ReadBuildInfo(); ok && strings.TrimSpace(info.Main.Version) != "" {
		return strings.TrimSpace(info.Main.Version)
	}
	return "devel"
}

func activeResetLegacyBinaryIdentityV1() (resetLegacyV1BinaryIdentity, error) {
	executable, err := os.Executable()
	if err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	return resetLegacyBinaryIdentityFromPathV1(executable, resetLegacyBinaryVersionV1())
}

func validateResetLegacyBinaryIdentityV1(identity resetLegacyV1BinaryIdentity) error {
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
