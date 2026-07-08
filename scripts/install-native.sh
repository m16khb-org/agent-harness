#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN="$ROOT/bin/agent-harness"
SKIP_BUILD="${HARNESS_SKIP_BUILD:-0}"
HARNESS_ARGS=()

usage() {
  cat <<'EOF'
Usage: scripts/install-native.sh [agent-harness install-native flags]

Build and install agent-harness native Codex/Claude integrations.

Harness flags are passed to `agent-harness install-native`, for example:
  --project-local
  --dry-run
  --json

Harness binary:
  --skip-build            Do not rebuild bin/agent-harness before installing integrations.

User command:
  PATH setup is handled by `agent-harness install --path-mode=auto|manual|skip`.
  The default auto mode creates ~/.local/bin/agent-harness and adds ~/.local/bin
  to the detected shell rc when it is not already on PATH.

Environment:
  HARNESS_SKIP_BUILD=1              Same as --skip-build.
EOF
}

is_truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

log() {
  printf '[agent-harness] %s\n' "$*" >&2
}

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --help|-h)
      usage
      exit 0
      ;;
    --skip-build)
      SKIP_BUILD=1
      ;;
    --dry-run)
      DRY_RUN=1
      HARNESS_ARGS+=("$arg")
      ;;
    *)
      HARNESS_ARGS+=("$arg")
      ;;
  esac
done

if [[ "$DRY_RUN" == "1" ]]; then
  if is_truthy "$SKIP_BUILD"; then
    log "dry-run: would leave existing agent-harness binary unchanged"
  elif [[ -x "$BIN" ]]; then
    log "dry-run: would update agent-harness binary from current checkout"
  else
    log "dry-run: would build agent-harness binary"
  fi
elif is_truthy "$SKIP_BUILD"; then
  log "skip-build: leaving existing harness binary unchanged"
elif [[ -x "$BIN" ]]; then
  log "updating agent-harness binary from current checkout"
  (cd "$ROOT" && go build -o bin/agent-harness ./cmd/harness)
else
  log "building agent-harness binary"
  (cd "$ROOT" && go build -o bin/agent-harness ./cmd/harness)
fi

if [[ -x "$BIN" ]]; then
  if ((${#HARNESS_ARGS[@]})); then
    "$BIN" install-native "${HARNESS_ARGS[@]}"
  else
    "$BIN" install-native
  fi
elif [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run: binary missing; skipping install-native plan because ${BIN} does not exist yet"
else
  log "agent-harness binary missing after build: ${BIN}"
  exit 1
fi

if command -v claude >/dev/null 2>&1 && [[ "$DRY_RUN" != "1" ]]; then
  log "refreshing Claude user-scope MCP server agent_harness"
  claude mcp remove agent_harness -s user >/dev/null 2>&1 || true
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
)" >/dev/null 2>&1 || true
elif [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run: would refresh Claude user-scope MCP server agent_harness"
fi

# --- GJC host integration: plugin bundle (MCP+launcher), first-party lifecycle hook,
# and filesystem skill discovery. The plugin manifest only carries `mcps` (GJC
# forbids `skills` in bundles and plugin hooks are constrained to one declared
# event, which does not fit agent-harness's multi-event hook), so skills and
# hooks are wired via GJC's first-party discovery surfaces here.
if command -v gjc >/dev/null 2>&1 && [[ "$DRY_RUN" != "1" ]]; then
  log "refreshing GJC plugin agent-harness (user scope)"
  gjc plugin install --user --force "$ROOT" >/dev/null 2>&1 || true

  log "syncing agent-harness lifecycle hook to ~/.gjc/agent/hooks/"
  mkdir -p "$HOME/.gjc/agent/hooks"
  cp "$ROOT/gjc-plugin/hook.ts" "$HOME/.gjc/agent/hooks/agent-harness.ts"

  log "ensuring GJC filesystem skill discovery points at agent-harness skills/"
  gjc config set skills.enabled true >/dev/null 2>&1 || true
  gjc config set skills.customDirectories "[\"$ROOT/skills\"]" >/dev/null 2>&1 || true
elif [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run: would refresh GJC plugin, lifecycle hook, and skill discovery"
fi

# --- Optional glab MCP sync across hosts (no-op when glab-mcp-wrapper absent).
# Keeps glab-api-servers / glab-cloud-platform consistent on Codex, Claude Code,
# and GJC for users who already have a glab-mcp-wrapper + GitLab token set up.
if [[ -x "${HOME}/.local/bin/glab-mcp-wrapper" || -n "${GLAB_MCP_WRAPPER:-}" ]]; then
  if [[ "$DRY_RUN" == "1" ]]; then
    log "dry-run: would sync glab MCP servers across hosts (scripts/sync-glab-mcp.sh)"
  else
    log "syncing glab MCP servers across hosts (best-effort)"
    bash "$ROOT/scripts/sync-glab-mcp.sh" || true
  fi
fi
