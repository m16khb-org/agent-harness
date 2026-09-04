// Package upstream runs one upstream provisioning pass: observe the host, ask
// the domain what is missing, and provision only that. A pass never fails the
// caller — the issueops install path must not depend on a third-party plugin CLI
// or on network reachability.
package upstream

import (
	"context"
	"errors"

	upstreamcontract "issueops/internal/contract/upstream"
	upstreamdomain "issueops/internal/domain/upstream"
	upstreamport "issueops/internal/port/upstream"
)

// Service provisions declared upstream plugins and skills onto one host.
type Service struct {
	Plugins upstreamport.PluginHost
	Skills  upstreamport.SkillStore
}

// Sync provisions everything the host is missing and reports every declared
// entry.
//
// With dryRun nothing is provisioned and the plugin host CLI is not executed at
// all — not even to read its inventory. Executing it is itself a write: the
// Claude CLI creates $HOME/.claude and $HOME/.claude.json on startup, which made
// `install --dry-run` mutate the very home directory it was only describing. A
// dry-run therefore reports declared plugins as planned instead of checking
// which are already installed. Skills stay observed because that is a directory
// read with no side effects.
func (s Service) Sync(ctx context.Context, cfg upstreamcontract.Config, dryRun bool) (upstreamcontract.Report, error) {
	report := upstreamcontract.Report{DryRun: dryRun, Items: []upstreamcontract.ItemResult{}}
	pluginHostAvailable := s.Plugins != nil && s.Plugins.Available()
	observed := s.observe(ctx, pluginHostAvailable && !dryRun)

	items := upstreamdomain.PlanWithoutPluginHost(cfg, observed)
	if pluginHostAvailable {
		items = upstreamdomain.Plan(cfg, observed)
	}
	for _, item := range items {
		report.Items = append(report.Items, s.apply(ctx, item, dryRun))
	}
	return report, nil
}

// observe reads what the host already has. A host that cannot be read is
// treated as empty: the planner then proposes installs, and the install step
// itself reports the real failure, which is more actionable than a silent skip.
// queryPluginHost is false whenever the plugin CLI must not be executed — it is
// absent, or this is a dry-run, where spawning it would write to the home
// directory.
func (s Service) observe(ctx context.Context, queryPluginHost bool) upstreamcontract.Observed {
	observed := upstreamcontract.Observed{}
	if queryPluginHost {
		if plugins, err := s.Plugins.InstalledPlugins(ctx); err == nil {
			observed.Plugins = plugins
		}
		if marketplaces, err := s.Plugins.Marketplaces(ctx); err == nil {
			observed.Marketplaces = marketplaces
		}
	}
	if s.Skills != nil {
		if skills, err := s.Skills.InstalledSkills(); err == nil {
			observed.Skills = skills
		}
	}
	return observed
}

func (s Service) apply(ctx context.Context, item upstreamcontract.PlanItem, dryRun bool) upstreamcontract.ItemResult {
	result := upstreamcontract.ItemResult{Kind: item.Kind, Name: item.Name, Reason: item.Reason}
	if item.Action == upstreamcontract.ActionSkip {
		result.Status = upstreamcontract.StatusSkipped
		return result
	}
	if dryRun {
		result.Status = upstreamcontract.StatusPlanned
		return result
	}
	if err := s.install(ctx, item); err != nil {
		result.Status = upstreamcontract.StatusFailed
		result.Error = err.Error()
		return result
	}
	result.Status = upstreamcontract.StatusInstalled
	return result
}

func (s Service) install(ctx context.Context, item upstreamcontract.PlanItem) error {
	if item.Kind == upstreamcontract.KindSkill {
		// A missing injection is surfaced, never silently passed over: the
		// composition root owns which store provisions upstream skills.
		if s.Skills == nil {
			return errors.New("upstream skill store is not configured")
		}
		return s.Skills.InstallSkill(ctx, *item.Skill)
	}
	if item.AddMarketplace {
		if err := s.Plugins.AddMarketplace(ctx, item.Plugin.Source); err != nil {
			return err
		}
	}
	return s.Plugins.InstallPlugin(ctx, item.Plugin.ID())
}
