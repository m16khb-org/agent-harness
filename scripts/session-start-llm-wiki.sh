#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN="${HARNESS_BIN:-$ROOT/bin/harness}"
PROJECT="${CLAUDE_PROJECT_DIR:-${CODEX_PROJECT_DIR:-${PWD:-}}}"
MODE="${HARNESS_SESSION_CONTEXT_MODE:-claude-json}"

context="$($BIN llm-wiki session-context --project "$PROJECT")"
# Claude Code caps injected hook context at 10,000 chars; keep a small margin.
context="${context:0:9500}"

if [[ "$MODE" == "plain" ]]; then
  printf '%s\n' "$context"
  exit 0
fi

python3 - "$context" <<'PY'
import json
import sys
context = sys.argv[1]
print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": context,
    }
}, ensure_ascii=False))
PY
