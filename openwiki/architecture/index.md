# Files

- [Dependency Ratchet](dependency-ratchet.md) - The test-only fitness boundary in internal/architecture that enforces layering directions, a zero legacy-adapter baseline, capability-scoped adapter rules, ownership manifest rules, and orphan-package guards over the production import graph.
- [Architecture Overview](overview.md) - One host-neutral Go core with thin Codex/Claude Code/Omo adapters, exposed through CLI one-shot, MCP stdio proxy, daemon, and worker execution surfaces under five fixed boundary commitments.
- [Source Map](source-map.md) - Maps every major source tree of agent-harness to its responsibility, allowed dependencies, and forbidden imports, with the capability-vertical pattern worked through issueopslease and issueopspublication.
