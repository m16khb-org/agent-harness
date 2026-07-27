# Files

- [CLI and MCP Surface](cli-and-mcp.md) - CLI command tree, MCP server architecture with stdio proxy and daemon backend, tool-to-usecase dispatch mapping, and the contract golden testing model.
- [Policy, Guard, and Testing](policy-guard-testing.md) - Command execution policy catalog with tier-based classification, code anti-pattern guard with block/warn/review severities, and the testing conventions including golden contracts and cross-host tool conformance.
- [State and Storage](state-and-storage.md) - SQLite-backed state store with namespace isolation, dual-database locking via BEGIN IMMEDIATE, schema versioning, install/bootstrap flow, and host adapter pattern for Codex and Claude Code.
