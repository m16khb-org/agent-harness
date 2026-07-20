package issueops

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/linking"
	"agent-harness/internal/core/preflight"
	"context"
)

type coordinatorPlanCheckpoint struct {
	record IssueOpsRecord
	head   string
	active bool
}

func linkIssueOpsPlanWithCoordinatorCheckpoint(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return linkIssueOpsPlanWithCoordinatorCheckpointActor(stateRoot, id, planPath, nil)
}

func LinkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsPlanWithCoordinatorCheckpointActor(stateRoot, id, planPath, &actor)
}

func linkIssueOpsPlanWithCoordinatorCheckpointActor(stateRoot, id, planPath string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	validated, err := validateCoordinatorPlanCheckpoint(stateRoot, id, planPath)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	var persisted IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		store := issueOpsLinkingStore()
		read := store.Read
		write := store.TouchWrite
		store.Read = func(root, recordID string) (IssueOpsRecord, error) {
			current, readErr := read(root, recordID)
			if readErr != nil {
				return current, readErr
			}
			if validated.active && !reflect.DeepEqual(current, validated.record) {
				return IssueOpsRecord{}, fmt.Errorf("stale coordinator plan checkpoint")
			}
			if current.ExecutionWorkspace != nil {
				if actor == nil {
					return IssueOpsRecord{}, fmt.Errorf("workspace preparation requires a native actor; use the actor-aware plan linker")
				}
				if actorErr := validateReadyWorkspacePreparationActor(current, *actor); actorErr != nil {
					return IssueOpsRecord{}, actorErr
				}
			}
			return current, nil
		}
		store.TouchWrite = func(root string, record IssueOpsRecord) (IssueOpsRecord, error) {
			if validated.active {
				record.ExecutionHandoff.AttemptBaseHead = validated.head
			}
			return write(root, record)
		}
		var linkErr error
		persisted, linkErr = linking.LinkPlan(store, stateRoot, id, planPath)
		return linkErr
	})
	return persisted, err
}

func validateCoordinatorPlanCheckpoint(stateRoot, id, planPath string) (coordinatorPlanCheckpoint, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return coordinatorPlanCheckpoint{}, err
	}
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateCoordinatorPreparing || strings.TrimSpace(h.ContextSHA256) != "" || h.PendingOperation != nil {
		return coordinatorPlanCheckpoint{record: record}, nil
	}
	workerRoot := filepath.Clean(strings.TrimSpace(h.WorkerRoot))
	if workerRoot == "." || workerRoot != filepath.Clean(strings.TrimSpace(record.WorktreePath)) {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("coordinator plan commit requires the exact worker root")
	}
	candidate := strings.TrimSpace(planPath)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workerRoot, candidate)
	}
	branch := strings.TrimSpace(preflight.GitOut(workerRoot, "branch", "--show-current"))
	if branch == "" || branch != strings.TrimSpace(record.Branch) {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("coordinator plan commit requires the exact handoff branch")
	}
	code, head, _ := preflight.GitCmd(workerRoot, "rev-parse", "--verify", "HEAD^{commit}")
	head = strings.TrimSpace(head)
	if code != 0 || head == "" {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("coordinator plan commit requires a readable HEAD")
	}
	code, status, _ := preflight.GitCmd(workerRoot, "status", "--porcelain=v1")
	if code != 0 || strings.TrimSpace(status) != "" {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("link-plan requires a clean coordinator plan commit")
	}
	base := strings.TrimSpace(h.AttemptBaseHead)
	if base == "" {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("coordinator plan commit requires the persisted attempt base head")
	}
	if code, _, _ := preflight.GitCmd(workerRoot, "merge-base", "--is-ancestor", base, head); code != 0 {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("coordinator plan commit must descend from the current attempt base head")
	}
	if head != base {
		if err := requireCoordinatorPlanOnlyDiff(workerRoot, base, head, candidate); err != nil {
			return coordinatorPlanCheckpoint{}, err
		}
	} else if linked := strings.TrimSpace(record.PlanPath); linked != "" && filepath.Clean(linked) != filepath.Clean(candidate) {
		return coordinatorPlanCheckpoint{}, fmt.Errorf("replacing a linked plan requires a coordinator plan commit")
	}
	return coordinatorPlanCheckpoint{record: record, head: head, active: true}, nil
}

func requireCoordinatorPlanOnlyDiff(workerRoot, base, head, planPath string) error {
	relative, err := filepath.Rel(workerRoot, filepath.Clean(planPath))
	if err != nil || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(filepath.ToSlash(relative), "../") {
		return fmt.Errorf("coordinator plan commit must target a plan inside the worker root")
	}
	code, output, _ := preflight.GitCmdRaw(workerRoot, "diff", "--name-only", "-z", base+".."+head)
	if code != 0 {
		return fmt.Errorf("read coordinator plan commit diff")
	}
	actual := canonicalGitPathSet(splitNULTerminatedPaths(output))
	expected, err := canonicalChangedFileSet([]string{filepath.ToSlash(relative)})
	if err != nil || !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("coordinator plan commit must contain only the current cycle plan")
	}
	return nil
}
