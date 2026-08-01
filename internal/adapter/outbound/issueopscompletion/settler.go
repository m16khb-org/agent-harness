package issueopscompletion

import "context"

type SettleFunc func(context.Context, string, string) error

type TaskSettler struct{ settle SettleFunc }

func NewTaskSettler(settle SettleFunc) *TaskSettler { return &TaskSettler{settle: settle} }

func (s *TaskSettler) Settle(ctx context.Context, runID, taskID string) error {
	if s == nil || s.settle == nil {
		return nil
	}
	return s.settle(ctx, runID, taskID)
}
