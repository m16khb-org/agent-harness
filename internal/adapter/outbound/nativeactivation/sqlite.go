package nativeactivation

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"agent-harness/internal/port"
	activationport "agent-harness/internal/port/nativeactivation"
)

const (
	storeDirectory = "native-activation"
	storeBucket    = "native_activation_v1"
	pendingID      = "pending"
	receiptID      = "receipt"
	schemaVersion  = 1
)

type StoreOpen func(string) (port.TransactionalRecordStore, error)

type Backend struct {
	open       StoreOpen
	executable func() (string, error)
	now        func() time.Time
	transition func() (string, error)
}

type binaryIdentity struct {
	Executable string `json:"executable"`
	SHA256     string `json:"sha256"`
	Mode       uint32 `json:"mode"`
	Size       int64  `json:"size"`
	Device     uint64 `json:"device"`
	Inode      uint64 `json:"inode"`
}

type physicalFileIdentity struct {
	device uint64
	inode  uint64
	links  uint64
}

type pendingRecord struct {
	SchemaVersion int            `json:"schema_version"`
	StateRoot     string         `json:"state_root"`
	HarnessRoot   string         `json:"harness_root"`
	TargetBinary  string         `json:"target_binary"`
	Candidate     binaryIdentity `json:"candidate"`
	TransitionID  string         `json:"transition_id"`
	StartedAt     string         `json:"started_at"`
}

type receiptRecord struct {
	SchemaVersion int                       `json:"schema_version"`
	StateRoot     string                    `json:"state_root"`
	HarnessRoot   string                    `json:"harness_root"`
	TargetBinary  string                    `json:"target_binary"`
	Binary        binaryIdentity            `json:"binary"`
	CatalogSHA256 string                    `json:"catalog_sha256"`
	Evidence      []activationport.Evidence `json:"evidence"`
	TransitionID  string                    `json:"transition_id"`
	SealedAt      string                    `json:"sealed_at"`
}

var _ activationport.Backend = Backend{}

func NewBackend(open StoreOpen) Backend {
	return Backend{open: open, executable: os.Executable, now: time.Now, transition: newTransitionID}
}

func (backend Backend) Begin(ctx context.Context, request activationport.BeginRequest) (activationport.Result, error) {
	if err := validatePaths(request.StateRoot, request.HarnessRoot, request.TargetBinary); err != nil {
		return activationport.Result{}, err
	}
	candidate, err := backend.activeBinary()
	if err != nil {
		return activationport.Result{}, fmt.Errorf("inspect native activation candidate: %w", err)
	}
	if filepath.Dir(candidate.Executable) != filepath.Dir(request.TargetBinary) ||
		(candidate.Executable != request.TargetBinary && !strings.HasPrefix(filepath.Base(candidate.Executable), ".agent-harness.activate-")) {
		return activationport.Result{}, fmt.Errorf("native activation candidate must be the canonical target or a same-directory staged binary")
	}
	if backend.now == nil {
		return activationport.Result{}, fmt.Errorf("native activation clock is unavailable")
	}
	startedAt := backend.now().UTC().Format(time.RFC3339Nano)
	if backend.transition == nil {
		return activationport.Result{}, fmt.Errorf("native activation transition ID generator is unavailable")
	}
	transitionID, err := backend.transition()
	if err != nil || !validTransitionID(transitionID) {
		return activationport.Result{}, fmt.Errorf("generate native activation transition ID")
	}
	record := pendingRecord{
		SchemaVersion: schemaVersion, StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot,
		TargetBinary: request.TargetBinary, Candidate: candidate, TransitionID: transitionID, StartedAt: startedAt,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return activationport.Result{}, err
	}
	store, err := backendStore(request.StateRoot, backend.open)
	if err != nil {
		return activationport.Result{}, err
	}
	if err := store.WithSpan(ctx, func(spanCtx context.Context) error {
		return store.Apply(spanCtx, []port.RecordMutation{
			{Bucket: storeBucket, ID: pendingID, Data: data},
			{Bucket: storeBucket, ID: receiptID, Delete: true},
		})
	}); err != nil {
		return activationport.Result{}, err
	}
	return activationport.Result{
		StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot, TargetBinary: request.TargetBinary,
		BinarySHA256: candidate.SHA256, TransitionID: transitionID, Pending: true, UpdatedAt: startedAt,
	}, nil
}

func (backend Backend) Seal(ctx context.Context, request activationport.SealRequest) (activationport.Result, error) {
	if err := validatePaths(request.StateRoot, request.HarnessRoot, request.TargetBinary); err != nil {
		return activationport.Result{}, err
	}
	if !validTransitionID(request.TransitionID) {
		return activationport.Result{}, fmt.Errorf("native activation transition ID is invalid")
	}
	active, err := backend.activeBinary()
	if err != nil {
		return activationport.Result{}, fmt.Errorf("inspect native activation binary: %w", err)
	}
	if active.Executable != request.TargetBinary {
		return activationport.Result{}, fmt.Errorf("native activation seal must run from the canonical target binary")
	}
	store, err := backendStore(request.StateRoot, backend.open)
	if err != nil {
		return activationport.Result{}, err
	}
	var result activationport.Result
	if err := store.WithSpan(ctx, func(spanCtx context.Context) error {
		pendingData, ok, err := store.Get(storeBucket, pendingID)
		if err != nil {
			return err
		}
		if !ok {
			receiptData, receiptOK, readErr := store.Get(storeBucket, receiptID)
			if readErr != nil {
				return readErr
			}
			if !receiptOK {
				return fmt.Errorf("native activation pending record is missing")
			}
			receipt, decodeErr := decodeExact[receiptRecord](receiptData)
			if decodeErr != nil || !sameReceipt(receipt, request, active) {
				return fmt.Errorf("invalid native activation state")
			}
			result = sealedResult(receipt)
			return nil
		}
		pending, err := decodeExact[pendingRecord](pendingData)
		if err != nil || !samePending(pending, request, active) {
			return fmt.Errorf("invalid native activation state")
		}
		if backend.now == nil {
			return fmt.Errorf("native activation clock is unavailable")
		}
		sealedAt := backend.now().UTC().Format(time.RFC3339Nano)
		receipt := receiptRecord{
			SchemaVersion: schemaVersion, StateRoot: request.StateRoot, HarnessRoot: request.HarnessRoot,
			TargetBinary: request.TargetBinary, Binary: active, CatalogSHA256: request.CatalogSHA256,
			Evidence: append([]activationport.Evidence(nil), request.Evidence...), TransitionID: request.TransitionID, SealedAt: sealedAt,
		}
		data, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err := store.Apply(spanCtx, []port.RecordMutation{
			{Bucket: storeBucket, ID: receiptID, Data: data},
			{Bucket: storeBucket, ID: pendingID, Delete: true},
		}); err != nil {
			return err
		}
		result = sealedResult(receipt)
		return nil
	}); err != nil {
		return activationport.Result{}, err
	}
	return result, nil
}

func (backend Backend) Abort(ctx context.Context, request activationport.AbortRequest) (activationport.Result, error) {
	if err := validatePaths(request.StateRoot, request.HarnessRoot, request.TargetBinary); err != nil {
		return activationport.Result{}, err
	}
	if !validTransitionID(request.TransitionID) {
		return activationport.Result{}, fmt.Errorf("native activation transition ID is invalid")
	}
	active, err := backend.activeBinary()
	if err != nil {
		return activationport.Result{}, fmt.Errorf("inspect native activation abort candidate: %w", err)
	}
	store, err := backendStore(request.StateRoot, backend.open)
	if err != nil {
		return activationport.Result{}, err
	}
	var result activationport.Result
	if err := store.WithSpan(ctx, func(spanCtx context.Context) error {
		data, ok, err := store.Get(storeBucket, pendingID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("native activation pending record is missing")
		}
		pending, err := decodeExact[pendingRecord](data)
		if err != nil || !sameAbortPending(pending, request, active) {
			return fmt.Errorf("invalid native activation abort state")
		}
		if err := store.Apply(spanCtx, []port.RecordMutation{{Bucket: storeBucket, ID: pendingID, Delete: true}}); err != nil {
			return err
		}
		result = activationport.Result{
			StateRoot: pending.StateRoot, HarnessRoot: pending.HarnessRoot, TargetBinary: pending.TargetBinary,
			BinarySHA256: pending.Candidate.SHA256, TransitionID: pending.TransitionID, Aborted: true, UpdatedAt: pending.StartedAt,
		}
		return nil
	}); err != nil {
		return activationport.Result{}, err
	}
	return result, nil
}

func backendStore(stateRoot string, open StoreOpen) (port.TransactionalRecordStore, error) {
	if open == nil {
		return nil, fmt.Errorf("native activation store factory is unavailable")
	}
	store, err := open(filepath.Join(stateRoot, storeDirectory))
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("native activation store factory returned nil")
	}
	return store, nil
}

func validatePaths(stateRoot, harnessRoot, targetBinary string) error {
	for _, path := range []string{stateRoot, harnessRoot, targetBinary} {
		if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("native activation paths must be absolute and canonical")
		}
	}
	info, err := os.Lstat(harnessRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("native activation harness root must be a physical directory")
	}
	if targetBinary != filepath.Join(harnessRoot, "bin", "agent-harness") {
		return fmt.Errorf("native activation target must be the canonical harness binary")
	}
	if info, err := os.Lstat(targetBinary); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("native activation target must not be a symbolic link")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (backend Backend) activeBinary() (binaryIdentity, error) {
	if backend.executable == nil {
		return binaryIdentity{}, fmt.Errorf("native activation executable inspector is unavailable")
	}
	path, err := backend.executable()
	if err != nil {
		return binaryIdentity{}, err
	}
	return binaryIdentityFromPath(path)
}

func binaryIdentityFromPath(path string) (binaryIdentity, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return binaryIdentity{}, err
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return binaryIdentity{}, err
	}
	physical, ok := physicalIdentity(info)
	if !ok || !validBinaryFile(info, physical) {
		return binaryIdentity{}, fmt.Errorf("activation binary must be an executable single-link regular file")
	}
	hash, err := hashStableBinary(path, info, physical)
	if err != nil {
		return binaryIdentity{}, err
	}
	return binaryIdentity{
		Executable: path, SHA256: hash, Mode: uint32(info.Mode()), Size: info.Size(),
		Device: physical.device, Inode: physical.inode,
	}, nil
}

func hashStableBinary(path string, expected os.FileInfo, physical physicalFileIdentity) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	openedBefore, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !samePhysicalFile(openedBefore, expected, physical) {
		return "", fmt.Errorf("activation binary has no stable physical identity")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	openedAfter, err := file.Stat()
	if err != nil {
		return "", err
	}
	pathAfter, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !samePhysicalFile(openedAfter, expected, physical) || !samePhysicalFile(pathAfter, expected, physical) {
		return "", fmt.Errorf("activation binary identity changed during hashing")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validBinaryFile(info os.FileInfo, physical physicalFileIdentity) bool {
	return physical.device != 0 && physical.inode != 0 && physical.links == 1 &&
		info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 && info.Mode().Perm()&0o111 != 0
}

func samePhysicalFile(actual, expected os.FileInfo, physical physicalFileIdentity) bool {
	actualPhysical, ok := physicalIdentity(actual)
	return ok && actualPhysical == physical && actual.Mode() == expected.Mode() && actual.Size() == expected.Size()
}

func physicalIdentity(info os.FileInfo) (physicalFileIdentity, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return physicalFileIdentity{}, false
	}
	return physicalFileIdentity{device: uint64(stat.Dev), inode: uint64(stat.Ino), links: uint64(stat.Nlink)}, true
}

func decodeExact[T any](data []byte) (T, error) {
	var record T
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return record, fmt.Errorf("trailing native activation data")
	}
	return record, nil
}

func samePending(record pendingRecord, request activationport.SealRequest, active binaryIdentity) bool {
	return record.SchemaVersion == schemaVersion && record.StateRoot == request.StateRoot &&
		record.HarnessRoot == request.HarnessRoot && record.TargetBinary == request.TargetBinary &&
		record.TransitionID == request.TransitionID && sameBinaryContent(record.Candidate, active) && record.StartedAt != ""
}

func sameAbortPending(record pendingRecord, request activationport.AbortRequest, active binaryIdentity) bool {
	return record.SchemaVersion == schemaVersion && record.StateRoot == request.StateRoot &&
		record.HarnessRoot == request.HarnessRoot && record.TargetBinary == request.TargetBinary &&
		record.TransitionID == request.TransitionID && sameBinaryContent(record.Candidate, active) && record.StartedAt != ""
}

func sameReceipt(record receiptRecord, request activationport.SealRequest, active binaryIdentity) bool {
	return record.SchemaVersion == schemaVersion && record.StateRoot == request.StateRoot &&
		record.HarnessRoot == request.HarnessRoot && record.TargetBinary == request.TargetBinary &&
		record.Binary == active && record.CatalogSHA256 == request.CatalogSHA256 &&
		record.TransitionID == request.TransitionID && slices.Equal(record.Evidence, request.Evidence) && record.SealedAt != ""
}

func sameBinaryContent(left, right binaryIdentity) bool {
	return left.SHA256 == right.SHA256 && left.Mode == right.Mode && left.Size == right.Size &&
		left.Device == right.Device && left.Inode == right.Inode
}

func sealedResult(record receiptRecord) activationport.Result {
	return activationport.Result{
		StateRoot: record.StateRoot, HarnessRoot: record.HarnessRoot, TargetBinary: record.TargetBinary,
		BinarySHA256: record.Binary.SHA256, TransitionID: record.TransitionID, Sealed: true, UpdatedAt: record.SealedAt,
	}
}

func newTransitionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validTransitionID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16 && value == strings.ToLower(value)
}
