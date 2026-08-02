#!/usr/bin/env bash
set -Eeuo pipefail

usage() {
  printf '%s\n' 'Usage: verify-child-host-smoke.sh --issue N --source-root DIR --child-root DIR --head SHA --remote-ref REF --json-out FILE --confirm-user-activation'
}

issue=""
source_root=""
child_root=""
expected_head=""
remote_ref=""
json_out=""
confirmed=0
receipt_ready=0
while (($#)); do
  case "$1" in
    --issue|--source-root|--child-root|--head|--remote-ref|--json-out)
      (($# >= 2)) || { usage >&2; exit 2; }
      case "$1" in
        --issue) issue="$2" ;;
        --source-root) source_root="$2" ;;
        --child-root) child_root="$2" ;;
        --head) expected_head="$2" ;;
        --remote-ref) remote_ref="$2" ;;
        --json-out) json_out="$2" ;;
      esac
      shift 2
      ;;
    --confirm-user-activation)
      confirmed=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
done

fail_before_mutation() {
  printf 'child-host-smoke: %s\n' "$1" >&2
  if [[ "$receipt_ready" == 1 ]] && declare -F emit_receipt >/dev/null; then
    emit_receipt fail >/dev/null 2>&1 || true
  fi
  exit 1
}

[[ "$issue" =~ ^[1-9][0-9]*$ ]] || fail_before_mutation 'issue must be a positive decimal integer'
[[ "$expected_head" =~ ^[0-9a-f]{40}$ ]] || fail_before_mutation 'head must be a lowercase 40-character SHA'
[[ "$remote_ref" =~ ^refs/heads/[A-Za-z0-9._/-]+$ && "$remote_ref" != *..* ]] || fail_before_mutation 'remote-ref must be one exact branch ref'
[[ -n "$source_root" && -n "$child_root" && -n "$json_out" ]] || fail_before_mutation 'all required arguments must be non-empty'
[[ "$source_root" == /* && "$child_root" == /* && "$json_out" == /* ]] || fail_before_mutation 'source-root, child-root, and json-out must be absolute'

canonical_directory() {
  python3 - "$1" <<'PY'
import os
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
if not path.is_absolute():
    path = pathlib.Path.cwd() / path
path = path.resolve(strict=True)
if not path.is_dir() or path.is_symlink():
    raise SystemExit(1)
print(os.fspath(path))
PY
}

requested_source_root="$source_root"
requested_child_root="$child_root"
requested_json_out="$json_out"
source_root="$(canonical_directory "$source_root")" || fail_before_mutation 'source-root must be a real directory'
child_root="$(canonical_directory "$child_root")" || fail_before_mutation 'child-root must be a real directory'
[[ "$source_root" == "$requested_source_root" && "$child_root" == "$requested_child_root" ]] || fail_before_mutation 'source-root and child-root must already be canonical'
[[ "$source_root" != "$child_root" ]] || fail_before_mutation 'source-root and child-root must differ'

json_out="$(python3 - "$json_out" <<'PY'
import os
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
if not path.is_absolute():
    path = pathlib.Path.cwd() / path
print(os.fspath(path.parent.resolve(strict=True) / path.name))
PY
)" || fail_before_mutation 'json-out parent must exist'
[[ "$json_out" == "$requested_json_out" ]] || fail_before_mutation 'json-out must already be canonical'

python3 - "$json_out" <<'PY' || fail_before_mutation 'json-out must be a private regular-file target'
import os
import stat
import sys

path = sys.argv[1]
parent = os.path.dirname(path)
parent_info = os.lstat(parent)
if not stat.S_ISDIR(parent_info.st_mode) or stat.S_ISLNK(parent_info.st_mode) or stat.S_IMODE(parent_info.st_mode) != 0o700:
    raise SystemExit(1)
try:
    info = os.lstat(path)
except FileNotFoundError:
    pass
else:
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600:
        raise SystemExit(1)
PY

[[ -x "$source_root/bin/agent-harness" ]] || fail_before_mutation 'source binary must exist'
[[ -x "$child_root/scripts/install-native.sh" ]] || fail_before_mutation 'child installer must exist'

temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/agent-harness-child-smoke.XXXXXX")"
chmod 0700 "$temporary_root"
before_file="$temporary_root/before.json"
activated_file="$temporary_root/activated.json"
restore_file="$temporary_root/restore.json"
codex_observation="$temporary_root/codex-observation.json"
claude_observation="$temporary_root/claude-observation.json"
dry_run_file="$temporary_root/install-dry-run.json"
activation_snapshot="$temporary_root/source-activation-snapshot"
mutation_started=0
finalized=0
lock_held=0
restoring=0
pending_signal=0
state_root="${HARNESS_STATE_DIR:-${HOME:?}/.local/state/agent-harness}"
lock_path="$state_root/child-host-smoke.lock"
local_head=""
remote_head=""
child_binary_sha256=""
codex_version=""
claude_version=""

cleanup() {
	local status=0
	if ((lock_held == 1)); then
		rmdir "$lock_path" 2>/dev/null || status=1
	fi
	rm -rf -- "$temporary_root" || status=1
	return "$status"
}

activation_digest() {
  local root="$1"
  local output="$2"
  python3 - "$root" "$output" "${HOME:?}" "${CODEX_HOME:-${HOME:?}/.codex}" <<'PY'
import hashlib
import json
import os
import stat
import sys
import tomllib

root, output, home, codex_home = sys.argv[1:]
root = os.path.realpath(root)
binary = os.path.join(root, "bin", "agent-harness")
surfaces = [
    ("claude", "hooks", os.path.join(home, ".claude", "settings.json")),
    ("claude", "mcp", os.path.join(home, ".claude.json")),
    ("codex", "hooks", os.path.join(codex_home, "hooks.json")),
    ("codex", "mcp", os.path.join(codex_home, "config.toml")),
]

def read_regular(path):
    info = os.lstat(path)
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode):
        raise SystemExit(1)
    with open(path, "rb") as handle:
        return handle.read()

def raw_digest(path):
    return hashlib.sha256(read_regular(path)).hexdigest()

def semantic_digest(path):
    data = read_regular(path)
    if path.endswith(".toml"):
        value = tomllib.loads(data.decode("utf-8"))
    else:
        value = json.loads(data)
    canonical = json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False, default=lambda item: item.isoformat()).encode("utf-8")
    return hashlib.sha256(canonical).hexdigest()

items = []
for host, surface, path in surfaces:
    items.append({"host": host, "surface": surface, "semantic_sha256": semantic_digest(path), "sha256": raw_digest(path)})
result = {
    "root_sha256": hashlib.sha256(root.encode()).hexdigest(),
    "binary_sha256": raw_digest(binary),
    "surfaces": items,
}
with open(output, "w", encoding="utf-8") as handle:
    json.dump(result, handle, sort_keys=True, separators=(",", ":"))
    handle.write("\n")
os.chmod(output, 0o600)
PY
}

capture_activation_snapshot() {
  python3 - "$1" "${HOME:?}" "${CODEX_HOME:-${HOME:?}/.codex}" <<'PY'
import hashlib
import json
import os
import stat
import sys

snapshot, home, codex_home = sys.argv[1:]
os.mkdir(snapshot, 0o700)
targets = [
    ("claude-hooks", os.path.join(home, ".claude", "settings.json")),
    ("claude-mcp", os.path.join(home, ".claude.json")),
    ("codex-hooks", os.path.join(codex_home, "hooks.json")),
    ("codex-mcp", os.path.join(codex_home, "config.toml")),
]
records = []
for name, target in targets:
    info = os.lstat(target)
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode) or info.st_size > 1 << 20:
        raise SystemExit(1)
    with open(target, "rb") as handle:
        data = handle.read((1 << 20) + 1)
    if len(data) > 1 << 20:
        raise SystemExit(1)
    destination = os.path.join(snapshot, name)
    descriptor = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as handle:
        handle.write(data)
        handle.flush()
        os.fsync(handle.fileno())
    records.append({"name": name, "mode": stat.S_IMODE(info.st_mode), "sha256": hashlib.sha256(data).hexdigest()})
manifest = os.path.join(snapshot, "manifest.json")
descriptor = os.open(manifest, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
    json.dump(records, handle, sort_keys=True, separators=(",", ":"))
    handle.write("\n")
    handle.flush()
    os.fsync(handle.fileno())
directory = os.open(snapshot, os.O_RDONLY)
try:
    os.fsync(directory)
finally:
    os.close(directory)
PY
}

restore_activation_snapshot() {
  python3 - "$1" "${HOME:?}" "${CODEX_HOME:-${HOME:?}/.codex}" <<'PY'
import hashlib
import json
import os
import stat
import sys
import tempfile

snapshot, home, codex_home = sys.argv[1:]
expected = {
    "claude-hooks": os.path.join(home, ".claude", "settings.json"),
    "claude-mcp": os.path.join(home, ".claude.json"),
    "codex-hooks": os.path.join(codex_home, "hooks.json"),
    "codex-mcp": os.path.join(codex_home, "config.toml"),
}
manifest_path = os.path.join(snapshot, "manifest.json")
manifest_info = os.lstat(manifest_path)
if not stat.S_ISREG(manifest_info.st_mode) or stat.S_ISLNK(manifest_info.st_mode) or stat.S_IMODE(manifest_info.st_mode) != 0o600:
    raise SystemExit(1)
with open(manifest_path, encoding="utf-8") as handle:
    records = json.load(handle)
if not isinstance(records, list) or len(records) != len(expected) or {item.get("name") for item in records if isinstance(item, dict)} != set(expected):
    raise SystemExit(1)
for item in records:
    if set(item) != {"name", "mode", "sha256"} or not isinstance(item["mode"], int) or item["mode"] < 0 or item["mode"] > 0o777:
        raise SystemExit(1)
    source = os.path.join(snapshot, item["name"])
    source_info = os.lstat(source)
    if not stat.S_ISREG(source_info.st_mode) or stat.S_ISLNK(source_info.st_mode) or stat.S_IMODE(source_info.st_mode) != 0o600 or source_info.st_size > 1 << 20:
        raise SystemExit(1)
    with open(source, "rb") as handle:
        data = handle.read((1 << 20) + 1)
    if len(data) > 1 << 20 or hashlib.sha256(data).hexdigest() != item["sha256"]:
        raise SystemExit(1)
    target = expected[item["name"]]
    parent = os.path.dirname(target)
    parent_info = os.lstat(parent)
    if not stat.S_ISDIR(parent_info.st_mode) or stat.S_ISLNK(parent_info.st_mode):
        raise SystemExit(1)
    try:
        target_info = os.lstat(target)
    except FileNotFoundError:
        pass
    else:
        if not stat.S_ISREG(target_info.st_mode) or stat.S_ISLNK(target_info.st_mode):
            raise SystemExit(1)
    descriptor, temporary = tempfile.mkstemp(prefix=".child-host-restore-", dir=parent)
    try:
        os.fchmod(descriptor, item["mode"])
        with os.fdopen(descriptor, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, target)
        directory = os.open(parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
PY
}

instrument_claude_child_smoke_hooks() {
  python3 - "$1" "$2" "${HOME:?}" <<'PY'
import json
import os
import shlex
import stat
import sys
import tempfile

root, claude_observation, home = sys.argv[1:]
binary = os.path.join(root, "bin", "agent-harness")

def instrument(path, observation):
    info = os.lstat(path)
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode):
        raise SystemExit(1)
    with open(path, encoding="utf-8") as handle:
        document = json.load(handle)
    hooks = document.get("hooks")
    if not isinstance(hooks, dict):
        raise SystemExit(1)
    prefix = f"/usr/bin/env HARNESS_CHILD_SMOKE_HOOKS=1 HARNESS_CHILD_SMOKE_OBSERVATION_FILE={shlex.quote(observation)} "
    for event, subcommand in (("SessionStart", "session-start"), ("PreToolUse", "pre-tool-use")):
        matches = 0
        for group in hooks.get(event, []):
            if not isinstance(group, dict):
                continue
            for hook in group.get("hooks", []):
                if not isinstance(hook, dict):
                    continue
                command = hook.get("command")
                expected = f"'{binary}' hook {subcommand}"
                if isinstance(command, str) and command.startswith(expected):
                    hook["command"] = prefix + command
                    matches += 1
        if matches != 1:
            raise SystemExit(1)
    parent = os.path.dirname(path)
    descriptor, temporary = tempfile.mkstemp(prefix=".child-host-hooks-", dir=parent)
    try:
        os.fchmod(descriptor, stat.S_IMODE(info.st_mode))
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            json.dump(document, handle, sort_keys=True, separators=(",", ":"))
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
        directory = os.open(parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass

instrument(os.path.join(home, ".claude", "settings.json"), claude_observation)
PY
}

validate_managed_activation_identity() {
  local root="$1"
  python3 - "$root" "${HOME:?}" "${CODEX_HOME:-${HOME:?}/.codex}" <<'PY'
import json
import os
import sys
import tomllib

root, home, codex_home = sys.argv[1:]
binary = os.path.join(root, "bin", "agent-harness")
with open(os.path.join(codex_home, "config.toml"), "rb") as handle:
    codex_mcp = tomllib.load(handle).get("mcp_servers", {}).get("agent_harness", {})
with open(os.path.join(codex_home, "hooks.json"), encoding="utf-8") as handle:
    codex_hooks = json.load(handle)
with open(os.path.join(home, ".claude.json"), encoding="utf-8") as handle:
    claude_mcp = json.load(handle).get("mcpServers", {}).get("agent_harness", {})
with open(os.path.join(home, ".claude", "settings.json"), encoding="utf-8") as handle:
    claude_hooks = json.load(handle)

for server in (codex_mcp, claude_mcp):
    if server.get("command") != binary or server.get("args") != ["mcp"] or server.get("env", {}).get("HARNESS_ROOT") != root:
        raise SystemExit(1)

def event_commands(document, event):
    hooks = []
    for group in document.get("hooks", {}).get(event, []):
        for hook in group.get("hooks", []):
            if isinstance(hook, dict):
                hooks.append(hook)
    return hooks

enforcement = "--enforce-worktree --enforce-korean-remote-artifacts --enforce-vcs-issue-linking --enforce-staged-checks --enforce-gitops-kubectl"
contracts = (
    (codex_hooks, {
        "SessionStart": f"'{binary}' hook session-start --host codex",
        "PreToolUse": f"'{binary}' hook pre-tool-use --host codex {enforcement}",
    }),
    (claude_hooks, {
        "SessionStart": f"'{binary}' hook session-start",
        "PreToolUse": f"'{binary}' hook pre-tool-use --host claude {enforcement}",
    }),
)
for document, expected_by_event in contracts:
    for event, expected in expected_by_event.items():
        subcommand = "session-start" if event == "SessionStart" else "pre-tool-use"
        managed_prefix = f"'{binary}' hook {subcommand}"
        managed = []
        for hook in event_commands(document, event):
            command = hook.get("command")
            if isinstance(command, str) and "/bin/agent-harness' hook" in command and not command.startswith(f"'{binary}' hook"):
                raise SystemExit(1)
            if isinstance(command, str) and command.startswith(managed_prefix):
                managed.append(hook)
        if len(managed) != 1:
            raise SystemExit(1)
        hook = managed[0]
        if set(hook) != {"type", "command", "timeout"} or hook.get("type") != "command" or hook.get("command") != expected or hook.get("timeout") != 5:
            raise SystemExit(1)
PY
}

host_mcp_readback() {
  local root="$1"
  local _label="$2"
  python3 - "$root" <<'PY'
import json
import os
import subprocess
import sys

root = sys.argv[1]
binary = os.path.join(root, "bin", "agent-harness")

def run_bounded(command):
    import resource
    import tempfile

    environment = dict(os.environ)
    environment["HARNESS_ROOT"] = root
    limit = 64 << 10

    def cap_files():
        resource.setrlimit(resource.RLIMIT_FSIZE, (limit + 1, limit + 1))

    with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
        try:
            result = subprocess.run(command, env=environment, stdout=stdout, stderr=stderr, timeout=30, check=False, preexec_fn=cap_files)
        except (OSError, subprocess.TimeoutExpired):
            raise SystemExit(1)
        stdout_size = os.fstat(stdout.fileno()).st_size
        stderr_size = os.fstat(stderr.fileno()).st_size
        if result.returncode != 0 or stdout_size > limit or stderr_size > limit:
            raise SystemExit(1)
        stdout.seek(0)
        try:
            return stdout.read(limit + 1).decode("utf-8", errors="strict")
        except UnicodeDecodeError:
            raise SystemExit(1)

codex = json.loads(run_bounded(["codex", "mcp", "get", "agent_harness", "--json"]))
transport = codex.get("transport", {})
if codex.get("name") != "agent_harness" or codex.get("enabled") is not True:
    raise SystemExit(1)
if transport.get("type") != "stdio" or transport.get("command") != binary or transport.get("args") != ["mcp"] or transport.get("env", {}).get("HARNESS_ROOT") != root:
    raise SystemExit(1)
claude = run_bounded(["claude", "mcp", "get", "agent_harness"])
for expected in ("Status: ✔ Connected", f"Command: {binary}", "Args: mcp", f"HARNESS_ROOT={root}"):
    if expected not in claude:
        raise SystemExit(1)
PY
}

file_sha256() {
  python3 - "$1" <<'PY'
import hashlib
import sys

with open(sys.argv[1], "rb") as handle:
    print(hashlib.sha256(handle.read()).hexdigest())
PY
}

bounded_version() {
  local executable="$1"
  python3 - "$executable" <<'PY'
import os
import resource
import subprocess
import sys
import tempfile

limit = 4096
def cap_files():
    resource.setrlimit(resource.RLIMIT_FSIZE, (limit + 1, limit + 1))

with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
    try:
        result = subprocess.run([sys.argv[1], "--version"], stdout=stdout, stderr=stderr, timeout=30, check=False, preexec_fn=cap_files)
    except (OSError, subprocess.TimeoutExpired):
        raise SystemExit(1)
    stdout_size = os.fstat(stdout.fileno()).st_size
    stderr_size = os.fstat(stderr.fileno()).st_size
    if result.returncode != 0 or stdout_size > limit or stderr_size > limit:
        raise SystemExit(1)
    stdout.seek(0)
    try:
        value = stdout.read(limit + 1).decode("utf-8", errors="strict").rstrip("\n")
    except UnicodeDecodeError:
        raise SystemExit(1)
if not value or len(value) > 256 or "\n" in value or "\r" in value:
    raise SystemExit(1)
sys.stdout.write(value)
PY
}

validate_activation_digest() {
  python3 - "$1" "$2" <<'PY'
import hashlib
import json
import os
import re
import sys

path, root = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    value = json.load(handle)
if set(value) != {"root_sha256", "binary_sha256", "surfaces"}:
    raise SystemExit(1)
sha = re.compile(r"[0-9a-f]{64}")
expected_root = hashlib.sha256(os.path.realpath(root).encode()).hexdigest()
with open(os.path.join(root, "bin", "agent-harness"), "rb") as handle:
    expected_binary = hashlib.sha256(handle.read()).hexdigest()
if value["root_sha256"] != expected_root or value["binary_sha256"] != expected_binary:
    raise SystemExit(1)
surfaces = value["surfaces"]
if not isinstance(surfaces, list) or len(surfaces) != 4:
    raise SystemExit(1)
identities = {(item.get("host"), item.get("surface")) for item in surfaces if isinstance(item, dict)}
if identities != {("claude", "hooks"), ("claude", "mcp"), ("codex", "hooks"), ("codex", "mcp")}:
    raise SystemExit(1)
for item in surfaces:
    if set(item) != {"host", "surface", "semantic_sha256", "sha256"}:
        raise SystemExit(1)
    if not isinstance(item["semantic_sha256"], str) or sha.fullmatch(item["semantic_sha256"]) is None:
        raise SystemExit(1)
    if not isinstance(item["sha256"], str) or sha.fullmatch(item["sha256"]) is None:
        raise SystemExit(1)
PY
}

validate_install_dry_run() {
  python3 - "$1" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    value = json.load(handle)
if value.get("ok") is not True or value.get("dry_run") is not True or value.get("project_local") is not True:
    raise SystemExit(1)
hosts = value.get("hosts")
if not isinstance(hosts, list) or sorted(item.get("host") for item in hosts) != ["claude", "codex"]:
    raise SystemExit(1)
if any(item.get("ok") is not True or item.get("dry_run") is not True for item in hosts):
    raise SystemExit(1)
for item in value.get("files", []):
    if item.get("written") is True:
        raise SystemExit(1)
for item in value.get("links", []):
    if item.get("created") is True:
        raise SystemExit(1)
PY
}

validate_observation() {
  python3 - "$1" <<'PY'
import json
import re
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    value = json.load(handle)
expected = {"session_start_observed", "pre_tool_use_observed", "mcp_call_count", "response_sha256", "exit_code", "duration_ms"}
if set(value) != expected:
    raise SystemExit(1)
if value["session_start_observed"] is not True or value["pre_tool_use_observed"] is not True:
    raise SystemExit(1)
if type(value["mcp_call_count"]) is not int or value["mcp_call_count"] != 1:
    raise SystemExit(1)
if not isinstance(value["response_sha256"], str) or re.fullmatch(r"[0-9a-f]{64}", value["response_sha256"]) is None:
    raise SystemExit(1)
if type(value["exit_code"]) is not int or value["exit_code"] != 0:
    raise SystemExit(1)
if type(value["duration_ms"]) is not int or value["duration_ms"] < 0:
    raise SystemExit(1)
PY
}

emit_receipt() {
  local verdict="$1"
  python3 - "$json_out" "$issue" "$local_head" "$remote_head" "$child_binary_sha256" "$before_file" "$activated_file" "$codex_observation" "$claude_observation" "$restore_file" "$codex_version" "$claude_version" "$verdict" <<'PY'
import json
import os
import re
import tempfile
import sys

(output, issue, local_head, remote_head, child_binary, before_path, activated_path,
 codex_path, claude_path, restore_path, codex_version, claude_version, verdict) = sys.argv[1:]

empty_digest = {"root_sha256": "", "binary_sha256": "", "surfaces": []}
empty_host = {
    "session_start_observed": False,
    "pre_tool_use_observed": False,
    "mcp_call_count": 0,
    "response_sha256": "",
    "exit_code": -1,
    "duration_ms": 0,
}

def load(path, default):
    try:
        with open(path, encoding="utf-8") as handle:
            return json.load(handle)
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return dict(default)

def load_host(path):
    expected = set(empty_host)
    try:
        if os.path.getsize(path) > 64 << 10:
            return dict(empty_host)
        with open(path, encoding="utf-8") as handle:
            value = json.load(handle)
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return dict(empty_host)
    if not isinstance(value, dict) or set(value) != expected:
        return dict(empty_host)
    if type(value["session_start_observed"]) is not bool or type(value["pre_tool_use_observed"]) is not bool:
        return dict(empty_host)
    if type(value["mcp_call_count"]) is not int or type(value["exit_code"]) is not int or type(value["duration_ms"]) is not int:
        return dict(empty_host)
    if not isinstance(value["response_sha256"], str) or (value["response_sha256"] and re.fullmatch(r"[0-9a-f]{64}", value["response_sha256"]) is None):
        return dict(empty_host)
    return {key: value[key] for key in empty_host}

before = load(before_path, empty_digest)
activated = load(activated_path, empty_digest)
restore = load(restore_path, empty_digest)
codex = load_host(codex_path)
claude = load_host(claude_path)
codex["version"] = codex_version
claude["version"] = claude_version
receipt = {
    "schema_version": 1,
    "issue": int(issue),
    "local_head": local_head,
    "remote_head": remote_head,
    "child_binary_sha256": child_binary,
    "before": before,
    "activated": activated,
    "activated_root_sha256": activated.get("root_sha256", ""),
    "activated_binary_sha256": activated.get("binary_sha256", ""),
    "codex": codex,
    "claude": claude,
    "restore": restore,
    "verdict": verdict,
}
parent = os.path.dirname(output)
descriptor, temporary = tempfile.mkstemp(prefix=".child-host-smoke-", dir=parent)
try:
    os.fchmod(descriptor, 0o600)
    with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
        json.dump(receipt, handle, sort_keys=True, separators=(",", ":"))
        handle.write("\n")
        handle.flush()
        os.fsync(handle.fileno())
    os.replace(temporary, output)
    directory = os.open(parent, os.O_RDONLY)
    try:
        os.fsync(directory)
    finally:
        os.close(directory)
finally:
    try:
        os.unlink(temporary)
    except FileNotFoundError:
        pass
PY
}

finish() {
  local requested_verdict="$1"
  local return_code="$2"
  local verdict="$requested_verdict"
  restoring=1
  set +e
  HARNESS_ROOT="$source_root" "$child_root/scripts/install-native.sh" --skip-build --path-mode=skip --json >"$temporary_root/restore-install.json" 2>"$temporary_root/restore-install.err"
  local restore_install_status=$?
  restore_activation_snapshot "$activation_snapshot"
  local restore_snapshot_status=$?
  validate_managed_activation_identity "$source_root"
  local restore_identity_status=$?
  host_mcp_readback "$source_root" restore
  local restore_mcp_status=$?
  activation_digest "$source_root" "$restore_file"
  local restore_digest_status=$?
  validate_activation_digest "$restore_file" "$source_root" >/dev/null 2>&1
  local restore_digest_contract_status=$?
  local exact_restore_status=0
  cmp -s "$before_file" "$restore_file" || exact_restore_status=$?
  if ((restore_install_status != 0 || restore_snapshot_status != 0 || restore_identity_status != 0 || restore_mcp_status != 0 || restore_digest_status != 0 || restore_digest_contract_status != 0 || exact_restore_status != 0)); then
    printf 'child-host-smoke: restore stages failed: install=%d snapshot=%d identity=%d mcp=%d digest=%d contract=%d exact=%d\n' \
      "$restore_install_status" "$restore_snapshot_status" "$restore_identity_status" "$restore_mcp_status" \
      "$restore_digest_status" "$restore_digest_contract_status" "$exact_restore_status" >&2
    verdict="fail"
    return_code=1
  fi
  if ((pending_signal != 0)); then
    verdict="fail"
    return_code="$pending_signal"
  fi
  if rmdir "$lock_path" 2>/dev/null; then
    lock_held=0
  else
    printf 'child-host-smoke: activation lock release failed\n' >&2
    verdict="fail"
    return_code=1
  fi
  emit_receipt "$verdict"
  if (($? != 0)); then
    return_code=1
  fi
  if ((pending_signal != 0)); then
    verdict="fail"
    return_code="$pending_signal"
    emit_receipt "$verdict" >/dev/null 2>&1 || return_code=1
  fi
  finalized=1
  cleanup || return_code=1
  if ((pending_signal != 0)); then
    return_code="$pending_signal"
  fi
  trap - EXIT
  exit "$return_code"
}

fail_after_mutation() {
  printf 'child-host-smoke: %s\n' "$1" >&2
  finish fail 1
}

on_exit() {
  local status=$?
  if ((finalized == 0 && mutation_started == 1)); then
    finish fail "${status:-1}"
  fi
  cleanup || status=1
  trap - EXIT
  exit "$status"
}
on_signal() {
  local signal_status="$1"
  if ((restoring == 1)); then
    if ((pending_signal == 0)); then
      pending_signal="$signal_status"
    fi
    return
  fi
  exit "$signal_status"
}
trap on_exit EXIT
trap 'on_signal 130' INT
trap 'on_signal 143' TERM

receipt_ready=1
[[ "$confirmed" == 1 ]] || fail_before_mutation 'explicit --confirm-user-activation is required'
umask 077
mkdir -p "$state_root"
python3 - "$state_root" <<'PY' || fail_before_mutation 'state root must be a private real directory'
import os
import stat
import sys

info = os.lstat(sys.argv[1])
if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o700:
    raise SystemExit(1)
PY
mkdir "$lock_path" 2>/dev/null || fail_before_mutation 'another child-host smoke holds the activation lock'
lock_held=1

[[ -z "$(git -C "$child_root" status --porcelain)" ]] || fail_before_mutation 'child worktree must be clean'
local_head="$(git -C "$child_root" rev-parse HEAD)" || fail_before_mutation 'cannot read child HEAD'
[[ "$local_head" == "$expected_head" ]] || fail_before_mutation 'child HEAD does not match requested head'
remote_output="$(git -C "$child_root" ls-remote origin "$remote_ref")" || fail_before_mutation 'cannot read exact remote ref'
[[ "$(printf '%s\n' "$remote_output" | wc -l | tr -d ' ')" == 1 ]] || fail_before_mutation 'remote ref must resolve exactly once'
IFS=$'\t' read -r remote_head remote_name remote_extra <<<"$remote_output"
[[ "$remote_head" == "$expected_head" && "$remote_name" == "$remote_ref" && -z "${remote_extra:-}" ]] || fail_before_mutation 'remote ref does not match requested head'

codex_version="$(bounded_version codex)" || fail_before_mutation 'Codex version probe failed'
claude_version="$(bounded_version claude)" || fail_before_mutation 'Claude version probe failed'

(cd "$child_root" && go build -o "$child_root/bin/agent-harness" ./cmd/harness) || fail_before_mutation 'child build failed'
child_binary="$child_root/bin/agent-harness"
[[ -x "$child_binary" ]] || fail_before_mutation 'child binary is not executable'
"$child_binary" version >/dev/null || fail_before_mutation 'child binary version check failed'
child_binary_sha256="$(file_sha256 "$child_binary")" || fail_before_mutation 'child binary digest failed'

dry_home="$temporary_root/dry-home"
mkdir -p "$dry_home/.codex"
chmod 0700 "$dry_home" "$dry_home/.codex"
HOME="$dry_home" CODEX_HOME="$dry_home/.codex" HARNESS_ROOT="$child_root" \
  "$child_binary" install --dry-run --project-local --path-mode=skip --json >"$dry_run_file" || fail_before_mutation 'child install dry-run failed'
validate_install_dry_run "$dry_run_file" || fail_before_mutation 'child install dry-run contract failed'
validate_managed_activation_identity "$source_root" || fail_before_mutation 'current activation does not belong to source-root'
host_mcp_readback "$source_root" before || fail_before_mutation 'source host-native MCP readback failed'
activation_digest "$source_root" "$before_file" || fail_before_mutation 'cannot capture source activation digest'
validate_activation_digest "$before_file" "$source_root" || fail_before_mutation 'source activation digest contract failed'
capture_activation_snapshot "$activation_snapshot" || fail_before_mutation 'cannot capture exact source activation snapshot'

mutation_started=1
HARNESS_ROOT="$child_root" "$child_root/scripts/install-native.sh" --skip-build --path-mode=skip --json >"$temporary_root/activate.json" 2>"$temporary_root/activate.err" || fail_after_mutation 'child activation failed'
validate_managed_activation_identity "$child_root" || fail_after_mutation 'activated managed identity drifted'
host_mcp_readback "$child_root" activated || fail_after_mutation 'activated host-native MCP readback failed'
instrument_claude_child_smoke_hooks "$child_root" "$claude_observation" || fail_after_mutation 'Claude child smoke hook instrumentation failed'
activation_digest "$child_root" "$activated_file" || fail_after_mutation 'activated surface digest failed'
validate_activation_digest "$activated_file" "$child_root" || fail_after_mutation 'activated surface digest contract failed'
activated_binary_sha256="$(python3 - "$activated_file" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as handle:
    print(json.load(handle)["binary_sha256"])
PY
)" || fail_after_mutation 'activated binary digest missing'
[[ "$activated_binary_sha256" == "$child_binary_sha256" ]] || fail_after_mutation 'activated binary identity drifted'

[[ "$(bounded_version codex)" == "$codex_version" ]] || fail_after_mutation 'Codex version drifted after activation'
[[ "$(bounded_version claude)" == "$claude_version" ]] || fail_after_mutation 'Claude version drifted after activation'

evidence_dir="$child_root/.agent-harness/evidence/child-host-smoke"
for host in codex claude; do
  observation="$temporary_root/$host-observation.json"
  (
    cd "$child_root"
    HARNESS_TOOL_CONFORMANCE_LIVE=1 \
    HARNESS_CHILD_SMOKE_HOOKS=1 \
    HARNESS_CHILD_SMOKE_OBSERVATION_FILE="$observation" \
      "$child_binary" contract conformance live \
        --hosts "$host" \
        --only "$host:empty_object" \
        --profile clean \
        --target-completed 1 \
        --max-attempts-per-case 1 \
        --evidence-dir "$evidence_dir" \
        --json
  ) >"$temporary_root/$host-live.json" || fail_after_mutation "$host live session failed"
  validate_observation "$observation" || fail_after_mutation "$host native event observation failed"
done

finish pass 0
