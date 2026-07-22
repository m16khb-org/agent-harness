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
	issueOpsResetV1ActivationPendingID = "activation_pending"
	issueOpsResetV1ActivationReceiptID = "activation_receipt"
)

type LegacyResetActivationBeginRequestV1 struct {
	TargetSchema int    `json:"target_schema"`
	HarnessRoot  string `json:"harness_root"`
	TargetBinary string `json:"target_binary"`
}

type LegacyResetActivationSealRequestV1 struct {
	TargetSchema  int                             `json:"target_schema"`
	HarnessRoot   string                          `json:"harness_root"`
	TargetBinary  string                          `json:"target_binary"`
	CatalogSHA256 string                          `json:"catalog_sha256"`
	Evidence      []port.NativeActivationEvidence `json:"evidence"`
}

type LegacyResetActivationResultV1 struct {
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

type legacyResetActivationPendingV1 struct {
	SchemaVersion int                         `json:"schema_version"`
	TargetSchema  int                         `json:"target_schema"`
	StateRoot     string                      `json:"state_root"`
	HarnessRoot   string                      `json:"harness_root"`
	TargetBinary  string                      `json:"target_binary"`
	Candidate     resetLegacyV1BinaryIdentity `json:"candidate"`
	StartedAt     string                      `json:"started_at"`
}

type legacyResetActivationReceiptV1 struct {
	SchemaVersion int                                 `json:"schema_version"`
	TargetSchema  int                                 `json:"target_schema"`
	StateRoot     string                              `json:"state_root"`
	HarnessRoot   string                              `json:"harness_root"`
	TargetBinary  string                              `json:"target_binary"`
	Binary        resetLegacyV1BinaryIdentity         `json:"binary"`
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

type resetLegacyActivationDepsV1 struct {
	Now          func() time.Time
	ActiveBinary func() (resetLegacyV1BinaryIdentity, error)
	SmokeDigest  func() (string, error)
	AfterStep    func(string) error
}

func BeginLegacyResetActivationV1(stateDir string, req LegacyResetActivationBeginRequestV1) (LegacyResetActivationResultV1, error) {
	return beginLegacyResetActivationV1(stateDir, req, defaultResetLegacyActivationDepsV1())
}

func SealLegacyResetActivationV1(stateDir string, req LegacyResetActivationSealRequestV1) (LegacyResetActivationResultV1, error) {
	return sealLegacyResetActivationV1(stateDir, req, defaultResetLegacyActivationDepsV1())
}

func beginLegacyResetActivationV1(stateDir string, req LegacyResetActivationBeginRequestV1, deps resetLegacyActivationDepsV1) (LegacyResetActivationResultV1, error) {
	stateRoot, harnessRoot, targetBinary, err := normalizeLegacyResetActivationV1(stateDir, req.TargetSchema, req.HarnessRoot, req.TargetBinary)
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	if deps.Now == nil || deps.ActiveBinary == nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("issueops reset activation dependencies are incomplete")
	}
	candidate, err := deps.ActiveBinary()
	if err != nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("inspect activation candidate: %w", err)
	}
	if err := validateResetLegacyBinaryIdentityV1(candidate); err != nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("inspect activation candidate: %w", err)
	}
	if filepath.Dir(candidate.Executable) != filepath.Dir(targetBinary) ||
		(candidate.Executable != targetBinary && !strings.HasPrefix(filepath.Base(candidate.Executable), ".agent-harness.activate-")) {
		return LegacyResetActivationResultV1{}, fmt.Errorf("activation candidate must be the canonical target or a same-directory .agent-harness.activate-* file")
	}
	pending := legacyResetActivationPendingV1{
		SchemaVersion: 1, TargetSchema: req.TargetSchema, StateRoot: stateRoot,
		HarnessRoot: harnessRoot, TargetBinary: targetBinary, Candidate: candidate,
		StartedAt: deps.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(pending)
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	control, err := sqlstore.Open(filepath.Join(stateRoot, issueOpsResetV1Directory))
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	err = control.WithSpan(context.Background(), func(context.Context) error {
		if _, ok, err := readLegacyResetJournalV1(control); err != nil {
			return err
		} else if ok {
			return fmt.Errorf("cannot begin native activation while an issueops reset journal is in progress")
		}
		if err := control.Apply(context.Background(), []sqlstore.Mutation{
			{Bucket: issueOpsResetV1Bucket, ID: issueOpsResetV1ActivationPendingID, Data: data},
			{Bucket: issueOpsResetV1Bucket, ID: issueOpsResetV1ActivationReceiptID, Delete: true},
		}); err != nil {
			return err
		}
		return resetLegacyActivationAfterStepV1(deps, "activation_pending")
	})
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	return legacyResetActivationResultV1(pending, false, pending.StartedAt), nil
}

func sealLegacyResetActivationV1(stateDir string, req LegacyResetActivationSealRequestV1, deps resetLegacyActivationDepsV1) (LegacyResetActivationResultV1, error) {
	stateRoot, harnessRoot, targetBinary, err := normalizeLegacyResetActivationV1(stateDir, req.TargetSchema, req.HarnessRoot, req.TargetBinary)
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	if deps.Now == nil || deps.ActiveBinary == nil || deps.SmokeDigest == nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("issueops reset activation dependencies are incomplete")
	}
	if !validSHA256V1(req.CatalogSHA256) {
		return LegacyResetActivationResultV1{}, fmt.Errorf("activation catalog SHA-256 is invalid")
	}
	active, err := deps.ActiveBinary()
	if err != nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("inspect activated binary: %w", err)
	}
	if err := validateResetLegacyBinaryIdentityV1(active); err != nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("inspect activated binary: %w", err)
	}
	if active.Executable != targetBinary {
		return LegacyResetActivationResultV1{}, fmt.Errorf("activation seal must run from the canonical target binary")
	}
	evidence, err := captureLegacyResetActivationEvidenceV1(req.Evidence)
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	smokeDigest, err := deps.SmokeDigest()
	if err != nil {
		return LegacyResetActivationResultV1{}, fmt.Errorf("run reset_required activation smoke: %w", err)
	}
	if !validSHA256V1(smokeDigest) {
		return LegacyResetActivationResultV1{}, fmt.Errorf("activation reset_required smoke SHA-256 is invalid")
	}
	control, err := sqlstore.Open(filepath.Join(stateRoot, issueOpsResetV1Directory))
	if err != nil {
		return LegacyResetActivationResultV1{}, err
	}
	var result LegacyResetActivationResultV1
	err = control.WithSpan(context.Background(), func(context.Context) error {
		pending, ok, err := readLegacyResetActivationPendingV1(control)
		if err != nil {
			return err
		}
		if !ok {
			receipt, receiptOK, readErr := readLegacyResetActivationReceiptV1(control)
			if readErr != nil {
				return readErr
			}
			if receiptOK && receipt.TargetSchema == req.TargetSchema && receipt.StateRoot == stateRoot && receipt.HarnessRoot == harnessRoot && receipt.TargetBinary == targetBinary && sameResetLegacyBinaryIdentityV1(receipt.Binary, active) && receipt.CatalogSHA256 == req.CatalogSHA256 && receipt.SmokeSHA256 == smokeDigest && sameLegacyResetActivationEvidenceV1(receipt.Evidence, evidence) {
				if err := requireLegacyResetActivationV1(control, stateRoot, req.TargetSchema, active); err != nil {
					return err
				}
				result = legacyResetActivationReceiptResultV1(receipt)
				return nil
			}
			return fmt.Errorf("native activation pending marker is missing")
		}
		if err := validateLegacyResetActivationPendingV1(pending, stateRoot, req.TargetSchema, harnessRoot, targetBinary); err != nil {
			return err
		}
		if !sameResetLegacyBinaryContentIdentityV1(pending.Candidate, active) {
			return fmt.Errorf("activated binary does not match the sealed candidate")
		}
		if err := resetLegacyActivationAfterStepV1(deps, "activation_verified"); err != nil {
			return err
		}
		receipt := legacyResetActivationReceiptV1{
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
			{Bucket: issueOpsResetV1Bucket, ID: issueOpsResetV1ActivationReceiptID, Data: data},
			{Bucket: issueOpsResetV1Bucket, ID: issueOpsResetV1ActivationPendingID, Delete: true},
		}); err != nil {
			return err
		}
		result = legacyResetActivationReceiptResultV1(receipt)
		return resetLegacyActivationAfterStepV1(deps, "activation_sealed")
	})
	return result, err
}

func requireLegacyResetActivationV1(control *sqlstore.DB, stateRoot string, targetSchema int, active resetLegacyV1BinaryIdentity) error {
	if _, ok, err := readLegacyResetActivationPendingV1(control); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("native activation is pending; finish install-native successfully before confirming legacy reset")
	}
	receipt, ok, err := readLegacyResetActivationReceiptV1(control)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("native activation receipt is missing; run scripts/install-native.sh before confirming legacy reset")
	}
	if err := validateLegacyResetActivationReceiptV1(receipt, stateRoot, targetSchema); err != nil {
		return err
	}
	if !sameResetLegacyBinaryIdentityV1(receipt.Binary, active) {
		return fmt.Errorf("native activation binary changed after receipt seal")
	}
	currentEvidence := make([]legacyResetActivationFileEvidence, 0, len(receipt.Evidence))
	for _, sealed := range receipt.Evidence {
		current, err := captureLegacyResetActivationFileV1(port.NativeActivationEvidence{
			Host: sealed.Host, Surface: sealed.Surface, Path: sealed.Path, SemanticSHA256: sealed.SemanticSHA256,
			SHA256: sealed.SHA256, Mode: sealed.Mode, Size: sealed.Size, Device: sealed.Device, Inode: sealed.Inode,
		})
		if err != nil {
			return fmt.Errorf("native activation evidence %s/%s changed: %w", sealed.Host, sealed.Surface, err)
		}
		currentEvidence = append(currentEvidence, current)
	}
	if !sameLegacyResetActivationEvidenceV1(receipt.Evidence, currentEvidence) {
		return fmt.Errorf("native activation host evidence changed after receipt seal")
	}
	return nil
}

func normalizeLegacyResetActivationV1(stateDir string, targetSchema int, harnessRoot, targetBinary string) (string, string, string, error) {
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

func captureLegacyResetActivationEvidenceV1(input []port.NativeActivationEvidence) ([]legacyResetActivationFileEvidence, error) {
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
		captured, err := captureLegacyResetActivationFileV1(item)
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

func captureLegacyResetActivationFileV1(input port.NativeActivationEvidence) (legacyResetActivationFileEvidence, error) {
	host, surface := strings.TrimSpace(input.Host), strings.TrimSpace(input.Surface)
	if !validSHA256V1(input.SemanticSHA256) {
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
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || fileLinkCountV1(info) > 1 {
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
	device, inode, ok := fileIdentityV1(openedBefore)
	pathDevice, pathInode, pathOK := fileIdentityV1(info)
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
	afterDevice, afterInode, afterOK := fileIdentityV1(openedAfter)
	pathAfter, err := os.Lstat(path)
	if err != nil {
		return legacyResetActivationFileEvidence{}, err
	}
	finalDevice, finalInode, finalOK := fileIdentityV1(pathAfter)
	if !afterOK || !finalOK || afterDevice != device || afterInode != inode || finalDevice != device || finalInode != inode ||
		openedAfter.Mode() != openedBefore.Mode() || openedAfter.Size() != openedBefore.Size() || pathAfter.Mode() != openedAfter.Mode() || pathAfter.Size() != openedAfter.Size() ||
		!pathAfter.Mode().IsRegular() || pathAfter.Mode()&os.ModeSymlink != 0 || fileLinkCountV1(pathAfter) > 1 {
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

func resetLegacyBinaryIdentityFromPathV1(path, version string) (resetLegacyV1BinaryIdentity, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	path = filepath.Clean(path)
	if resolved, resolveErr := filepath.EvalSymlinks(path); resolveErr == nil {
		path = filepath.Clean(resolved)
	} else {
		return resetLegacyV1BinaryIdentity{}, resolveErr
	}
	info, err := os.Lstat(path)
	if err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o111 == 0 || fileLinkCountV1(info) > 1 {
		return resetLegacyV1BinaryIdentity{}, fmt.Errorf("activation binary must be an executable single-link regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return resetLegacyV1BinaryIdentity{}, err
	}
	device, inode, ok := fileIdentityV1(openedInfo)
	pathDevice, pathInode, pathOK := fileIdentityV1(info)
	pathAfter, pathErr := os.Lstat(path)
	var finalDevice, finalInode uint64
	finalOK := false
	if pathErr == nil {
		finalDevice, finalInode, finalOK = fileIdentityV1(pathAfter)
	}
	if pathErr != nil || !ok || !pathOK || !finalOK || device == 0 || inode == 0 || device != pathDevice || inode != pathInode ||
		device != finalDevice || inode != finalInode || openedInfo.Mode() != info.Mode() || openedInfo.Size() != info.Size() ||
		pathAfter.Mode() != openedInfo.Mode() || pathAfter.Size() != openedInfo.Size() || fileLinkCountV1(pathAfter) > 1 {
		return resetLegacyV1BinaryIdentity{}, fmt.Errorf("activation binary identity changed during hashing")
	}
	return resetLegacyV1BinaryIdentity{
		Version: strings.TrimSpace(version), Executable: path, SHA256: hex.EncodeToString(hash.Sum(nil)),
		Mode: uint32(openedInfo.Mode()), Size: openedInfo.Size(), Device: device, Inode: inode,
	}, nil
}

func legacyResetActivationSmokeDigestV1() (string, error) {
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
	err = RequireIssueOpsV1MutationAllowed(filepath.Join(root, issueOpsV1Directory))
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
	return sha256BytesV1(canonical), nil
}

func readLegacyResetActivationPendingV1(db *sqlstore.DB) (legacyResetActivationPendingV1, bool, error) {
	data, ok, err := db.Get(issueOpsResetV1Bucket, issueOpsResetV1ActivationPendingID)
	if err != nil || !ok {
		return legacyResetActivationPendingV1{}, ok, err
	}
	var pending legacyResetActivationPendingV1
	if err := json.Unmarshal(data, &pending); err != nil {
		return pending, false, fmt.Errorf("decode native activation pending marker: %w", err)
	}
	return pending, true, nil
}

func readLegacyResetActivationReceiptV1(db *sqlstore.DB) (legacyResetActivationReceiptV1, bool, error) {
	data, ok, err := db.Get(issueOpsResetV1Bucket, issueOpsResetV1ActivationReceiptID)
	if err != nil || !ok {
		return legacyResetActivationReceiptV1{}, ok, err
	}
	var receipt legacyResetActivationReceiptV1
	if err := json.Unmarshal(data, &receipt); err != nil {
		return receipt, false, fmt.Errorf("decode native activation receipt: %w", err)
	}
	return receipt, true, nil
}

func validateLegacyResetActivationPendingV1(pending legacyResetActivationPendingV1, stateRoot string, targetSchema int, harnessRoot, targetBinary string) error {
	if pending.SchemaVersion != 1 || pending.TargetSchema != targetSchema || pending.StateRoot != stateRoot || pending.HarnessRoot != harnessRoot || pending.TargetBinary != targetBinary || strings.TrimSpace(pending.StartedAt) == "" {
		return fmt.Errorf("native activation pending marker does not match the requested target")
	}
	return validateResetLegacyBinaryIdentityV1(pending.Candidate)
}

func validateLegacyResetActivationReceiptV1(receipt legacyResetActivationReceiptV1, stateRoot string, targetSchema int) error {
	if receipt.SchemaVersion != 1 || receipt.TargetSchema != targetSchema || receipt.StateRoot != stateRoot || receipt.TargetBinary != filepath.Join(receipt.HarnessRoot, "bin", "agent-harness") || strings.TrimSpace(receipt.SealedAt) == "" || !validSHA256V1(receipt.CatalogSHA256) || !validSHA256V1(receipt.SmokeSHA256) || len(receipt.Evidence) != 4 {
		return fmt.Errorf("native activation receipt is malformed or targets a different state root")
	}
	if err := validateResetLegacyBinaryIdentityV1(receipt.Binary); err != nil || receipt.Binary.Executable != receipt.TargetBinary {
		return fmt.Errorf("native activation receipt binary identity is invalid")
	}
	return nil
}

func sameResetLegacyBinaryIdentityV1(left, right resetLegacyV1BinaryIdentity) bool {
	return left.Version == right.Version && left.Executable == right.Executable && sameResetLegacyBinaryContentIdentityV1(left, right)
}

func sameResetLegacyBinaryContentIdentityV1(left, right resetLegacyV1BinaryIdentity) bool {
	return left.SHA256 == right.SHA256 && left.Mode == right.Mode && left.Size == right.Size && left.Device == right.Device && left.Inode == right.Inode
}

func sameLegacyResetActivationEvidenceV1(left, right []legacyResetActivationFileEvidence) bool {
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

func validSHA256V1(value string) bool {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
}

func legacyResetActivationResultV1(pending legacyResetActivationPendingV1, sealed bool, updatedAt string) LegacyResetActivationResultV1 {
	return LegacyResetActivationResultV1{
		OK: true, SchemaVersion: 1, TargetSchema: pending.TargetSchema, StateRoot: pending.StateRoot,
		HarnessRoot: pending.HarnessRoot, TargetBinary: pending.TargetBinary, BinarySHA256: pending.Candidate.SHA256,
		Pending: !sealed, Sealed: sealed, UpdatedAt: updatedAt,
	}
}

func legacyResetActivationReceiptResultV1(receipt legacyResetActivationReceiptV1) LegacyResetActivationResultV1 {
	return LegacyResetActivationResultV1{
		OK: true, SchemaVersion: 1, TargetSchema: receipt.TargetSchema, StateRoot: receipt.StateRoot,
		HarnessRoot: receipt.HarnessRoot, TargetBinary: receipt.TargetBinary, BinarySHA256: receipt.Binary.SHA256,
		Sealed: true, UpdatedAt: receipt.SealedAt,
	}
}

func resetLegacyActivationAfterStepV1(deps resetLegacyActivationDepsV1, step string) error {
	if deps.AfterStep == nil {
		return nil
	}
	return deps.AfterStep(step)
}

func defaultResetLegacyActivationDepsV1() resetLegacyActivationDepsV1 {
	return resetLegacyActivationDepsV1{
		Now: time.Now, ActiveBinary: activeResetLegacyBinaryIdentityV1, SmokeDigest: legacyResetActivationSmokeDigestV1,
	}
}
