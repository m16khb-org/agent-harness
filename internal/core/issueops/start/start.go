package start

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
)

type Store struct {
	Read           func(stateRoot, id string) (model.IssueOpsRecord, error)
	Write          func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	NewID          func(repo, branch string) string
	ValidateBranch func(branch string) error
}

func Start(store Store, stateRoot string, req model.IssueOpsStartRequest) (model.IssueOpsRecord, error) {
	rawRepo := strings.TrimSpace(req.Repo)
	if rawRepo == "" {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("repo is required")
	}
	repo := rawRepo
	if absRepo, err := filepath.Abs(rawRepo); err == nil {
		repo = absRepo
	}
	branch := strings.TrimSpace(req.Branch)
	if err := store.ValidateBranch(branch); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	id := store.NewID(repo, branch)
	if existing, err := store.Read(stateRoot, id); err == nil {
		return existing, nil
	}
	if legacyID := store.NewID(rawRepo, branch); legacyID != id {
		if existing, err := store.Read(stateRoot, legacyID); err == nil {
			return existing, nil
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := model.IssueOpsRecord{
		OK:        true,
		ID:        id,
		Repo:      repo,
		Branch:    branch,
		Phase:     model.IssueOpsPhaseProblem,
		Feedback:  []model.IssueOpsFeedbackItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	return store.Write(stateRoot, record)
}
