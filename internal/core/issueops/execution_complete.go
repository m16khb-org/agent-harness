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

type ExecutionCompleteRequest struct {
	ID                string            `json:"id"`
	Generation        uint64            `json:"generation"`
	Actor             model.NativeActor `json:"actor"`
	CWD               string            `json:"cwd"`
	FinalHead         string            `json:"final_head"`
	TuringReportPath  string            `json:"turing_report_path"`
	Verification      []string          `json:"verification"`
	RemoteArtifactURL string            `json:"remote_artifact_url"`
	Confirm           bool              `json:"confirm"`
}

// ExecutionCompleteDeps는 완료 시점의 외부 표면 주입점이다.
type ExecutionCompleteDeps struct {
	// SettleOrcaTask는 사이클에 묶인 orca task를 terminal 상태로 옮긴다.
	//
	// 왜 완료 시점인가: task는 자원이 아니라 오케스트레이션 이력이므로 종결은
	// 자원 회수가 아니라 작업 종료 사실의 기록이고, 그 시점은 owner의 마지막
	// durable 행위인 완료다. cleanup finish에 두면 완료와 정리 사이 구간
	// (사용자 머지 대기를 포함한다)에서 Orca가 task를 진행 중으로 오인한다.
	//
	// nil이면 종결을 건너뛴다. 종결 수단이 없다는 사실이 완료를 막아서는 안
	// 된다 — Orca 상태 갱신은 durable state의 전제 조건이 아니다(#130).
	SettleOrcaTask func(ctx context.Context, runID, taskID string) error
}

func CompleteExecution(stateRoot string, req ExecutionCompleteRequest, deps ExecutionCompleteDeps) (ExecutionResult, error) {
	return completeExecutionWithClock(stateRoot, req, deps, time.Now)
}

// completeExecutionWithClock는 legacy/new persisted-byte differential이 두
// completion timestamp를 결정적으로 고정할 수 있게 한다. production은 위
// facade에서 time.Now를 주입하므로 기존 wall-clock 의미를 유지한다.
func completeExecutionWithClock(stateRoot string, req ExecutionCompleteRequest, deps ExecutionCompleteDeps, now func() time.Time) (ExecutionResult, error) {
	if err := RequireIssueOpsMutationAllowed(stateRoot); err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	actor, err := normalizeNativeActor(req.Actor)
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	if !req.Confirm {
		return ExecutionResult{OK: false, ID: req.ID}, fmt.Errorf("execution complete requires confirm")
	}
	verification, err := normalizeExecutionVerification(req.Verification)
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	if err := validateExecutionRemoteArtifactURL(req.RemoteArtifactURL); err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
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
			if err := validateTerminalExecutionCompletion(record, req, verification); err == nil {
				persisted = record
				return nil
			}
			return fmt.Errorf("execution completion already exists with different evidence")
		}
		if record.Phase != IssueOpsPhasePR {
			return fmt.Errorf("execution completion requires pr phase")
		}
		if err := validateExecutionCompletionArtifact(record, req.RemoteArtifactURL); err != nil {
			return err
		}
		lease := &record.Execution.Lease
		if lease.Status != model.LeaseStatusActive || lease.Generation != req.Generation || !sameNativeActor(lease.Holder, &actor) {
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
		reportPath, err := validateExecutionReportPath(record.Execution.Workspace.Root, req.TuringReportPath)
		if err != nil {
			return err
		}
		previous := *lease.Holder
		completedAt := now().UTC().Format(time.RFC3339Nano)
		record.Execution.Completion = &model.ExecutionCompletion{
			FinalHead: strings.ToLower(strings.TrimSpace(req.FinalHead)), TuringReportPath: reportPath,
			Verification: verification, RemoteArtifactURL: strings.TrimSpace(req.RemoteArtifactURL), CompletedAt: completedAt,
		}
		lease.Status = model.LeaseStatusReleased
		lease.Holder = nil
		lease.ClaimTokenSHA256 = ""
		lease.ReleasedAt = completedAt
		if err := validateIssueOpsPhaseTransition(stateRoot, record, IssueOpsPhaseDone); err != nil {
			return err
		}
		record = applyIssueOpsPhaseTransitionAt(record, IssueOpsPhaseDone, now().UTC().Format(time.RFC3339Nano))
		persisted, err = persistExecutionTransition(stateRoot, record, &previous)
		return err
	})
	if err != nil {
		return ExecutionResult{OK: false, ID: req.ID}, err
	}
	result := executionResult(persisted)
	// 종결은 락 밖에서, 상태가 이미 커밋된 뒤에 일어난다. 완료는 원자적
	// 상태 전이이고 외부 호출이 그 원자성을 흐려서는 안 된다.
	settleExecutionOrcaTask(persisted, deps, &result)
	return result, nil
}

// settleExecutionOrcaTask는 완료된 orca 사이클의 task를 terminal 상태로 옮기고
// 그 결과를 표면화한다. best-effort다: 실패는 이미 커밋된 완료를 되돌리지 않고
// 오류 사유만 결과에 남긴다(cleanup remote-branch의 audit 반영과 같은 계약).
// 침묵하면 진단이 불가능하므로 실패를 삼키지는 않는다.
func settleExecutionOrcaTask(record IssueOpsRecord, deps ExecutionCompleteDeps, result *ExecutionResult) {
	if deps.SettleOrcaTask == nil || record.Execution == nil ||
		record.Execution.Mode != model.ExecutionModeOrca || record.Execution.Orca == nil {
		return
	}
	runID := strings.TrimSpace(record.Execution.Orca.RunID)
	taskID := strings.TrimSpace(record.Execution.Orca.TaskID)
	if taskID == "" {
		return
	}
	if err := deps.SettleOrcaTask(context.Background(), runID, taskID); err != nil {
		result.OrcaTaskError = err.Error()
		return
	}
	result.OrcaTaskSettled = true
}

func validateTerminalExecutionCompletion(record IssueOpsRecord, req ExecutionCompleteRequest, verification []string) error {
	if record.Execution == nil || record.Execution.Completion == nil || record.Phase != IssueOpsPhaseDone {
		return fmt.Errorf("execution completion is not fully terminal")
	}
	lease := record.Execution.Lease
	if lease.Generation != req.Generation || lease.Status != model.LeaseStatusReleased || lease.Holder != nil ||
		lease.ClaimTokenSHA256 != "" || strings.TrimSpace(lease.ReleasedAt) == "" {
		return fmt.Errorf("execution completion lease is not fully released")
	}
	if strings.TrimSpace(record.Execution.Completion.CompletedAt) == "" ||
		!executionCompletionMatches(*record.Execution.Completion, req, verification) {
		return fmt.Errorf("execution completion evidence differs")
	}
	return validateExecutionCompletionArtifact(record, req.RemoteArtifactURL)
}

func validateExecutionCompletionArtifact(record IssueOpsRecord, requestedURL string) error {
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

// ValidateExecutionCompletionArtifact is the shared compatibility validator
// used by the migrated completion composition root. It exposes validation
// only; legacy orchestration remains a test oracle.
func ValidateExecutionCompletionArtifact(record IssueOpsRecord, requestedURL string) error {
	return validateExecutionCompletionArtifact(record, requestedURL)
}

func normalizeExecutionVerification(values []string) ([]string, error) {
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

func validateExecutionRemoteArtifactURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path == "" {
		return fmt.Errorf("execution completion requires an HTTPS draft PR or MR URL")
	}
	return nil
}

func validateExecutionReportPath(root, path string) (string, error) {
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

func executionCompletionMatches(completion model.ExecutionCompletion, req ExecutionCompleteRequest, verification []string) bool {
	return strings.EqualFold(completion.FinalHead, strings.TrimSpace(req.FinalHead)) &&
		samePath(completion.TuringReportPath, req.TuringReportPath) && slices.Equal(completion.Verification, verification) &&
		completion.RemoteArtifactURL == strings.TrimSpace(req.RemoteArtifactURL)
}
