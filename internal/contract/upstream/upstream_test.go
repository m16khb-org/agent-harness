package upstream

import (
	"encoding/json"
	"testing"
)

func TestPluginEntryID(t *testing.T) {
	entry := PluginEntry{
		Name:        "foo",
		Marketplace: "bar",
		Source:      "https://example.com",
	}
	if got, want := entry.ID(), "foo@bar"; got != want {
		t.Fatalf("PluginEntry.ID() = %q, want %q", got, want)
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	cfg := Config{
		Version: 1,
		Plugins: []PluginEntry{
			{Name: "p1", Marketplace: "m1", Source: "s1"},
		},
		Skills: []SkillEntry{
			{Name: "s1", Repo: "r1", Path: "p1", Ref: "main"},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if decoded.Version != 1 || len(decoded.Plugins) != 1 || len(decoded.Skills) != 1 {
		t.Fatalf("unexpected decoded config: %+v", decoded)
	}
}

func TestPlanItemAndReportJSONSerialization(t *testing.T) {
	plugin := PluginEntry{Name: "p1", Marketplace: "m1"}
	skill := SkillEntry{Name: "s1", Repo: "r1"}

	item1 := PlanItem{
		Kind:           KindPlugin,
		Name:           "p1",
		Action:         ActionInstall,
		Reason:         "new plugin",
		AddMarketplace: true,
		Plugin:         &plugin,
	}
	item2 := PlanItem{
		Kind:   KindSkill,
		Name:   "s1",
		Action: ActionSkip,
		Reason: "already up to date",
		Skill:  &skill,
	}

	report := Report{
		DryRun: true,
		Items: []ItemResult{
			{Kind: KindPlugin, Name: "p1", Status: StatusInstalled},
			{Kind: KindSkill, Name: "s1", Status: StatusSkipped, Reason: "skipped reason"},
			{Kind: KindSkill, Name: "s2", Status: StatusFailed, Error: "fetch error"},
			{Kind: KindPlugin, Name: "p2", Status: StatusPlanned},
		},
	}

	for _, v := range []any{item1, item2, report, Observed{Plugins: []string{"p1"}, Marketplaces: []string{"m1"}, Skills: []string{"s1"}}} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("json.Marshal failed: %v", err)
		}
		if len(data) == 0 {
			t.Fatalf("expected non-empty json")
		}
	}
}
