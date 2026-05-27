#!/usr/bin/env bash
# Generic plain-text session-start hook adapter for Codex-compatible hook runners.
# It intentionally emits plain context instead of Claude's hookSpecificOutput JSON.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
export HARNESS_ROOT="$ROOT"
export HARNESS_SESSION_CONTEXT_MODE=plain
exec "$ROOT/scripts/session-start-llm-wiki.sh"
