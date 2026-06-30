// Package gjc installs agent-harness into GJC (gajae-code) sessions.
//
// GJC integration has three surfaces, each matching a distinct GJC discovery
// path discovered in src/discovery/builtin.ts of @gajae-code/coding-agent:
//
//  1. Skill symlinks under ~/.gjc/agent/skills/ — GJC's loadSkills scans the
//     user-agent dir. Same shape as the Codex/Claude/Reasonix skill links.
//  2. Pre/post tool hook shell scripts under ~/.gjc/agent/hooks/{pre,post}/ —
//     GJC's loadHooks treats files in these dirs as shell scripts run around
//     every tool call. Only pre/post-tool exist in GJC's model, so only those
//     two agent-harness lifecycle hooks are wired (user-prompt / compact /
//     session-start / stop have no GJC equivalent).
//  3. GJC plugin bundle install (`gjc plugin install`) for always-on MCP — the
//     gajae-plugin.json at the repo root registers the agent-harness stdio MCP
//     server so it auto-loads in new GJC sessions without /mcp reload.
package gjc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

const pluginManifestName = "gajae-plugin.json"

// Installer implements port.HostInstaller for GJC.
type Installer struct{}

// NewInstaller returns the GJC host installer.
func NewInstaller() Installer { return Installer{} }

// Name reports the host identifier used in skill install.json host filters.
func (Installer) Name() string { return "gjc" }

// Install plans and applies the GJC integration surfaces.
func (Installer) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	plan := installutil.NewPlan("gjc", req.DryRun)

	enabled, skipped := installutil.SkillNamesForHost(req.Root, req.SkillNames, "gjc")
	for _, s := range skipped {
		plan.Message("skip skill for gjc: " + s)
	}

	gjcAgentDir := filepath.Join(req.Home, ".gjc", "agent")
	homeWriteable := req.DryRun || canWriteTo(gjcAgentDir)

	if homeWriteable {
		gjcSkillDir := filepath.Join(gjcAgentDir, "skills")
		links, linkErrs := installutil.PlanSkillLinks(req.Root, gjcSkillDir, enabled, req.DryRun)
		plan.Links(links)
		plan.Errs(linkErrs)

		hookFile, hookErr := writeGJCHookShim(filepath.Join(gjcAgentDir, "hooks"), req.Root, req)
		plan.File(hookFile, hookErr)

		if req.ProjectLocal {
			for _, skillName := range enabled {
				plan.Link(installutil.EnsureSymlinkPlan(filepath.ToSlash(filepath.Join("..", "..", "skills", skillName)), filepath.Join(req.Root, ".gjc", "skills", skillName), req.DryRun))
			}
		}
	} else {
		plan.Message("GJC user directory ~/.gjc/agent not writable under current sandbox; skipping skill links and hooks")
		plan.Message("run `./bin/agent-harness update` in a regular terminal to enable GJC integration")
	}

	// Plugin bundle install (always-on MCP). The manifest lives at the repo
	// root; GJC's `gjc plugin install` is the canonical installer and owns the
	// registry/hash/drift bookkeeping, so this adapter delegates rather than
	// copying files itself.
	manifestPath := filepath.Join(req.Root, pluginManifestName)
	if _, err := os.Stat(manifestPath); err != nil {
		plan.Message("skip GJC plugin bundle install: " + pluginManifestName + " not found at repo root")
		plan.Message("add a gajae-plugin.json to enable always-on agent-harness MCP in GJC sessions")
	} else if req.DryRun {
		plan.Message(fmt.Sprintf("dry-run: would run `gjc plugin install %s --user --force`", req.Root))
	} else if gjcPath, err := exec.LookPath("gjc"); err != nil {
		plan.Message("gjc not found on PATH; skipping plugin bundle install")
		plan.Message("install GJC (https://github.com/Yeachan-Heo/gajae-code) to enable always-on agent-harness MCP")
	} else {
		out, runErr := exec.Command(gjcPath, "plugin", "install", req.Root, "--user", "--force").CombinedOutput()
		if runErr != nil {
			plan.Message("gjc plugin install failed: " + runErr.Error())
			if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
				plan.Message("gjc output: " + trimmed)
			}
			plan.Err(fmt.Errorf("gjc plugin install failed: %w", runErr))
		} else {
			plan.Message("installed GJC plugin bundle agent-harness (user); agent-harness MCP auto-loads in new GJC sessions")
		}
	}

	return plan.Finish()
}

// canWriteTo reports whether a directory can be used for home-dir writes. It
// mirrors the Reasonix adapter's sandbox-tolerant probe: ensure the directory
// exists (as on a fresh install where ~/.gjc has never been created), then probe
// with a temp file. Returning false lets the installer skip home-dir writes
// gracefully under a sandbox while still succeeding on a normal first install.
func canWriteTo(dir string) bool {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".harness-write-test-*")
	if err != nil {
		return false
	}
	f.Close()
	_ = os.Remove(f.Name())
	return true
}
