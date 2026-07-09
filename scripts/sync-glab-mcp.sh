#!/usr/bin/env bash
# Sync profile-scoped glab MCP servers across Codex, Claude Code, and GJC.
# Idempotent and best-effort: acts only when glab-mcp-wrapper and a matching
# GitLab token are detected, so it is a no-op on machines without glab MCP.
#
# Profiles are user configuration, not harness code:
#   1. GLAB_MCP_PROFILES env var (space- or comma-separated), else
#   2. ~/.config/glab-mcp/profiles (one profile per line, # comments allowed)
# Each profile "<name>" maps to server "glab-<name>" and token env var
# "<NAME_WITH_UNDERSCORES_UPPERCASED>_GITLAB_TOKEN".
#
# Usage:
#   sync-glab-mcp.sh              # apply
#   sync-glab-mcp.sh --dry-run    # preview only
#   sync-glab-mcp.sh --remove     # remove the configured servers from all hosts
set -euo pipefail

WRAPPER="${GLAB_MCP_WRAPPER:-$HOME/.local/bin/glab-mcp-wrapper}"
PROFILES_FILE="${GLAB_MCP_PROFILES_FILE:-$HOME/.config/glab-mcp/profiles}"
DRY_RUN=0
REMOVE=0
case "${1:-}" in
  --dry-run) DRY_RUN=1 ;;
  --remove) REMOVE=1 ;;
  "") ;;
  *) echo "Usage: $0 [--dry-run|--remove]" >&2; exit 1 ;;
esac

log() { printf '[glab-mcp] %s\n' "$*" >&2; }

if [[ "$REMOVE" != "1" && ! -x "$WRAPPER" ]]; then
  log "glab-mcp-wrapper not found at $WRAPPER; nothing to sync"
  exit 0
fi

profiles=()
if [[ -n "${GLAB_MCP_PROFILES:-}" ]]; then
  # shellcheck disable=SC2206
  profiles=(${GLAB_MCP_PROFILES//,/ })
elif [[ -f "$PROFILES_FILE" ]]; then
  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(printf '%s' "$line" | tr -d '[:space:]')"
    [[ -n "$line" ]] && profiles+=("$line")
  done < "$PROFILES_FILE"
fi
if [[ ${#profiles[@]} -eq 0 ]]; then
  log "no profiles configured (set GLAB_MCP_PROFILES or $PROFILES_FILE); nothing to sync"
  exit 0
fi

token_var_for() {
  printf '%s_GITLAB_TOKEN' "$(printf '%s' "$1" | tr '[:lower:]-' '[:upper:]_')"
}

token_for() {
  local var
  var="$(token_var_for "$1")"
  if [[ -n "${!var:-}" ]]; then printf '%s' "${!var}"; return 0; fi
  grep -E "^export ${var}=" "$HOME/.zshrc" 2>/dev/null | head -1 \
    | sed -E "s/^export ${var}=//; s/^\"//; s/\"$//; s/^'//; s/'$//" | tr -d '\n'
}

remove_codex_section() {
  local server="$1" codex_cfg="$HOME/.codex/config.toml"
  [[ -f "$codex_cfg" ]] || return 0
  grep -qF "[mcp_servers.$server]" "$codex_cfg" || return 0
  local tmp
  tmp="$(mktemp)"
  awk -v section="[mcp_servers.$server]" '
    $0 == section { skipping = 1; next }
    skipping && /^\[/ { skipping = 0 }
    !skipping { print }
  ' "$codex_cfg" > "$tmp" && mv "$tmp" "$codex_cfg"
  log "codex: $server removed"
}

for profile in "${profiles[@]}"; do
  server="glab-$profile"

  if [[ "$REMOVE" == "1" ]]; then
    if [[ "$DRY_RUN" == "1" ]]; then
      log "dry-run: would remove $server from claude/gjc/codex"
      continue
    fi
    command -v claude >/dev/null 2>&1 && { claude mcp remove "$server" -s user >/dev/null 2>&1 || true; log "claude: $server removed (if present)"; }
    command -v gjc >/dev/null 2>&1 && { gjc mcp remove "$server" >/dev/null 2>&1 || true; log "gjc: $server removed (if present)"; }
    remove_codex_section "$server"
    continue
  fi

  tok="$(token_for "$profile" || true)"
  if [[ -z "$tok" ]]; then
    log "no GitLab token ($(token_var_for "$profile")) for $profile; skipping this profile"
    continue
  fi

  # --- Claude Code (user scope)
  if command -v claude >/dev/null 2>&1; then
    if [[ "$DRY_RUN" == "1" ]]; then
      log "dry-run: claude mcp add $server (user)"
    else
      claude mcp remove "$server" -s user >/dev/null 2>&1 || true
      if claude mcp add "$server" -s user -- "$WRAPPER" "$profile" >/dev/null 2>&1; then
        if claude mcp list 2>/dev/null | grep -qF "$server"; then
          log "claude: $server synced and verified"
        else
          log "claude: $server added but not visible in claude mcp list"
        fi
      else
        log "claude: $server FAILED (non-fatal)"
      fi
    fi
  fi

  # --- GJC (user scope)
  if command -v gjc >/dev/null 2>&1; then
    if [[ "$DRY_RUN" == "1" ]]; then
      log "dry-run: gjc mcp add $server"
    else
      if gjc mcp add "$server" "$WRAPPER" --arg="$profile" --type=stdio --force >/dev/null 2>&1; then
        log "gjc: $server synced"
      else
        log "gjc: $server FAILED (non-fatal)"
      fi
    fi
  fi

  # --- Codex (~/.codex/config.toml): idempotent section append
  codex_cfg="$HOME/.codex/config.toml"
  if [[ -f "$codex_cfg" ]]; then
    section="[mcp_servers.$server]"
    if grep -qF "$section" "$codex_cfg"; then
      log "codex: $server already present"
    elif [[ "$DRY_RUN" == "1" ]]; then
      log "dry-run: codex append $server"
    else
      {
        printf '\n[mcp_servers.%s]\n' "$server"
        printf 'command = "%s"\n' "$WRAPPER"
        printf 'args = ["%s"]\n' "$profile"
      } >> "$codex_cfg"
      log "codex: $server appended"
    fi
  fi
done

log "glab MCP sync complete"
