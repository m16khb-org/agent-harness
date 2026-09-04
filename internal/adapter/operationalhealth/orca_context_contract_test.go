package operationalhealth

import (
	"context"
	"testing"

	"issueops/internal/port"
)

type frozenRunInventoryReader struct{}

func (frozenRunInventoryReader) ListRunInventory(context.Context) (port.OrcaRunInventory, error) {
	return port.OrcaRunInventory{}, nil
}

func (frozenRunInventoryReader) ListAllTasksFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, nil
}

func (frozenRunInventoryReader) ListDispatchedTasksFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return nil, nil
}

func (frozenRunInventoryReader) ListGatesFromRuns(context.Context, port.OrcaRunInventory) ([]port.OrcaGate, error) {
	return nil, nil
}

var _ port.OrcaRunInventoryReader = frozenRunInventoryReader{}

func TestFrozenRunInventoryContractExposesRuntimeAndRuns(t *testing.T) {
	inventory := port.OrcaRunInventory{
		RuntimeID: "runtime-1",
		Runs:      []port.OrcaRun{{RuntimeID: "runtime-1", ID: "run-1"}},
	}
	if inventory.RuntimeID != "runtime-1" || len(inventory.Runs) != 1 || inventory.Runs[0].ID != "run-1" {
		t.Fatalf("inventory = %#v", inventory)
	}
}
