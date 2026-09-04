package nativeactivation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	activationcontract "issueops/internal/contract/nativeactivation"
	activationport "issueops/internal/port/nativeactivation"
)

type Service struct {
	backend  activationport.Backend
	readback activationport.ReadbackVerifier
}

func NewService(backend activationport.Backend, readback activationport.ReadbackVerifier) *Service {
	return &Service{backend: backend, readback: readback}
}

func (service *Service) Begin(ctx context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	if service == nil || service.backend == nil {
		return activationcontract.Result{}, fmt.Errorf("native activation backend is required")
	}
	if err := validateRequest(request); err != nil {
		return activationcontract.Result{}, err
	}
	if request.TransitionID != "" {
		return activationcontract.Result{}, fmt.Errorf("native activation begin must not provide a transition ID")
	}
	result, err := service.backend.Begin(ctx, activationport.BeginRequest{StateRoot: request.StateRoot, IssueOpsRoot: request.IssueOpsRoot, TargetBinary: request.TargetBinary})
	if err != nil {
		return activationcontract.Result{}, err
	}
	if err := validateBackendResult(request, result, false); err != nil {
		return activationcontract.Result{}, err
	}
	return publicResult(result, activationport.Readback{}), nil
}

func (service *Service) Seal(ctx context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	if service == nil || service.backend == nil || service.readback == nil {
		return activationcontract.Result{}, fmt.Errorf("native activation dependencies are required")
	}
	if err := validateRequest(request); err != nil {
		return activationcontract.Result{}, err
	}
	if !validTransitionID(request.TransitionID) {
		return activationcontract.Result{}, fmt.Errorf("native activation seal requires the exact transition ID")
	}
	readback, err := service.readback.Verify(ctx, request.IssueOpsRoot, request.TargetBinary)
	if err != nil {
		return activationcontract.Result{}, err
	}
	readback, err = validateReadback(readback)
	if err != nil {
		return activationcontract.Result{}, err
	}
	result, err := service.backend.Seal(ctx, activationport.SealRequest{
		StateRoot: request.StateRoot, IssueOpsRoot: request.IssueOpsRoot, TargetBinary: request.TargetBinary,
		TransitionID:  request.TransitionID,
		CatalogSHA256: readback.CatalogSHA256,
		Evidence:      append([]activationport.Evidence(nil), readback.Evidence...),
	})
	if err != nil {
		return activationcontract.Result{}, err
	}
	if err := validateBackendResult(request, result, true); err != nil {
		return activationcontract.Result{}, err
	}
	return publicResult(result, readback), nil
}

func (service *Service) Abort(ctx context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	if service == nil || service.backend == nil {
		return activationcontract.Result{}, fmt.Errorf("native activation backend is required")
	}
	if err := validateRequest(request); err != nil {
		return activationcontract.Result{}, err
	}
	if !validTransitionID(request.TransitionID) {
		return activationcontract.Result{}, fmt.Errorf("native activation abort requires the exact transition ID")
	}
	result, err := service.backend.Abort(ctx, activationport.AbortRequest{
		StateRoot: request.StateRoot, IssueOpsRoot: request.IssueOpsRoot, TargetBinary: request.TargetBinary, TransitionID: request.TransitionID,
	})
	if err != nil {
		return activationcontract.Result{}, err
	}
	if err := validateBackendIdentity(request, result); err != nil || !result.Aborted || result.Pending || result.Sealed || !validSHA256(result.BinarySHA256) {
		return activationcontract.Result{}, fmt.Errorf("native activation backend did not abort the pending transition")
	}
	return publicResult(result, activationport.Readback{}), nil
}

func validateRequest(request activationcontract.Request) error {
	if strings.TrimSpace(request.StateRoot) == "" || strings.TrimSpace(request.IssueOpsRoot) == "" || strings.TrimSpace(request.TargetBinary) == "" ||
		request.StateRoot != strings.TrimSpace(request.StateRoot) || request.IssueOpsRoot != strings.TrimSpace(request.IssueOpsRoot) || request.TargetBinary != strings.TrimSpace(request.TargetBinary) {
		return fmt.Errorf("native activation state root, harness root, and target binary are required")
	}
	if request.TransitionID != "" && !validTransitionID(request.TransitionID) {
		return fmt.Errorf("native activation transition ID is invalid")
	}
	return nil
}

func validateBackendResult(request activationcontract.Request, result activationport.Result, sealed bool) error {
	if err := validateBackendIdentity(request, result); err != nil {
		return err
	}
	if sealed {
		if !result.Sealed || result.Pending || !validSHA256(result.BinarySHA256) {
			return fmt.Errorf("native activation backend did not seal the receipt")
		}
		return nil
	}
	if !result.Pending || result.Sealed || !validSHA256(result.BinarySHA256) {
		return fmt.Errorf("native activation backend did not persist a pending activation")
	}
	return nil
}

func validateBackendIdentity(request activationcontract.Request, result activationport.Result) error {
	if result.StateRoot != request.StateRoot || result.IssueOpsRoot != request.IssueOpsRoot || result.TargetBinary != request.TargetBinary ||
		(request.TransitionID != "" && result.TransitionID != request.TransitionID) || !validTransitionID(result.TransitionID) {
		return fmt.Errorf("native activation backend identity mismatch")
	}
	if !validTimestamp(result.UpdatedAt) {
		return fmt.Errorf("native activation backend transition timestamp is invalid")
	}
	return nil
}

func validTransitionID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16 && value == strings.ToLower(value)
}

func validateReadback(readback activationport.Readback) (activationport.Readback, error) {
	if !validSHA256(readback.CatalogSHA256) {
		return activationport.Readback{}, fmt.Errorf("native activation readback digest is invalid")
	}
	expected := map[string]bool{
		"codex\x00mcp": true, "codex\x00hooks": true,
		"claude\x00mcp": true, "claude\x00hooks": true,
		"omo\x00mcp": true, "omo\x00hooks": true,
		"agy\x00mcp": true,
	}
	paths := map[string]bool{}
	for _, evidence := range readback.Evidence {
		key := evidence.Host + "\x00" + evidence.Surface
		if evidence.Host != strings.TrimSpace(evidence.Host) || evidence.Surface != strings.TrimSpace(evidence.Surface) ||
			evidence.Path != strings.TrimSpace(evidence.Path) || !expected[key] || evidence.Path == "" || paths[evidence.Path] ||
			!validSHA256(evidence.SemanticSHA256) || !validSHA256(evidence.SHA256) {
			return activationport.Readback{}, fmt.Errorf("native activation requires one valid readback for each first-party MCP/hook surface")
		}
		delete(expected, key)
		paths[evidence.Path] = true
	}
	if len(expected) != 0 || len(readback.Evidence) != 7 {
		return activationport.Readback{}, fmt.Errorf("native activation requires exactly seven first-party MCP/hook readbacks")
	}
	readback.Evidence = append([]activationport.Evidence(nil), readback.Evidence...)
	sort.Slice(readback.Evidence, func(left, right int) bool {
		return readback.Evidence[left].Host+"\x00"+readback.Evidence[left].Surface < readback.Evidence[right].Host+"\x00"+readback.Evidence[right].Surface
	})
	return readback, nil
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.TrimSpace(value) && value == strings.ToLower(value)
}

func validTimestamp(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.UTC().Format(time.RFC3339Nano) == value
}

func publicResult(result activationport.Result, readback activationport.Readback) activationcontract.Result {
	evidence := make([]activationcontract.Evidence, 0, len(readback.Evidence))
	for _, item := range readback.Evidence {
		evidence = append(evidence, activationcontract.Evidence{
			Host: item.Host, Surface: item.Surface, Path: item.Path, SemanticSHA256: item.SemanticSHA256,
			SHA256: item.SHA256, Mode: item.Mode, Size: item.Size, Device: item.Device, Inode: item.Inode,
		})
	}
	public := activationcontract.Result{
		OK: true, StateRoot: result.StateRoot, IssueOpsRoot: result.IssueOpsRoot, TargetBinary: result.TargetBinary,
		BinarySHA256: result.BinarySHA256, TransitionID: result.TransitionID, Pending: result.Pending, Sealed: result.Sealed, Aborted: result.Aborted, UpdatedAt: result.UpdatedAt,
	}
	if result.Sealed {
		public.Receipt = &activationcontract.Receipt{
			SchemaVersion: activationcontract.SchemaVersion, StateRoot: result.StateRoot, IssueOpsRoot: result.IssueOpsRoot,
			TargetBinary: result.TargetBinary, BinarySHA256: result.BinarySHA256, TransitionID: result.TransitionID, CatalogSHA256: readback.CatalogSHA256,
			Evidence: evidence, SealedAt: result.UpdatedAt,
		}
	}
	return public
}
