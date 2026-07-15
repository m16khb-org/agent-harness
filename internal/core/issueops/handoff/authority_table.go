package handoff

import "slices"

// This file is the single declarative source for the *state dimension* of the
// supervised-handoff authority decision (Task A). The recurring-bug history
// showed the same state+role+command invariants re-implemented across the
// storage layer (envelope validation), the hook authority layer, and the
// execution layer; consolidating the state-gating here gives one table those
// layers consult instead of re-deriving it.
//
// Scope and layering:
//   - This table answers "in which states is lifecycle command C authorized for
//     role R?". The hook layer keeps its default-deny wrapper and layers the
//     fence (Attempt/OwnershipEpoch/ContextSHA256), native-identity, and
//     cwd/source-vs-worker predicates on top — those are NOT encoded here.
//   - state.go owns the *transition* guards (which mutation is valid from which
//     state) and the structural envelope validation; it enforces the same state
//     machine at write time. The state constants it defines are the shared
//     vocabulary for this table.
//
// Behavior is byte-identical to the prior inlined coordinatorLifecycleStateAllows
// switch; a characterization test pins the full (command × state) matrix.

// coordinatorCommandAllowedStates lists, per coordinator lifecycle command path,
// the handoff states in which the command is state-allowed. Commands with an
// extra sub-condition (handoff publish requires the accepted disposition;
// handoff start is an always-allowed read-only projection) are handled
// explicitly in CoordinatorCommandStateAllows. Any path absent from this map
// falls back to the coordinator_preparing-only default (link-plan,
// compatibility review, execution decide, devils-advocate review, worktree
// prepare, worktree prepare-tools, and phase).
var coordinatorCommandAllowedStates = map[string][]string{
	"phase":           {StateCoordinatorPreparing},
	"handoff accept":  {StateSubmitted, StateClosed},
	"handoff recover": {StateRecoveryRequired, StateSubmitted, StateClosed, StateClaimed},
}

// CoordinatorCommandStateAllows reports whether a coordinator lifecycle command
// path is state-allowed given the current handoff state and closed disposition.
// It is the declarative source consulted by the hook layer's
// coordinatorLifecycleStateAllows; the hook layer wraps it with default-deny and
// the fence/identity/cwd predicates.
func CoordinatorCommandStateAllows(path, state, closedDisposition string) bool {
	switch path {
	case "handoff publish":
		return state == StateClosed && closedDisposition == DispositionAccepted
	case "handoff start":
		return true // active-attempt repeats are read-only projections
	}
	if states, ok := coordinatorCommandAllowedStates[path]; ok {
		return slices.Contains(states, state)
	}
	return state == StateCoordinatorPreparing
}
