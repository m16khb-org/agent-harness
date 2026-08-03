package issueopslease

import (
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/port"
)

func TestValidateReseedRuntimeRolloverAcceptsSettledTaskWithStaleDispatch(t *testing.T) {
	record := leasecontract.Record{Execution: &leasecontract.Execution{
		Mode:  "orca",
		Lease: leasecontract.Lease{Generation: 2, Status: "claimable"},
		Orca:  &leasecontract.OrcaBinding{RuntimeID: "runtime-sealed"},
	}}
	inventory := port.ExecutionOrcaOwnerInventory{
		RuntimeID: "runtime-current", TaskStatus: "failed", DispatchStatus: "dispatched",
	}

	if err := validateReseedRuntimeRollover(record, inventory); err != nil {
		t.Fatalf("settled task with a stale dispatched row must not deadlock reseed: %v", err)
	}
}

func TestValidateReseedRuntimeRolloverRejectsUnsafeStaleDispatchStates(t *testing.T) {
	base := leasecontract.Record{Execution: &leasecontract.Execution{
		Mode:  "orca",
		Lease: leasecontract.Lease{Generation: 2, Status: "claimable"},
		Orca:  &leasecontract.OrcaBinding{RuntimeID: "runtime-sealed"},
	}}
	tests := []struct {
		name       string
		editRecord func(*leasecontract.Record)
		inventory  port.ExecutionOrcaOwnerInventory
	}{
		{name: "live task", inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current", TaskLive: true, TaskStatus: "failed", DispatchStatus: "dispatched"}},
		{name: "nonterminal task", inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current", TaskStatus: "dispatched", DispatchStatus: "dispatched"}},
		{name: "ghost terminal", inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current", TerminalID: "pty-old", TaskStatus: "failed", DispatchStatus: "dispatched"}},
		{name: "live terminal", inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current", TerminalLive: true, TaskStatus: "failed", DispatchStatus: "dispatched"}},
		{name: "same runtime nonterminal", inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-sealed", TaskStatus: "dispatched", DispatchStatus: "dispatched"}},
		{name: "active holder", editRecord: func(record *leasecontract.Record) {
			record.Execution.Lease.Status = "active"
			record.Execution.Lease.Holder = &leasecontract.Actor{Host: "codex", SessionID: "owner"}
		}, inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-current", TaskStatus: "failed", DispatchStatus: "dispatched"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := base
			execution := *base.Execution
			record.Execution = &execution
			if test.editRecord != nil {
				test.editRecord(&record)
			}
			if err := validateReseedRuntimeRollover(record, test.inventory); err == nil {
				t.Fatal("unsafe stale-dispatch inventory was accepted")
			}
		})
	}
}
