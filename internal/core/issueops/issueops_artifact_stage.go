package issueops

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/adapter/outbound/sqlstore"
	"agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
)

// artifactStageBucket은 prepare 이전에 코디네이터가 스테이징한 artifact를
// 담는다. issueops bucket과 분리되어 있어 훅의 레코드 스캔 비용에 영향이
// 없다(설계 v5 WS2).
const artifactStageBucket = "artifact_stage_v1"

// IssueOpsArtifactDir은 워크트리 안에서 materialize된 artifact가 놓이는
// 상대 경로다. `.gitignore` 대상이며 보존은 completion 섹션이 담당한다.
const IssueOpsArtifactDir = ".agent-harness/artifact"

var issueOpsArtifactNames = map[string]bool{"plan": true, "spec": true, "turing-loop": true}

type PlanIdentity struct {
	Path   string
	Digest string
}

type OwnerPlanIdentity = PlanIdentity

type planArtifactRequiredError struct {
	nextCommand string
}

type artifactStageRecoveryError struct {
	id string
}

func (e *artifactStageRecoveryError) Error() string {
	return "artifacts are sealed after execution prepare; only a clean released Orca generation may stage a plan, and execution replace --reseed is required before resume"
}

func (e *artifactStageRecoveryError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":            "artifact_stage_requires_reseed",
		"required_action": "execution replace --reseed",
		"next_command":    "agent-harness issueops execution status --id " + quoteExecutionOwnerArg(e.id) + " --json",
	}
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

// StageIssueOpsArtifact는 plan|spec|turing-loop artifact를 스테이징한다.
// prepare 이후(Execution 존재)에는 조용한 no-op 대신 명시적으로 실패한다 —
// materialize와 manifest 봉인이 이미 끝났으므로 재스테이징은 반영되지 않는다.
func StageIssueOpsArtifact(stateRoot, id, name string, content []byte) (issueops.IssueOpsRecord, error) {
	name = strings.TrimSpace(name)
	if !issueOpsArtifactNames[name] {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("artifact name must be plan|spec|turing-loop")
	}
	if len(content) == 0 {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("artifact content is empty")
	}
	if len(content) > leasecontract.OwnerArtifactMaxBytes {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("artifact exceeds %d bytes", leasecontract.OwnerArtifactMaxBytes)
	}
	// 거부형 redaction: 스크럽은 사람이 쓴 문서를 훼손하므로 반려한다.
	if err := rejectSecretLikeContent(string(content)); err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	var record issueops.IssueOpsRecord
	// read-modify-write이므로 다른 레코드 변형과 동일하게 사이클 락 안에서
	// 수행한다 — 동시 stage가 서로를 덮어쓰지 않는다(C4a-F2).
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if !canStageIssueOpsArtifact(rec, name) {
			return &artifactStageRecoveryError{id: rec.ID}
		}
		staged, e := readStagedArtifacts(stateRoot, rec.ID)
		if e != nil {
			return e
		}
		staged[name] = string(content)
		data, e := json.Marshal(staged)
		if e != nil {
			return e
		}
		db, e := sqlstore.Open(stateRoot)
		if e != nil {
			return e
		}
		if e := db.Put(artifactStageBucket, rec.ID, data); e != nil {
			return e
		}
		record = rec
		return nil
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

func canStageIssueOpsArtifact(record issueops.IssueOpsRecord, name string) bool {
	if record.Execution == nil {
		return true
	}
	execution := record.Execution
	return name == "plan" &&
		execution.Mode == issueops.ExecutionModeOrca &&
		execution.Lease.Status == issueops.LeaseStatusReleased &&
		execution.Lease.Holder == nil &&
		execution.Pending == nil &&
		execution.Completion == nil
}

// UnstageIssueOpsArtifact는 스테이징된 artifact 하나를 되돌린다. prepare
// 이전에만 의미가 있으며, 없는 이름의 unstage는 no-op 성공이다(C4a-F4).
func UnstageIssueOpsArtifact(stateRoot, id, name string) (issueops.IssueOpsRecord, error) {
	name = strings.TrimSpace(name)
	if !issueOpsArtifactNames[name] {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("artifact name must be plan|spec|turing-loop")
	}
	var record issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, e := ReadIssueOps(stateRoot, id)
		if e != nil {
			return e
		}
		if rec.Execution != nil {
			return fmt.Errorf("staged artifacts are sealed after execution prepare and cannot be unstaged")
		}
		staged, e := readStagedArtifacts(stateRoot, rec.ID)
		if e != nil {
			return e
		}
		delete(staged, name)
		db, e := sqlstore.Open(stateRoot)
		if e != nil {
			return e
		}
		if len(staged) == 0 {
			if e := db.Delete(artifactStageBucket, rec.ID); e != nil {
				return e
			}
		} else {
			data, e := json.Marshal(staged)
			if e != nil {
				return e
			}
			if e := db.Put(artifactStageBucket, rec.ID, data); e != nil {
				return e
			}
		}
		record = rec
		return nil
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return record, nil
}

// StagedIssueOpsArtifactNames는 스테이징된 artifact 이름을 정렬해 돌려준다.
func StagedIssueOpsArtifactNames(stateRoot, id string) ([]string, error) {
	staged, err := readStagedArtifacts(stateRoot, id)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(staged))
	for name := range staged {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
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
func rejectSecretLikeContent(content string) error {
	if containsSecretPattern(content) {
		return fmt.Errorf("artifact content contains secret-like values; redact them before staging")
	}
	return nil
}
