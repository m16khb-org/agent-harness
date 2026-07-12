package issueops

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
	"agent-harness/internal/core/preflight"
)

type IssueOpsHandoffClaimRequest struct {
	ID             string `json:"id"`
	Attempt        int    `json:"attempt"`
	OwnershipEpoch string `json:"ownership_epoch"`
	ContextSHA256  string `json:"context_sha256"`
	Host           string `json:"host"`
	SessionID      string `json:"session_id"`
	AgentID        string `json:"agent_id,omitempty"`
	CWD            string `json:"cwd"`
	OrcaWorktreeID string `json:"orca_worktree_id"`
}

type IssueOpsHeartbeatRequest struct {
	ID             string `json:"id"`
	Attempt        int    `json:"attempt,omitempty"`
	OwnershipEpoch string `json:"ownership_epoch,omitempty"`
	ContextSHA256  string `json:"context_sha256,omitempty"`
	Host           string `json:"host,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
}

type IssueOpsHandoffFinishRequest struct {
	ID               string   `json:"id"`
	Attempt          int      `json:"attempt"`
	OwnershipEpoch   string   `json:"ownership_epoch"`
	ContextSHA256    string   `json:"context_sha256"`
	Host             string   `json:"host"`
	SessionID        string   `json:"session_id"`
	AgentID          string   `json:"agent_id,omitempty"`
	Outcome          string   `json:"outcome"`
	FinalHead        string   `json:"final_head,omitempty"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	TuringReportPath string   `json:"turing_report_path,omitempty"`
	Verification     []string `json:"verification,omitempty"`
	CleanupReceipts  []string `json:"cleanup_receipts,omitempty"`
	EvidenceDigest   string   `json:"evidence_digest,omitempty"`
	TaskID           string   `json:"task_id,omitempty"`
	DispatchID       string   `json:"dispatch_id,omitempty"`
}

type IssueOpsHandoffAcceptRequest struct {
	ID             string `json:"id"`
	Attempt        int    `json:"attempt"`
	OwnershipEpoch string `json:"ownership_epoch"`
	ContextSHA256  string `json:"context_sha256"`
	FinalHead      string `json:"final_head"`
	Host           string `json:"host"`
	SessionID      string `json:"session_id"`
	AgentID        string `json:"agent_id,omitempty"`
	SourceCWD      string `json:"source_cwd"`
}

type issueOpsHandoffLifecycleHooks struct {
	BeforeLockedRevalidation func()
}

func ClaimIssueOpsHandoff(stateRoot string, req IssueOpsHandoffClaimRequest) (IssueOpsRecord, error) {
	return claimIssueOpsHandoff(stateRoot, req, issueOpsHandoffLifecycleHooks{})
}

func claimIssueOpsHandoff(stateRoot string, req IssueOpsHandoffClaimRequest, hooks issueOpsHandoffLifecycleHooks) (IssueOpsRecord, error) {
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if err := validateHandoffClaimIdentity(validated, req); err != nil {
		return IssueOpsRecord{}, err
	}
	if validated.ExecutionHandoff.State != handoff.StateClaimed {
		if err := validateHandoffClaim(validated, req); err != nil {
			return IssueOpsRecord{}, err
		}
	}
	runHandoffLifecycleHook(hooks.BeforeLockedRevalidation)
	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, validated) {
			return fmt.Errorf("handoff changed after claim validation; retry with the current fence")
		}
		alreadyClaimed := record.ExecutionHandoff.State == handoff.StateClaimed
		if !alreadyClaimed {
			if err := validateHandoffClaim(record, req); err != nil {
				return err
			}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record, err = handoff.Claim(record, handoff.ClaimRequest{
			Fence:      handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256},
			Worker:     model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID},
			WorkerRoot: pathutil.CleanAbsPath(req.CWD), Now: now,
		})
		if err != nil {
			return err
		}
		if !alreadyClaimed && record.Phase != IssueOpsPhaseImplement {
			if err := validateIssueOpsPhaseTransition(stateRoot, record, IssueOpsPhaseImplement); err != nil {
				return err
			}
			record = applyIssueOpsPhaseTransition(record, IssueOpsPhaseImplement)
		}
		record.LastHeartbeatAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func validateHandoffClaimIdentity(record IssueOpsRecord, req IssueOpsHandoffClaimRequest) error {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil {
		return fmt.Errorf("dispatched Orca handoff is required")
	}
	h := record.ExecutionHandoff
	if strings.TrimSpace(req.ContextSHA256) == "" || req.ContextSHA256 != h.ContextSHA256 {
		return fmt.Errorf("stale handoff context")
	}
	if pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(h.WorkerRoot) || pathutil.CleanAbsPath(req.CWD) != pathutil.CleanAbsPath(record.WorktreePath) {
		return fmt.Errorf("worker cwd/root does not match handoff worktree")
	}
	if strings.TrimSpace(req.OrcaWorktreeID) == "" || req.OrcaWorktreeID != h.Orca.WorktreeID {
		return fmt.Errorf("Orca worktree locator does not match handoff")
	}
	if strings.TrimSpace(req.Host) == "" || strings.TrimSpace(req.SessionID) == "" {
		return fmt.Errorf("native host and session identity are required")
	}
	return nil
}

func validateHandoffClaim(record IssueOpsRecord, req IssueOpsHandoffClaimRequest) error {
	if err := validateHandoffClaimIdentity(record, req); err != nil {
		return err
	}
	if err := validateHandoffContextSource(record); err != nil {
		return err
	}
	if err := validateHandoffCleanExactCheckpoint(record); err != nil {
		return err
	}
	return nil
}

func RecordIssueOpsHeartbeatWithRequest(stateRoot string, req IssueOpsHeartbeatRequest) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.Phase == IssueOpsPhaseDone {
			persisted = record
			return nil
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if record.ExecutionHandoff != nil {
			record, err = handoff.Heartbeat(record, handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256}, model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, now)
			if err != nil {
				return err
			}
		}
		record.LastHeartbeatAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func finishIssueOpsHandoffWithoutProjection(stateRoot string, req IssueOpsHandoffFinishRequest) (IssueOpsRecord, error) {
	return finishIssueOpsHandoff(stateRoot, req, issueOpsHandoffLifecycleHooks{})
}

func finishIssueOpsHandoff(stateRoot string, req IssueOpsHandoffFinishRequest, hooks issueOpsHandoffLifecycleHooks) (IssueOpsRecord, error) {
	req = normalizeHandoffFinishRequest(req)
	if err := validateHandoffFinishRequest(req); err != nil {
		return IssueOpsRecord{}, err
	}
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if err := validateHandoffResultIdentity(validated, req); err != nil {
		return IssueOpsRecord{}, err
	}
	if validated.ExecutionHandoff.State == handoff.StateClaimed {
		if err := validateHandoffContextSource(validated); err != nil {
			return IssueOpsRecord{}, err
		}
	}
	finishRequest := handoff.FinishRequest{
		Fence:  handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256},
		Worker: model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID},
		Result: model.IssueOpsExecutionHandoffResult{
			Outcome: req.Outcome, FinalHead: req.FinalHead, ChangedFiles: req.ChangedFiles, TuringReportPath: req.TuringReportPath,
			Verification: req.Verification, CleanupReceipts: req.CleanupReceipts, EvidenceDigest: req.EvidenceDigest, TaskID: req.TaskID, DispatchID: req.DispatchID,
		}, Now: validated.UpdatedAt,
	}
	if _, err := handoff.Finish(validated, finishRequest); err != nil {
		return IssueOpsRecord{}, err
	}
	if validated.ExecutionHandoff.State != handoff.StateClaimed {
		return validated, nil
	}
	runHandoffLifecycleHook(hooks.BeforeLockedRevalidation)
	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, validated) {
			if _, finishErr := handoff.Finish(record, finishRequest); finishErr == nil && record.ExecutionHandoff.State != handoff.StateClaimed {
				persisted = record
				return nil
			}
			return fmt.Errorf("handoff changed after finish validation; retry with the current fence")
		}
		if record.ExecutionHandoff.State == handoff.StateClaimed {
			if err := validateHandoffContextSource(record); err != nil {
				return err
			}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record, err = handoff.Finish(record, handoff.FinishRequest{
			Fence:  handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256},
			Worker: model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID},
			Result: model.IssueOpsExecutionHandoffResult{
				Outcome: req.Outcome, FinalHead: req.FinalHead, ChangedFiles: req.ChangedFiles, TuringReportPath: req.TuringReportPath,
				Verification: req.Verification, CleanupReceipts: req.CleanupReceipts, EvidenceDigest: req.EvidenceDigest, TaskID: req.TaskID, DispatchID: req.DispatchID,
			}, Now: now,
		})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func validateHandoffResultIdentity(record IssueOpsRecord, req IssueOpsHandoffFinishRequest) error {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil {
		return fmt.Errorf("execution handoff is required")
	}
	orca := record.ExecutionHandoff.Orca
	if strings.TrimSpace(orca.TaskID) == "" || strings.TrimSpace(req.TaskID) == "" || req.TaskID != orca.TaskID {
		return fmt.Errorf("worker result task id must exactly match the persisted handoff")
	}
	if strings.TrimSpace(orca.DispatchID) == "" || strings.TrimSpace(req.DispatchID) == "" || req.DispatchID != orca.DispatchID {
		return fmt.Errorf("worker result dispatch id must exactly match the persisted handoff")
	}
	return nil
}

func normalizeHandoffFinishRequest(req IssueOpsHandoffFinishRequest) IssueOpsHandoffFinishRequest {
	req.ID = strings.TrimSpace(req.ID)
	req.OwnershipEpoch = strings.TrimSpace(req.OwnershipEpoch)
	req.ContextSHA256 = strings.TrimSpace(req.ContextSHA256)
	req.Host = strings.TrimSpace(req.Host)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Outcome = strings.TrimSpace(req.Outcome)
	req.FinalHead = strings.TrimSpace(req.FinalHead)
	req.TuringReportPath = strings.TrimSpace(req.TuringReportPath)
	req.EvidenceDigest = strings.TrimSpace(req.EvidenceDigest)
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.DispatchID = strings.TrimSpace(req.DispatchID)
	req.ChangedFiles = canonicalChangedFileList(req.ChangedFiles)
	req.Verification = canonicalEvidenceList(req.Verification)
	req.CleanupReceipts = canonicalEvidenceList(req.CleanupReceipts)
	return req
}

func canonicalChangedFileList(values []string) []string {
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	return clean
}

func canonicalEvidenceList(values []string) []string {
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	return clean
}

func validateHandoffFinishRequest(req IssueOpsHandoffFinishRequest) error {
	if req.Outcome == handoff.OutcomeCompleted {
		if strings.TrimSpace(req.FinalHead) == "" || strings.TrimSpace(req.TuringReportPath) == "" || !hasNonEmptyHandoffEvidence(req.Verification) || !hasNonEmptyHandoffEvidence(req.CleanupReceipts) {
			return fmt.Errorf("completed finish requires final head, Turing report, verification, and cleanup receipts")
		}
		if !safeRelativeHandoffResultPath(req.TuringReportPath) {
			return fmt.Errorf("Turing report path must be a safe relative worker path")
		}
	}
	if len(req.ChangedFiles) > 512 || len(req.Verification) > 128 || len(req.CleanupReceipts) > 128 {
		return fmt.Errorf("handoff result evidence exceeds bounded limits")
	}
	for _, value := range append(append(append([]string{}, req.ChangedFiles...), req.Verification...), req.CleanupReceipts...) {
		if len(value) > 4096 {
			return fmt.Errorf("handoff result evidence item exceeds bounded limit")
		}
	}
	return nil
}

func AcceptIssueOpsHandoff(stateRoot string, req IssueOpsHandoffAcceptRequest) (IssueOpsRecord, error) {
	return acceptIssueOpsHandoff(stateRoot, req, issueOpsHandoffLifecycleHooks{})
}

func acceptIssueOpsHandoff(stateRoot string, req IssueOpsHandoffAcceptRequest, hooks issueOpsHandoffLifecycleHooks) (IssueOpsRecord, error) {
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if validated.ExecutionHandoff == nil {
		return IssueOpsRecord{}, fmt.Errorf("execution handoff is required")
	}
	if !handoff.CoordinatorIdentityMatches(validated, model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.SourceCWD) {
		return IssueOpsRecord{}, fmt.Errorf("handoff accept requires the sealed coordinator native session from the exact source checkout")
	}
	if validated.ExecutionHandoff.State != handoff.StateClosed {
		if strings.TrimSpace(req.ContextSHA256) == "" || req.ContextSHA256 != validated.ExecutionHandoff.ContextSHA256 {
			return IssueOpsRecord{}, fmt.Errorf("stale handoff context")
		}
		if err := validateHandoffContextSource(validated); err != nil {
			return IssueOpsRecord{}, err
		}
		if err := validateHandoffAcceptEvidence(validated, req); err != nil {
			return IssueOpsRecord{}, err
		}
	}
	runHandoffLifecycleHook(hooks.BeforeLockedRevalidation)
	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, req.ID, func() error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, validated) {
			return fmt.Errorf("handoff changed after accept validation; retry with the current fence")
		}
		if !handoff.CoordinatorIdentityMatches(record, model.IssueOpsHostSessionIdentity{Host: req.Host, SessionID: req.SessionID, AgentID: req.AgentID}, req.SourceCWD) {
			return fmt.Errorf("handoff accept requires the sealed coordinator native session from the exact source checkout")
		}
		if record.ExecutionHandoff.State != handoff.StateClosed {
			if strings.TrimSpace(req.ContextSHA256) == "" || req.ContextSHA256 != record.ExecutionHandoff.ContextSHA256 {
				return fmt.Errorf("stale handoff context")
			}
			if err := validateHandoffContextSource(record); err != nil {
				return err
			}
			if err := validateHandoffAcceptEvidence(record, req); err != nil {
				return err
			}
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record, err = handoff.Accept(record, handoff.AcceptRequest{Fence: handoff.Fence{Attempt: req.Attempt, OwnershipEpoch: req.OwnershipEpoch, ContextSHA256: req.ContextSHA256}, FinalHead: req.FinalHead, Now: now})
		if err != nil {
			return err
		}
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	return persisted, err
}

func runHandoffLifecycleHook(hook func()) {
	if hook != nil {
		hook()
	}
}

func validateHandoffContextSource(record IssueOpsRecord) error {
	if record.ExecutionHandoff == nil || len(strings.TrimSpace(record.ExecutionHandoff.ContextSourceSHA256)) != 64 {
		return fmt.Errorf("persisted context source fingerprint is required")
	}
	current, err := handoff.ContextSourceSHA256(record)
	if err != nil {
		return fmt.Errorf("re-render handoff context source: %w", err)
	}
	if current != record.ExecutionHandoff.ContextSourceSHA256 {
		return fmt.Errorf("stale handoff context source fingerprint")
	}
	return nil
}

func validateHandoffAcceptEvidence(record IssueOpsRecord, req IssueOpsHandoffAcceptRequest) error {
	h := record.ExecutionHandoff
	if h == nil || h.Result == nil || h.Orca == nil {
		return fmt.Errorf("submitted completed result evidence is required")
	}
	result := h.Result
	if result.Outcome != handoff.OutcomeCompleted || strings.TrimSpace(result.FinalHead) == "" || strings.TrimSpace(result.TuringReportPath) == "" || !hasNonEmptyHandoffEvidence(result.Verification) || !hasNonEmptyHandoffEvidence(result.CleanupReceipts) {
		return fmt.Errorf("submitted completed result evidence is incomplete")
	}
	if strings.TrimSpace(result.FinalHead) != strings.TrimSpace(req.FinalHead) {
		return fmt.Errorf("submitted result head does not match accepted head")
	}
	if strings.TrimSpace(h.Orca.TaskID) == "" || strings.TrimSpace(result.TaskID) == "" || result.TaskID != h.Orca.TaskID {
		return fmt.Errorf("submitted result task identity does not match handoff")
	}
	if strings.TrimSpace(h.Orca.DispatchID) == "" || strings.TrimSpace(result.DispatchID) == "" || result.DispatchID != h.Orca.DispatchID {
		return fmt.Errorf("submitted result dispatch identity does not match handoff")
	}
	workerRoot := pathutil.CleanAbsPath(h.WorkerRoot)
	if workerRoot == "" || workerRoot != pathutil.CleanAbsPath(record.WorktreePath) {
		return fmt.Errorf("canonical worker root does not match the linked worktree")
	}
	reportPath := strings.TrimSpace(result.TuringReportPath)
	if reportPath == "" {
		return fmt.Errorf("Turing report is required")
	}
	if !safeRelativeHandoffResultPath(reportPath) {
		return fmt.Errorf("Turing report path must be a safe relative worker path")
	}
	reportPath = filepath.Join(workerRoot, reportPath)
	leafInfo, err := os.Lstat(reportPath)
	if err != nil {
		return fmt.Errorf("Turing report does not exist: %w", err)
	}
	if leafInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Turing report leaf must not be a symlink")
	}
	resolvedRoot, err := filepath.EvalSymlinks(workerRoot)
	if err != nil {
		return fmt.Errorf("resolve canonical worker root: %w", err)
	}
	resolvedReport, err := filepath.EvalSymlinks(reportPath)
	if err != nil {
		return fmt.Errorf("Turing report does not exist: %w", err)
	}
	if !pathutil.PathWithin(resolvedReport, resolvedRoot) {
		return fmt.Errorf("Turing report must exist inside the canonical worker root")
	}
	info, err := os.Stat(resolvedReport)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("Turing report must be a regular file")
	}
	if branch := strings.TrimSpace(preflight.GitOut(workerRoot, "branch", "--show-current")); branch == "" || branch != strings.TrimSpace(record.Branch) {
		return fmt.Errorf("worker branch does not match handoff branch at accept")
	}
	code, head, _ := preflight.GitCmd(workerRoot, "rev-parse", "--verify", "HEAD^{commit}")
	if code != 0 || strings.TrimSpace(head) != strings.TrimSpace(req.FinalHead) {
		return fmt.Errorf("current worktree head does not match submitted head")
	}
	code, status, _ := preflight.GitCmd(workerRoot, "status", "--porcelain=v1")
	if code != 0 {
		return fmt.Errorf("worker worktree status is unreadable")
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("worker worktree must be clean before accept")
	}
	base := strings.TrimSpace(h.AttemptBaseHead)
	finalHead := strings.TrimSpace(req.FinalHead)
	if base == "" {
		return fmt.Errorf("attempt base head is required for accept lineage")
	}
	if code, _, _ := preflight.GitCmd(workerRoot, "merge-base", "--is-ancestor", base, finalHead); code != 0 {
		return fmt.Errorf("submitted head must descend from the attempt base head")
	}
	code, diffOutput, _ := preflight.GitCmdRaw(workerRoot, "diff", "--name-only", "-z", base+".."+finalHead)
	if code != 0 {
		return fmt.Errorf("read committed handoff diff")
	}
	actual := canonicalGitPathSet(splitNULTerminatedPaths(diffOutput))
	expected, err := canonicalChangedFileSet(result.ChangedFiles)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("submitted changed_files do not exactly match the committed attempt diff")
	}
	reportRelative, err := filepath.Rel(resolvedRoot, resolvedReport)
	if err != nil || filepath.IsAbs(reportRelative) || reportRelative == ".." || strings.HasPrefix(filepath.ToSlash(reportRelative), "../") {
		return fmt.Errorf("Turing report must resolve to a safe relative worker path")
	}
	if _, ok := expected[filepath.ToSlash(filepath.Clean(reportRelative))]; !ok {
		return fmt.Errorf("committed Turing report must be included in changed_files")
	}
	return nil
}

func safeRelativeHandoffResultPath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.ContainsRune(value, 0) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	return clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func canonicalChangedFileSet(values []string) (map[string]struct{}, error) {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(value))
		if value == "" || value == "." || filepath.IsAbs(value) || value == ".." || strings.HasPrefix(value, "../") {
			return nil, fmt.Errorf("changed_files must contain only safe relative paths")
		}
		set[value] = struct{}{}
	}
	return set, nil
}

func canonicalGitPathSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(value))
		if value != "" && value != "." {
			set[value] = struct{}{}
		}
	}
	return set
}

func splitNULTerminatedPaths(value string) []string {
	parts := strings.Split(value, "\x00")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func hasNonEmptyHandoffEvidence(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
