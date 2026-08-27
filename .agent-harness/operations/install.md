---
name: install.md
description: First-run install, refresh, and compatibility install-native operations.
---

# Install And Refresh

Public setup UX has two primary commands:

```bash
# First run from a fresh clone, before agent-harness is on PATH.
./install.sh

# Scriptable install or refresh after agent-harness is on PATH.
agent-harness install --dry-run --json
agent-harness install

# Ongoing refresh from the current checkout.
ah update
ah inspect --json

# Initialize project docs/profile for a target repository.
agent-harness project bootstrap --repo /path/to/repo

# Refresh existing project docs/profile from current templates and evidence.
agent-harness project bootstrap --repo /path/to/repo --sync
```

`./install.sh` computes the checkout root, builds `bin/agent-harness` when needed, and then runs `agent-harness install`. In a real terminal with no arguments it enters the interactive installer. Non-interactive automation can pass explicit flags such as `--dry-run --json`.

`install` owns environment setup. Normal users should not export `HARNESS_ROOT` manually; the installer writes it into Codex, Claude, and Omo MCP configuration. `CODEX_HOME` is honored when already set and otherwise defaults to `~/.codex`; Omo uses its native flat-layout `~/.omo` root. PATH setup is selected with `--path-mode=auto|manual|skip`. Every mode plans or writes the canonical `~/.local/bin/agent-harness` shim and the managed `~/.local/bin/ah -> ~/.local/bin/agent-harness` shorthand; `manual` and `skip` only omit shell rc changes. The default `auto` mode also adds a shell rc PATH line when needed.

Each install/update refreshes user skill links for all three first-party hosts,
managed MCP registration, and the two-event lifecycle surface. It also prunes
stale links in each host skill directory whose target lies under this
checkout's `skills/` but no longer exists (a removed or renamed shared skill);
links that point elsewhere or still resolve are left alone, and `--dry-run`
reports them as `would_remove` instead of deleting. Omo receives
`~/.omo/mcp.json` plus `~/.omo/extensions/agent-harness.js`; explicit
`--project-local` additionally writes `.omo/skills/*` and `.omo/mcp.json`.
Before any non-dry-run activation, the installer renders the complete host and
shell-path plan and snapshots every affected file, symlink, mode, and newly
created parent directory. Any host write or activation-seal failure restores
those snapshots together with the command shims before aborting the transition.

`agent-harness` remains the canonical command identity. `ah` is a command symlink, not a shell alias or wrapper. If `~/.local/bin/ah` is a regular file, directory, or points elsewhere, install/update refuses to overwrite it and requires manual resolution.

기존 `~/.local/bin/agent-harness`가 regular file이면 기본 install과 dry-run은 변경 없이 거부한다. 그 파일과 실제 실행 중인 staged/canonical candidate가 모두 정적 Go build identity `agent-harness/cmd/harness` / module `agent-harness`를 만족할 때만 `--adopt-command-file`로 adoption을 명시할 수 있다. 승인된 실행은 같은 디렉터리의 mode `0600` backup을 만든 뒤 temporary symlink와 command path를 atomic exchange하고 displaced identity를 재검증한다. native activation Seal 전 오류에서는 원래 bytes와 mode를 복원하고 exact transition을 Abort한다. Seal이 성공한 뒤 backup 정리만 실패하면 activation은 committed 상태로 유지되고 JSON receipt의 `backup_retained`와 recovery path를 따른다. `ah`에는 이 승인 플래그가 적용되지 않는다.

`bootstrap` and `update` use the current `agent-harness` checkout. They build `bin/agent-harness`, refresh both command shims through the same installer path, run native host installation, refresh agent-harness MCP registration, and restart the shared daemon when it is already running so the MCP backend uses the rebuilt binary. They do not run `git pull`. Executable symlinks are resolved back to the checkout, so `ah update` works outside the repository directory.

`ah update`는 host가 소유한 stdio MCP 프로세스를 열거하거나 종료하지 않는다. 살아 있는 agent-harness proxy는 daemon generation 교체를 감지해 동일한 protocol/capability 계약으로 다시 초기화한다. 교체 시점에 완료 여부를 확정할 수 없는 요청은 자동 재실행하지 않고 `daemon_generation_changed`, `outcome=unknown`, `reconcile_required=true` 오류로 끝낸다. 새 daemon의 handshake 계약이 달라지면 proxy를 종료해 host가 새 세션으로 다시 연결하게 한다.

Omo는 MCP tool catalog를 server config hash 기준으로 최대 7일 재사용하므로, 같은
경로의 binary만 교체하면 새 세션도 이전 input schema를 유지할 수 있다. Omo
installer는 현재 advertised tool catalog의 SHA-256을
`HARNESS_MCP_CATALOG_SHA256` env에 기록한다. 따라서 `install`/`bootstrap`/`update`
후 catalog가 바뀌면 Omo server config hash도 바뀌고, 다음 세션은 새
`tools/list`를 조회한다. 이 값은 cache revision token이며 MCP handler 동작을
제어하지 않는다.

외부 GitLab MCP와 개인 wrapper 등록은 update에 포함되지 않는다. 필요할 때만 `scripts/sync-glab-mcp.sh --dry-run`으로 확인한 뒤 `scripts/sync-glab-mcp.sh`를 명시적으로 실행한다.

Default user-level install updates:

- Command shims: `~/.local/bin/agent-harness -> <agent-harness>/bin/agent-harness`, `~/.local/bin/ah -> ~/.local/bin/agent-harness`
- Codex skill symlinks: `~/.codex/skills/* -> <agent-harness>/skills/*`
- Claude skill symlinks: `~/.claude/skills/* -> <agent-harness>/skills/*`
- Codex MCP config: `~/.codex/config.toml` `[mcp_servers.agent_harness]`
- Codex hooks: `~/.codex/hooks.json`
- Claude hooks: `~/.claude/settings.json`
- Claude user-scope MCP config: `~/.claude.json` key `mcpServers.agent_harness` (written directly by the installer)
- Omo skill symlinks: `~/.omo/agent/skills/* -> <agent-harness>/skills/*`
- Omo MCP config: `~/.omo/mcp.json`
- Omo lifecycle extension: `~/.omo/extensions/agent-harness.js`
- Optional Claude Code plugins and Git skills declared in `configs/upstream.json` (currently Claude-scoped): entries already present are skipped, and upstream failures are reported as `upstream ...` messages without failing native installation. See [hosts.md](hosts.md#upstream-plugins-and-skills).

Default install does not create target-repo `.claude/skills`,
`.claude/settings.json`, `.mcp.json`, `.omo/skills`, or `.omo/mcp.json`. Use
explicit project-local options only when a repo should own those files.

Dry-run checks:

```bash
./install.sh --dry-run --json
agent-harness install --dry-run --json
agent-harness bootstrap --dry-run --json
```

Release reproducibility smoke:

```bash
scripts/release-repro-smoke.sh
```

This script builds the current checkout, then verifies `install --dry-run --project-local --json` in temporary `HOME`, `CODEX_HOME`, and fixture `HARNESS_ROOT` directories. It also checks the clean `inspect/docs/state` workflow under a temporary state directory.

Release build matrix smoke:

```bash
scripts/release-build-matrix.sh
```

The default release matrix cross-builds `darwin/arm64`, `darwin/amd64`, `linux/amd64`, and `linux/arm64` with `CGO_ENABLED=0`.

## Project `--sync`

`agent-harness project bootstrap --sync` refreshes target repo `AGENTS.md` routing block, `.agent-harness/*.md`, and user-state repo profile metadata from current evidence.

Use low-level `scripts/install-native.sh` and `install-native` directly only for automation or focused installer debugging.
