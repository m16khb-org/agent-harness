package adapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	install "agent-harness/internal/adapter/install"
	omoadapter "agent-harness/internal/adapter/omo"
	"agent-harness/internal/port"
)

var updateAdapterContractGolden = flag.Bool("update-adapter-contract", false, "update adapter install contract golden files")

type installContractSnapshot struct {
	Cases []installContractCaseSnapshot `json:"cases"`
}

type installContractCaseSnapshot struct {
	Name         string                        `json:"name"`
	ProjectLocal bool                          `json:"project_local"`
	OK           bool                          `json:"ok"`
	SkillNames   []string                      `json:"skill_names"`
	Hosts        []installContractHostSnapshot `json:"hosts"`
	Assertions   []string                      `json:"assertions"`
}

type installContractHostSnapshot struct {
	Host     string                        `json:"host"`
	OK       bool                          `json:"ok"`
	Files    []installContractFileSnapshot `json:"files"`
	Links    []installContractLinkSnapshot `json:"links"`
	Messages []string                      `json:"messages,omitempty"`
}

type installContractFileSnapshot struct {
	Kind          string `json:"kind"`
	Path          string `json:"path"`
	Written       bool   `json:"written"`
	ContentSHA256 string `json:"content_sha256"`
	Content       string `json:"content"`
}

type installContractLinkSnapshot struct {
	Path           string `json:"path"`
	Target         string `json:"target"`
	Created        bool   `json:"created"`
	ResolvesToRoot bool   `json:"resolves_to_root_skill"`
}

func TestNativeInstallAdapterContractMatrix(t *testing.T) {
	cases := []struct {
		name         string
		projectLocal bool
	}{
		{name: "user-global-default", projectLocal: false},
		{name: "project-local-opt-in", projectLocal: true},
	}

	snapshot := installContractSnapshot{Cases: []installContractCaseSnapshot{}}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			home := t.TempDir()
			codexHome := filepath.Join(home, ".codex")
			binPath := filepath.Join(root, "bin", "harness")
			writeContractSkill(t, root, "beta")
			writeContractSkill(t, root, "alpha")
			writeContractSkill(t, root, "codex-only", "codex")
			writeContractSkill(t, root, "claude-only", "claude")
			writeContractSkill(t, root, "omo-only", "omo")

			req := install.DefaultNativeInstallRequest(root, home, codexHome, binPath)
			req.ProjectLocal = tc.projectLocal
			result, err := install.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller(), omoadapter.NewInstaller())
			if err != nil {
				t.Fatalf("InstallNative returned error: %v\n%+v", err, result)
			}
			assertInstallContractSemantics(t, req, result)
			snapshot.Cases = append(snapshot.Cases, normalizeInstallContractCase(t, tc.name, req, result))
		})
	}
	sort.Slice(snapshot.Cases, func(i, j int) bool { return snapshot.Cases[i].Name < snapshot.Cases[j].Name })
	assertAdapterContractGolden(t, "native_install_contract_matrix.golden.json", snapshot)
}

func TestNativeInstallDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	binPath := filepath.Join(root, "bin", "harness")
	writeContractSkill(t, root, "alpha")

	req := install.DefaultNativeInstallRequest(root, home, codexHome, binPath)
	req.ProjectLocal = true
	req.DryRun = true
	result, err := install.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller(), omoadapter.NewInstaller())
	if err != nil {
		t.Fatalf("dry-run InstallNative returned error: %v\n%+v", err, result)
	}
	if !result.OK || !result.DryRun {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	for _, path := range []string{
		filepath.Join(codexHome, "skills", "alpha"),
		filepath.Join(codexHome, "config.toml"),
		filepath.Join(home, ".claude", "skills", "alpha"),
		filepath.Join(home, ".omo"),
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, ".claude"),
		filepath.Join(root, ".omo"),
		filepath.Join(root, "configs"),
	} {
		if exists(path) {
			t.Fatalf("dry-run wrote unexpected path %s", path)
		}
	}
	if !hasPlannedWrite(result) || !hasPlannedLink(result) {
		t.Fatalf("dry-run did not expose planned files and links: %+v", result)
	}
}

func TestInstallNativeScriptDoesNotWireCompanionTools(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	for _, gone := range []string{
		"install_claude_mem_for_ide \"codex-cli\"",
		"install_claude_mem_for_ide \"claude-code\"",
		"ensure_codex_plugin \"claude-mem@claude-mem-local\"",
		"ensure_claude_plugin \"claude-mem@thedotmack\"",
		"remove_codex_plugin \"agentmemory@agentmemory\"",
		"remove_codex_marketplace \"agentmemory\"",
		"remove_claude_plugin \"agentmemory@agentmemory\"",
		"remove_claude_marketplace \"agentmemory\"",
		"ensure_agentmemory_cli",
		"refresh_agentmemory_host_wiring",
		"ensure_codex_marketplace \"agentmemory\"",
		"ensure_codex_plugin \"agentmemory@agentmemory\"",
		"ensure_claude_marketplace \"agentmemory\"",
		"ensure_claude_plugin \"agentmemory@agentmemory\"",
		"npm install -g @agentmemory/agentmemory",
		`[mcp_servers.llm-wiki]`,
		`[mcp_servers.llm-wiki.env]`,
		`LLM_WIKI_VAULT = {vault}`,
		`"env": {"LLM_WIKI_VAULT": vault_path}`,
		`claude mcp add-json -s user llm-wiki`,
		`remove_codex_plugin "wiki@llm-wiki"`,
		`remove_codex_marketplace "llm-wiki"`,
		`remove_claude_plugin "wiki@llm-wiki"`,
		`remove_claude_marketplace "llm-wiki"`,
		`ensure_codex_marketplace "llm-wiki" "nvk/llm-wiki"`,
		`ensure_codex_plugin "wiki@llm-wiki"`,
		`ensure_claude_marketplace "llm-wiki" "nvk/llm-wiki"`,
		`ensure_claude_plugin "wiki@llm-wiki"`,
		`llm-wiki Codex source is nvk/llm-wiki`,
		`llm-wiki Claude source is nvk/llm-wiki`,
		"lazycodex-ai",
		"HARNESS_INSTALL_UPSTREAM_TOOLS",
		"HARNESS_INIT_CODEGRAPH",
		"codegraph install --target=codex,claude",
		"npm install -g @colbymchenry/codegraph",
	} {
		if strings.Contains(script, gone) {
			t.Fatalf("install-native.sh must not retain companion tool installer path %q", gone)
		}
	}
}

func TestInstallNativeScriptBindsStagedCandidateBeforeReplacement(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	begin := strings.Index(script, "HARNESS_NATIVE_ACTIVATION_STEP=begin")
	replace := strings.Index(script, "os.replace(source, target)")
	seal := strings.Index(script, "HARNESS_NATIVE_ACTIVATION_STEP=seal")
	if begin < 0 || replace < 0 || seal < 0 || !(begin < replace && replace < seal) {
		t.Fatalf("native activation order must be staged begin -> atomic replace -> canonical seal: begin=%d replace=%d seal=%d", begin, replace, seal)
	}
}

func TestInstallNativeScriptRefusesRegularCommandBeforeActivationBegin(t *testing.T) {
	harnessRoot := t.TempDir()
	home := t.TempDir()
	binDir := filepath.Join(harnessRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	commandDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "agent-harness"), []byte("prior managed command\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	callsPath := filepath.Join(harnessRoot, "calls")
	pendingPath := filepath.Join(harnessRoot, "pending")
	receiptPath := filepath.Join(harnessRoot, "receipt")
	const priorReceipt = "prior sealed receipt\n"
	if err := os.WriteFile(receiptPath, []byte(priorReceipt), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBinary := `#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
  version)
    exit 0
    ;;
  install)
    dry_run=0
    for arg in "$@"; do
      if [[ "$arg" == "--dry-run" ]]; then
        dry_run=1
      fi
    done
    case "${HARNESS_NATIVE_ACTIVATION_STEP:-}" in
      begin)
        printf 'begin\n' >>"$FAKE_CALLS"
        rm -f -- "$FAKE_RECEIPT"
        printf 'pending\n' >"$FAKE_PENDING"
        printf '{"transition_id":"0123456789abcdef0123456789abcdef","binary_sha256":"%s"}\n' "$FAKE_BINARY_SHA256"
        ;;
      abort)
        printf 'abort\n' >>"$FAKE_CALLS"
        rm -f -- "$FAKE_PENDING"
        printf '{"aborted":true}\n'
        ;;
      *)
        if [[ "$dry_run" == "1" ]]; then
          printf 'preflight\n' >>"$FAKE_CALLS"
        else
          printf 'seal\n' >>"$FAKE_CALLS"
        fi
        if [[ -f "$HOME/.local/bin/agent-harness" ]]; then
          printf 'refusing to replace non-symlink command path\n' >&2
          exit 1
        fi
        exit 2
        ;;
    esac
    ;;
  *)
    exit 2
    ;;
esac
`
	binPath := filepath.Join(binDir, "agent-harness")
	if err := os.WriteFile(binPath, []byte(fakeBinary), 0o755); err != nil {
		t.Fatal(err)
	}

	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "scripts", "install-native.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", scriptPath, "--skip-build", "--json")
	cmd.Env = append(os.Environ(),
		"HARNESS_ROOT="+harnessRoot,
		"HARNESS_SKIP_BUILD=1",
		"HOME="+home,
		"FAKE_CALLS="+callsPath,
		"FAKE_PENDING="+pendingPath,
		"FAKE_RECEIPT="+receiptPath,
		"FAKE_BINARY_SHA256="+sha256Hex(fakeBinary),
	)
	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("unapproved regular command path unexpectedly installed: %s", output)
	}
	calls, err := os.ReadFile(callsPath)
	if err != nil {
		t.Fatal(err)
	}
	receipt, receiptErr := os.ReadFile(receiptPath)
	_, pendingErr := os.Stat(pendingPath)
	if got := strings.TrimSpace(string(calls)); got != "preflight" || receiptErr != nil || string(receipt) != priorReceipt || !os.IsNotExist(pendingErr) {
		t.Fatalf("refusal must occur before activation mutation: calls=%q receipt=%q receiptErr=%v pendingErr=%v output=%s", got, receipt, receiptErr, pendingErr, output)
	}
}

func TestInstallNativeScriptSupportsEmptyActivationArguments(t *testing.T) {
	harnessRoot := t.TempDir()
	home := t.TempDir()
	binDir := filepath.Join(harnessRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	callsPath := filepath.Join(harnessRoot, "calls")
	fakeBinary := `#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
  version)
    exit 0
    ;;
  install)
    case "${HARNESS_NATIVE_ACTIVATION_STEP:-}" in
      begin)
        printf 'begin\n' >>"$FAKE_CALLS"
        printf '{"transition_id":"0123456789abcdef0123456789abcdef","binary_sha256":"%s"}\n' "$FAKE_BINARY_SHA256"
        ;;
      seal)
        printf 'seal\n' >>"$FAKE_CALLS"
        printf '{"committed":true,"transition_id":"%s"}\n' "$HARNESS_NATIVE_ACTIVATION_TRANSITION_ID"
        ;;
      *)
        printf 'preflight\n' >>"$FAKE_CALLS"
        ;;
    esac
    ;;
  *)
    exit 2
    ;;
esac
`
	binPath := filepath.Join(binDir, "agent-harness")
	if err := os.WriteFile(binPath, []byte(fakeBinary), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "scripts", "install-native.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", scriptPath, "--skip-build")
	cmd.Env = append(os.Environ(),
		"HARNESS_ROOT="+harnessRoot,
		"HARNESS_SKIP_BUILD=1",
		"HOME="+home,
		"FAKE_CALLS="+callsPath,
		"FAKE_BINARY_SHA256="+sha256Hex(fakeBinary),
	)
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("empty activation argument arrays failed: %v\n%s", runErr, output)
	}
	calls, err := os.ReadFile(callsPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(calls)); got != "preflight\nbegin\nseal" {
		t.Fatalf("activation calls=%q output=%s", got, output)
	}
}

func TestInstallNativeScriptForwardsAdoptionAndOwnsExactTransitionCleanup(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	for _, want := range []string{
		"--adopt-command-file",
		"ACTIVATION_ARGS+=(\"$arg\")",
		"HARNESS_NATIVE_ACTIVATION_TRANSITION_ID=\"$ACTIVATION_TRANSITION_ID\"",
		"HARNESS_NATIVE_ACTIVATION_STEP=abort",
		"ACTIVATION_ABORTED=1",
		"ACTIVATION_COMMITTED=1",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install-native.sh transition/adoption contract missing %q", want)
		}
	}
	if strings.Count(script, "HARNESS_NATIVE_ACTIVATION_STEP=abort") != 1 {
		t.Fatal("install-native.sh must have exactly one abort invocation site")
	}
}

func TestNativeInstallAndChildSmokeScriptsHaveValidBashSyntax(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "scripts", "install-native.sh"),
		filepath.Join("..", "..", "scripts", "verify-child-host-smoke.sh"),
	} {
		if output, err := exec.Command("bash", "-n", path).CombinedOutput(); err != nil {
			t.Fatalf("bash -n %s: %v\n%s", path, err, output)
		}
	}
}

func TestReleaseReproSmokeUsesCurrentStateRoundTrip(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "release-repro-smoke.sh"))
	retired := strings.Join([]string{"state", "migrate"}, " ")
	if strings.Contains(script, retired) {
		t.Fatalf("release smoke still invokes retired state command %q", retired)
	}
	for _, want := range []string{"state write", "state read"} {
		if !strings.Contains(script, want) {
			t.Fatalf("release smoke does not exercise current %q path", want)
		}
	}
}

func TestInstallNativeScriptDocumentsCommandShims(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	for _, want := range []string{
		"~/.local/bin/agent-harness",
		"~/.local/bin/ah",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("install-native.sh user command help missing %q", want)
		}
	}
}

func TestInstallNativeScriptLeavesGlabMCPSyncExplicit(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	for _, forbidden := range []string{
		"sync-glab-mcp.sh",
		"GLAB_MCP_WRAPPER",
		"GLAB_MCP_PROFILES",
		"claude mcp ",
		"codex mcp ",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("install-native.sh must not invoke or probe explicit glab MCP sync state: %q", forbidden)
		}
	}
}

func TestInstallNativeScriptExcludesRemovedProxyCompanion(t *testing.T) {
	script := readFile(t, filepath.Join("..", "..", "scripts", "install-native.sh"))
	removed := strings.Join([]string{"head", "room"}, "")
	removedTitle := strings.Join([]string{"Head", "room"}, "")
	for _, gone := range []string{
		"install_" + removed + "_cli",
		"enable_" + removed + "_runtime",
		"--enable-" + removed + "-runtime",
		"HARNESS_ENABLE_" + strings.ToUpper(removed) + "_RUNTIME",
		"scripts/setup-" + removed + "-runtime.sh",
		"bash \"$ROOT/scripts/setup-" + removed + "-runtime.sh\"",
		removed + "-ai[all]",
		"pipx install --python python3.13 \"" + removed + "-ai[all]\"",
		"pipx upgrade " + removed + "-ai",
		strings.ToUpper(removed) + "_TELEMETRY=off",
		removedTitle,
		removed,
		removed + " wrap codex",
		removed + " wrap claude",
		removed + " proxy --port",
		removed + " learn",
	} {
		if strings.Contains(script, gone) {
			t.Fatalf("install-native.sh must not retain removed proxy companion integration %q", gone)
		}
	}

	if _, err := os.Stat(filepath.Join("..", "..", "scripts", "setup-"+removed+"-runtime.sh")); !os.IsNotExist(err) {
		t.Fatalf("removed proxy companion setup script must be removed, stat error: %v", err)
	}
}

func TestInstallNativeScriptActivatesLinkedWorktreeBuildAtStableSource(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	worktree := filepath.Join(base, "source.worktrees", "feature")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	runInstallScriptTestCommand(t, source, "git", "init", "-b", "main")
	scriptSource, err := filepath.Abs(filepath.Join("..", "..", "scripts", "install-native.sh"))
	if err != nil {
		t.Fatal(err)
	}
	scriptBytes, err := os.ReadFile(scriptSource)
	if err != nil {
		t.Fatal(err)
	}
	writeInstallScriptTestFile(t, filepath.Join(source, "scripts", "install-native.sh"), scriptBytes, 0o755)
	writeInstallScriptTestFile(t, filepath.Join(source, "README.md"), []byte("fixture\n"), 0o644)
	runInstallScriptTestCommand(t, source, "git", "add", ".")
	runInstallScriptTestCommand(t, source, "git", "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "fixture")
	runInstallScriptTestCommand(t, source, "git", "worktree", "add", "-b", "feature", worktree)

	stableBinary := filepath.Join(source, "bin", "agent-harness")
	writeInstallScriptTestFile(t, stableBinary, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	before := installScriptTestInode(t, stableBinary)
	fakeBin := filepath.Join(base, "fake-bin")
	fakeGo := `#!/bin/sh
set -eu
test "$1" = "build"
shift
output=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    output="$2"
    shift 2
    continue
  fi
  shift
done
test -n "$output"
cat >"$output" <<'RUNTIME'
#!/bin/sh
case "${1:-}" in
  version) exit 0 ;;
  issueops) exit 0 ;;
  install)
    printf '%s\n' "$HARNESS_ROOT" >"$FAKE_INSTALL_LOG"
    transition_id=0123456789abcdef0123456789abcdef
    case "${HARNESS_NATIVE_ACTIVATION_STEP:-}" in
      begin)
        binary_sha256="$(shasum -a 256 "$0" | awk '{print $1}')"
        printf '{"transition_id":"%s","binary_sha256":"%s"}\n' "$transition_id" "$binary_sha256"
        ;;
      seal)
        printf '{"committed":true,"transition_id":"%s"}\n' "${HARNESS_NATIVE_ACTIVATION_TRANSITION_ID:-$transition_id}"
        ;;
      abort)
        printf '{"aborted":true,"transition_id":"%s"}\n' "${HARNESS_NATIVE_ACTIVATION_TRANSITION_ID:-$transition_id}"
        ;;
    esac
    exit 0
    ;;
  *) exit 0 ;;
esac
# new-runtime
RUNTIME
chmod 0755 "$output"
`
	writeInstallScriptTestFile(t, filepath.Join(fakeBin, "go"), []byte(fakeGo), 0o755)
	installLog := filepath.Join(base, "installed-root.log")
	command := exec.Command("bash", filepath.Join(worktree, "scripts", "install-native.sh"), "--json")
	command.Dir = worktree
	command.Env = append(withoutInstallScriptTestEnv(os.Environ(), "HARNESS_ROOT", "HARNESS_SKIP_BUILD"),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_INSTALL_LOG="+installLog,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("install-native.sh failed: %v\n%s", err, output)
	}

	after := installScriptTestInode(t, stableBinary)
	if before == after {
		t.Fatalf("stable binary inode did not change: before=%d after=%d", before, after)
	}
	installed, err := os.ReadFile(stableBinary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), "new-runtime") {
		t.Fatalf("stable binary was not replaced: %q", installed)
	}
	if _, err := os.Stat(filepath.Join(worktree, "bin", "agent-harness")); !os.IsNotExist(err) {
		t.Fatalf("linked worktree retained a runtime binary: %v", err)
	}
	recordedRoot, err := os.ReadFile(installLog)
	if err != nil {
		t.Fatal(err)
	}
	recordedInfo, recordedErr := os.Stat(strings.TrimSpace(string(recordedRoot)))
	sourceInfo, sourceErr := os.Stat(source)
	if recordedErr != nil || sourceErr != nil || !os.SameFile(recordedInfo, sourceInfo) {
		t.Fatalf("installed HARNESS_ROOT = %q, want stable source %q", strings.TrimSpace(string(recordedRoot)), source)
	}
}

func runInstallScriptTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, output)
	}
}

func writeInstallScriptTestFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func installScriptTestInode(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat identity unavailable for %s", path)
	}
	return uint64(stat.Ino)
}

func withoutInstallScriptTestEnv(environment []string, names ...string) []string {
	blocked := map[string]bool{}
	for _, name := range names {
		blocked[name+"="] = true
	}
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		keep := true
		for prefix := range blocked {
			if strings.HasPrefix(entry, prefix) {
				keep = false
				break
			}
		}
		if keep {
			result = append(result, entry)
		}
	}
	return result
}

func writeContractSkill(t *testing.T, root, name string, hosts ...string) {
	t.Helper()
	dir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+name+" test skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "agents", "openai.yaml"), []byte("name: "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if len(hosts) > 0 {
		b, err := json.Marshal(map[string][]string{"hosts": hosts})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "install.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func hasPlannedWrite(result port.NativeInstallResult) bool {
	for _, file := range result.Files {
		if file.WouldWrite && !file.Written {
			return true
		}
	}
	return false
}

func hasPlannedLink(result port.NativeInstallResult) bool {
	for _, link := range result.Links {
		if link.WouldCreate && !link.Created {
			return true
		}
	}
	return false
}

func assertInstallContractSemantics(t *testing.T, req port.NativeInstallRequest, result port.NativeInstallResult) {
	t.Helper()
	if !result.OK {
		t.Fatalf("install result ok=false: %+v", result)
	}
	if len(result.Hosts) != 3 || result.Hosts[0].Host != "codex" || result.Hosts[1].Host != "claude" || result.Hosts[2].Host != "omo" {
		t.Fatalf("host order/coverage drifted: %+v", result.Hosts)
	}
	if got := strings.Join(result.SkillNames, ","); got != "alpha,beta,claude-only,codex-only,omo-only" {
		t.Fatalf("skill discovery must be deterministic and sorted, got %q", got)
	}
	for _, skill := range []string{"alpha", "beta", "codex-only"} {
		assertRootSkillSymlink(t, filepath.Join(req.CodexHome, "skills", skill), filepath.Join(req.Root, "skills", skill))
	}
	assertPathMissing(t, filepath.Join(req.CodexHome, "skills", "claude-only"))
	for _, skill := range []string{"alpha", "beta", "claude-only"} {
		assertRootSkillSymlink(t, filepath.Join(req.Home, ".claude", "skills", skill), filepath.Join(req.Root, "skills", skill))
	}
	assertPathMissing(t, filepath.Join(req.Home, ".claude", "skills", "codex-only"))
	assertPathMissing(t, filepath.Join(req.Home, ".claude", "skills", "omo-only"))
	for _, skill := range []string{"alpha", "beta", "omo-only"} {
		assertRootSkillSymlink(t, filepath.Join(req.Home, ".omo", "agent", "skills", skill), filepath.Join(req.Root, "skills", skill))
	}
	assertPathMissing(t, filepath.Join(req.Home, ".omo", "agent", "skills", "codex-only"))
	assertPathMissing(t, filepath.Join(req.Home, ".omo", "agent", "skills", "claude-only"))
	if req.ProjectLocal {
		for _, skill := range []string{"alpha", "beta", "claude-only"} {
			assertRootSkillSymlink(t, filepath.Join(req.Root, ".claude", "skills", skill), filepath.Join(req.Root, "skills", skill))
		}
		assertPathMissing(t, filepath.Join(req.Root, ".claude", "skills", "codex-only"))
		for _, skill := range []string{"alpha", "beta", "omo-only"} {
			assertRootSkillSymlink(t, filepath.Join(req.Root, ".omo", "skills", skill), filepath.Join(req.Root, "skills", skill))
		}
		assertPathMissing(t, filepath.Join(req.Root, ".omo", "skills", "codex-only"))
		assertPathMissing(t, filepath.Join(req.Root, ".omo", "skills", "claude-only"))
		for _, path := range []string{filepath.Join(req.Root, ".mcp.json"), filepath.Join(req.Root, ".omo", "mcp.json")} {
			if !exists(path) {
				t.Fatalf("project-local opt-in did not write %s", path)
			}
		}
	} else {
		for _, path := range []string{filepath.Join(req.Root, ".mcp.json"), filepath.Join(req.Root, ".claude"), filepath.Join(req.Root, ".omo")} {
			if exists(path) {
				t.Fatalf("default install must not create repo-local path %s", path)
			}
		}
	}
	claudeSettings := readFile(t, filepath.Join(req.Home, ".claude", "settings.json"))
	for _, needle := range []string{"SessionStart", req.BinPath, "hook session-start --host claude"} {
		if !strings.Contains(claudeSettings, needle) {
			t.Fatalf("Claude settings missing lifecycle hook %q:\n%s", needle, claudeSettings)
		}
	}
	for _, gone := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "PreCompact", "Stop", "--enforce-", "--relay-next-action-judgement"} {
		if strings.Contains(claudeSettings, gone) {
			t.Fatalf("Claude default hooks retained removed lifecycle surface %q:\n%s", gone, claudeSettings)
		}
	}
	codexConfig := readFile(t, filepath.Join(req.CodexHome, "config.toml"))
	for _, needle := range []string{"[mcp_servers.agent_harness]", req.BinPath, req.Root} {
		if !strings.Contains(codexConfig, needle) {
			t.Fatalf("Codex config missing %q:\n%s", needle, codexConfig)
		}
	}
	codexHooks := readFile(t, filepath.Join(req.CodexHome, "hooks.json"))
	for _, needle := range []string{"SessionStart", "hook session-start --host codex"} {
		if !strings.Contains(codexHooks, needle) {
			t.Fatalf("Codex hooks missing lifecycle hook %q:\n%s", needle, codexHooks)
		}
	}
	for _, gone := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "PreCompact", "Stop", "--enforce-", "--relay-next-action-judgement"} {
		if strings.Contains(codexHooks, gone) {
			t.Fatalf("Codex default hooks retained removed lifecycle surface %q:\n%s", gone, codexHooks)
		}
	}
	omoMCP := readFile(t, filepath.Join(req.Home, ".omo", "mcp.json"))
	for _, needle := range []string{`"agent_harness"`, req.BinPath, req.Root} {
		if !strings.Contains(omoMCP, needle) {
			t.Fatalf("Omo MCP config missing %q:\n%s", needle, omoMCP)
		}
	}
	omoExtension := readFile(t, filepath.Join(req.Home, ".omo", "extensions", "agent-harness.js"))
	for _, needle := range []string{`pi.on("session_start"`, `pi.on("session_compact"`, `"--json"`, req.BinPath} {
		if !strings.Contains(omoExtension, needle) {
			t.Fatalf("Omo lifecycle extension missing %q:\n%s", needle, omoExtension)
		}
	}
}

func assertRootSkillSymlink(t *testing.T, linkPath, wantTarget string) {
	t.Helper()
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("missing skill symlink %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("skill install path must be symlink, got non-symlink: %s", linkPath)
	}
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		t.Fatalf("cannot resolve symlink %s: %v", linkPath, err)
	}
	wantResolved, err := filepath.EvalSymlinks(wantTarget)
	if err != nil {
		t.Fatalf("cannot resolve target %s: %v", wantTarget, err)
	}
	if resolved != wantResolved {
		t.Fatalf("symlink %s resolves to %s, want %s", linkPath, resolved, wantResolved)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if exists(path) {
		t.Fatalf("path should not exist: %s", path)
	}
}

func normalizeInstallContractCase(t *testing.T, name string, req port.NativeInstallRequest, result port.NativeInstallResult) installContractCaseSnapshot {
	t.Helper()
	caseSnapshot := installContractCaseSnapshot{
		Name:         name,
		ProjectLocal: req.ProjectLocal,
		OK:           result.OK,
		SkillNames:   append([]string{}, result.SkillNames...),
		Hosts:        []installContractHostSnapshot{},
		Assertions: []string{
			"core discovers shared skills once and passes sorted names to all host adapters",
			"Codex, Claude, and Omo user skill installs are symlinks resolving to $ROOT/skills/*",
			"Codex, Claude, and Omo user-level lifecycle hooks route through the same agent-harness hook CLI",
			"default install writes no repo-local .claude, .omo, or .mcp.json paths",
			"project-local repo files are created only when project_local=true",
		},
	}
	for _, host := range result.Hosts {
		hostSnapshot := installContractHostSnapshot{Host: host.Host, OK: host.OK, Messages: append([]string{}, host.Messages...)}
		for _, file := range host.Files {
			content := ""
			if exists(file.Path) {
				content = normalizeInstallContractString(readFile(t, file.Path), req)
			}
			snapshotContent := content
			if host.Host == "omo" {
				snapshotContent = ""
			}
			hostSnapshot.Files = append(hostSnapshot.Files, installContractFileSnapshot{
				Kind:          file.Kind,
				Path:          normalizeInstallContractString(file.Path, req),
				Written:       file.Written,
				ContentSHA256: sha256Hex(content),
				Content:       snapshotContent,
			})
		}
		for _, link := range host.Links {
			hostSnapshot.Links = append(hostSnapshot.Links, installContractLinkSnapshot{
				Path:           normalizeInstallContractString(link.Path, req),
				Target:         normalizeInstallContractString(link.Target, req),
				Created:        link.Created,
				ResolvesToRoot: linkResolvesUnderRootSkills(link.Path, req.Root),
			})
		}
		sort.Slice(hostSnapshot.Files, func(i, j int) bool {
			if hostSnapshot.Files[i].Kind != hostSnapshot.Files[j].Kind {
				return hostSnapshot.Files[i].Kind < hostSnapshot.Files[j].Kind
			}
			return hostSnapshot.Files[i].Path < hostSnapshot.Files[j].Path
		})
		sort.Slice(hostSnapshot.Links, func(i, j int) bool { return hostSnapshot.Links[i].Path < hostSnapshot.Links[j].Path })
		caseSnapshot.Hosts = append(caseSnapshot.Hosts, hostSnapshot)
	}
	sort.Slice(caseSnapshot.Hosts, func(i, j int) bool { return caseSnapshot.Hosts[i].Host < caseSnapshot.Hosts[j].Host })
	return caseSnapshot
}

func linkResolvesUnderRootSkills(linkPath, root string) bool {
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil {
		return false
	}
	rootSkills, err := filepath.EvalSymlinks(filepath.Join(root, "skills"))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootSkills, resolved)
	return err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func normalizeInstallContractString(value string, req port.NativeInstallRequest) string {
	replacements := map[string]string{
		req.CodexHome: "$CODEX_HOME",
		req.Home:      "$HOME",
		req.Root:      "$ROOT",
		req.BinPath:   "$BIN",
	}
	keys := make([]string, 0, len(replacements))
	for from := range replacements {
		if from != "" {
			keys = append(keys, from)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, from := range keys {
		value = strings.ReplaceAll(value, from, replacements[from])
	}
	return value
}

func assertAdapterContractGolden(t *testing.T, name string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	path := filepath.Join("testdata", name)
	if *updateAdapterContractGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -update-adapter-contract)", path, err)
	}
	if string(b) != string(want) {
		t.Fatalf("adapter contract golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, string(b), string(want))
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
