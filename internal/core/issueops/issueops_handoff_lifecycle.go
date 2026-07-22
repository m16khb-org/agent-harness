package issueops

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/pathutil"
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

type IssueOpsHandoffClaimResult struct {
	IssueOpsRecord
	State          string `json:"state"`
	Attempt        int    `json:"attempt"`
	OwnershipEpoch string `json:"ownership_epoch"`
	ContextSHA256  string `json:"context_sha256"`
	PlanSHA256     string `json:"plan_sha256"`
	NextCommand    string `json:"next_command"`
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

type IssueOpsHandoffCompleteRequest struct {
	ID               string   `json:"id"`
	Attempt          int      `json:"attempt"`
	OwnershipEpoch   string   `json:"ownership_epoch"`
	ContextSHA256    string   `json:"context_sha256"`
	Host             string   `json:"host"`
	SessionID        string   `json:"session_id"`
	AgentID          string   `json:"agent_id,omitempty"`
	CWD              string   `json:"cwd,omitempty"`
	FinalHead        string   `json:"final_head"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	TuringReportPath string   `json:"turing_report_path"`
	Verification     []string `json:"verification"`
}

type issueOpsHandoffLifecycleHooks struct {
	BeforeLockedRevalidation func()
}

func ClaimIssueOpsHandoff(stateRoot string, req IssueOpsHandoffClaimRequest) (IssueOpsHandoffClaimResult, error) {
	return claimIssueOpsHandoff(stateRoot, req, issueOpsHandoffLifecycleHooks{})
}

func claimIssueOpsHandoff(stateRoot string, req IssueOpsHandoffClaimRequest, hooks issueOpsHandoffLifecycleHooks) (IssueOpsHandoffClaimResult, error) {
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsHandoffClaimResult{}, err
	}
	if err := validateHandoffClaimIdentity(validated, req); err != nil {
		return IssueOpsHandoffClaimResult{}, err
	}
	if currentIssueOpsHandoff(validated).State != handoff.StateOwnerOrienting {
		if _, err := validateHandoffClaim(validated, req); err != nil {
			return IssueOpsHandoffClaimResult{}, err
		}
	}
	runHandoffLifecycleHook(hooks.BeforeLockedRevalidation)
	var persisted IssueOpsRecord
	var planSHA256 string
	err = withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(record, validated) {
			return fmt.Errorf("handoff changed after claim validation; retry with the current fence")
		}
		alreadyClaimed := currentIssueOpsHandoff(record).State == handoff.StateOwnerOrienting
		if !alreadyClaimed {
			planSHA256, err = validateHandoffClaim(record, req)
			if err != nil {
				return err
			}
		} else {
			planSHA256, err = validatedHandoffClaimPlanSHA256(record)
			if err != nil {
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
		record.LastHeartbeatAt = now
		record.UpdatedAt = now
		persisted, err = writeIssueOps(stateRoot, record)
		return err
	})
	if err != nil {
		return IssueOpsHandoffClaimResult{}, err
	}
	return projectIssueOpsHandoffClaim(persisted, planSHA256)
}

func IssueOpsHandoffAcknowledgeCommand(record IssueOpsRecord) (string, error) {
	planSHA256, err := validatedHandoffClaimPlanSHA256(record)
	if err != nil {
		return "", err
	}
	return issueOpsHandoffAcknowledgeCommand(record, planSHA256)
}

func projectIssueOpsHandoffClaim(record IssueOpsRecord, planSHA256 string) (IssueOpsHandoffClaimResult, error) {
	h := currentIssueOpsHandoff(record)
	if h == nil || h.State != handoff.StateOwnerOrienting || h.OwnerSession == nil {
		return IssueOpsHandoffClaimResult{}, fmt.Errorf("claimed ownership handoff must be owner_orienting with an exact owner session")
	}
	nextCommand, err := issueOpsHandoffAcknowledgeCommand(record, planSHA256)
	if err != nil {
		return IssueOpsHandoffClaimResult{}, err
	}
	return IssueOpsHandoffClaimResult{
		IssueOpsRecord: record, State: h.State, Attempt: h.Attempt, OwnershipEpoch: h.OwnershipEpoch,
		ContextSHA256: h.ContextSHA256, PlanSHA256: planSHA256, NextCommand: nextCommand,
	}, nil
}

func issueOpsHandoffAcknowledgeCommand(record IssueOpsRecord, planSHA256 string) (string, error) {
	h := currentIssueOpsHandoff(record)
	if h == nil || h.State != handoff.StateOwnerOrienting || h.OwnerSession == nil {
		return "", fmt.Errorf("acknowledgement command requires owner_orienting with an exact owner session")
	}
	if strings.TrimSpace(planSHA256) == "" || strings.TrimSpace(record.IssueURL) == "" || strings.TrimSpace(h.WorkerRoot) == "" {
		return "", fmt.Errorf("acknowledgement command requires exact issue, plan, and worker-root context")
	}
	owner := *h.OwnerSession
	parts := []string{
		"agent-harness issueops handoff acknowledge-context",
		"--id " + issueOpsShellQuote(record.ID),
		"--attempt " + strconv.Itoa(h.Attempt),
		"--ownership-epoch " + issueOpsShellQuote(h.OwnershipEpoch),
		"--context-sha256 " + issueOpsShellQuote(h.ContextSHA256),
		"--host " + issueOpsShellQuote(owner.Host),
		"--session-id " + issueOpsShellQuote(owner.SessionID),
	}
	if strings.TrimSpace(owner.AgentID) != "" {
		parts = append(parts, "--agent-id "+issueOpsShellQuote(owner.AgentID))
	}
	parts = append(parts,
		"--cwd "+issueOpsShellQuote(h.WorkerRoot),
		"--issue-url "+issueOpsShellQuote(record.IssueURL),
		"--plan-sha256 "+issueOpsShellQuote(planSHA256),
		"--understanding "+issueOpsShellQuote("I reviewed the sealed issue and plan context."),
		"--scope-confirmation "+issueOpsShellQuote("I will implement only the sealed worker-root scope."),
	)
	return strings.Join(parts, " "), nil
}

func issueOpsShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func validatedHandoffClaimPlanSHA256(record IssueOpsRecord) (string, error) {
	if currentIssueOpsHandoff(record) == nil {
		return "", fmt.Errorf("execution handoff is required")
	}
	options := handoff.ContextOptions{}
	if currentIssueOpsHandoff(record).ContextOptions != nil {
		options = handoff.ContextOptionsFromModel(*currentIssueOpsHandoff(record).ContextOptions)
	}
	packet, err := handoff.BuildContext(record, options)
	if err != nil {
		return "", fmt.Errorf("re-render exact handoff context: %w", err)
	}
	if packet.SourceSHA256 != currentIssueOpsHandoff(record).ContextSourceSHA256 {
		return "", fmt.Errorf("stale handoff context source fingerprint")
	}
	if packet.SHA256 != currentIssueOpsHandoff(record).ContextSHA256 {
		return "", fmt.Errorf("stale handoff context")
	}
	return packet.PlanSHA256, nil
}

func validateHandoffClaimIdentity(record IssueOpsRecord, req IssueOpsHandoffClaimRequest) error {
	if currentIssueOpsHandoff(record) == nil || currentIssueOpsHandoff(record).Orca == nil {
		return fmt.Errorf("dispatched Orca handoff is required")
	}
	h := currentIssueOpsHandoff(record)
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

func validateHandoffClaim(record IssueOpsRecord, req IssueOpsHandoffClaimRequest) (string, error) {
	if err := validateHandoffClaimIdentity(record, req); err != nil {
		return "", err
	}
	if err := validateHandoffStartCheckpoint(record); err != nil {
		return "", err
	}
	return validatedHandoffClaimPlanSHA256(record)
}

func RecordIssueOpsHeartbeatWithRequest(stateRoot string, req IssueOpsHeartbeatRequest) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.Phase == IssueOpsPhaseDone {
			persisted = record
			return nil
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		if currentIssueOpsHandoff(record) != nil {
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

func normalizeHandoffCompleteRequest(req IssueOpsHandoffCompleteRequest) IssueOpsHandoffCompleteRequest {
	req.ID = strings.TrimSpace(req.ID)
	req.OwnershipEpoch = strings.TrimSpace(req.OwnershipEpoch)
	req.ContextSHA256 = strings.TrimSpace(req.ContextSHA256)
	req.Host = strings.TrimSpace(req.Host)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.CWD = pathutil.CleanAbsPath(req.CWD)
	req.FinalHead = strings.TrimSpace(req.FinalHead)
	req.TuringReportPath = strings.TrimSpace(req.TuringReportPath)
	req.ChangedFiles = canonicalChangedFileList(req.ChangedFiles)
	req.Verification = canonicalEvidenceList(req.Verification)
	return req
}

func validateHandoffCompleteRequest(req IssueOpsHandoffCompleteRequest) error {
	if !safeRelativeHandoffResultPath(req.TuringReportPath) {
		return fmt.Errorf("Turing report path must be a safe relative owner-worktree path")
	}
	if len(req.ChangedFiles) > 512 || len(req.Verification) > 128 {
		return fmt.Errorf("handoff completion evidence exceeds bounded limits")
	}
	for _, value := range append(append([]string{}, req.ChangedFiles...), req.Verification...) {
		if len(value) > 4096 {
			return fmt.Errorf("handoff completion evidence item exceeds bounded limit")
		}
	}
	return nil
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
	if len(clean) == 0 {
		return nil
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

func runHandoffLifecycleHook(hook func()) {
	if hook != nil {
		hook()
	}
}

func validateHandoffContextSource(record IssueOpsRecord) error {
	if currentIssueOpsHandoff(record) == nil || len(strings.TrimSpace(currentIssueOpsHandoff(record).ContextSourceSHA256)) != 64 {
		return fmt.Errorf("persisted context source fingerprint is required")
	}
	current, err := handoff.ContextSourceSHA256(record)
	if err != nil {
		return fmt.Errorf("re-render handoff context source: %w", err)
	}
	if current != currentIssueOpsHandoff(record).ContextSourceSHA256 {
		return fmt.Errorf("stale handoff context source fingerprint")
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
