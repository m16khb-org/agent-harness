package adapter_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	claudeadapter "agent-harness/internal/adapter/claude"
	codexadapter "agent-harness/internal/adapter/codex"
	"agent-harness/internal/core"
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

			req := core.DefaultNativeInstallRequest(root, home, codexHome, binPath)
			req.ProjectLocal = tc.projectLocal
			result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
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

	req := core.DefaultNativeInstallRequest(root, home, codexHome, binPath)
	req.ProjectLocal = true
	req.DryRun = true
	result, err := core.InstallNative(req, codexadapter.NewInstaller(), claudeadapter.NewInstaller())
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
		filepath.Join(root, ".mcp.json"),
		filepath.Join(root, ".claude"),
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
	if len(result.Hosts) != 2 || result.Hosts[0].Host != "codex" || result.Hosts[1].Host != "claude" {
		t.Fatalf("host order/coverage drifted: %+v", result.Hosts)
	}
	if got := strings.Join(result.SkillNames, ","); got != "alpha,beta,claude-only,codex-only" {
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
	if req.ProjectLocal {
		for _, skill := range []string{"alpha", "beta", "claude-only"} {
			assertRootSkillSymlink(t, filepath.Join(req.Root, ".claude", "skills", skill), filepath.Join(req.Root, "skills", skill))
		}
		assertPathMissing(t, filepath.Join(req.Root, ".claude", "skills", "codex-only"))
		for _, path := range []string{filepath.Join(req.Root, ".mcp.json")} {
			if !exists(path) {
				t.Fatalf("project-local opt-in did not write %s", path)
			}
		}
	} else {
		for _, path := range []string{filepath.Join(req.Root, ".mcp.json"), filepath.Join(req.Root, ".claude")} {
			if exists(path) {
				t.Fatalf("default install must not create repo-local path %s", path)
			}
		}
	}
	claudeSettings := readFile(t, filepath.Join(req.Home, ".claude", "settings.json"))
	for _, needle := range []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", req.BinPath} {
		if !strings.Contains(claudeSettings, needle) {
			t.Fatalf("Claude settings missing lifecycle hook %q:\n%s", needle, claudeSettings)
		}
	}
	if !strings.Contains(claudeSettings, "hook pre-tool-use --host claude --enforce-worktree") {
		t.Fatalf("Claude PreToolUse hook missing native host identity:\n%s", claudeSettings)
	}
	codexConfig := readFile(t, filepath.Join(req.CodexHome, "config.toml"))
	for _, needle := range []string{"[mcp_servers.agent_harness]", req.BinPath, req.Root} {
		if !strings.Contains(codexConfig, needle) {
			t.Fatalf("Codex config missing %q:\n%s", needle, codexConfig)
		}
	}
	codexHooks := readFile(t, filepath.Join(req.CodexHome, "hooks.json"))
	if !strings.Contains(codexHooks, "hook pre-tool-use --host codex --enforce-worktree") {
		t.Fatalf("Codex PreToolUse hook missing native host identity:\n%s", codexHooks)
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
			"Codex and Claude user skill installs are symlinks resolving to $ROOT/skills/*",
			"Codex and Claude user-level lifecycle hooks route through the same agent-harness hook CLI",
			"default install writes no repo-local .claude or .mcp.json paths",
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
			hostSnapshot.Files = append(hostSnapshot.Files, installContractFileSnapshot{
				Kind:          file.Kind,
				Path:          normalizeInstallContractString(file.Path, req),
				Written:       file.Written,
				ContentSHA256: sha256Hex(content),
				Content:       content,
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
