package issueops

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/artifactverify"
	"agent-harness/internal/core/issueops/model"
)

func validFullCommitSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type ExecutionCompleteRequestV1 struct {
	ID                string              `json:"id"`
	Generation        uint64              `json:"generation"`
	Actor             model.NativeActorV1 `json:"actor"`
	CWD               string              `json:"cwd"`
	FinalHead         string              `json:"final_head"`
	TuringReportPath  string              `json:"turing_report_path"`
	Verification      []string            `json:"verification"`
	RemoteArtifactURL string              `json:"remote_artifact_url"`
	Confirm           bool                `json:"confirm"`
}

func CompleteExecutionV1(stateRoot string, req ExecutionCompleteRequestV1) (ExecutionResultV1, error) {
	if err := RequireIssueOpsV1MutationAllowed(stateRoot); err != nil {
		return ExecutionResultV1{OK: false, ID: req.ID}, err
	}
	actor, err := normalizeNativeActorV1(req.Actor)
	if err != nil {
		return ExecutionResultV1{OK: false, ID: req.ID}, err
	}
	if !req.Confirm {
		return ExecutionResultV1{OK: false, ID: req.ID}, fmt.Errorf("execution complete requires confirm")
	}
	verification, err := normalizeExecutionVerificationV1(req.Verification)
	if err != nil {
		return ExecutionResultV1{OK: false, ID: req.ID}, err
	}
	if err := validateExecutionRemoteArtifactURLV1(req.RemoteArtifactURL); err != nil {
		return ExecutionResultV1{OK: false, ID: req.ID}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, req.ID, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, req.ID)
		if err != nil {
			return err
		}
		if record.Execution == nil {
			return fmt.Errorf("IssueOps execution v1 is not prepared")
		}
		if record.Execution.Completion != nil {
			if err := validateTerminalExecutionCompletionV1(record, req, verification); err == nil {
				persisted = record
				return nil
			}
			return fmt.Errorf("execution completion already exists with different evidence")
		}
		if record.Phase != IssueOpsPhasePR {
			return fmt.Errorf("execution completion requires pr phase")
		}
		if err := validateExecutionCompletionArtifactV1(record, req.RemoteArtifactURL); err != nil {
			return err
		}
		lease := &record.Execution.Lease
		if lease.Status != model.LeaseStatusActive || lease.Generation != req.Generation || !sameNativeActorV1(lease.Holder, &actor) {
			return fmt.Errorf("only the current holder may complete generation %d", req.Generation)
		}
		if !samePath(req.CWD, record.Execution.Workspace.Root) {
			return fmt.Errorf("completion cwd must be the canonical worktree")
		}
		currentHead, err := gitOutput(record.Execution.Workspace.Root, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if !validFullCommitSHA(req.FinalHead) || !strings.EqualFold(strings.TrimSpace(req.FinalHead), currentHead) {
			return fmt.Errorf("final_head must match canonical worktree HEAD")
		}
		reportPath, err := validateExecutionReportPathV1(record.Execution.Workspace.Root, req.TuringReportPath)
		if err != nil {
			return err
		}
		previous := *lease.Holder
		now := time.Now().UTC().Format(time.RFC3339Nano)
		record.Execution.Completion = &model.ExecutionCompletionV1{
			FinalHead: strings.ToLower(strings.TrimSpace(req.FinalHead)), TuringReportPath: reportPath,
			Verification: verification, RemoteArtifactURL: strings.TrimSpace(req.RemoteArtifactURL), CompletedAt: now,
		}
		lease.Status = model.LeaseStatusReleased
		lease.Holder = nil
		lease.ClaimTokenSHA256 = ""
		lease.ReleasedAt = now
		if err := validateIssueOpsPhaseTransition(stateRoot, record, IssueOpsPhaseDone); err != nil {
			return err
		}
		record = applyIssueOpsPhaseTransition(record, IssueOpsPhaseDone)
		persisted, err = persistExecutionTransition(stateRoot, record, &previous)
		return err
	})
	if err != nil {
		return ExecutionResultV1{OK: false, ID: req.ID}, err
	}
	return executionResult(persisted), nil
}

func validateTerminalExecutionCompletionV1(record IssueOpsRecord, req ExecutionCompleteRequestV1, verification []string) error {
	if record.Execution == nil || record.Execution.Completion == nil || record.Phase != IssueOpsPhaseDone {
		return fmt.Errorf("execution completion is not fully terminal")
	}
	lease := record.Execution.Lease
	if lease.Generation != req.Generation || lease.Status != model.LeaseStatusReleased || lease.Holder != nil ||
		lease.ClaimTokenSHA256 != "" || strings.TrimSpace(lease.ReleasedAt) == "" {
		return fmt.Errorf("execution completion lease is not fully released")
	}
	if strings.TrimSpace(record.Execution.Completion.CompletedAt) == "" ||
		!executionCompletionMatchesV1(*record.Execution.Completion, req, verification) {
		return fmt.Errorf("execution completion evidence differs")
	}
	return validateExecutionCompletionArtifactV1(record, req.RemoteArtifactURL)
}

func validateExecutionCompletionArtifactV1(record IssueOpsRecord, requestedURL string) error {
	artifact := record.RemoteArtifact
	if artifact == nil {
		return fmt.Errorf("execution completion requires a durable verified remote artifact")
	}
	if strings.TrimSpace(artifact.VerifiedAt) == "" {
		return fmt.Errorf("execution completion requires remote artifact verification")
	}
	if record.BranchPrepare == nil || strings.TrimSpace(record.BranchPrepare.BaseBranch) == "" {
		return fmt.Errorf("execution completion requires a prepared target branch")
	}
	baseBranch := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	if strings.TrimSpace(artifact.TargetBranch) != baseBranch {
		return fmt.Errorf("remote artifact target branch must match prepared base branch")
	}
	projectionRecord := record
	projectionRecord.Phase = IssueOpsPhasePR
	projection, err := artifactverify.Projection(projectionRecord, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider: artifact.Provider, Kind: artifact.Kind, URL: artifact.URL,
		Labels: artifact.Labels, Assignees: artifact.Assignees, TargetBranch: artifact.TargetBranch,
	})
	if err != nil {
		return fmt.Errorf("remote artifact verification is invalid: %w", err)
	}
	if projection.Provider != artifact.Provider || projection.Kind != artifact.Kind || projection.URL != artifact.URL ||
		!slices.Equal(projection.Labels, artifact.Labels) || !slices.Equal(projection.Assignees, artifact.Assignees) ||
		projection.TargetBranch != artifact.TargetBranch {
		return fmt.Errorf("remote artifact verification is not canonical")
	}
	if artifact.URL != strings.TrimSpace(requestedURL) {
		return fmt.Errorf("completion remote_artifact_url must match the durable verified artifact")
	}
	return nil
}

func normalizeExecutionVerificationV1(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("verification entries must be nonempty")
		}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("execution completion requires verification evidence")
	}
	return result, nil
}

func validateExecutionRemoteArtifactURLV1(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path == "" {
		return fmt.Errorf("execution completion requires an HTTPS draft PR or MR URL")
	}
	return nil
}

func validateExecutionReportPathV1(root, path string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path, err = filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("Turing report must exist in the canonical worktree")
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Turing report must be inside the canonical worktree")
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("Turing report must be a regular file")
	}
	return resolved, nil
}

func executionCompletionMatchesV1(completion model.ExecutionCompletionV1, req ExecutionCompleteRequestV1, verification []string) bool {
	return strings.EqualFold(completion.FinalHead, strings.TrimSpace(req.FinalHead)) &&
		samePath(completion.TuringReportPath, req.TuringReportPath) && slices.Equal(completion.Verification, verification) &&
		completion.RemoteArtifactURL == strings.TrimSpace(req.RemoteArtifactURL)
}
