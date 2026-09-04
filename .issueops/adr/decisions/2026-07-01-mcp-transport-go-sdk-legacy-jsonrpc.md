# 2026-07-01 — MCP transport: adopt modelcontextprotocol/go-sdk with a retained legacy JSON-RPC path

← [ADR index](../../ADR.md)

- Kind: `adr`
- Source: issueops doc reconcile (BC5)
- Summary: The MCP transport stack is settled. `github.com/modelcontextprotocol/go-sdk` v1.6.1 is the confirmed SDK for daemon-socket MCP; a legacy hand-rolled JSON-RPC path is retained for the split reader/writer stdio smoke that the SDK transport cannot cover.
- Context: TECH_STACK §3 previously listed the MCP layer as an open candidate — "안정적인 Go MCP SDK 또는 직접 JSON-RPC 최소 구현 (SDK 선택 전 schema 안정성 확인)". The harness has since shipped both: `go.mod` pins `github.com/modelcontextprotocol/go-sdk` v1.6.1 (used by `cmd/issueops/mcpcli/mcp_sdk_server.go`), and PROJECT_AUDIT item M1 records the dual-transport reality where `serveMCPStreamLegacy` is load-bearing for the stdio smoke path.
- Decision: Adopt `github.com/modelcontextprotocol/go-sdk` v1.6.1 as the confirmed MCP SDK for the daemon socket transport, and keep the legacy JSON-RPC stream (`serveMCPStreamLegacy`) intentionally for the split reader/writer stdio smoke. Both transports are kept by design (M1 accepted), not as a cleanup target.
- Consequences: TECH_STACK lists go-sdk as a confirmed dependency rather than a candidate. A go-sdk version bump must re-run the MCP tool-catalog and response-contract goldens (`cmd/issueops/testdata/mcp_tools.golden.json`, `cmd/issueops/testdata/response_contracts.golden.json`). Removing the legacy path would drop the stdio-smoke coverage, so it stays until an SDK equivalent exists.
