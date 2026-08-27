package harnessapp

import (
	"context"
	"os"
	"path/filepath"

	statestore "agent-harness/internal/adapter/outbound/state"
	upstreamadapter "agent-harness/internal/adapter/outbound/upstream"
	upstreamapp "agent-harness/internal/application/upstream"
	upstreamcontract "agent-harness/internal/contract/upstream"
)

// upstreamConfigFile is the harness-owned declaration of upstream host plugins
// and skills the harness should provision when they are missing.
const upstreamConfigFile = "upstream.json"

// syncUpstream provisions declared upstream plugins and skills for the Claude
// Code host. Plugins are owned by the host CLI; skills are fetched into a
// harness-owned cache under the state directory and linked into the host skill
// directory the same way the harness links its own skills.
func syncUpstream(ctx context.Context, root string, dryRun bool) (upstreamcontract.Report, error) {
	cfg, err := upstreamadapter.ReadConfig(filepath.Join(root, "configs", upstreamConfigFile))
	if err != nil {
		return upstreamcontract.Report{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return upstreamcontract.Report{}, err
	}
	service := upstreamapp.Service{
		Plugins: upstreamadapter.ClaudePluginHost{},
		Skills: upstreamadapter.GitSkillStore{
			SkillsDir: filepath.Join(home, ".claude", "skills"),
			CacheDir:  filepath.Join(statestore.StateDir(), "upstream", "skills"),
		},
	}
	return service.Sync(ctx, cfg, dryRun)
}
