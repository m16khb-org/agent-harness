#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$ROOT/bin"
BIN="$BIN_DIR/harness"
LLM_WIKI_ROOT="${LLM_WIKI_ROOT:-$HOME/workspace/knowledge-base/llm-wiki}"

mkdir -p "$BIN_DIR"
go build -o "$BIN" ./cmd/harness
"$BIN" daemon stop --json >/dev/null 2>&1 || true
"$BIN" install-native --llm-wiki-root "$LLM_WIKI_ROOT" "$@"

if command -v claude >/dev/null 2>&1; then
  claude_mcp_json="$(python3 - "$BIN" "$ROOT" "$LLM_WIKI_ROOT" <<'PY'
import json
import pathlib
import sys
bin_path = pathlib.Path(sys.argv[1]).resolve()
root = pathlib.Path(sys.argv[2]).resolve()
wiki = pathlib.Path(sys.argv[3]).expanduser().resolve()
print(json.dumps({
    "type": "stdio",
    "command": str(bin_path),
    "args": ["mcp"],
    "env": {"HARNESS_ROOT": str(root), "LLM_WIKI_ROOT": str(wiki)},
}))
PY
)"
  claude mcp remove agent-harness -s user >/dev/null 2>&1 || true
  if claude mcp add-json -s user agent-harness "$claude_mcp_json" >/dev/null 2>&1; then
    echo "- Claude user MCP config: claude mcp add-json -s user agent-harness"
  else
    echo "Warning: failed to add Claude user MCP server; run manually if needed:" >&2
    echo "  claude mcp add-json -s user agent-harness '$claude_mcp_json'" >&2
  fi
else
  echo "Warning: claude command not found; skipped Claude user MCP registration" >&2
fi
