#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN="${AGENT_HARNESS_BIN:-$ROOT/bin/agent-harness}"
SKIP_BUILD="${HARNESS_RELEASE_SKIP_BUILD:-0}"
KEEP_TMP="${HARNESS_RELEASE_KEEP_TMP:-0}"

usage() {
  cat <<'EOF'
Usage: scripts/release-repro-smoke.sh

Build and smoke-test the release install path in temporary clean-machine
directories. The smoke never writes to the operator's real HOME/CODEX_HOME.

Environment:
  AGENT_HARNESS_BIN=/path/to/agent-harness  Use an existing binary.
  HARNESS_RELEASE_SKIP_BUILD=1              Skip `go build`.
  HARNESS_RELEASE_KEEP_TMP=1                Keep temp files for inspection.
EOF
}

case "${1:-}" in
  -h|--help)
    usage
    exit 0
    ;;
  "")
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

log() {
  printf '[release-repro] %s\n' "$*" >&2
}

is_truthy() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|on|ON) return 0 ;;
    *) return 1 ;;
  esac
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'required command not found: %s\n' "$1" >&2
    exit 127
  fi
}

require_command go
require_command python3

tmp="$(mktemp -d "${TMPDIR:-/tmp}/agent-harness-release-repro.XXXXXX")"
cleanup() {
  if is_truthy "$KEEP_TMP"; then
    log "kept temp directory: $tmp"
  else
    rm -rf "$tmp"
  fi
}
trap cleanup EXIT

home="$tmp/home"
state="$tmp/state"
fixture_root="$tmp/harness-root"
mkdir -p "$home" "$state" "$fixture_root/skills/atomic-commit-push"

cat > "$fixture_root/skills/atomic-commit-push/SKILL.md" <<'EOF'
---
name: atomic-commit-push
description: Release reproducibility smoke fixture.
---

# Release Smoke Fixture

This minimal skill exists only to verify install dry-run planning in a clean
temporary HARNESS_ROOT.
EOF

if ! is_truthy "$SKIP_BUILD"; then
  log "building $BIN"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/harness)
fi

install_json="$tmp/install-native-dry-run.json"
inspect_json="$tmp/inspect.json"
docs_json="$tmp/docs.json"
state_write_json="$tmp/state-write.json"
state_read_json="$tmp/state-read.json"

log "checking project-local install dry-run in temp HOME/CODEX_HOME/HARNESS_ROOT"
HOME="$home" \
CODEX_HOME="$home/.codex" \
HARNESS_STATE_DIR="$state" \
HARNESS_ROOT="$fixture_root" \
  "$BIN" install-native --dry-run --project-local --json > "$install_json"

python3 - "$install_json" <<'PY'
import json
import sys

path = sys.argv[1]
data = json.load(open(path))
errors = []
if not data.get("ok"):
    errors.append("install-native result ok=false")
if not data.get("dry_run"):
    errors.append("install-native result dry_run=false")
if not data.get("project_local"):
    errors.append("install-native result project_local=false")
hosts = data.get("hosts", [])
names = [host.get("host") for host in hosts]
if names != ["codex", "claude"]:
    errors.append(f"unexpected host set/order: {names}")
for name, host in zip(("codex", "claude"), hosts):
    if not host or not host.get("ok") or not host.get("dry_run"):
        errors.append(f"{name} host did not report ok dry-run")
for host in data.get("hosts", []):
    for file_entry in host.get("files", []):
        if file_entry.get("written"):
            errors.append(f"dry-run wrote file: {file_entry.get('path')}")
    for link_entry in host.get("links", []):
        if link_entry.get("created"):
            errors.append(f"dry-run created link: {link_entry.get('path')}")
if "atomic-commit-push" not in data.get("skill_names", []):
    errors.append("fixture skill missing from install plan")
if errors:
    raise SystemExit("; ".join(errors))
PY

log "checking inspect/docs/state workflow under temp HOME/CODEX_HOME"
HOME="$home" CODEX_HOME="$home/.codex" HARNESS_STATE_DIR="$state" \
  "$BIN" inspect --json > "$inspect_json"
HOME="$home" CODEX_HOME="$home/.codex" HARNESS_STATE_DIR="$state" \
  "$BIN" docs --json > "$docs_json"
HOME="$home" CODEX_HOME="$home/.codex" HARNESS_STATE_DIR="$state" \
  "$BIN" state write --key release-repro-smoke --value current-v1 --json > "$state_write_json"
HOME="$home" CODEX_HOME="$home/.codex" HARNESS_STATE_DIR="$state" \
  "$BIN" state read --key release-repro-smoke --json > "$state_read_json"

python3 - "$inspect_json" "$docs_json" "$state_write_json" "$state_read_json" <<'PY'
import json
import sys

for path in sys.argv[1:]:
    data = json.load(open(path))
    if not data.get("ok"):
        raise SystemExit(f"{path} reported ok=false")

state_read = json.load(open(sys.argv[4]))
record = state_read.get("record", {})
if record.get("schema_version") != 1 or record.get("key") != "release-repro-smoke" or record.get("content") != "current-v1":
    raise SystemExit(f"state read did not return the exact current-v1 record: {record}")
PY

log "release reproducibility smoke passed"
if is_truthy "$KEEP_TMP"; then
  printf 'ok=true temp_dir=%s\n' "$tmp"
else
  printf 'ok=true\n'
fi
