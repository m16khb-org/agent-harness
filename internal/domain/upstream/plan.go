// Package upstream decides, without touching the host, which declared upstream
// plugins and skills the harness still has to provision. Deciding here keeps the
// "install only what is missing" rule testable without a Claude CLI, a network,
// or a home directory.
package upstream

import (
	"regexp"
	"strings"

	upstreamcontract "agent-harness/internal/contract/upstream"
)

// Reasons a declared entry is skipped rather than installed.
const (
	ReasonPluginAlreadyInstalled = "already installed on the host"
	ReasonSkillAlreadyInstalled  = "already present in the host skill directory"
	ReasonPluginHostUnavailable  = "host plugin CLI is unavailable"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Plan decides what one sync pass should do, given the declaration and what the
// host already has. Plugins come first, then skills; both keep declaration order.
func Plan(cfg upstreamcontract.Config, observed upstreamcontract.Observed) []upstreamcontract.PlanItem {
	return plan(cfg, observed, true)
}

// PlanWithoutPluginHost is Plan for a host whose plugin CLI is missing: plugin
// entries are skipped with a reason instead of being planned for an install
// that cannot run. Skills are unaffected because they are provisioned by the
// harness itself.
func PlanWithoutPluginHost(cfg upstreamcontract.Config, observed upstreamcontract.Observed) []upstreamcontract.PlanItem {
	return plan(cfg, observed, false)
}

func plan(cfg upstreamcontract.Config, observed upstreamcontract.Observed, pluginHostAvailable bool) []upstreamcontract.PlanItem {
	installedPlugins := indexOf(observed.Plugins)
	marketplaces := indexOf(observed.Marketplaces)
	installedSkills := indexOf(observed.Skills)

	items := make([]upstreamcontract.PlanItem, 0, len(cfg.Plugins)+len(cfg.Skills))
	seen := map[string]bool{}
	for _, entry := range cfg.Plugins {
		entry, ok := normalizePlugin(entry)
		if !ok || seen[upstreamcontract.KindPlugin+"/"+entry.ID()] {
			continue
		}
		seen[upstreamcontract.KindPlugin+"/"+entry.ID()] = true
		item := upstreamcontract.PlanItem{Kind: upstreamcontract.KindPlugin, Name: entry.ID(), Action: upstreamcontract.ActionInstall, Plugin: &entry}
		switch {
		case installedPlugins[entry.ID()]:
			item.Action = upstreamcontract.ActionSkip
			item.Reason = ReasonPluginAlreadyInstalled
		case !pluginHostAvailable:
			item.Action = upstreamcontract.ActionSkip
			item.Reason = ReasonPluginHostUnavailable
		default:
			item.AddMarketplace = !marketplaces[entry.Marketplace]
		}
		items = append(items, item)
	}
	for _, entry := range cfg.Skills {
		entry, ok := normalizeSkill(entry)
		if !ok || seen[upstreamcontract.KindSkill+"/"+entry.Name] {
			continue
		}
		seen[upstreamcontract.KindSkill+"/"+entry.Name] = true
		item := upstreamcontract.PlanItem{Kind: upstreamcontract.KindSkill, Name: entry.Name, Action: upstreamcontract.ActionInstall, Skill: &entry}
		if installedSkills[entry.Name] {
			item.Action = upstreamcontract.ActionSkip
			item.Reason = ReasonSkillAlreadyInstalled
		}
		items = append(items, item)
	}
	return items
}

func normalizePlugin(entry upstreamcontract.PluginEntry) (upstreamcontract.PluginEntry, bool) {
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Marketplace = strings.TrimSpace(entry.Marketplace)
	entry.Source = strings.TrimSpace(entry.Source)
	if entry.Name == "" || entry.Marketplace == "" || entry.Source == "" {
		return entry, false
	}
	return entry, true
}

func normalizeSkill(entry upstreamcontract.SkillEntry) (upstreamcontract.SkillEntry, bool) {
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Repo = strings.TrimSpace(entry.Repo)
	entry.Path = strings.Trim(strings.TrimSpace(entry.Path), "/")
	entry.Ref = strings.TrimSpace(entry.Ref)
	if !skillNamePattern.MatchString(entry.Name) || entry.Repo == "" {
		return entry, false
	}
	if entry.Path != "" && (strings.Contains(entry.Path, "..") || strings.HasPrefix(entry.Path, "/")) {
		return entry, false
	}
	return entry, true
}

func indexOf(values []string) map[string]bool {
	index := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			index[value] = true
		}
	}
	return index
}
