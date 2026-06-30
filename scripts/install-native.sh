#!/usr/bin/env bash
set -euo pipefail

ROOT="${HARNESS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
BIN="$ROOT/bin/agent-harness"
WITH_UPSTREAM_TOOLS="${HARNESS_INSTALL_UPSTREAM_TOOLS:-0}"
EXPLICIT_UPSTREAM_TOOLS=0
INIT_CODEGRAPH="${HARNESS_INIT_CODEGRAPH:-1}"
SKIP_BUILD="${HARNESS_SKIP_BUILD:-0}"
HARNESS_ARGS=()
LLM_WIKI_BIN="${HOME}/.local/bin/llm-wiki"
LLM_WIKI_SOURCE="github.com/m16khb/llm-wiki/cmd/llm-wiki@latest"

usage() {
  cat <<'EOF'
Usage: scripts/install-native.sh [agent-harness install-native flags] [--with-upstream-tools] [--skip-upstream-tools]

Build and install agent-harness native Codex/Claude integrations.

Harness flags are passed to `agent-harness install-native`, for example:
  --project-local
  --dry-run
  --json

Optional upstream tools:
  --with-upstream-tools   Also install/update upstream llm-wiki, codegraph, and claude-mem integrations.
  --skip-upstream-tools   Do not install upstream tools, even if HARNESS_INSTALL_UPSTREAM_TOOLS=1.

Harness binary:
  --skip-build            Do not rebuild bin/agent-harness before installing integrations.

User command:
  PATH setup is handled by `agent-harness install --path-mode=auto|manual|skip`.
  The default auto mode creates ~/.local/bin/agent-harness and adds ~/.local/bin
  to the detected shell rc when it is not already on PATH.

Environment:
  HARNESS_INSTALL_UPSTREAM_TOOLS=1  Same as --with-upstream-tools.
  HARNESS_INIT_CODEGRAPH=0          Skip `codegraph init -i` for this harness repo.
  HARNESS_SKIP_BUILD=1              Same as --skip-build.

Philosophy:
  agent-harness does not reinvent llm-wiki, codegraph, or claude-mem.
  It can wire their upstream installers as optional dependencies while keeping
  harness core focused on shared CLI/MCP/state/policy orchestration.
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


command_output_contains_line() {
  local pattern="$1"
  local output
  shift
  output="$({ "$@" 2>/dev/null || true; })"
  grep -Eq "$pattern" <<<"$output"
}

codex_marketplace_exists() {
  local marketplace="$1"
  command -v codex >/dev/null 2>&1 && command_output_contains_line "^${marketplace}[[:space:]]" codex plugin marketplace list
}

codex_plugin_installed() {
  local selector="$1"
  command -v codex >/dev/null 2>&1 && command_output_contains_line "^${selector}[[:space:]]+installed" codex plugin list
}

claude_marketplace_exists() {
  local marketplace="$1"
  command -v claude >/dev/null 2>&1 && claude plugin marketplace list 2>/dev/null | grep -Eq "^[[:space:]]*❯ ${marketplace}$"
}

claude_plugin_installed() {
  local selector="$1"
  command -v claude >/dev/null 2>&1 && claude plugin list 2>/dev/null | grep -Eq "^[[:space:]]*❯ ${selector}$"
}

ensure_codex_marketplace() {
  local marketplace="$1"
  local source="$2"
  if codex_marketplace_exists "$marketplace"; then
    log "Codex marketplace ${marketplace} exists; updating snapshot"
    codex plugin marketplace upgrade "$marketplace" >/dev/null 2>&1 || true
  else
    log "Codex marketplace ${marketplace} missing; adding ${source}"
    codex plugin marketplace add "$source" >/dev/null 2>&1 || log "warning: failed to add Codex marketplace ${marketplace}; continuing"
  fi
}

ensure_codex_plugin() {
  local selector="$1"
  if codex_plugin_installed "$selector"; then
    log "Codex plugin ${selector} exists; keeping enabled and using updated marketplace snapshot"
  else
    log "Codex plugin ${selector} missing; installing"
    codex plugin add "$selector" >/dev/null 2>&1 || log "warning: failed to install Codex plugin ${selector}; continuing"
  fi
}

remove_codex_plugin() {
  local selector="$1"
  if codex_plugin_installed "$selector"; then
    log "removing legacy Codex plugin ${selector}"
    codex plugin remove "$selector" >/dev/null 2>&1 || true
  fi
}

remove_codex_marketplace() {
  local marketplace="$1"
  if codex_marketplace_exists "$marketplace"; then
    log "removing legacy Codex marketplace ${marketplace}"
    codex plugin marketplace remove "$marketplace" >/dev/null 2>&1 || true
  fi
}

ensure_claude_marketplace() {
  local marketplace="$1"
  local source="$2"
  if claude_marketplace_exists "$marketplace"; then
    log "Claude marketplace ${marketplace} exists; updating snapshot"
    claude plugin marketplace update "$marketplace" >/dev/null 2>&1 || true
  else
    log "Claude marketplace ${marketplace} missing; adding ${source}"
    claude plugin marketplace add "$source" >/dev/null 2>&1 || log "warning: failed to add Claude marketplace ${marketplace}; continuing"
  fi
}

ensure_claude_plugin() {
  local selector="$1"
  if claude_plugin_installed "$selector"; then
    log "Claude plugin ${selector} exists; updating"
    claude plugin update "$selector" >/dev/null 2>&1 || true
  else
    log "Claude plugin ${selector} missing; installing"
    claude plugin install "$selector" >/dev/null 2>&1 || log "warning: failed to install Claude plugin ${selector}; continuing"
  fi
}

remove_claude_plugin() {
  local selector="$1"
  if claude_plugin_installed "$selector"; then
    log "removing legacy Claude plugin ${selector} (preserving plugin data)"
    claude plugin uninstall "$selector" --keep-data -s user -y >/dev/null 2>&1 || true
  fi
}

remove_claude_marketplace() {
  local marketplace="$1"
  if claude_marketplace_exists "$marketplace"; then
    log "removing legacy Claude marketplace ${marketplace}"
    claude plugin marketplace remove "$marketplace" --scope user >/dev/null 2>&1 || true
  fi
}

install_llm_wiki_cli() {
  if [[ -x "$LLM_WIKI_BIN" ]]; then
    log "llm-wiki CLI exists at ${LLM_WIKI_BIN}; updating from ${LLM_WIKI_SOURCE}"
  else
    log "installing llm-wiki CLI from ${LLM_WIKI_SOURCE}"
  fi
  if ! command -v go >/dev/null 2>&1; then
    log "warning: go not found; skipping llm-wiki CLI install"
    return 1
  fi
  mkdir -p "$(dirname "$LLM_WIKI_BIN")"
  GOBIN="$(dirname "$LLM_WIKI_BIN")" go install "$LLM_WIKI_SOURCE" || {
    log "warning: failed to install llm-wiki CLI from ${LLM_WIKI_SOURCE}; continuing"
    return 1
  }
  "$LLM_WIKI_BIN" --version >/dev/null 2>&1 || {
    log "warning: installed llm-wiki CLI did not pass version smoke; continuing"
    return 1
  }
  return 0
}

resolve_llm_wiki_bin() {
  if [[ -x "$LLM_WIKI_BIN" ]]; then
    printf '%s\n' "$LLM_WIKI_BIN"
    return 0
  fi
  command -v llm-wiki 2>/dev/null || return 1
}

remove_legacy_llm_wiki_plugins() {
  if command -v codex >/dev/null 2>&1; then
    remove_codex_plugin "wiki@llm-wiki"
    remove_codex_marketplace "llm-wiki"
  fi
  if command -v claude >/dev/null 2>&1; then
    remove_claude_plugin "wiki@llm-wiki"
    remove_claude_marketplace "llm-wiki"
  fi
}

ensure_codex_llm_wiki_mcp() {
  local llm_wiki_bin="$1"
  if ! command -v codex >/dev/null 2>&1; then
    log "codex not found; skipping Codex llm-wiki MCP setup"
    return 0
  fi
  local codex_home config_path vault_path
  codex_home="${CODEX_HOME:-${HOME}/.codex}"
  config_path="${codex_home}/config.toml"
  vault_path="${LLM_WIKI_VAULT:-${HOME}/workspace/knowledge-base/llm-wiki}"
  log "refreshing Codex MCP server llm-wiki"
  python3 - "$config_path" "$llm_wiki_bin" "$vault_path" <<'PY'
import json
import os
import sys

path, bin_path, vault_path = sys.argv[1], sys.argv[2], sys.argv[3]
text = ""
try:
    with open(path, "r", encoding="utf-8") as f:
        text = f.read()
except FileNotFoundError:
    pass

def remove_section(src, section):
    marker = "[" + section + "]"
    while True:
        pos = src.find(marker)
        if pos < 0:
            return src
        next_pos = src.find("\n[", pos + len(marker))
        if next_pos < 0:
            src = src[:pos].rstrip(" \t\r\n") + "\n"
            continue
        src = src[:pos] + src[next_pos + 1:]

for section in (
    "mcp_servers.llm-wiki",
    "mcp_servers.llm-wiki.env",
    "mcp_servers.llm_wiki",
    "mcp_servers.llm_wiki.env",
):
    text = remove_section(text, section)

if text.strip() and not text.endswith("\n"):
    text += "\n"
if text.strip() and not text.endswith("\n\n"):
    text += "\n"

text += """[mcp_servers.llm-wiki]
command = {command}
args = ["mcp"]
startup_timeout_sec = 10
tool_timeout_sec = 60

[mcp_servers.llm-wiki.env]
LLM_WIKI_VAULT = {vault}
""".format(command=json.dumps(bin_path), vault=json.dumps(vault_path))

existing = ""
try:
    with open(path, "r", encoding="utf-8") as f:
        existing = f.read()
except FileNotFoundError:
    pass
if existing == text:
    sys.exit(0)

os.makedirs(os.path.dirname(path), exist_ok=True)
backup = path + ".harness.bak"
if existing and not os.path.exists(backup):
    with open(backup, "w", encoding="utf-8") as f:
        f.write(existing)
with open(path, "w", encoding="utf-8") as f:
    f.write(text)
os.chmod(path, 0o600)
PY
}

ensure_claude_llm_wiki_mcp() {
  local llm_wiki_bin="$1"
  if ! command -v claude >/dev/null 2>&1; then
    log "claude not found; skipping Claude llm-wiki MCP setup"
    return 0
  fi
  local vault_path
  vault_path="${LLM_WIKI_VAULT:-${HOME}/workspace/knowledge-base/llm-wiki}"
  log "refreshing Claude user-scope MCP server llm-wiki"
  claude mcp remove llm-wiki -s user >/dev/null 2>&1 || true
  claude mcp remove llm_wiki -s user >/dev/null 2>&1 || true
  claude mcp add-json -s user llm-wiki "$(python3 - "$llm_wiki_bin" "$vault_path" <<'PY'
import json
import sys
bin_path, vault_path = sys.argv[1], sys.argv[2]
print(json.dumps({
  "type": "stdio",
  "command": bin_path,
  "args": ["mcp"],
  "env": {"LLM_WIKI_VAULT": vault_path},
}))
PY
)" >/dev/null 2>&1 || log "warning: failed to register Claude llm-wiki MCP server; continuing"
}

install_llm_wiki_mcp() {
  local llm_wiki_bin
  if ! install_llm_wiki_cli; then
    if ! llm_wiki_bin="$(resolve_llm_wiki_bin)"; then
      log "warning: llm-wiki CLI unavailable; keeping any existing legacy llm-wiki plugin wiring"
      return 0
    fi
  else
    llm_wiki_bin="$(resolve_llm_wiki_bin)"
  fi
  remove_legacy_llm_wiki_plugins
  ensure_codex_llm_wiki_mcp "$llm_wiki_bin"
  ensure_claude_llm_wiki_mcp "$llm_wiki_bin"
}

install_claude_mem_for_ide() {
  local ide="$1"
  if ! command -v npm >/dev/null 2>&1; then
    log "npm not found; skipping claude-mem setup for ${ide}"
    return 0
  fi
  log "installing/updating claude-mem for ${ide}"
  # claude-mem decides interactive vs non-interactive from process.stdin.isTTY: with a TTY
  # stdin it shows an "Overwrite existing installation? Yes/No" confirm and blocks forever
  # (stdout is /dev/null here, so the prompt is invisible and the install appears to hang).
  # Redirect stdin from /dev/null so it takes the non-TTY path and auto-skips the prompt.
  #
  # On a successful install claude-mem still prints a benign warning for the Codex
  # local marketplace: `codex plugin marketplace upgrade claude-mem-local` only works on
  # Git marketplaces, but claude-mem registers claude-mem-local as source_type=local, so
  # the best-effort upgrade fails with "is not configured as a Git marketplace". The
  # plugin is installed and enabled regardless (exit 0). Capture combined output and
  # suppress it on success; surface the full log only when the install actually fails.
  local cm_out
  if cm_out="$(npx -y claude-mem@latest install --ide "$ide" --provider claude --runtime worker --no-auto-start </dev/null 2>&1)"; then
    return 0
  fi
  log "warning: failed to install claude-mem for ${ide}; continuing"
  printf '%s\n' "$cm_out" >&2
}

# Work around claude-mem marketplace packaging bug: plugin.json references ./hooks/codex-hooks.json
# but the marketplace directory places hooks under plugin/hooks/ instead of hooks/ at the root.
# Symlink hooks -> plugin/hooks so Codex can find the hook scripts.
fix_claude_mem_codex_hooks_path() {
  local marketplace_root="${HOME}/.claude/plugins/marketplaces/thedotmack"
  local hooks_link="${marketplace_root}/hooks"
  local hooks_target="plugin/hooks"
  if [[ -d "${marketplace_root}/${hooks_target}" ]] && [[ ! -e "$hooks_link" ]]; then
    log "fixing claude-mem Codex hook path: ${hooks_link} -> ${hooks_target}"
    ln -sf "$hooks_target" "$hooks_link"
  fi
}

ensure_codegraph_on_path() {
  if command -v codegraph >/dev/null 2>&1; then
    return 0
  fi
  if command -v npm >/dev/null 2>&1; then
    local npm_prefix candidate
    npm_prefix="$(npm prefix -g 2>/dev/null || true)"
    candidate="$npm_prefix/bin/codegraph"
    if [[ -x "$candidate" ]]; then
      mkdir -p "$HOME/.local/bin"
      ln -sf "$candidate" "$HOME/.local/bin/codegraph"
    fi
  fi
  command -v codegraph >/dev/null 2>&1
}

install_upstream_tools() {
  local dry_run="$1"
  if [[ "$dry_run" == "1" ]]; then
    log "dry-run: would install/update upstream tools: llm-wiki CLI/MCP, codegraph, claude-mem; would remove legacy llm-wiki marketplace/plugin wiring and legacy agentmemory plugin wiring"
    return 0
  fi

  log "setting up llm-wiki CLI/MCP"
  install_llm_wiki_mcp

  if command -v codex >/dev/null 2>&1; then
    log "setting up Codex plugins: claude-mem"
    remove_codex_plugin "agentmemory@agentmemory"
    remove_codex_marketplace "agentmemory"
    install_claude_mem_for_ide "codex-cli"
    ensure_codex_plugin "claude-mem@claude-mem-local"
    fix_claude_mem_codex_hooks_path
  else
    log "codex not found; skipping Codex claude-mem plugin setup"
  fi

  if command -v claude >/dev/null 2>&1; then
    log "setting up Claude plugins: claude-mem"
    remove_claude_plugin "agentmemory@agentmemory"
    remove_claude_marketplace "agentmemory"
    install_claude_mem_for_ide "claude-code"
    ensure_claude_plugin "claude-mem@thedotmack"
  else
    log "claude not found; skipping Claude claude-mem plugin setup"
  fi

  if command -v npm >/dev/null 2>&1; then
    if command -v codegraph >/dev/null 2>&1; then
      log "CodeGraph exists; updating npm package"
      npm update -g @colbymchenry/codegraph >/dev/null || npm install -g @colbymchenry/codegraph >/dev/null
    else
      log "CodeGraph missing; installing npm package"
      npm install -g @colbymchenry/codegraph >/dev/null
    fi
    ensure_codegraph_on_path || true
    if command -v codegraph >/dev/null 2>&1; then
      log "refreshing CodeGraph MCP registration"
      codegraph install --target=codex,claude --location=global --yes >/dev/null || true
      if is_truthy "$INIT_CODEGRAPH"; then
        log "refreshing CodeGraph index for ${ROOT}"
        codegraph init -i "$ROOT" >/dev/null || true
      fi
    else
      log "codegraph installed by npm but not found on PATH; add npm global bin to PATH"
    fi
  else
    log "npm not found; skipping CodeGraph setup"
  fi

}

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --help|-h)
      usage
      exit 0
      ;;
    --with-upstream-tools)
      WITH_UPSTREAM_TOOLS=1
      EXPLICIT_UPSTREAM_TOOLS=1
      ;;
    --skip-upstream-tools|--without-upstream-tools)
      WITH_UPSTREAM_TOOLS=0
      EXPLICIT_UPSTREAM_TOOLS=1
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

if is_truthy "$WITH_UPSTREAM_TOOLS"; then
  install_upstream_tools "$DRY_RUN"
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
