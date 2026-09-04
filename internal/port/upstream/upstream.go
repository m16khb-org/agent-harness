// Package upstream declares the narrow host capabilities that upstream
// provisioning needs: reading and writing the host's plugin inventory, and
// reading and writing the host's skill directory.
package upstream

import (
	"context"

	upstreamcontract "issueops/internal/contract/upstream"
)

// PluginHost is the host CLI that owns plugin marketplaces and installs.
type PluginHost interface {
	// Available reports whether the host plugin CLI can be invoked at all.
	// An unavailable host is a skip, never an install failure: the harness
	// does not gate its own install on a third-party tool being present.
	Available() bool
	// InstalledPlugins returns host plugin ids ("name@marketplace").
	InstalledPlugins(ctx context.Context) ([]string, error)
	// Marketplaces returns the marketplace names already registered.
	Marketplaces(ctx context.Context) ([]string, error)
	AddMarketplace(ctx context.Context, source string) error
	InstallPlugin(ctx context.Context, id string) error
}

// SkillStore is the host skill directory the harness links upstream skills into.
type SkillStore interface {
	// InstalledSkills returns the skill names already visible to the host,
	// whoever installed them.
	InstalledSkills() ([]string, error)
	InstallSkill(ctx context.Context, entry upstreamcontract.SkillEntry) error
}
