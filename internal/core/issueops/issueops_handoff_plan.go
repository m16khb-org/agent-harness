package issueops

import (
	"context"
	"fmt"

	"agent-harness/internal/core/issueops/linking"
)

func linkIssueOpsPlanWithCoordinatorCheckpoint(stateRoot, id, planPath string) (IssueOpsRecord, error) {
	return linkIssueOpsPlanWithActor(stateRoot, id, planPath, nil)
}

func LinkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor IssueOpsActor) (IssueOpsRecord, error) {
	return linkIssueOpsPlanWithActor(stateRoot, id, planPath, &actor)
}

func linkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor *IssueOpsActor) (IssueOpsRecord, error) {
	var persisted IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		store := issueOpsLinkingStore()
		read := store.Read
		store.Read = func(root, recordID string) (IssueOpsRecord, error) {
			current, readErr := read(root, recordID)
			if readErr != nil {
				return current, readErr
			}
			if currentIssueOpsWorkspace(current) != nil {
				if actor == nil {
					return IssueOpsRecord{}, fmt.Errorf("workspace preparation requires a native actor; use the actor-aware plan linker")
				}
				if actorErr := validateReadyWorkspacePreparationActor(current, *actor); actorErr != nil {
					return IssueOpsRecord{}, actorErr
				}
			}
			return current, nil
		}
		var linkErr error
		persisted, linkErr = linking.LinkPlan(store, stateRoot, id, planPath)
		return linkErr
	})
	return persisted, err
}
