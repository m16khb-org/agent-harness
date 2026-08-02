package issueops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"agent-harness/internal/contract/issueops"
	"agent-harness/internal/core/sqlstore"
)

// 현재 schema namespace 이름들은 issueops.IssueOpsSchemaVersion에서 파생된다.
// 값은 온디스크 계약이므로 파생 결과가 기존 이름과 동일해야 한다.
var (
	issueOpsDirectory      = issueOpsBucket
	issueOpsResetDirectory = fmt.Sprintf("issueops_reset_v%d", issueops.IssueOpsSchemaVersion)
	issueOpsResetBucket    = fmt.Sprintf("issueops_reset_v%d", issueops.IssueOpsSchemaVersion)
	issueOpsMetaBucket     = fmt.Sprintf("issueops_meta_v%d", issueops.IssueOpsSchemaVersion)
)

const (
	issueOpsLegacyDirectory    = "issueops"
	issueOpsResetJournalID     = "journal"
	issueOpsResetReceiptID     = "receipt"
	issueOpsSchemaMarkerID     = "schema"
	legacyResetManifestVersion = 1
)

var (
	legacyCycleFilePattern  = regexp.MustCompile(`^io-[0-9a-f]{12}\.json$`)
	legacyCycleLockPattern  = regexp.MustCompile(`^io-[0-9a-f]{12}\.lock$`)
	legacyCycleTempPattern  = regexp.MustCompile(`^\.io-[0-9a-f]{12}-[A-Za-z0-9._-]+\.tmp$`)
	legacyCycleBackup       = regexp.MustCompile(`^io-[0-9a-f]{12}\.json\.backup-[0-9]{8}T[0-9]{6}$`)
	legacySessionFile       = regexp.MustCompile(`^issueops-session-[0-9a-f]{16}(?:-io-[0-9a-f]{12})?\.json$`)
	legacySessionLock       = regexp.MustCompile(`^issueops-session-[0-9a-f]{16}(?:-io-[0-9a-f]{12})?\.lock$`)
	legacySessionTemp       = regexp.MustCompile(`^\.issueops-session-[0-9a-f]{16}(?:-io-[0-9a-f]{12})?-[A-Za-z0-9._-]+\.tmp$`)
	legacyPublicationRoot   = regexp.MustCompile(`^publication-locks/[0-9a-f]{64}$`)
	legacyRemoteCreateRoot  = regexp.MustCompile(`^remote-create-live/io-[0-9a-f]{12}$`)
	legacySQLiteFilePattern = regexp.MustCompile(`^(?:harness\.db|harness\.lock\.db)(?:-wal|-shm|-journal)?$`)
)

var legacyPreSchemaFields = map[string]struct{}{
	"ok": {}, "id": {}, "repo": {}, "branch": {}, "phase": {},
	"intent": {}, "design_review": {}, "domain_review": {}, "issue_url": {}, "plan_path": {}, "worktree_path": {},
	"issue_links": {}, "branch_prepare": {}, "remote_artifact": {}, "decisions": {}, "plan_prep": {},
	"worktree_tools": {}, "execution_decision": {}, "compatibility_review": {}, "devils_advocate_review": {},
	"feedback": {}, "routing_trace": {}, "ai_slop_clean_at": {}, "ai_slop_clean_head": {},
	"ai_slop_clean_fingerprint": {}, "ai_slop_clean_categories": {}, "ai_slop_clean_verification": {},
	"force_released_at": {}, "force_release_reason": {}, "stale_reset_at": {}, "stale_reset_prior_phase": {},
	"orphan_worktree_path": {}, "last_heartbeat_at": {}, "phase_ledger": {}, "created_at": {}, "updated_at": {},
}

var legacyPreSchemaPhases = map[string]struct{}{
	"problem": {}, "grill": {}, "plan": {}, "compatibility-review": {}, "implement": {},
	"ai-slop-clean": {}, "feedback": {}, "pr": {}, "done": {},
}

var legacyExternalAuthorityFields = map[string]struct{}{
	"execution_handoff": {}, "execution_workspace": {}, "ownership": {}, "remote_create_claim": {},
}

type LegacyResetRowCounts struct {
	IssueOps int `json:"issueops"`
	Session  int `json:"session"`
}

type LegacyRemoteCreateClaim struct {
	LifecycleID     string `json:"lifecycle_id"`
	ClaimID         string `json:"claim_id"`
	Provider        string `json:"provider,omitempty"`
	Kind            string `json:"kind,omitempty"`
	State           string `json:"state,omitempty"`
	InvocationState string `json:"invocation_state,omitempty"`
	KnownURL        string `json:"known_url,omitempty"`
	Reconciled      bool   `json:"reconciled"`
	NextCommand     string `json:"next_command,omitempty"`
}

type LegacyOrcaTask struct {
	LifecycleID string `json:"lifecycle_id"`
	State       string `json:"state,omitempty"`
	RuntimeID   string `json:"runtime_id,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	DispatchID  string `json:"dispatch_id,omitempty"`
	Reconciled  bool   `json:"reconciled"`
	NextCommand string `json:"next_command,omitempty"`
}

type LegacyResetPreview struct {
	OK                 bool                      `json:"ok"`
	SchemaVersion      int                       `json:"schema_version"`
	TargetSchema       int                       `json:"target_schema"`
	StateRoot          string                    `json:"state_root"`
	LegacyRoot         string                    `json:"legacy_root"`
	SchemaRoot         string                    `json:"v1_root"`
	ResetRequired      bool                      `json:"reset_required"`
	Fingerprint        string                    `json:"fingerprint"`
	RowCount           int                       `json:"row_count"`
	RowCounts          LegacyResetRowCounts      `json:"row_counts"`
	FileCount          int                       `json:"file_count"`
	ActiveCycles       []string                  `json:"active_cycles"`
	DrainedCycles      []string                  `json:"drained_cycles"`
	OrcaTasks          []LegacyOrcaTask          `json:"orca_tasks"`
	RemoteCreateClaims []LegacyRemoteCreateClaim `json:"remote_create_claims"`
	Blockers           []string                  `json:"blockers"`
	CanConfirm         bool                      `json:"can_confirm"`
	NextCommand        string                    `json:"next_command,omitempty"`
}

type LegacyResetResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	TargetSchema  int    `json:"target_schema"`
	StateRoot     string `json:"state_root"`
	Fingerprint   string `json:"fingerprint"`
	DeletedRows   int    `json:"deleted_rows"`
	DeletedFiles  int    `json:"deleted_files"`
	BinaryVersion string `json:"binary_version"`
	CompletedAt   string `json:"completed_at"`
	Completed     bool   `json:"completed"`
}

type LegacyResetStatus struct {
	OK            bool                         `json:"ok"`
	SchemaVersion int                          `json:"schema_version"`
	TargetSchema  int                          `json:"target_schema"`
	Preview       LegacyResetPreview           `json:"preview"`
	InProgress    bool                         `json:"in_progress"`
	Stage         string                       `json:"stage,omitempty"`
	Receipt       *LegacyResetResult           `json:"receipt,omitempty"`
	Activation    *LegacyResetActivationResult `json:"activation,omitempty"`
}

// ResetRequiredError is the fail-closed response returned by every v1
// mutation while legacy IssueOps state still exists.
type ResetRequiredError struct {
	Code         string `json:"code"`
	TargetSchema int    `json:"target_schema"`
	StateRoot    string `json:"state_root"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	NextCommand  string `json:"next_command"`
	PreviewError string `json:"preview_error,omitempty"`
}

func (e *ResetRequiredError) Error() string {
	if e == nil {
		return "issueops legacy reset required"
	}
	if e.PreviewError != "" {
		return fmt.Sprintf("issueops legacy reset required: %s; next: %s", e.PreviewError, e.NextCommand)
	}
	return fmt.Sprintf("issueops legacy reset required for target schema %d; next: %s", e.TargetSchema, e.NextCommand)
}

func (e *ResetRequiredError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":          e.Code,
		"target_schema": e.TargetSchema,
		"state_root":    e.StateRoot,
		"fingerprint":   e.Fingerprint,
		"next_command":  e.NextCommand,
		"preview_error": e.PreviewError,
	}
}

type legacyResetManifest struct {
	StateRoot     string
	LegacyRoot    string
	RootDevice    uint64
	RootInode     uint64
	Rows          []legacyResetRow
	Entries       []legacyResetEntry
	Blockers      []string
	Active        map[string]struct{}
	Claims        map[string]LegacyRemoteCreateClaim
	Orca          map[string]LegacyOrcaTask
	DrainedClaims map[string]struct{}
	DrainedOrca   map[string]struct{}
	DrainedCycles map[string]struct{}
	Fingerprint   string
}

type legacyResetRow struct {
	Store  string `json:"store"`
	Bucket string `json:"bucket"`
	ID     string `json:"id"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
	Data   []byte `json:"-"`
}

type legacyResetEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	Device uint64 `json:"device,omitempty"`
	Inode  uint64 `json:"inode,omitempty"`
	Known  bool   `json:"known"`
}

type legacyRecordHeader struct {
	SchemaVersion      int                               `json:"schema_version"`
	ID                 string                            `json:"id"`
	Repo               string                            `json:"repo"`
	IssueURL           string                            `json:"issue_url"`
	Phase              string                            `json:"phase"`
	CycleState         string                            `json:"cycle_state"`
	RemoteCreateClaim  *legacyRemoteCreateClaimAuthority `json:"remote_create_claim"`
	ExecutionHandoff   *legacyExecutionAuthority         `json:"execution_handoff"`
	ExecutionWorkspace *legacyExecutionAuthority         `json:"execution_workspace"`
	Ownership          *legacyOwnershipLedger            `json:"ownership"`
}

type legacyRemoteCreateClaimAuthority struct {
	ClaimID         string   `json:"claim_id"`
	Provider        string   `json:"provider"`
	Kind            string   `json:"kind"`
	ProjectKey      string   `json:"project_key"`
	Head            string   `json:"head"`
	Base            string   `json:"base"`
	FinalHead       string   `json:"final_head"`
	Title           string   `json:"title"`
	BodySHA256      string   `json:"body_sha256"`
	Labels          []string `json:"labels"`
	Assignees       []string `json:"assignees"`
	Draft           bool     `json:"draft"`
	State           string   `json:"state"`
	InvocationState string   `json:"invocation_state"`
	KnownURL        string   `json:"known_url"`
}

type legacyExecutionAuthority struct {
	State            string                           `json:"state"`
	Orca             *legacyOrcaIdentity              `json:"orca"`
	PendingOperation *legacyExecutionPendingOperation `json:"pending_operation"`
	Attempt          int                              `json:"attempt"`
	OwnershipEpoch   string                           `json:"ownership_epoch"`
}

type legacyOrcaIdentity struct {
	RuntimeID  string `json:"runtime_id"`
	TaskID     string `json:"task_id"`
	DispatchID string `json:"dispatch_id"`
}

type legacyExecutionPendingOperation struct {
	Kind            string   `json:"kind"`
	StartedAt       string   `json:"started_at"`
	BaselineTaskIDs []string `json:"baseline_task_ids"`
}

type legacyOwnershipLedger struct {
	ActiveAttempt int                      `json:"active_attempt"`
	Attempts      []legacyOwnershipAttempt `json:"attempts"`
}

type legacyOwnershipAttempt struct {
	Number    int                       `json:"number"`
	Workspace *legacyExecutionAuthority `json:"workspace"`
	Handoff   *legacyExecutionAuthority `json:"handoff"`
	StartedAt string                    `json:"started_at"`
	ClosedAt  string                    `json:"closed_at"`
}

// PreviewLegacyReset builds a read-only, deterministic manifest. It never
// creates the legacy root, a reset journal, or a raw payload backup.
func PreviewLegacyReset(stateDir string, targetSchema int) (LegacyResetPreview, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, targetSchema)
	if err != nil {
		return LegacyResetPreview{}, err
	}
	manifest, exists, err := buildLegacyResetManifest(stateRoot, targetSchema)
	if err != nil {
		return LegacyResetPreview{}, err
	}
	preview := projectLegacyResetPreview(manifest, exists, targetSchema)
	return preview, nil
}

// StatusLegacyReset combines the current read-only preview with any
// hash-only in-progress journal or completed receipt. It does not create the
// reset-control store when none exists.
func StatusLegacyReset(stateDir string, targetSchema int) (LegacyResetStatus, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, targetSchema)
	if err != nil {
		return LegacyResetStatus{}, err
	}
	preview, err := PreviewLegacyReset(stateRoot, targetSchema)
	if err != nil {
		return LegacyResetStatus{}, err
	}
	status := LegacyResetStatus{OK: true, SchemaVersion: 1, TargetSchema: targetSchema, Preview: preview}
	controlRoot := filepath.Join(stateRoot, issueOpsResetDirectory)
	if raw, ok, err := sqlstore.GetExisting(controlRoot, issueOpsResetBucket, issueOpsResetJournalID); err == nil && ok {
		var journal resetLegacyJournal
		if err := json.Unmarshal(raw, &journal); err != nil {
			return LegacyResetStatus{}, fmt.Errorf("decode issueops reset journal: %w", err)
		}
		status.InProgress = true
		status.Stage = journal.Stage
	} else if err != nil && !isLegacyResetMissing(err) {
		return LegacyResetStatus{}, err
	}
	if raw, ok, err := sqlstore.GetExisting(controlRoot, issueOpsResetBucket, issueOpsResetReceiptID); err == nil && ok {
		var receipt LegacyResetResult
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return LegacyResetStatus{}, fmt.Errorf("decode issueops reset receipt: %w", err)
		}
		status.Receipt = &receipt
	} else if err != nil && !isLegacyResetMissing(err) {
		return LegacyResetStatus{}, err
	}
	if raw, ok, err := sqlstore.GetExisting(controlRoot, issueOpsResetBucket, issueOpsResetActivationPendingID); err == nil && ok {
		var pending legacyResetActivationPending
		if err := json.Unmarshal(raw, &pending); err != nil {
			return LegacyResetStatus{}, fmt.Errorf("decode native activation pending marker: %w", err)
		}
		activation := legacyResetActivationResult(pending, false, pending.StartedAt)
		status.Activation = &activation
	} else if err != nil && !isLegacyResetMissing(err) {
		return LegacyResetStatus{}, err
	} else if raw, ok, err := sqlstore.GetExisting(controlRoot, issueOpsResetBucket, issueOpsResetActivationReceiptID); err == nil && ok {
		var receipt legacyResetActivationReceipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return LegacyResetStatus{}, fmt.Errorf("decode native activation receipt: %w", err)
		}
		activation := legacyResetActivationReceiptResult(receipt)
		status.Activation = &activation
	} else if err != nil && !isLegacyResetMissing(err) {
		return LegacyResetStatus{}, err
	}
	return status, nil
}

func normalizeLegacyResetStateDir(stateDir string, targetSchema int) (string, error) {
	if targetSchema != 1 {
		return "", fmt.Errorf("issueops legacy reset requires --target-schema 1")
	}
	stateDir = strings.TrimSpace(stateDir)
	if stateDir == "" {
		return "", fmt.Errorf("issueops legacy reset state root is required")
	}
	abs, err := filepath.Abs(stateDir)
	if err != nil {
		return "", fmt.Errorf("resolve issueops reset state root: %w", err)
	}
	abs = filepath.Clean(abs)
	if abs == string(filepath.Separator) || filepath.Dir(abs) == abs {
		return "", fmt.Errorf("refusing broad issueops reset state root %s", abs)
	}
	return abs, nil
}

func buildLegacyResetManifest(stateRoot string, targetSchema int) (legacyResetManifest, bool, error) {
	legacyRoot := filepath.Join(stateRoot, issueOpsLegacyDirectory)
	manifest := legacyResetManifest{
		StateRoot: stateRoot, LegacyRoot: legacyRoot,
		Active: map[string]struct{}{}, Claims: map[string]LegacyRemoteCreateClaim{}, Orca: map[string]LegacyOrcaTask{},
		DrainedClaims: map[string]struct{}{}, DrainedOrca: map[string]struct{}{}, DrainedCycles: map[string]struct{}{},
	}
	rootInfo, err := os.Lstat(legacyRoot)
	if os.IsNotExist(err) {
		manifest.Fingerprint = fingerprintLegacyResetManifest(manifest, targetSchema)
		return manifest, false, nil
	}
	if err != nil {
		return manifest, false, fmt.Errorf("inspect legacy issueops root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy root is not a real directory: %s", legacyRoot))
		manifest.Fingerprint = fingerprintLegacyResetManifest(manifest, targetSchema)
		return manifest, true, nil
	}
	manifest.RootDevice, manifest.RootInode, _ = fileIdentity(rootInfo)

	storeRoots, storeBlockers, err := inspectLegacySQLStores(legacyRoot, &manifest)
	if err != nil {
		return manifest, true, err
	}
	manifest.Blockers = append(manifest.Blockers, storeBlockers...)

	err = filepath.WalkDir(legacyRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == legacyRoot {
			return nil
		}
		rel, err := filepath.Rel(legacyRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		manifestEntry, blocker, err := inspectLegacyEntry(path, rel, info)
		if err != nil {
			return err
		}
		manifest.Entries = append(manifest.Entries, manifestEntry)
		if blocker != "" {
			manifest.Blockers = append(manifest.Blockers, blocker)
		}
		if manifestEntry.Known && legacyCycleFilePattern.MatchString(filepath.Base(rel)) && !strings.Contains(rel, "/") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			inspectLegacyRecordAuthority(filepath.Base(strings.TrimSuffix(rel, ".json")), data, &manifest)
		}
		return nil
	})
	if err != nil {
		return manifest, true, fmt.Errorf("walk legacy issueops manifest: %w", err)
	}

	_ = storeRoots
	sort.Slice(manifest.Rows, func(i, j int) bool {
		left, right := manifest.Rows[i], manifest.Rows[j]
		if left.Store != right.Store {
			return left.Store < right.Store
		}
		if left.Bucket != right.Bucket {
			return left.Bucket < right.Bucket
		}
		return left.ID < right.ID
	})
	sort.Slice(manifest.Entries, func(i, j int) bool { return manifest.Entries[i].Path < manifest.Entries[j].Path })
	manifest.Blockers = sortedUniqueStrings(manifest.Blockers)
	manifest.Fingerprint = fingerprintLegacyResetManifest(manifest, targetSchema)
	if err := loadLegacyResetDrainReceipts(&manifest); err != nil {
		return manifest, true, err
	}
	return manifest, true, nil
}

func inspectLegacySQLStores(legacyRoot string, manifest *legacyResetManifest) ([]string, []string, error) {
	storeRoots := []string{}
	blockers := []string{}
	sqliteRoots := map[string]struct{}{}
	walkErr := filepath.WalkDir(legacyRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if legacySQLiteFilePattern.MatchString(entry.Name()) {
			sqliteRoots[filepath.Dir(path)] = struct{}{}
		}
		if entry.Name() != "harness.db" {
			return nil
		}
		storeRoots = append(storeRoots, filepath.Dir(path))
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	sort.Strings(storeRoots)
	completeStoreRoots := make(map[string]struct{}, len(storeRoots))
	for _, storeRoot := range storeRoots {
		completeStoreRoots[storeRoot] = struct{}{}
	}
	for sqliteRoot := range sqliteRoots {
		if _, ok := completeStoreRoots[sqliteRoot]; ok {
			continue
		}
		rel, _ := filepath.Rel(legacyRoot, sqliteRoot)
		blockers = append(blockers, fmt.Sprintf("incomplete legacy SQLite family at %q", filepath.ToSlash(rel)))
	}
	for _, storeRoot := range storeRoots {
		rel, err := filepath.Rel(legacyRoot, storeRoot)
		if err != nil {
			return nil, nil, err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		layout, err := sqlstore.InspectExisting(storeRoot)
		if err != nil {
			blockers = append(blockers, fmt.Sprintf("cannot inspect legacy sqlstore %q: %v", rel, err))
			continue
		}
		if !canonicalLegacySQLStoreLayout(layout) {
			blockers = append(blockers, fmt.Sprintf("unknown legacy SQLite schema at %q", rel))
		}
		rootStore := rel == ""
		for _, bucket := range layout.Buckets {
			if !rootStore || (bucket != "issueops" && bucket != "session") {
				blockers = append(blockers, fmt.Sprintf("unknown legacy SQLite bucket %q at %q", bucket, rel))
			}
			rows, err := sqlstore.GetAllExisting(storeRoot, bucket)
			if err != nil {
				return nil, nil, err
			}
			for _, row := range rows {
				sum := sha256.Sum256(row.Data)
				manifest.Rows = append(manifest.Rows, legacyResetRow{
					Store: rel, Bucket: bucket, ID: row.ID, Size: len(row.Data), SHA256: hex.EncodeToString(sum[:]), Data: append([]byte(nil), row.Data...),
				})
				if rootStore && bucket == "issueops" {
					inspectLegacyRecordAuthority(row.ID, row.Data, manifest)
				}
			}
		}
	}
	return storeRoots, blockers, nil
}

func canonicalLegacySQLStoreLayout(layout sqlstore.ExistingLayout) bool {
	if len(layout.DataSchema) != 1 || len(layout.SpanSchema) != 1 {
		return false
	}
	data, span := layout.DataSchema[0], layout.SpanSchema[0]
	return data.Type == "table" && data.Name == "records" && data.Table == "records" &&
		span.Type == "table" && span.Name == "span" && span.Table == "span"
}

func inspectLegacyEntry(path, rel string, info fs.FileInfo) (legacyResetEntry, string, error) {
	entry := legacyResetEntry{Path: rel, Mode: uint32(info.Mode()), Size: info.Size()}
	entry.Device, entry.Inode, _ = fileIdentity(info)
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		entry.Kind = "symlink"
		target, err := os.Readlink(path)
		if err != nil {
			return entry, "", err
		}
		sum := sha256.Sum256([]byte(target))
		entry.SHA256 = hex.EncodeToString(sum[:])
		return entry, fmt.Sprintf("unknown legacy entry %q: symbolic links are forbidden", rel), nil
	case info.IsDir():
		entry.Kind = "directory"
		entry.Known = knownLegacyDirectory(rel)
	case info.Mode().IsRegular():
		entry.Kind = "file"
		entry.Known = knownLegacyFile(rel)
		sum, err := hashFile(path)
		if err != nil {
			return entry, "", err
		}
		entry.SHA256 = sum
		if nlink := fileLinkCount(info); nlink > 1 {
			return entry, fmt.Sprintf("unknown legacy entry %q: hard link count is %d", rel, nlink), nil
		}
	default:
		entry.Kind = "other"
		return entry, fmt.Sprintf("unknown legacy entry %q: unsupported file type", rel), nil
	}
	if !entry.Known {
		return entry, fmt.Sprintf("unknown legacy entry %q", rel), nil
	}
	return entry, "", nil
}

func knownLegacyDirectory(rel string) bool {
	return rel == "publication-locks" || rel == "remote-create-live" ||
		legacyPublicationRoot.MatchString(rel) || legacyRemoteCreateRoot.MatchString(rel)
}

func knownLegacyFile(rel string) bool {
	if !strings.Contains(rel, "/") {
		return legacySQLiteFilePattern.MatchString(rel) || legacyCycleFilePattern.MatchString(rel) ||
			legacyCycleLockPattern.MatchString(rel) || legacyCycleTempPattern.MatchString(rel) || legacyCycleBackup.MatchString(rel) ||
			legacySessionFile.MatchString(rel) || legacySessionLock.MatchString(rel) || legacySessionTemp.MatchString(rel)
	}
	dir, name := filepath.ToSlash(filepath.Dir(rel)), filepath.Base(rel)
	return (legacyPublicationRoot.MatchString(dir) || legacyRemoteCreateRoot.MatchString(dir)) && legacySQLiteFilePattern.MatchString(name)
}

func inspectLegacyRecordAuthority(expectedID string, data []byte, manifest *legacyResetManifest) {
	var record legacyRecordHeader
	if err := json.Unmarshal(data, &record); err != nil {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("cannot decode legacy cycle %s: %v", expectedID, err))
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("cannot decode legacy cycle fields %s: %v", expectedID, err))
		return
	}
	if record.ID != expectedID {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle id mismatch: key %s contains %q", expectedID, record.ID))
	}
	_, hasSchemaVersion := fields["schema_version"]
	preSchema := !hasSchemaVersion && validateLegacyPreSchemaRecord(expectedID, record, fields, manifest)
	if !preSchema && (record.SchemaVersion < 7 || record.SchemaVersion > 9) {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("unsupported legacy cycle schema %d for %s", record.SchemaVersion, expectedID))
	}
	phase, state := strings.TrimSpace(record.Phase), strings.TrimSpace(record.CycleState)
	closed := state == "closed" || (state == "" && phase == "done")
	if !closed {
		manifest.Active[expectedID] = struct{}{}
	}
	if claim := record.RemoteCreateClaim; claim != nil {
		projected := LegacyRemoteCreateClaim{
			LifecycleID: expectedID, ClaimID: strings.TrimSpace(claim.ClaimID), Provider: strings.TrimSpace(claim.Provider), Kind: strings.TrimSpace(claim.Kind),
			State: strings.TrimSpace(claim.State), InvocationState: strings.TrimSpace(claim.InvocationState), KnownURL: strings.TrimSpace(claim.KnownURL),
		}
		key := projected.LifecycleID + "\x00" + projected.ClaimID
		manifest.Claims[key] = projected
	}
	inspectLegacyExecutionAuthorities(expectedID, record, manifest)
}

func validateLegacyPreSchemaRecord(id string, record legacyRecordHeader, fields map[string]json.RawMessage, manifest *legacyResetManifest) bool {
	valid := true
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, forbidden := legacyExternalAuthorityFields[key]; forbidden {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s contains forbidden external authority field %q", id, key))
			valid = false
			continue
		}
		if _, known := legacyPreSchemaFields[key]; !known {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s contains unknown pre-schema field %q", id, key))
			valid = false
			continue
		}
		if !validLegacyPreSchemaFieldShape(key, fields[key]) {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s contains invalid shape for field %q", id, key))
			valid = false
		}
	}
	var ok bool
	if raw, exists := fields["ok"]; !exists || json.Unmarshal(raw, &ok) != nil || !ok {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s has invalid ok marker", id))
		valid = false
	}
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Repo) == "" || strings.TrimSpace(record.Phase) == "" {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s is missing required identity fields", id))
		valid = false
	}
	if _, known := legacyPreSchemaPhases[strings.TrimSpace(record.Phase)]; !known {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s has unknown phase %q", id, record.Phase))
		valid = false
	}
	for _, key := range []string{"created_at", "updated_at"} {
		var value string
		raw, exists := fields[key]
		if !exists || json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value) == "" {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pre-schema cycle %s has invalid %s", id, key))
			valid = false
		}
	}
	return valid
}

func validLegacyPreSchemaFieldShape(key string, raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return false
	}
	switch key {
	case "ok":
		return bytes.Equal(trimmed, []byte("true")) || bytes.Equal(trimmed, []byte("false"))
	case "id", "repo", "branch", "phase", "issue_url", "plan_path", "worktree_path",
		"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "force_released_at",
		"force_release_reason", "stale_reset_at", "stale_reset_prior_phase", "orphan_worktree_path",
		"last_heartbeat_at", "created_at", "updated_at":
		return trimmed[0] == '"'
	case "issue_links", "decisions", "feedback", "routing_trace", "ai_slop_clean_categories", "ai_slop_clean_verification":
		return trimmed[0] == '['
	default:
		return trimmed[0] == '{'
	}
}

func inspectLegacyOrcaAuthority(id string, authority *legacyExecutionAuthority, manifest *legacyResetManifest) {
	if authority == nil {
		return
	}
	state := strings.TrimSpace(authority.State)
	pendingPresent := authority.PendingOperation != nil
	pendingKind := ""
	if pendingPresent {
		pendingKind = strings.TrimSpace(authority.PendingOperation.Kind)
	}
	if authority.Orca == nil {
		if pendingPresent {
			manifest.Blockers = append(manifest.Blockers, legacyPendingOrcaBlocker(id, pendingKind))
		}
		return
	}
	task := LegacyOrcaTask{
		LifecycleID: id, State: state, RuntimeID: strings.TrimSpace(authority.Orca.RuntimeID),
		TaskID: strings.TrimSpace(authority.Orca.TaskID), DispatchID: strings.TrimSpace(authority.Orca.DispatchID),
	}
	if task.TaskID == "" {
		if task.DispatchID != "" {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy Orca dispatch without task id for %s", id))
		} else if pendingPresent {
			manifest.Blockers = append(manifest.Blockers, legacyPendingOrcaBlocker(id, pendingKind))
		}
		return
	}
	if pendingPresent && pendingKind != "task_create" && pendingKind != "dispatch" {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy pending Orca %s for %s cannot be reconciled from exact task authority", pendingKind, id))
	}
	manifest.Orca[legacyResetOrcaAuthorityKey(task)] = task
}

func inspectLegacyExecutionAuthorities(id string, record legacyRecordHeader, manifest *legacyResetManifest) {
	switch record.SchemaVersion {
	case 7, 8:
		if record.Ownership != nil {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy schema-v%d cycle %s contains unexpected ownership ledger", record.SchemaVersion, id))
			inspectLegacyOwnershipAuthorities(id, record.Ownership, manifest)
		}
		inspectLegacyOrcaAuthority(id, record.ExecutionWorkspace, manifest)
		inspectLegacyOrcaAuthority(id, record.ExecutionHandoff, manifest)
	case 9:
		if record.ExecutionWorkspace != nil || record.ExecutionHandoff != nil {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s schema-v9 ownership authority must not use top-level execution fields", id))
			// Even malformed mixed records can retain live external authority.
			inspectLegacyOrcaAuthority(id, record.ExecutionWorkspace, manifest)
			inspectLegacyOrcaAuthority(id, record.ExecutionHandoff, manifest)
		}
		inspectLegacyOwnershipAuthorities(id, record.Ownership, manifest)
	default:
		// Unsupported records remain blocked, but known authority locations are
		// still projected so a live external writer cannot be hidden by version drift.
		inspectLegacyOrcaAuthority(id, record.ExecutionWorkspace, manifest)
		inspectLegacyOrcaAuthority(id, record.ExecutionHandoff, manifest)
		inspectLegacyOwnershipAuthorities(id, record.Ownership, manifest)
	}
}

func inspectLegacyOwnershipAuthorities(id string, ledger *legacyOwnershipLedger, manifest *legacyResetManifest) {
	if ledger == nil {
		return
	}
	if len(ledger.Attempts) == 0 {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s ownership ledger has no attempts", id))
		return
	}
	lastNumber := 0
	activeFound := ledger.ActiveAttempt == 0
	for index := range ledger.Attempts {
		attempt := &ledger.Attempts[index]
		if attempt.Number <= lastNumber {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s ownership attempt numbers are malformed", id))
		}
		lastNumber = attempt.Number
		if attempt.Number == ledger.ActiveAttempt {
			activeFound = true
		}
		if attempt.Workspace == nil {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s ownership attempt %d has no workspace", id, attempt.Number))
		} else {
			inspectLegacyOrcaAuthority(id, attempt.Workspace, manifest)
		}
		if attempt.Handoff == nil {
			if attempt.Number != ledger.ActiveAttempt || strings.TrimSpace(attempt.ClosedAt) != "" {
				manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s ownership attempt %d has no handoff authority", id, attempt.Number))
			}
			continue
		}
		if attempt.Handoff.Attempt != 0 && attempt.Handoff.Attempt != attempt.Number {
			manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s ownership attempt %d handoff number is inconsistent", id, attempt.Number))
		}
		inspectLegacyOrcaAuthority(id, attempt.Handoff, manifest)
	}
	if !activeFound {
		manifest.Blockers = append(manifest.Blockers, fmt.Sprintf("legacy cycle %s active ownership attempt does not resolve", id))
	}
}

func legacyPendingOrcaBlocker(id, kind string) string {
	if kind == "" {
		return fmt.Sprintf("legacy pending Orca operation is malformed and has no exact task id for %s", id)
	}
	if kind == "task_create" {
		return fmt.Sprintf("legacy pending Orca task_create has no exact task id for %s", id)
	}
	if kind == "dispatch" {
		return fmt.Sprintf("legacy pending Orca dispatch has no exact task id for %s", id)
	}
	return fmt.Sprintf("legacy pending Orca %s has no exact task id for %s", kind, id)
}

func projectLegacyResetPreview(manifest legacyResetManifest, exists bool, targetSchema int) LegacyResetPreview {
	preview := LegacyResetPreview{
		OK: true, SchemaVersion: 1, TargetSchema: targetSchema,
		StateRoot: manifest.StateRoot, LegacyRoot: manifest.LegacyRoot, SchemaRoot: filepath.Join(manifest.StateRoot, issueOpsDirectory),
		ResetRequired: exists, Fingerprint: manifest.Fingerprint,
		ActiveCycles: []string{}, DrainedCycles: []string{}, OrcaTasks: []LegacyOrcaTask{}, RemoteCreateClaims: []LegacyRemoteCreateClaim{}, Blockers: append([]string(nil), manifest.Blockers...),
	}
	for _, row := range manifest.Rows {
		if row.Store != "" {
			continue
		}
		switch row.Bucket {
		case "issueops":
			preview.RowCounts.IssueOps++
		case "session":
			preview.RowCounts.Session++
		}
	}
	preview.RowCount = preview.RowCounts.IssueOps + preview.RowCounts.Session
	for _, entry := range manifest.Entries {
		if entry.Known && entry.Kind == "file" {
			preview.FileCount++
		}
	}
	for id := range manifest.Active {
		preview.ActiveCycles = append(preview.ActiveCycles, id)
	}
	sort.Strings(preview.ActiveCycles)
	for id := range manifest.DrainedCycles {
		preview.DrainedCycles = append(preview.DrainedCycles, id)
	}
	sort.Strings(preview.DrainedCycles)
	for _, claim := range manifest.Claims {
		_, claim.Reconciled = manifest.DrainedClaims[claim.LifecycleID+"\x00"+claim.ClaimID]
		if !claim.Reconciled {
			claim.NextCommand = fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --reconcile-remote --id %s --claim-id %s --expected-fingerprint %s --confirm --json", claim.LifecycleID, claim.ClaimID, manifest.Fingerprint)
		}
		preview.RemoteCreateClaims = append(preview.RemoteCreateClaims, claim)
	}
	sort.Slice(preview.RemoteCreateClaims, func(i, j int) bool {
		left, right := preview.RemoteCreateClaims[i], preview.RemoteCreateClaims[j]
		if left.LifecycleID != right.LifecycleID {
			return left.LifecycleID < right.LifecycleID
		}
		return left.ClaimID < right.ClaimID
	})
	for key, task := range manifest.Orca {
		_, task.Reconciled = manifest.DrainedOrca[key]
		if !task.Reconciled {
			task.NextCommand = legacyResetOrcaReconcileCommand(task, manifest.Fingerprint)
		}
		preview.OrcaTasks = append(preview.OrcaTasks, task)
	}
	sort.Slice(preview.OrcaTasks, func(i, j int) bool {
		left, right := preview.OrcaTasks[i], preview.OrcaTasks[j]
		return left.LifecycleID+"\x00"+left.TaskID < right.LifecycleID+"\x00"+right.TaskID
	})
	for _, id := range preview.ActiveCycles {
		if _, drained := manifest.DrainedCycles[id]; drained {
			continue
		}
		preview.Blockers = append(preview.Blockers, fmt.Sprintf("active legacy cycle %s must be drained", id))
	}
	for _, claim := range preview.RemoteCreateClaims {
		if claim.Reconciled {
			continue
		}
		preview.Blockers = append(preview.Blockers, fmt.Sprintf("legacy remote create claim %s/%s requires exact provider reconciliation", claim.LifecycleID, claim.ClaimID))
	}
	for _, task := range preview.OrcaTasks {
		if task.Reconciled {
			continue
		}
		preview.Blockers = append(preview.Blockers, fmt.Sprintf("live legacy Orca task for %s must be quiescent", task.LifecycleID))
	}
	preview.Blockers = sortedUniqueStrings(preview.Blockers)
	preview.CanConfirm = exists && len(preview.Blockers) == 0
	if preview.CanConfirm {
		preview.NextCommand = fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --confirm --expected-fingerprint %s --json", preview.Fingerprint)
	} else if exists {
		for _, claim := range preview.RemoteCreateClaims {
			if !claim.Reconciled {
				preview.NextCommand = claim.NextCommand
				break
			}
		}
		if preview.NextCommand == "" {
			for _, task := range preview.OrcaTasks {
				if !task.Reconciled {
					preview.NextCommand = task.NextCommand
					break
				}
			}
		}
		if preview.NextCommand == "" {
			for _, id := range preview.ActiveCycles {
				if _, drained := manifest.DrainedCycles[id]; !drained {
					preview.NextCommand = fmt.Sprintf("agent-harness issueops reset-legacy --target-schema 1 --drain-cycle --id %s --expected-fingerprint %s --confirm --json", id, preview.Fingerprint)
					break
				}
			}
		}
		if preview.NextCommand == "" {
			preview.NextCommand = "agent-harness issueops reset-legacy --target-schema 1 --preview --json"
		}
	}
	return preview
}

func fingerprintLegacyResetManifest(manifest legacyResetManifest, targetSchema int) string {
	h := sha256.New()
	writeLegacyFingerprintPart(h, "manifest", strconv.Itoa(legacyResetManifestVersion))
	writeLegacyFingerprintPart(h, "target_schema", strconv.Itoa(targetSchema))
	writeLegacyFingerprintPart(h, "state_root", manifest.StateRoot)
	writeLegacyFingerprintPart(h, "legacy_root", manifest.LegacyRoot)
	writeLegacyFingerprintPart(h, "root_device", strconv.FormatUint(manifest.RootDevice, 10))
	writeLegacyFingerprintPart(h, "root_inode", strconv.FormatUint(manifest.RootInode, 10))
	for _, row := range manifest.Rows {
		writeLegacyFingerprintPart(h, "row", row.Store, row.Bucket, row.ID, strconv.Itoa(row.Size), row.SHA256)
	}
	for _, entry := range manifest.Entries {
		writeLegacyFingerprintPart(h, "entry", entry.Path, entry.Kind, strconv.FormatUint(uint64(entry.Mode), 10), strconv.FormatInt(entry.Size, 10), entry.SHA256,
			strconv.FormatUint(entry.Device, 10), strconv.FormatUint(entry.Inode, 10), strconv.FormatBool(entry.Known))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeLegacyFingerprintPart(w io.Writer, values ...string) {
	for _, value := range values {
		_, _ = io.WriteString(w, strconv.Itoa(len(value)))
		_, _ = io.WriteString(w, ":")
		_, _ = io.WriteString(w, value)
	}
	_, _ = io.WriteString(w, "\n")
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileIdentity(info fs.FileInfo) (uint64, uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, false
	}
	return uint64(stat.Dev), uint64(stat.Ino), true
}

func fileLinkCount(info fs.FileInfo) uint64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 1
	}
	return uint64(stat.Nlink)
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// RequireIssueOpsMutationAllowed enforces the one-time mixed-version
// maintenance barrier. Read-only status and preview paths do not call it.
func RequireIssueOpsMutationAllowed(stateRoot string) error {
	abs, err := filepath.Abs(stateRoot)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	if filepath.Base(abs) != issueOpsDirectory {
		return nil
	}
	stateDir := filepath.Dir(abs)
	legacyRoot := filepath.Join(stateDir, issueOpsLegacyDirectory)
	legacyExists := false
	if _, err := os.Lstat(legacyRoot); err == nil {
		legacyExists = true
	} else if !os.IsNotExist(err) {
		return err
	}
	controlRoot := filepath.Join(stateDir, issueOpsResetDirectory)
	if raw, ok, err := sqlstore.GetExisting(controlRoot, issueOpsResetBucket, issueOpsResetJournalID); err == nil && ok {
		var journal resetLegacyJournal
		if err := json.Unmarshal(raw, &journal); err != nil {
			return &ResetRequiredError{
				Code: "reset_required", TargetSchema: 1, StateRoot: stateDir,
				NextCommand: "agent-harness issueops reset-legacy --target-schema 1 --status --json", PreviewError: "reset journal is unreadable",
			}
		}
		return &ResetRequiredError{
			Code: "reset_required", TargetSchema: 1, StateRoot: stateDir, Fingerprint: journal.Fingerprint,
			NextCommand: "agent-harness issueops reset-legacy --target-schema 1 --status --json",
		}
	} else if err != nil && !isLegacyResetMissing(err) {
		return &ResetRequiredError{
			Code: "reset_required", TargetSchema: 1, StateRoot: stateDir,
			NextCommand: "agent-harness issueops reset-legacy --target-schema 1 --status --json", PreviewError: err.Error(),
		}
	}
	if !legacyExists {
		return nil
	}
	preview, previewErr := PreviewLegacyReset(stateDir, 1)
	resetErr := &ResetRequiredError{
		Code: "reset_required", TargetSchema: 1, StateRoot: stateDir,
		NextCommand: "agent-harness issueops reset-legacy --target-schema 1 --preview --json",
	}
	if previewErr != nil {
		resetErr.PreviewError = previewErr.Error()
		return resetErr
	}
	resetErr.Fingerprint = preview.Fingerprint
	if preview.NextCommand != "" {
		resetErr.NextCommand = preview.NextCommand
	}
	return resetErr
}

func isLegacyResetMissing(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err)
}
