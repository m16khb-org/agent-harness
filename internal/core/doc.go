// Package core is the harness domain layer and the single stable public surface
// that the cmd/ command and MCP layers depend on. Callers import core, not core's
// internal subpackages (core/issueops, core/lifecycle, core/worker, ...), so the
// internal package layout can change without touching every caller.
//
// # Facade convention (*_facade.go)
//
// The *_facade.go files (issueops, workflow, utility, policy, project_doc,
// state_trace, draft_wiki, issueops_remote) are that public surface. They are
// intentionally thin and may contain ONLY:
//
//  1. Type aliases that re-export a subpackage type as part of the public
//     surface, e.g. `type WorkerJob = worker.WorkerJob`.
//  2. Type conversions across the boundary, e.g. converting a core-aliased
//     request into the subpackage's own type before delegating.
//  3. Composition that joins more than one subpackage into a single result
//     (e.g. SummarizeHookFailureStats joins the failure log with hook metrics).
//  4. Boundary enforcement — adapting or guarding a call so the domain boundary
//     holds (e.g. resolving a port.IssueProvider supplied by the caller rather
//     than importing a concrete adapter).
//
// Pure one-line delegation (`func F(x) T { return sub.F(x) }`) is allowed and
// expected: it is what keeps the public surface stable and decoupled from the
// internal package layout, not accidental barrel overhead. A facade audit found
// every exported facade symbol is used by a real caller, so the surface is lean
// rather than bloated; do not "flatten" delegates into direct subpackage imports
// from cmd/, which would couple cmd/ to core's internal structure.
//
// cmd/harness may import a core subpackage directly only for cmd-local tooling,
// diagnostics, or tests whose subject is that subpackage-specific mechanism
// rather than the stable harness domain API. State and IssueOps lifecycle access
// stay behind the core facade because those records are cross-command contracts.
// Do not add a facade wrapper just to hide an internal-only inspection helper;
// either keep the direct import local to the tool or promote the behavior when a
// real external caller needs it.
//
// What does NOT belong in a facade: new domain logic. If a facade function grows
// beyond conversion/composition/enforcement, move the logic into the owning
// subpackage and keep the facade thin.
package core
