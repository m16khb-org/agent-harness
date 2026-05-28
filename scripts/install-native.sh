#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN="$ROOT/bin/harness"

if [[ ! -x "$BIN" ]]; then
  (cd "$ROOT" && go build -o bin/harness ./cmd/harness)
fi

"$BIN" install-native "$@"

if command -v claude >/dev/null 2>&1; then
  claude mcp remove agent-harness -s user >/dev/null 2>&1 || true
  claude mcp add-json -s user agent_harness "$(python3 - "$BIN" "$ROOT" <<'PY'
import json
import sys
bin_path, root = sys.argv[1], sys.argv[2]
print(json.dumps({
  "type": "stdio",
  "command": bin_path,
  "args": ["mcp"],
  "env": {"HARNESS_ROOT": root},
}))
PY
)" >/dev/null || true
fi
