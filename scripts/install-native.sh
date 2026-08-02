#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
export HARNESS_ROOT="$ROOT"
BIN="$ROOT/bin/agent-harness"
SKIP_BUILD="${HARNESS_SKIP_BUILD:-0}"
HARNESS_ARGS=()
STAGED_BIN=""
ACTIVATION_BEGUN=0

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
  The default auto mode creates ~/.local/bin/agent-harness plus the safe
  ~/.local/bin/ah shorthand, and adds ~/.local/bin to the detected shell rc
  when it is not already on PATH.

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

cleanup_stage() {
  if [[ -n "$STAGED_BIN" && -e "$STAGED_BIN" ]]; then
    rm -f -- "$STAGED_BIN"
  fi
}

trap cleanup_stage EXIT

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
else
  mkdir -p "$ROOT/bin"
  STAGED_BIN="$(mktemp "$ROOT/bin/.agent-harness.activate-XXXXXX")"
  if [[ -x "$BIN" ]]; then
    log "staging agent-harness binary update from current checkout"
  else
    log "staging initial agent-harness binary from current checkout"
  fi
  (cd "$ROOT" && go build -o "$STAGED_BIN" ./cmd/harness)
  chmod 0755 "$STAGED_BIN"
  "$STAGED_BIN" version >/dev/null
  HARNESS_NATIVE_ACTIVATION_STEP=begin \
    "$STAGED_BIN" install-native --path-mode=skip --json >/dev/null
  ACTIVATION_BEGUN=1
  python3 - "$STAGED_BIN" "$BIN" <<'PY'
import os
import sys

source, target = sys.argv[1], sys.argv[2]
os.replace(source, target)
directory = os.open(os.path.dirname(target), os.O_RDONLY)
try:
    os.fsync(directory)
finally:
    os.close(directory)
PY
  STAGED_BIN=""
  "$BIN" version >/dev/null
fi

if [[ -x "$BIN" ]]; then
  if [[ "$DRY_RUN" != "1" && "$ACTIVATION_BEGUN" != "1" ]]; then
    HARNESS_NATIVE_ACTIVATION_STEP=begin \
      "$BIN" install-native --path-mode=skip --json >/dev/null
  fi
  if [[ "$DRY_RUN" == "1" ]]; then
    if ((${#HARNESS_ARGS[@]})); then
      "$BIN" install-native "${HARNESS_ARGS[@]}"
    else
      "$BIN" install-native
    fi
  elif ((${#HARNESS_ARGS[@]})); then
    HARNESS_NATIVE_ACTIVATION_STEP=seal "$BIN" install-native "${HARNESS_ARGS[@]}"
  else
    HARNESS_NATIVE_ACTIVATION_STEP=seal "$BIN" install-native
  fi
elif [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run: binary missing; skipping install-native plan because ${BIN} does not exist yet"
else
  log "agent-harness binary missing after build: ${BIN}"
  exit 1
fi
