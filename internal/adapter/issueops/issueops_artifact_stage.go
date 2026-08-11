package issueops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/contract/issueops"
)

// artifactStageBucket은 prepare 이전에 코디네이터가 스테이징한 artifact를
// 담는다. issueops bucket과 분리되어 있어 훅의 레코드 스캔 비용에 영향이
// 없다(설계 v5 WS2).
const artifactStageBucket = "artifact_stage_v1"

// IssueOpsArtifactDir은 워크트리 안에서 materialize된 artifact가 놓이는
// 상대 경로다. `.gitignore` 대상이며 보존은 completion 섹션이 담당한다.
const IssueOpsArtifactDir = ".agent-harness/artifact"

type PlanIdentity struct {
	Path   string
	Digest string
}

type OwnerPlanIdentity = PlanIdentity

type planArtifactRequiredError struct {
	nextCommand string
}

func (e *planArtifactRequiredError) Error() string {
	return "Orca execution requires a staged plan artifact"
}

func (e *planArtifactRequiredError) IssueOpsErrorFields() map[string]any {
	fields := map[string]any{
		"code":    "orca_plan_artifact_required",
		"missing": []string{"plan"},
	}
	if e.nextCommand != "" {
		fields["next_command"] = e.nextCommand
	}
	return fields
}

func RequireStagedExecutionOwnerPlan(stateRoot string, record issueops.IssueOpsRecord) (PlanIdentity, error) {
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err != nil {
		return PlanIdentity{}, err
	}
	plan, ok := staged["plan"]
	if !ok || strings.TrimSpace(plan) == "" {
		return PlanIdentity{}, newPlanArtifactRequiredError(record, true)
	}
	identity := PlanIdentity{Digest: digestExecutionOwnerBytes([]byte(plan))}
	if strings.TrimSpace(record.PlanPath) == "" {
		return identity, nil
	}
	linked, err := readLinkedPlanIdentity(record)
	if err != nil || linked.Digest != identity.Digest {
		return PlanIdentity{}, newPlanArtifactRequiredError(record, false)
	}
	return linked, nil
}

func newPlanArtifactRequiredError(record issueops.IssueOpsRecord, allowStageCommand bool) error {
	typed := &planArtifactRequiredError{}
	if !allowStageCommand {
		return typed
	}
	if identity, err := readLinkedPlanIdentity(record); err == nil {
		typed.nextCommand = "agent-harness issueops artifact stage --id " + quoteExecutionOwnerArg(record.ID) +
			" --name plan --file " + quoteExecutionOwnerArg(identity.Path) + " --json"
	}
	return typed
}

func newPlanResumeArtifactRequiredError(record issueops.IssueOpsRecord) error {
	typed := &planArtifactRequiredError{}
	if record.Execution != nil && record.Execution.Lease.Generation > 0 {
		typed.nextCommand = executionReplacementPreviewCommand(record.ID, record.Execution.Lease.Generation)
	}
	return typed
}

func readLinkedPlanIdentity(record issueops.IssueOpsRecord) (PlanIdentity, error) {
	worktree := strings.TrimSpace(record.WorktreePath)
	planPath := strings.TrimSpace(record.PlanPath)
	if worktree == "" || planPath == "" || strings.Contains(worktree, "\x00") || strings.Contains(planPath, "\x00") {
		return PlanIdentity{}, fmt.Errorf("durable plan path is unavailable")
	}
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(worktree, planPath)
	}
	worktree, err := filepath.Abs(worktree)
	if err != nil {
		return PlanIdentity{}, err
	}
	planPath, err = filepath.Abs(planPath)
	if err != nil {
		return PlanIdentity{}, err
	}
	worktree, planPath = filepath.Clean(worktree), filepath.Clean(planPath)
	worktreeInfo, err := os.Stat(worktree)
	if err != nil || !worktreeInfo.IsDir() || !issueOpsPlanPathInsideWorktree(worktree, planPath) {
		return PlanIdentity{}, fmt.Errorf("durable plan path is outside the canonical worktree")
	}
	info, err := os.Lstat(planPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return PlanIdentity{}, fmt.Errorf("durable plan path is not a regular file")
	}
	content, err := os.ReadFile(planPath)
	if err != nil || strings.TrimSpace(string(content)) == "" {
		return PlanIdentity{}, fmt.Errorf("durable plan is empty or unreadable")
	}
	return PlanIdentity{Path: planPath, Digest: digestExecutionOwnerBytes(content)}, nil
}

func materializeExecutionOwnerArtifacts(stateRoot string, record issueops.IssueOpsRecord) (OwnerPlanIdentity, map[string]string, error) {
	preflight, err := RequireStagedExecutionOwnerPlan(stateRoot, record)
	if err != nil {
		return OwnerPlanIdentity{}, nil, err
	}
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		return OwnerPlanIdentity{}, nil, err
	}
	planDigest, ok := manifest["plan"]
	if !ok || !strings.EqualFold(planDigest, preflight.Digest) {
		return OwnerPlanIdentity{}, nil, newPlanArtifactRequiredError(record, false)
	}
	if preflight.Path != "" {
		return preflight, manifest, nil
	}
	prepared := record
	prepared.WorktreePath = record.Execution.Workspace.Root
	prepared.PlanPath = filepath.Join(record.Execution.Workspace.Root, filepath.FromSlash(IssueOpsArtifactDir), "plan.md")
	identity, err := readLinkedPlanIdentity(prepared)
	if err != nil || !strings.EqualFold(identity.Digest, planDigest) {
		return OwnerPlanIdentity{}, nil, newPlanArtifactRequiredError(prepared, false)
	}
	return identity, manifest, nil
}

func readStagedArtifacts(stateRoot, id string) (map[string]string, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	data, found, err := db.Get(artifactStageBucket, id)
	if err != nil {
		return nil, err
	}
	staged := map[string]string{}
	if !found || len(data) == 0 {
		return staged, nil
	}
	if err := json.Unmarshal(data, &staged); err != nil {
		return nil, fmt.Errorf("parse staged artifacts: %w", err)
	}
	return staged, nil
}

// materializeStagedArtifacts는 스테이징된 artifact를 워크트리의
// IssueOpsArtifactDir로 0600 파일로 옮기고 name→sha256 manifest를 돌려준다.
// writeExecutionOwnerArtifact의 immutable 계약을 재사용하므로 재실행은
// 동일 내용일 때만 통과한다. generic helper는 스테이징이 없으면 빈 manifest를
// 반환하지만 Orca owner 경로는 materializeExecutionOwnerArtifacts에서 plan을
// 필수로 검증한다. replacement 재봉인도 그 경로로 같은 내용을 다시 검증한다.
// 기존 파일을 바꾸는 재-materialize는 immutable writer가 거부한다.
func materializeStagedArtifacts(stateRoot string, record issueops.IssueOpsRecord) (map[string]string, error) {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return nil, fmt.Errorf("cannot materialize artifacts without a canonical worktree")
	}
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err != nil {
		return nil, err
	}
	manifest := map[string]string{}
	root := record.Execution.Workspace.Root
	for name, content := range staged {
		path := filepath.Join(root, filepath.FromSlash(IssueOpsArtifactDir), name+".md")
		if err := writeExecutionOwnerArtifact(root, path, []byte(content)); err != nil {
			return nil, fmt.Errorf("materialize artifact %s: %w", name, err)
		}
		manifest[name] = digestExecutionOwnerBytes([]byte(content))
	}
	return manifest, nil
}

// rejectSecretLikeContent는 issueops_decision.go의 거부형 secret 검사 계약을
// artifact 본문에 적용한다.
