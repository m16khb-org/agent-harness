package port

import "context"

type OrcaRunInventory struct {
	RuntimeID string
	Runs      []OrcaRun
}

type OrcaRunInventoryReader interface {
	ListRunInventory(context.Context) (OrcaRunInventory, error)
	ListAllTasksFromRuns(context.Context, OrcaRunInventory) ([]OrcaTask, error)
	ListDispatchedTasksFromRuns(context.Context, OrcaRunInventory) ([]OrcaTask, error)
	ListGatesFromRuns(context.Context, OrcaRunInventory) ([]OrcaGate, error)
}
