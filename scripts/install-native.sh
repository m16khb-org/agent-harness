#!/usr/bin/env bash
set -euo pipefail

BUILD_ROOT="${ISSUEOPS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
ROOT="$BUILD_ROOT"
if COMMON_GIT_DIR="$(git -C "$BUILD_ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)"; then
  if [[ "$(basename "$COMMON_GIT_DIR")" == ".git" && -d "$COMMON_GIT_DIR" && ! -L "$COMMON_GIT_DIR" ]]; then
    ROOT="$(cd "$COMMON_GIT_DIR/.." && pwd)"
  fi
fi
export ISSUEOPS_ROOT="$ROOT"
BIN="$ROOT/bin/issueops"
SKIP_BUILD="${ISSUEOPS_SKIP_BUILD:-0}"
ISSUEOPS_ARGS=()
STAGED_BIN=""
ACTIVATION_BEGUN=0
ACTIVATION_TRANSITION_ID=""
ACTIVATION_BINARY_SHA256=""
ACTIVATION_ABORTED=0
ACTIVATION_COMMITTED=0
ACTIVATION_ARGS=()

usage() {
  cat <<'EOF'
Usage: scripts/install-native.sh [issueops install flags]

Build and install issueops native Codex/Claude integrations.

Harness flags are passed to `issueops install`, for example:
  --project-local
  --dry-run
  --json
  --adopt-command-file   Explicitly adopt a managed regular command file.

Harness binary:
  --skip-build            Do not rebuild bin/issueops before installing integrations.

User command:
  PATH setup is handled by `issueops install --path-mode=auto|manual|skip`.
  The default auto mode creates ~/.local/bin/issueops plus the safe
  ~/.local/bin/io shorthand, and adds ~/.local/bin to the detected shell rc
  when it is not already on PATH.

Environment:
  ISSUEOPS_SKIP_BUILD=1              Same as --skip-build.
EOF
}

is_truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

log() {
  printf '[issueops] %s\n' "$*" >&2
}

file_sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

cleanup_install() {
  local original_status=$?
  trap - EXIT
  if [[ "$ACTIVATION_BEGUN" == "1" && "$ACTIVATION_COMMITTED" != "1" && "$ACTIVATION_ABORTED" != "1" ]]; then
    ACTIVATION_ABORTED=1
    local abort_bin=""
    if [[ -n "$STAGED_BIN" && -x "$STAGED_BIN" && "$(file_sha256 "$STAGED_BIN")" == "$ACTIVATION_BINARY_SHA256" ]]; then
      abort_bin="$STAGED_BIN"
    elif [[ -x "$BIN" && "$(file_sha256 "$BIN")" == "$ACTIVATION_BINARY_SHA256" ]]; then
      abort_bin="$BIN"
    fi
    if [[ -n "$abort_bin" ]]; then
      ISSUEOPS_NATIVE_ACTIVATION_STEP=abort \
        ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID="$ACTIVATION_TRANSITION_ID" \
        "$abort_bin" install --path-mode=skip --json >/dev/null ||
        log "native activation abort failed; transition remains pending for recovery"
    else
      log "native activation abort skipped: no exact candidate binary remains; transition remains pending for recovery"
    fi
  fi
  if [[ -n "$STAGED_BIN" && -e "$STAGED_BIN" ]]; then
    rm -f -- "$STAGED_BIN"
  fi
  exit "$original_status"
}

trap cleanup_install EXIT

begin_activation() {
  local executable="$1"
  local receipt
  local begin_status=0
  receipt="$(mktemp "$ROOT/bin/.issueops.begin-XXXXXX")"
  if ((${#ACTIVATION_ARGS[@]})); then
    ISSUEOPS_NATIVE_ACTIVATION_STEP=begin \
      "$executable" install --path-mode=skip --json "${ACTIVATION_ARGS[@]}" >"$receipt" || begin_status=$?
  else
    ISSUEOPS_NATIVE_ACTIVATION_STEP=begin \
      "$executable" install --path-mode=skip --json >"$receipt" || begin_status=$?
  fi
  if ((begin_status != 0)); then
    rm -f -- "$receipt"
    return 1
  fi
  IFS=$'\t' read -r ACTIVATION_TRANSITION_ID ACTIVATION_BINARY_SHA256 < <(
    python3 - "$receipt" <<'PY'
import json
import re
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    value = json.load(stream)
transition = value.get("transition_id", "")
digest = value.get("binary_sha256", "")
if not re.fullmatch(r"[0-9a-f]{32}", transition) or not re.fullmatch(r"[0-9a-f]{64}", digest):
    raise SystemExit("invalid native activation begin receipt")
print(f"{transition}\t{digest}")
PY
  )
  rm -f -- "$receipt"
  [[ -n "$ACTIVATION_TRANSITION_ID" && -n "$ACTIVATION_BINARY_SHA256" ]]
  ACTIVATION_BEGUN=1
}

preflight_install() {
  local executable="$1"
  if ((${#ISSUEOPS_ARGS[@]})); then
    "$executable" install "${ISSUEOPS_ARGS[@]}" --dry-run >/dev/null
  else
    "$executable" install --dry-run >/dev/null
  fi
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
      ISSUEOPS_ARGS+=("$arg")
      ;;
    --adopt-command-file)
      ISSUEOPS_ARGS+=("$arg")
      ACTIVATION_ARGS+=("$arg")
      ;;
    *)
      ISSUEOPS_ARGS+=("$arg")
      ;;
  esac
done

if [[ "$DRY_RUN" == "1" ]]; then
  if is_truthy "$SKIP_BUILD"; then
    log "dry-run: would leave existing issueops binary unchanged"
  elif [[ -x "$BIN" ]]; then
    log "dry-run: would update issueops binary from current checkout"
  else
    log "dry-run: would build issueops binary"
  fi
elif is_truthy "$SKIP_BUILD"; then
  log "skip-build: leaving existing harness binary unchanged"
else
  mkdir -p "$ROOT/bin"
  STAGED_BIN="$(mktemp "$ROOT/bin/.issueops.activate-XXXXXX")"
  if [[ -x "$BIN" ]]; then
    log "staging issueops binary update from current checkout"
  else
    log "staging initial issueops binary from current checkout"
  fi
  (cd "$BUILD_ROOT" && go build -o "$STAGED_BIN" ./cmd/issueops)
  chmod 0755 "$STAGED_BIN"
  "$STAGED_BIN" version >/dev/null
  preflight_install "$STAGED_BIN"
  begin_activation "$STAGED_BIN"
  python3 - "$STAGED_BIN" "$BIN" <<'PY'
import os
import sys

source, target = sys.argv[1], sys.argv[2]
staged = os.open(source, os.O_RDONLY)
try:
    os.fsync(staged)
finally:
    os.close(staged)
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
    preflight_install "$BIN"
    begin_activation "$BIN"
  fi
  if [[ "$DRY_RUN" == "1" ]]; then
    if ((${#ISSUEOPS_ARGS[@]})); then
      "$BIN" install "${ISSUEOPS_ARGS[@]}"
    else
      "$BIN" install
    fi
  else
    SEAL_RECEIPT="$(mktemp "$ROOT/bin/.issueops.seal-XXXXXX")"
    set +e
    if ((${#ISSUEOPS_ARGS[@]})); then
      ISSUEOPS_NATIVE_ACTIVATION_STEP=seal \
        ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID="$ACTIVATION_TRANSITION_ID" \
        "$BIN" install "${ISSUEOPS_ARGS[@]}" --json >"$SEAL_RECEIPT"
    else
      ISSUEOPS_NATIVE_ACTIVATION_STEP=seal \
        ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID="$ACTIVATION_TRANSITION_ID" \
        "$BIN" install --json >"$SEAL_RECEIPT"
    fi
    SEAL_STATUS=$?
    set -e
    cat "$SEAL_RECEIPT"
    if [[ "$SEAL_STATUS" == "0" ]] && python3 - "$SEAL_RECEIPT" "$ACTIVATION_TRANSITION_ID" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as stream:
    value = json.load(stream)
raise SystemExit(0 if value.get("committed") is True and value.get("transition_id") == sys.argv[2] else 1)
PY
    then
      ACTIVATION_COMMITTED=1
      rm -f -- "$SEAL_RECEIPT"
    else
      rm -f -- "$SEAL_RECEIPT"
      if [[ "$SEAL_STATUS" == "0" ]]; then
        exit 1
      fi
      exit "$SEAL_STATUS"
    fi
  fi
elif [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run: binary missing; skipping install plan because ${BIN} does not exist yet"
else
  log "issueops binary missing after build: ${BIN}"
  exit 1
fi
