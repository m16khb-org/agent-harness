#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=0
PROFILE="${HEADROOM_PROFILE:-init-user}"
PORT="${HEADROOM_PORT:-8787}"

usage() {
  cat <<'EOF'
Usage: scripts/setup-headroom-runtime.sh [--dry-run]

Explicitly enable Headroom runtime routing for both Codex and Claude Code while
preserving existing agent-harness hooks/settings.

This script requires an installed `headroom` CLI. It does not run `headroom learn`.
EOF
}

log() {
  printf '[agent-harness] %s\n' "$*" >&2
}

backup_file() {
  local path="$1"
  local stamp="$2"
  if [[ -f "$path" ]]; then
    cp "$path" "${path}.bak-${stamp}-before-headroom"
  fi
}

merge_codex_hooks() {
  local before="$1"
  local after="$2"
  [[ -f "$before" && -f "$after" ]] || return 0
  python3 - "$before" "$after" <<'PY'
import json
import sys
from pathlib import Path

before = Path(sys.argv[1])
after = Path(sys.argv[2])
base = json.loads(before.read_text())
current = json.loads(after.read_text())

def key(entry):
    hooks = entry.get("hooks", [])
    commands = tuple(h.get("command", "") for h in hooks if isinstance(h, dict))
    return (entry.get("matcher", ""), commands)

merged_hooks = {}
for source in (base.get("hooks", {}), current.get("hooks", {})):
    for event, entries in source.items():
        dest = merged_hooks.setdefault(event, [])
        seen = {key(e) for e in dest if isinstance(e, dict)}
        for entry in entries:
            if not isinstance(entry, dict):
                continue
            entry_key = key(entry)
            if entry_key not in seen:
                dest.append(entry)
                seen.add(entry_key)

current["hooks"] = merged_hooks
after.write_text(json.dumps(current, indent=2, ensure_ascii=False) + "\n")
PY
}

merge_claude_settings() {
  local before="$1"
  local after="$2"
  [[ -f "$before" && -f "$after" ]] || return 0
  python3 - "$before" "$after" <<'PY'
import json
import sys
from pathlib import Path

before = Path(sys.argv[1])
after = Path(sys.argv[2])
base = json.loads(before.read_text())
current = json.loads(after.read_text())

def merge_dict(name):
    merged = {}
    if isinstance(base.get(name), dict):
        merged.update(base[name])
    if isinstance(current.get(name), dict):
        merged.update(current[name])
    if merged:
        current[name] = merged

def hook_key(entry):
    hooks = entry.get("hooks", [])
    commands = tuple(h.get("command", "") for h in hooks if isinstance(h, dict))
    return (entry.get("matcher", ""), commands)

for name in ("env", "enabledPlugins", "extraKnownMarketplaces", "permissions", "statusLine"):
    merge_dict(name)

for key, value in base.items():
    current.setdefault(key, value)

merged_hooks = {}
for source in (base.get("hooks", {}), current.get("hooks", {})):
    for event, entries in source.items():
        dest = merged_hooks.setdefault(event, [])
        seen = {hook_key(e) for e in dest if isinstance(e, dict)}
        for entry in entries:
            if not isinstance(entry, dict):
                continue
            entry_key = hook_key(entry)
            if entry_key not in seen:
                dest.append(entry)
                seen.add(entry_key)

if merged_hooks:
    current["hooks"] = merged_hooks

after.write_text(json.dumps(current, indent=2, ensure_ascii=False) + "\n")
PY
}

validate_json_file() {
  local path="$1"
  [[ -f "$path" ]] || return 0
  python3 -m json.tool "$path" >/dev/null
}

headroom_health_ok() {
  python3 - "$PORT" <<'PY'
import json
import sys
import urllib.request

port = sys.argv[1]
try:
    with urllib.request.urlopen(f"http://127.0.0.1:{port}/health", timeout=5) as response:
        data = json.loads(response.read().decode("utf-8"))
except Exception:
    raise SystemExit(1)

if response.status == 200 and data.get("status") == "healthy" and data.get("ready") is True:
    raise SystemExit(0)
raise SystemExit(1)
PY
}

for arg in "$@"; do
  case "$arg" in
    --help|-h)
      usage
      exit 0
      ;;
    --dry-run)
      DRY_RUN=1
      ;;
    *)
      log "unknown argument: $arg"
      usage
      exit 2
      ;;
  esac
done

stamp="$(date +%Y%m%d-%H%M%S)"

if [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run: would back up Codex and Claude user config files"
  log "dry-run: would run Headroom init for Codex and Claude Code on port ${PORT}"
  log "dry-run: would merge pre-existing agent-harness hooks/settings back into host configs"
  log "dry-run: would start and verify Headroom profile ${PROFILE}"
  exit 0
fi

if ! command -v headroom >/dev/null 2>&1; then
  log "headroom not found; install with: pipx install --python python3.13 \"headroom-ai[all]\""
  exit 1
fi

codex_hooks_before="$(mktemp)"
claude_settings_before="$(mktemp)"
if [[ -f "$HOME/.codex/hooks.json" ]]; then
  cp "$HOME/.codex/hooks.json" "$codex_hooks_before"
fi
if [[ -f "$HOME/.claude/settings.json" ]]; then
  cp "$HOME/.claude/settings.json" "$claude_settings_before"
fi

backup_file "$HOME/.codex/config.toml" "$stamp"
backup_file "$HOME/.codex/hooks.json" "$stamp"
backup_file "$HOME/.claude/settings.json" "$stamp"
backup_file "$HOME/.claude/settings.local.json" "$stamp"
backup_file "$HOME/.claude.json" "$stamp"

if command -v codex >/dev/null 2>&1; then
  log "configuring Headroom runtime for Codex"
  HEADROOM_TELEMETRY=off headroom init -g --port "$PORT" codex
  merge_codex_hooks "$codex_hooks_before" "$HOME/.codex/hooks.json"
  validate_json_file "$HOME/.codex/hooks.json"
else
  log "codex not found; skipping Codex Headroom runtime configuration"
fi

if command -v claude >/dev/null 2>&1; then
  log "configuring Headroom runtime for Claude Code"
  HEADROOM_TELEMETRY=off headroom init -g --port "$PORT" claude
  merge_claude_settings "$claude_settings_before" "$HOME/.claude/settings.json"
  validate_json_file "$HOME/.claude/settings.json"
else
  log "claude not found; skipping Claude Headroom runtime configuration"
fi

if headroom_health_ok; then
  log "Headroom proxy on port ${PORT} is already healthy; skipping start"
else
  log "starting Headroom profile ${PROFILE}"
  HEADROOM_TELEMETRY=off headroom install start --profile "$PROFILE" \
    || log "warning: Headroom start did not become ready before timeout; checking status"
fi
HEADROOM_TELEMETRY=off headroom install status --profile "$PROFILE"
if headroom_health_ok; then
  log "verified Headroom health endpoint on port ${PORT}"
else
  log "Headroom health endpoint on port ${PORT} is not healthy"
  exit 1
fi

rm -f "$codex_hooks_before" "$claude_settings_before"
