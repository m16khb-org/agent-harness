package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

const (
	issueOpsResetActivationPendingID = "activation_pending"
	issueOpsResetActivationReceiptID = "activation_receipt"
)

type LegacyResetActivationBeginRequest struct {
	TargetSchema int    `json:"target_schema"`
	HarnessRoot  string `json:"harness_root"`
	TargetBinary string `json:"target_binary"`
}

type LegacyResetActivationSealRequest struct {
	TargetSchema  int                             `json:"target_schema"`
	HarnessRoot   string                          `json:"harness_root"`
	TargetBinary  string                          `json:"target_binary"`
	CatalogSHA256 string                          `json:"catalog_sha256"`
	Evidence      []port.NativeActivationEvidence `json:"evidence"`
}

type LegacyResetActivationResult struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	TargetSchema  int    `json:"target_schema"`
	StateRoot     string `json:"state_root"`
	HarnessRoot   string `json:"harness_root"`
	TargetBinary  string `json:"target_binary"`
	BinarySHA256  string `json:"binary_sha256"`
	Pending       bool   `json:"pending"`
	Sealed        bool   `json:"sealed"`
	UpdatedAt     string `json:"updated_at"`
}

type legacyResetActivationPending struct {
	SchemaVersion int                       `json:"schema_version"`
	TargetSchema  int                       `json:"target_schema"`
	StateRoot     string                    `json:"state_root"`
	HarnessRoot   string                    `json:"harness_root"`
	TargetBinary  string                    `json:"target_binary"`
	Candidate     resetLegacyBinaryIdentity `json:"candidate"`
	StartedAt     string                    `json:"started_at"`
}

type legacyResetActivationReceipt struct {
	SchemaVersion int                                 `json:"schema_version"`
	TargetSchema  int                                 `json:"target_schema"`
	StateRoot     string                              `json:"state_root"`
	HarnessRoot   string                              `json:"harness_root"`
	TargetBinary  string                              `json:"target_binary"`
	Binary        resetLegacyBinaryIdentity           `json:"binary"`
	CatalogSHA256 string                              `json:"catalog_sha256"`
	SmokeSHA256   string                              `json:"smoke_sha256"`
	Evidence      []legacyResetActivationFileEvidence `json:"evidence"`
	SealedAt      string                              `json:"sealed_at"`
}

type legacyResetActivationFileEvidence struct {
	Host           string `json:"host"`
	Surface        string `json:"surface"`
	Path           string `json:"path"`
	SemanticSHA256 string `json:"semantic_sha256"`
	SHA256         string `json:"sha256"`
	Mode           uint32 `json:"mode"`
	Size           int64  `json:"size"`
	Device         uint64 `json:"device"`
	Inode          uint64 `json:"inode"`
}

type resetLegacyActivationDeps struct {
	Now          func() time.Time
	ActiveBinary func() (resetLegacyBinaryIdentity, error)
	SmokeDigest  func() (string, error)
	AfterStep    func(string) error
}

func BeginLegacyResetActivation(stateDir string, req LegacyResetActivationBeginRequest) (LegacyResetActivationResult, error) {
	return beginLegacyResetActivation(stateDir, req, defaultResetLegacyActivationDeps())
}

func SealLegacyResetActivation(stateDir string, req LegacyResetActivationSealRequest) (LegacyResetActivationResult, error) {
	return sealLegacyResetActivation(stateDir, req, defaultResetLegacyActivationDeps())
}

func beginLegacyResetActivation(stateDir string, req LegacyResetActivationBeginRequest, deps resetLegacyActivationDeps) (LegacyResetActivationResult, error) {
	stateRoot, harnessRoot, targetBinary, err := normalizeLegacyResetActivation(stateDir, req.TargetSchema, req.HarnessRoot, req.TargetBinary)
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	if deps.Now == nil || deps.ActiveBinary == nil {
		return LegacyResetActivationResult{}, fmt.Errorf("issueops reset activation dependencies are incomplete")
	}
	candidate, err := deps.ActiveBinary()
	if err != nil {
		return LegacyResetActivationResult{}, fmt.Errorf("inspect activation candidate: %w", err)
	}
	if err := validateResetLegacyBinaryIdentity(candidate); err != nil {
		return LegacyResetActivationResult{}, fmt.Errorf("inspect activation candidate: %w", err)
	}
	if filepath.Dir(candidate.Executable) != filepath.Dir(targetBinary) ||
		(candidate.Executable != targetBinary && !strings.HasPrefix(filepath.Base(candidate.Executable), ".agent-harness.activate-")) {
		return LegacyResetActivationResult{}, fmt.Errorf("activation candidate must be the canonical target or a same-directory .agent-harness.activate-* file")
	}
	pending := legacyResetActivationPending{
		SchemaVersion: 1, TargetSchema: req.TargetSchema, StateRoot: stateRoot,
		HarnessRoot: harnessRoot, TargetBinary: targetBinary, Candidate: candidate,
		StartedAt: deps.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(pending)
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	control, err := sqlstore.Open(filepath.Join(stateRoot, issueOpsResetDirectory))
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	err = control.WithSpan(context.Background(), func(context.Context) error {
		if _, ok, err := readLegacyResetJournal(control); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("cannot begin native activation while an issueops reset journal is in progress")
		}
		if err := control.Apply(context.Background(), []sqlstore.Mutation{
			{Bucket: issueOpsResetBucket, ID: issueOpsResetActivationPendingID, Data: data},
			{Bucket: issueOpsResetBucket, ID: issueOpsResetActivationReceiptID, Delete: true},
		}); err != nil {
			return err
		}
		return resetLegacyActivationAfterStep(deps, "activation_pending")
	})
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	return legacyResetActivationResult(pending, false, pending.StartedAt), nil
}

func sealLegacyResetActivation(stateDir string, req LegacyResetActivationSealRequest, deps resetLegacyActivationDeps) (LegacyResetActivationResult, error) {
	stateRoot, harnessRoot, targetBinary, err := normalizeLegacyResetActivation(stateDir, req.TargetSchema, req.HarnessRoot, req.TargetBinary)
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	if deps.Now == nil || deps.ActiveBinary == nil || deps.SmokeDigest == nil {
		return LegacyResetActivationResult{}, fmt.Errorf("issueops reset activation dependencies are incomplete")
	}
	if !validSHA256(req.CatalogSHA256) {
		return LegacyResetActivationResult{}, fmt.Errorf("activation catalog SHA-256 is invalid")
	}
	active, err := deps.ActiveBinary()
	if err != nil {
		return LegacyResetActivationResult{}, fmt.Errorf("inspect activated binary: %w", err)
	}
	if err := validateResetLegacyBinaryIdentity(active); err != nil {
		return LegacyResetActivationResult{}, fmt.Errorf("inspect activated binary: %w", err)
	}
	if active.Executable != targetBinary {
		return LegacyResetActivationResult{}, fmt.Errorf("activation seal must run from the canonical target binary")
	}
	evidence, err := captureLegacyResetActivationEvidence(req.Evidence)
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	smokeDigest, err := deps.SmokeDigest()
	if err != nil {
		return LegacyResetActivationResult{}, fmt.Errorf("run reset_required activation smoke: %w", err)
	}
	if !validSHA256(smokeDigest) {
		return LegacyResetActivationResult{}, fmt.Errorf("activation reset_required smoke SHA-256 is invalid")
	}
	control, err := sqlstore.Open(filepath.Join(stateRoot, issueOpsResetDirectory))
	if err != nil {
		return LegacyResetActivationResult{}, err
	}
	var result LegacyResetActivationResult
	err = control.WithSpan(context.Background(), func(context.Context) error {
		pending, ok, err := readLegacyResetActivationPending(control)
		if err != nil {
			return err
		}
		if !ok {
			receipt, receiptOK, readErr := readLegacyResetActivationReceipt(control)
			if readErr != nil {
				return readErr
			}
			if receiptOK && receipt.TargetSchema == req.TargetSchema && receipt.StateRoot == stateRoot && receipt.HarnessRoot == harnessRoot && receipt.TargetBinary == targetBinary && sameResetLegacyBinaryIdentity(receipt.Binary, active) && receipt.CatalogSHA256 == req.CatalogSHA256 && receipt.SmokeSHA256 == smokeDigest && sameLegacyResetActivationEvidence(receipt.Evidence, evidence) {
				if err := requireLegacyResetActivation(control, stateRoot, req.TargetSchema, active); err != nil {
					return err
				}
				result = legacyResetActivationReceiptResult(receipt)
				return nil
			}
			return fmt.Errorf("native activation pending marker is missing")
		}
		if err := validateLegacyResetActivationPending(pending, stateRoot, req.TargetSchema, harnessRoot, targetBinary); err != nil {
			return err
		}
		if !sameResetLegacyBinaryContentIdentity(pending.Candidate, active) {
			return fmt.Errorf("activated binary does not match the sealed candidate")
		}
		if err := resetLegacyActivationAfterStep(deps, "activation_verified"); err != nil {
			return err
		}
		receipt := legacyResetActivationReceipt{
			SchemaVersion: 1, TargetSchema: req.TargetSchema, StateRoot: stateRoot,
			HarnessRoot: harnessRoot, TargetBinary: targetBinary, Binary: active,
			CatalogSHA256: req.CatalogSHA256, SmokeSHA256: smokeDigest, Evidence: evidence,
			SealedAt: deps.Now().UTC().Format(time.RFC3339Nano),
		}
		data, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err := control.Apply(context.Background(), []sqlstore.Mutation{
			{Bucket: issueOpsResetBucket, ID: issueOpsResetActivationReceiptID, Data: data},
			{Bucket: issueOpsResetBucket, ID: issueOpsResetActivationPendingID, Delete: true},
		}); err != nil {
			return err
		}
		result = legacyResetActivationReceiptResult(receipt)
		return resetLegacyActivationAfterStep(deps, "activation_sealed")
	})
	return result, err
}

func requireLegacyResetActivation(control *sqlstore.DB, stateRoot string, targetSchema int, active resetLegacyBinaryIdentity) error {
	if _, ok, err := readLegacyResetActivationPending(control); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("native activation is pending; finish install-native successfully before confirming legacy reset")
	}
	receipt, ok, err := readLegacyResetActivationReceipt(control)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("native activation receipt is missing; run scripts/install-native.sh before confirming legacy reset")
	}
	if err := validateLegacyResetActivationReceipt(receipt, stateRoot, targetSchema); err != nil {
		return err
	}
	if !sameResetLegacyBinaryIdentity(receipt.Binary, active) {
		return fmt.Errorf("native activation binary changed after receipt seal")
	}
	currentEvidence := make([]legacyResetActivationFileEvidence, 0, len(receipt.Evidence))
	for _, sealed := range receipt.Evidence {
		current, err := captureLegacyResetActivationFile(port.NativeActivationEvidence{
			Host: sealed.Host, Surface: sealed.Surface, Path: sealed.Path, SemanticSHA256: sealed.SemanticSHA256,
			SHA256: sealed.SHA256, Mode: sealed.Mode, Size: sealed.Size, Device: sealed.Device, Inode: sealed.Inode,
		})
		if err != nil {
			return fmt.Errorf("native activation evidence %s/%s changed: %w", sealed.Host, sealed.Surface, err)
		}
		currentEvidence = append(currentEvidence, current)
	}
	if !sameLegacyResetActivationEvidence(receipt.Evidence, currentEvidence) {
		return fmt.Errorf("native activation host evidence changed after receipt seal")
	}
	return nil
}

func normalizeLegacyResetActivation(stateDir string, targetSchema int, harnessRoot, targetBinary string) (string, string, string, error) {
	stateRoot, err := normalizeLegacyResetStateDir(stateDir, targetSchema)
	if err != nil {
		return "", "", "", err
	}
	harnessRoot, err = filepath.Abs(strings.TrimSpace(harnessRoot))
	if err != nil {
		return "", "", "", err
	}
	harnessRoot = filepath.Clean(harnessRoot)
	if resolved, err := filepath.EvalSymlinks(harnessRoot); err == nil {
		harnessRoot = filepath.Clean(resolved)
	} else {
		return "", "", "", fmt.Errorf("resolve harness root: %w", err)
	}
	info, err := os.Lstat(harnessRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", fmt.Errorf("harness root must be a physical directory")
	}
	targetBinary, err = filepath.Abs(strings.TrimSpace(targetBinary))
	if err != nil {
		return "", "", "", err
	}
	targetBinary = filepath.Clean(targetBinary)
	if resolvedParent, resolveErr := filepath.EvalSymlinks(filepath.Dir(targetBinary)); resolveErr == nil {
		targetBinary = filepath.Join(filepath.Clean(resolvedParent), filepath.Base(targetBinary))
	} else {
		return "", "", "", fmt.Errorf("resolve activation target directory: %w", resolveErr)
	}
	wantTarget := filepath.Join(harnessRoot, "bin", "agent-harness")
	if targetBinary != wantTarget {
		return "", "", "", fmt.Errorf("activation target must be the canonical harness binary %s", wantTarget)
	}
	if info, err := os.Lstat(targetBinary); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", fmt.Errorf("activation target must not be a symbolic link")
	} else if err != nil && !os.IsNotExist(err) {
		return "", "", "", err
	}
	return stateRoot, harnessRoot, targetBinary, nil
}

func captureLegacyResetActivationEvidence(input []port.NativeActivationEvidence) ([]legacyResetActivationFileEvidence, error) {
	if len(input) != 4 {
		return nil, fmt.Errorf("native activation requires exactly four Codex/Claude MCP/hook evidence files")
	}
	expected := map[string]struct{}{"codex\x00mcp": {}, "codex\x00hooks": {}, "claude\x00mcp": {}, "claude\x00hooks": {}}
	seenPaths := map[string]struct{}{}
	result := make([]legacyResetActivationFileEvidence, 0, len(input))
	for _, item := range input {
		key := strings.TrimSpace(item.Host) + "\x00" + strings.TrimSpace(item.Surface)
		if _, ok := expected[key]; !ok {
			return nil, fmt.Errorf("native activation contains unknown or duplicate host surface %q", key)
		}
		delete(expected, key)
		captured, err := captureLegacyResetActivationFile(item)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenPaths[captured.Path]; duplicate {
			return nil, fmt.Errorf("native activation evidence reuses path %s", captured.Path)
		}
		seenPaths[captured.Path] = struct{}{}
		result = append(result, captured)
	}
	if len(expected) != 0 {
		return nil, fmt.Errorf("native activation is missing a required Codex/Claude MCP/hook surface")
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Host+"\x00"+result[i].Surface < result[j].Host+"\x00"+result[j].Surface
	})
	return result, nil
}

func captureLegacyResetActivationFile(input port.NativeActivationEvidence) (legacyResetActivationFileEvidence, error) {
	host, surface := strings.TrimSpace(input.Host), strings.TrimSpace(input.Surface)
	if !validSHA256(input.SemanticSHA256) {
		return legacyResetActivationFileEvidence{}, fmt.Errorf("native activation semantic SHA-256 is invalid for %s/%s", host, surface)
	}
	path, err := filepath.Abs(strings.TrimSpace(input.Path))
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || fileLinkCount(info) > 1 {
		return legacyResetActivationFileEvidence{}, fmt.Errorf("native activation evidence must be a single-link regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	defer file.Close()
	openedBefore, err := file.Stat()
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	device, inode, ok := fileIdentity(openedBefore)
	pathDevice, pathInode, pathOK := fileIdentity(info)
	if !ok || !pathOK || device == 0 || inode == 0 || device != pathDevice || inode != pathInode || openedBefore.Mode() != info.Mode() || openedBefore.Size() != info.Size() {
		return legacyResetActivationFileEvidence{}, fmt.Errorf("native activation evidence has no physical identity: %s", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	openedAfter, err := file.Stat()
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	afterDevice, afterInode, afterOK := fileIdentity(openedAfter)
	pathAfter, err := os.Lstat(path)
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	finalDevice, finalInode, finalOK := fileIdentity(pathAfter)
	if !afterOK || !finalOK || afterDevice != device || afterInode != inode || finalDevice != device || finalInode != inode ||
		openedAfter.Mode() != openedBefore.Mode() || openedAfter.Size() != openedBefore.Size() || pathAfter.Mode() != openedAfter.Mode() || pathAfter.Size() != openedAfter.Size() ||
		!pathAfter.Mode().IsRegular() || pathAfter.Mode()&os.ModeSymlink != 0 || fileLinkCount(pathAfter) > 1 {
		return legacyResetActivationFileEvidence{}, fmt.Errorf("native activation evidence changed during hashing: %s", path)
	}
	captured := legacyResetActivationFileEvidence{
		Host: host, Surface: surface, Path: path, SemanticSHA256: input.SemanticSHA256,
		SHA256: hex.EncodeToString(hash.Sum(nil)), Mode: uint32(openedAfter.Mode()), Size: openedAfter.Size(), Device: device, Inode: inode,
	}
	if input.SHA256 != captured.SHA256 || input.Mode != captured.Mode || input.Size != captured.Size || input.Device != captured.Device || input.Inode != captured.Inode {
		return legacyResetActivationFileEvidence{}, fmt.Errorf("native activation evidence changed after semantic readback: %s", path)
	}
	return captured, nil
}

func resetLegacyBinaryIdentityFromPath(path, version string) (resetLegacyBinaryIdentity, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	path = filepath.Clean(path)
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = filepath.Clean(resolved)
	} else {
		return resetLegacyBinaryIdentity{}, resolveErr
	}
	info, err := os.Lstat(path)
	if err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o111 == 0 || fileLinkCount(info) > 1 {
		return resetLegacyBinaryIdentity{}, fmt.Errorf("activation binary must be an executable single-link regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return resetLegacyBinaryIdentity{}, err
	}
	device, inode, ok := fileIdentity(openedInfo)
	pathDevice, pathInode, pathOK := fileIdentity(info)
	pathAfter, pathErr := os.Lstat(path)
	var finalDevice, finalInode uint64
	finalOK := false
	if pathErr == nil {
		finalDevice, finalInode, finalOK = fileIdentity(pathAfter)
	}
	if pathErr != nil || !ok || !pathOK || !finalOK || device == 0 || inode == 0 || device != pathDevice || inode != pathInode ||
		device != finalDevice || inode != finalInode || openedInfo.Mode() != info.Mode() || openedInfo.Size() != info.Size() ||
		pathAfter.Mode() != openedInfo.Mode() || pathAfter.Size() != openedInfo.Size() || fileLinkCount(pathAfter) > 1 {
		return resetLegacyBinaryIdentity{}, fmt.Errorf("activation binary identity changed during hashing")
	}
	return resetLegacyBinaryIdentity{
		Version: strings.TrimSpace(version), Executable: path, SHA256: hex.EncodeToString(hash.Sum(nil)),
		Mode: uint32(openedInfo.Mode()), Size: openedInfo.Size(), Device: device, Inode: inode,
	}, nil
}

func legacyResetActivationSmokeDigest() (string, error) {
	root, err := os.MkdirTemp("", "agent-harness-reset-required-smoke-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(root)
	legacyRoot := filepath.Join(root, issueOpsLegacyDirectory)
	if err := os.MkdirAll(legacyRoot, 0o700); err != nil {
		return "", err
	}
	record := []byte(`{"schema_version":9,"id":"io-aaaaaaaaaaaa","phase":"done","cycle_state":"closed"}`)
	if err := os.WriteFile(filepath.Join(legacyRoot, "io-aaaaaaaaaaaa.json"), record, 0o600); err != nil {
		return "", err
	}
	err = RequireIssueOpsMutationAllowed(filepath.Join(root, issueOpsDirectory))
	var resetErr *ResetRequiredError
	if !errors.As(err, &resetErr) || resetErr.Code != "reset_required" || resetErr.TargetSchema != 1 || len(resetErr.Fingerprint) != 64 || strings.TrimSpace(resetErr.NextCommand) == "" {
		return "", fmt.Errorf("reset_required smoke did not return the canonical barrier")
	}
	canonical, err := json.Marshal(struct {
		Code              string `json:"code"`
		TargetSchema      int    `json:"target_schema"`
		FingerprintLength int    `json:"fingerprint_length"`
		HasNextCommand    bool   `json:"has_next_command"`
	}{resetErr.Code, resetErr.TargetSchema, len(resetErr.Fingerprint), true})
	if err != nil {
		return "", err
	}
	return sha256Bytes(canonical), nil
}

func readLegacyResetActivationPending(db *sqlstore.DB) (legacyResetActivationPending, bool, error) {
	data, ok, err := db.Get(issueOpsResetBucket, issueOpsResetActivationPendingID)
	if err != nil || !ok {
		return legacyResetActivationPending{}, ok, err
	}
	var pending legacyResetActivationPending
	if err := json.Unmarshal(data, &pending); err != nil {
		return pending, false, fmt.Errorf("decode native activation pending marker: %w", err)
	}
	return pending, true, nil
}

func readLegacyResetActivationReceipt(db *sqlstore.DB) (legacyResetActivationReceipt, bool, error) {
	data, ok, err := db.Get(issueOpsResetBucket, issueOpsResetActivationReceiptID)
	if err != nil || !ok {
		return legacyResetActivationReceipt{}, ok, err
	}
	var receipt legacyResetActivationReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return receipt, false, fmt.Errorf("decode native activation receipt: %w", err)
	}
	return receipt, true, nil
}

func validateLegacyResetActivationPending(pending legacyResetActivationPending, stateRoot string, targetSchema int, harnessRoot, targetBinary string) error {
	if pending.SchemaVersion != 1 || pending.TargetSchema != targetSchema || pending.StateRoot != stateRoot || pending.HarnessRoot != harnessRoot || pending.TargetBinary != targetBinary || strings.TrimSpace(pending.StartedAt) == "" {
		return fmt.Errorf("native activation pending marker does not match the requested target")
	}
	return validateResetLegacyBinaryIdentity(pending.Candidate)
}

func validateLegacyResetActivationReceipt(receipt legacyResetActivationReceipt, stateRoot string, targetSchema int) error {
	if receipt.SchemaVersion != 1 || receipt.TargetSchema != targetSchema || receipt.StateRoot != stateRoot || receipt.TargetBinary != filepath.Join(receipt.HarnessRoot, "bin", "agent-harness") || strings.TrimSpace(receipt.SealedAt) == "" || !validSHA256(receipt.CatalogSHA256) || !validSHA256(receipt.SmokeSHA256) || len(receipt.Evidence) != 4 {
		return fmt.Errorf("native activation receipt is malformed or targets a different state root")
	}
	if err := validateResetLegacyBinaryIdentity(receipt.Binary); err != nil || receipt.Binary.Executable != receipt.TargetBinary {
		return fmt.Errorf("native activation receipt binary identity is invalid")
	}
	return nil
}

func sameResetLegacyBinaryIdentity(left, right resetLegacyBinaryIdentity) bool {
	return left.Version == right.Version && left.Executable == right.Executable && sameResetLegacyBinaryContentIdentity(left, right)
}

func sameResetLegacyBinaryContentIdentity(left, right resetLegacyBinaryIdentity) bool {
	return left.SHA256 == right.SHA256 && left.Mode == right.Mode && left.Size == right.Size && left.Device == right.Device && left.Inode == right.Inode
}

func sameLegacyResetActivationEvidence(left, right []legacyResetActivationFileEvidence) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func legacyResetActivationResult(pending legacyResetActivationPending, sealed bool, updatedAt string) LegacyResetActivationResult {
	return LegacyResetActivationResult{
		OK: true, SchemaVersion: 1, TargetSchema: pending.TargetSchema, StateRoot: pending.StateRoot,
		HarnessRoot: pending.HarnessRoot, TargetBinary: pending.TargetBinary, BinarySHA256: pending.Candidate.SHA256,
		Pending: !sealed, Sealed: sealed, UpdatedAt: updatedAt,
	}
}

func legacyResetActivationReceiptResult(receipt legacyResetActivationReceipt) LegacyResetActivationResult {
	return LegacyResetActivationResult{
		OK: true, SchemaVersion: 1, TargetSchema: receipt.TargetSchema, StateRoot: receipt.StateRoot,
		HarnessRoot: receipt.HarnessRoot, TargetBinary: receipt.TargetBinary, BinarySHA256: receipt.Binary.SHA256,
		Sealed: true, UpdatedAt: receipt.SealedAt,
	}
}

func resetLegacyActivationAfterStep(deps resetLegacyActivationDeps, step string) error {
	if deps.AfterStep == nil {
		return nil
	}
	return deps.AfterStep(step)
}

func defaultResetLegacyActivationDeps() resetLegacyActivationDeps {
	return resetLegacyActivationDeps{
		Now: time.Now, ActiveBinary: activeResetLegacyBinaryIdentity, SmokeDigest: legacyResetActivationSmokeDigest,
	}
}
