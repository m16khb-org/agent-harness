#!/usr/bin/env bash
# Sync glab MCP servers (glab-api-servers, glab-cloud-platform) across Codex,
# Claude Code, and GJC. Idempotent. Best-effort: only acts when
# glab-mcp-wrapper and a matching GitLab token are detected, so it is a no-op
# on machines that do not use glab MCP.
#
# Usage:
#   sync-glab-mcp.sh            # apply
#   sync-glab-mcp.sh --dry-run  # preview only
set -euo pipefail

WRAPPER="${GLAB_MCP_WRAPPER:-$HOME/.local/bin/glab-mcp-wrapper}"
PROFILES=(api-servers cloud-platform)
DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1

log() { printf '[glab-mcp] %s\n' "$*" >&2; }

if [[ ! -x "$WRAPPER" ]]; then
  log "glab-mcp-wrapper not found at $WRAPPER; nothing to sync"
  exit 0
fi

token_for() {
  local profile="$1" var
  case "$profile" in
    api-servers) var="API_SERVERS_GITLAB_TOKEN" ;;
    cloud-platform) var="CLOUD_PLATFORM_GITLAB_TOKEN" ;;
    *) return 1 ;;
  esac
  if [[ -n "${!var:-}" ]]; then printf '%s' "${!var}"; return 0; fi
  grep -E "^export ${var}=" "$HOME/.zshrc" 2>/dev/null | head -1 \
    | sed -E "s/^export ${var}=//; s/^\"//; s/\"$//; s/^'//; s/'$//" | tr -d '\n'
}

for profile in "${PROFILES[@]}"; do
  tok="$(token_for "$profile" || true)"
  if [[ -z "$tok" ]]; then
    log "no GitLab token for $profile; skipping this profile"
    continue
  fi
  server="glab-$profile"

  # --- Claude Code (user scope)
  if command -v claude >/dev/null 2>&1; then
    if [[ "$DRY_RUN" == "1" ]]; then
      log "dry-run: claude mcp add $server (user)"
    else
      claude mcp remove "$server" -s user >/dev/null 2>&1 || true
      if claude mcp add "$server" -s user -- "$WRAPPER" "$profile" >/dev/null 2>&1; then
        log "claude: $server synced"
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
