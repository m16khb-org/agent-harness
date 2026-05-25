#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_SRC="$ROOT/skills/atomic-commit-push"
BIN_DIR="$ROOT/bin"
BIN="$BIN_DIR/harness"

mkdir -p "$BIN_DIR"
go build -o "$BIN" ./cmd/harness

install_link() {
  local src="$1"
  local dest="$2"
  mkdir -p "$(dirname "$dest")"
  if [ -e "$dest" ] || [ -L "$dest" ]; then
    if [ -L "$dest" ]; then
      ln -sfn "$src" "$dest"
      return 0
    fi
    echo "Refusing to replace non-symlink path: $dest" >&2
    return 1
  fi
  ln -s "$src" "$dest"
}

rm -f "${CODEX_HOME:-$HOME/.codex}/skills/atomic-commit-push" "$HOME/.claude/skills/atomic-commit-push"
install_link "$SKILL_SRC" "${CODEX_HOME:-$HOME/.codex}/skills/atomic-commit-push"
install_link "$SKILL_SRC" "$HOME/.claude/skills/atomic-commit-push"
install_link "../../skills/atomic-commit-push" "$ROOT/.claude/skills/atomic-commit-push"

python3 - "$ROOT" "$BIN" <<'PY'
import json
import pathlib
import sys
root = pathlib.Path(sys.argv[1])
bin_path = pathlib.Path(sys.argv[2])
config = {
  "mcpServers": {
    "agent-harness": {
      "type": "stdio",
      "command": str(bin_path),
      "args": ["mcp"],
      "env": {"HARNESS_ROOT": str(root)},
    }
  }
}
(root / ".mcp.json").write_text(json.dumps(config, indent=2) + "\n")
PY

python3 - "${CODEX_HOME:-$HOME/.codex}/config.toml" "$BIN" "$ROOT" <<'PY'
import pathlib
import re
import sys
config = pathlib.Path(sys.argv[1])
bin_path = sys.argv[2]
root = sys.argv[3]
config.parent.mkdir(parents=True, exist_ok=True)
text = config.read_text() if config.exists() else ""
backup = config.with_suffix(config.suffix + ".harness.bak")
if config.exists() and not backup.exists():
    backup.write_text(text)
block = f'''[mcp_servers.agent_harness]
command = "{bin_path}"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = "{root}"
'''

def remove_section(src, section):
    marker = "[" + section + "]"
    while marker in src:
        pos = src.index(marker)
        next_pos = src.find("\n[", pos + 1)
        if next_pos == -1:
            src = src[:pos].rstrip() + "\n"
        else:
            src = src[:pos] + src[next_pos + 1:]
    return src

for section in ("mcp_servers.agent_harness", "mcp_servers.agent_harness.env"):
    text = remove_section(text, section)
if text and not text.endswith("\n"):
    text += "\n"
if text and not text.endswith("\n\n"):
    text += "\n"
text += block
config.write_text(text)
PY

cat > "$ROOT/configs/claude/mcp.project.json" <<EOF2
{
  "mcpServers": {
    "agent-harness": {
      "type": "stdio",
      "command": "$BIN",
      "args": ["mcp"],
      "env": {"HARNESS_ROOT": "$ROOT"}
    }
  }
}
EOF2

cat > "$ROOT/configs/codex/mcp.config.toml" <<EOF2
[mcp_servers.agent_harness]
command = "$BIN"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.agent_harness.env]
HARNESS_ROOT = "$ROOT"
EOF2

cat <<EOF2
Installed agent harness native integrations:
- binary: $BIN
- Codex skill: ${CODEX_HOME:-$HOME/.codex}/skills/atomic-commit-push
- Claude user skill: $HOME/.claude/skills/atomic-commit-push
- Claude project skill: $ROOT/.claude/skills/atomic-commit-push
- Claude project MCP config: $ROOT/.mcp.json
- Codex MCP config: ${CODEX_HOME:-$HOME/.codex}/config.toml [mcp_servers.agent_harness]
EOF2
