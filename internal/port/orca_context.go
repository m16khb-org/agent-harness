package port

import "context"

// OrcaRunInventory is one complete, request-scoped Run-list observation.
// RuntimeID remains populated when Runs is empty, and Runs are ordered by ID.
type OrcaRunInventory struct {
	RuntimeID string
	Runs      []OrcaRun
}

// OrcaRunInventoryReader reuses an immutable Run inventory for independent
// operational reads. Implementations preserve Run order, cap each read at
// eight concurrent Run queries, report the lowest Run-index error, and reject
// count, runtime, row-identity, or cross-Run duplicate mismatches. Dispatched
// tasks must be queried with the server-side status filter for every Run.
type OrcaRunInventoryReader interface {
	ListRunInventory(context.Context) (OrcaRunInventory, error)
	ListAllTasksFromRuns(context.Context, OrcaRunInventory) ([]OrcaTask, error)
	ListDispatchedTasksFromRuns(context.Context, OrcaRunInventory) ([]OrcaTask, error)
	ListGatesFromRuns(context.Context, OrcaRunInventory) ([]OrcaGate, error)
}
