package issueops

import (
	"fmt"
	"strings"
)

type BaseSyncRequiredError struct {
	ID                   string
	CompletionGeneration uint64
	NextCommand          string
}

func NewBaseSyncRequiredError(id string, completionGeneration uint64) *BaseSyncRequiredError {
	quotedID := "'" + strings.ReplaceAll(id, "'", "'\\''") + "'"
	return &BaseSyncRequiredError{
		ID:                   id,
		CompletionGeneration: completionGeneration,
		NextCommand: fmt.Sprintf(
			"issueops execution sync-base --id %s --preview --completion-generation %d --json",
			quotedID,
			completionGeneration,
		),
	}
}

func (e *BaseSyncRequiredError) Error() string {
	return fmt.Sprintf("post-completion sync-base is required for %s at completion generation %d", e.ID, e.CompletionGeneration)
}

func (e *BaseSyncRequiredError) IssueOpsErrorFields() map[string]any {
	return map[string]any{
		"code":                  "post_completion_sync_base_required",
		"completion_generation": e.CompletionGeneration,
		"next_command":          e.NextCommand,
	}
}
