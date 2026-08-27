// Package upstream runs one upstream provisioning pass: observe the host, ask
// the domain what is missing, and provision only that. A pass never fails the
// caller — the harness install path must not depend on a third-party plugin CLI
// or on network reachability.
package upstream

import (
	"context"
	"errors"

	upstreamcontract "agent-harness/internal/contract/upstream"
	upstreamdomain "agent-harness/internal/domain/upstream"
	upstreamport "agent-harness/internal/port/upstream"
)

// Service provisions declared upstream plugins and skills onto one host.
type Service struct {
	Plugins upstreamport.PluginHost
	Skills  upstreamport.SkillStore
}

// Sync provisions everything the host is missing and reports every declared
// entry. With dryRun the host is only observed, never written to.
func (s Service) Sync(ctx context.Context, cfg upstreamcontract.Config, dryRun bool) (upstreamcontract.Report, error) {
	report := upstreamcontract.Report{DryRun: dryRun, Items: []upstreamcontract.ItemResult{}}
	pluginHostAvailable := s.Plugins != nil && s.Plugins.Available()
	observed := s.observe(ctx, pluginHostAvailable)

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
func (s Service) observe(ctx context.Context, pluginHostAvailable bool) upstreamcontract.Observed {
	observed := upstreamcontract.Observed{}
	if pluginHostAvailable {
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
