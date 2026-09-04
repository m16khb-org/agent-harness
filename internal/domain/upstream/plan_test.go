package upstream

import (
	"testing"

	upstreamcontract "issueops/internal/contract/upstream"
)

func TestPlanSkipsEntriesTheHostAlreadyHas(t *testing.T) {
	cfg := upstreamcontract.Config{
		Version: 1,
		Plugins: []upstreamcontract.PluginEntry{
			{Name: "eli5", Marketplace: "claude-community", Source: "anthropics/claude-plugins-community"},
			{Name: "open-code-review", Marketplace: "open-code-review", Source: "alibaba/open-code-review"},
		},
		Skills: []upstreamcontract.SkillEntry{
			{Name: "cua-driver", Repo: "https://github.com/trycua/cua", Path: "libs/cua-driver/rust/Skills/cua-driver"},
			{Name: "diagram-design", Repo: "https://github.com/example/skills", Path: "skills/diagram-design"},
		},
	}
	observed := upstreamcontract.Observed{
		Plugins:      []string{"open-code-review@open-code-review", "superpowers@claude-plugins-official"},
		Marketplaces: []string{"claude-plugins-official", "open-code-review"},
		Skills:       []string{"diagram-design", "verified-execution"},
	}

	items := Plan(cfg, observed)

	if len(items) != 4 {
		t.Fatalf("plan items = %d, want 4: %#v", len(items), items)
	}
	assertItem(t, items[0], upstreamcontract.KindPlugin, "eli5@claude-community", upstreamcontract.ActionInstall)
	if !items[0].AddMarketplace {
		t.Fatalf("eli5 must add its unregistered marketplace: %#v", items[0])
	}
	assertItem(t, items[1], upstreamcontract.KindPlugin, "open-code-review@open-code-review", upstreamcontract.ActionSkip)
	if items[1].Reason == "" {
		t.Fatalf("skip must carry a reason: %#v", items[1])
	}
	assertItem(t, items[2], upstreamcontract.KindSkill, "cua-driver", upstreamcontract.ActionInstall)
	assertItem(t, items[3], upstreamcontract.KindSkill, "diagram-design", upstreamcontract.ActionSkip)
}

func TestPlanDoesNotAddAnAlreadyRegisteredMarketplace(t *testing.T) {
	cfg := upstreamcontract.Config{Plugins: []upstreamcontract.PluginEntry{
		{Name: "eli5", Marketplace: "claude-community", Source: "anthropics/claude-plugins-community"},
	}}
	observed := upstreamcontract.Observed{Marketplaces: []string{"claude-community"}}

	items := Plan(cfg, observed)

	if len(items) != 1 || items[0].AddMarketplace {
		t.Fatalf("registered marketplace must not be re-added: %#v", items)
	}
}

func TestPlanDropsIncompleteAndDuplicateEntries(t *testing.T) {
	cfg := upstreamcontract.Config{
		Plugins: []upstreamcontract.PluginEntry{
			{Name: " eli5 ", Marketplace: "claude-community", Source: "anthropics/claude-plugins-community"},
			{Name: "eli5", Marketplace: "claude-community", Source: "anthropics/claude-plugins-community"},
			{Name: "nomarket", Source: "some/repo"},
			{Name: "nosource", Marketplace: "mkt"},
			{Marketplace: "mkt", Source: "some/repo"},
		},
		Skills: []upstreamcontract.SkillEntry{
			{Name: "cua-driver", Repo: "https://github.com/trycua/cua"},
			{Name: "cua-driver", Repo: "https://github.com/trycua/cua"},
			{Name: "norepo"},
			{Name: "../escape", Repo: "https://github.com/trycua/cua"},
		},
	}

	items := Plan(cfg, upstreamcontract.Observed{})

	if len(items) != 2 {
		t.Fatalf("plan items = %d, want 2: %#v", len(items), items)
	}
	assertItem(t, items[0], upstreamcontract.KindPlugin, "eli5@claude-community", upstreamcontract.ActionInstall)
	assertItem(t, items[1], upstreamcontract.KindSkill, "cua-driver", upstreamcontract.ActionInstall)
}

func TestPlanReportsPluginHostUnavailableAsSkip(t *testing.T) {
	cfg := upstreamcontract.Config{
		Plugins: []upstreamcontract.PluginEntry{{Name: "eli5", Marketplace: "claude-community", Source: "anthropics/claude-plugins-community"}},
		Skills:  []upstreamcontract.SkillEntry{{Name: "cua-driver", Repo: "https://github.com/trycua/cua"}},
	}

	items := PlanWithoutPluginHost(cfg, upstreamcontract.Observed{})

	if len(items) != 2 {
		t.Fatalf("plan items = %d, want 2: %#v", len(items), items)
	}
	assertItem(t, items[0], upstreamcontract.KindPlugin, "eli5@claude-community", upstreamcontract.ActionSkip)
	if items[0].Reason != ReasonPluginHostUnavailable {
		t.Fatalf("reason = %q, want %q", items[0].Reason, ReasonPluginHostUnavailable)
	}
	assertItem(t, items[1], upstreamcontract.KindSkill, "cua-driver", upstreamcontract.ActionInstall)
}

func assertItem(t *testing.T, item upstreamcontract.PlanItem, kind, name, action string) {
	t.Helper()
	if item.Kind != kind || item.Name != name || item.Action != action {
		t.Fatalf("item = %+v, want kind=%s name=%s action=%s", item, kind, name, action)
	}
}
