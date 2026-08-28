package upstream

import (
	"context"
	"errors"
	"testing"

	upstreamcontract "agent-harness/internal/contract/upstream"
)

func TestSyncInstallsOnlyMissingEntries(t *testing.T) {
	plugins := &fakePluginHost{
		available: true,
		installed: []string{"open-code-review@open-code-review"},
		markets:   []string{"open-code-review"},
	}
	skills := &fakeSkillStore{installed: []string{"diagram-design"}}
	service := Service{Plugins: plugins, Skills: skills}

	report, err := service.Sync(context.Background(), testConfig(), false)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	assertStatuses(t, report, map[string]string{
		"eli5@claude-community":             upstreamcontract.StatusInstalled,
		"open-code-review@open-code-review": upstreamcontract.StatusSkipped,
		"cua-driver":                        upstreamcontract.StatusInstalled,
		"diagram-design":                    upstreamcontract.StatusSkipped,
	})
	if len(plugins.addedMarkets) != 1 || plugins.addedMarkets[0] != "anthropics/claude-plugins-community" {
		t.Fatalf("added marketplaces = %#v, want the unregistered one only", plugins.addedMarkets)
	}
	if len(plugins.installedIDs) != 1 || plugins.installedIDs[0] != "eli5@claude-community" {
		t.Fatalf("installed plugins = %#v, want eli5 only", plugins.installedIDs)
	}
	if len(skills.installedEntries) != 1 || skills.installedEntries[0].Name != "cua-driver" {
		t.Fatalf("installed skills = %#v, want cua-driver only", skills.installedEntries)
	}
}

func TestSyncDryRunTouchesNothing(t *testing.T) {
	plugins := &fakePluginHost{available: true}
	skills := &fakeSkillStore{}
	service := Service{Plugins: plugins, Skills: skills}

	report, err := service.Sync(context.Background(), testConfig(), true)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if !report.DryRun {
		t.Fatalf("report must be marked dry-run: %#v", report)
	}
	for _, item := range report.Items {
		if item.Status != upstreamcontract.StatusPlanned {
			t.Fatalf("dry-run item %q status = %q, want planned", item.Name, item.Status)
		}
	}
	if len(plugins.installedIDs) != 0 || len(plugins.addedMarkets) != 0 || len(skills.installedEntries) != 0 {
		t.Fatalf("dry-run must not write: %#v %#v %#v", plugins.installedIDs, plugins.addedMarkets, skills.installedEntries)
	}
}

func TestSyncReportsFailuresWithoutAbortingRemainingEntries(t *testing.T) {
	plugins := &fakePluginHost{available: true, installErr: errors.New("marketplace unreachable")}
	skills := &fakeSkillStore{}
	service := Service{Plugins: plugins, Skills: skills}

	report, err := service.Sync(context.Background(), testConfig(), false)
	if err != nil {
		t.Fatalf("Sync must not fail the caller: %v", err)
	}

	assertStatuses(t, report, map[string]string{
		"eli5@claude-community":             upstreamcontract.StatusFailed,
		"open-code-review@open-code-review": upstreamcontract.StatusFailed,
		"cua-driver":                        upstreamcontract.StatusInstalled,
		"diagram-design":                    upstreamcontract.StatusInstalled,
	})
	for _, item := range report.Items {
		if item.Status == upstreamcontract.StatusFailed && item.Error == "" {
			t.Fatalf("failed item %q must carry its error", item.Name)
		}
	}
}

func TestSyncSkipsPluginsWhenHostCLIIsMissing(t *testing.T) {
	plugins := &fakePluginHost{available: false}
	skills := &fakeSkillStore{}
	service := Service{Plugins: plugins, Skills: skills}

	report, err := service.Sync(context.Background(), testConfig(), false)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	assertStatuses(t, report, map[string]string{
		"eli5@claude-community":             upstreamcontract.StatusSkipped,
		"open-code-review@open-code-review": upstreamcontract.StatusSkipped,
		"cua-driver":                        upstreamcontract.StatusInstalled,
		"diagram-design":                    upstreamcontract.StatusInstalled,
	})
	if plugins.listCalls != 0 {
		t.Fatalf("unavailable host must not be queried: %d calls", plugins.listCalls)
	}
}

func TestSyncTreatsAnUnreadableHostInventoryAsEmpty(t *testing.T) {
	plugins := &fakePluginHost{available: true, listErr: errors.New("cli broke")}
	skills := &fakeSkillStore{listErr: errors.New("no home")}
	service := Service{Plugins: plugins, Skills: skills}

	report, err := service.Sync(context.Background(), testConfig(), true)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if len(report.Items) != 4 {
		t.Fatalf("items = %d, want 4: %#v", len(report.Items), report.Items)
	}
}

func testConfig() upstreamcontract.Config {
	return upstreamcontract.Config{
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
}

func assertStatuses(t *testing.T, report upstreamcontract.Report, want map[string]string) {
	t.Helper()
	if len(report.Items) != len(want) {
		t.Fatalf("items = %d, want %d: %#v", len(report.Items), len(want), report.Items)
	}
	for _, item := range report.Items {
		expected, ok := want[item.Name]
		if !ok {
			t.Fatalf("unexpected item %q", item.Name)
		}
		if item.Status != expected {
			t.Fatalf("item %q status = %q, want %q (error=%q)", item.Name, item.Status, expected, item.Error)
		}
	}
}

type fakePluginHost struct {
	available    bool
	installed    []string
	markets      []string
	listErr      error
	installErr   error
	listCalls    int
	addedMarkets []string
	installedIDs []string
}

func (f *fakePluginHost) Available() bool { return f.available }

func (f *fakePluginHost) InstalledPlugins(context.Context) ([]string, error) {
	f.listCalls++
	return f.installed, f.listErr
}

func (f *fakePluginHost) Marketplaces(context.Context) ([]string, error) {
	f.listCalls++
	return f.markets, f.listErr
}

func (f *fakePluginHost) AddMarketplace(_ context.Context, source string) error {
	if f.installErr != nil {
		return f.installErr
	}
	f.addedMarkets = append(f.addedMarkets, source)
	return nil
}

func (f *fakePluginHost) InstallPlugin(_ context.Context, id string) error {
	if f.installErr != nil {
		return f.installErr
	}
	f.installedIDs = append(f.installedIDs, id)
	return nil
}

type fakeSkillStore struct {
	installed        []string
	listErr          error
	installErr       error
	installedEntries []upstreamcontract.SkillEntry
}

func (f *fakeSkillStore) InstalledSkills() ([]string, error) { return f.installed, f.listErr }

func (f *fakeSkillStore) InstallSkill(_ context.Context, entry upstreamcontract.SkillEntry) error {
	if f.installErr != nil {
		return f.installErr
	}
	f.installedEntries = append(f.installedEntries, entry)
	return nil
}

func TestSyncReportsAMissingSkillStoreInsteadOfPanicking(t *testing.T) {
	service := Service{Plugins: &fakePluginHost{available: true}}

	report, err := service.Sync(context.Background(), upstreamcontract.Config{
		Skills: []upstreamcontract.SkillEntry{{Name: "cua-driver", Repo: "https://github.com/trycua/cua"}},
	}, false)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	if len(report.Items) != 1 || report.Items[0].Status != upstreamcontract.StatusFailed {
		t.Fatalf("items = %#v, want one failed entry", report.Items)
	}
	if report.Items[0].Error == "" {
		t.Fatalf("a missing skill store must surface a structured error")
	}
}

// TestSyncDryRunNeverExecutesThePluginHostCLI fixes the rule the previous
// dry-run test only half-checked: it asserted that no write method ran, while
// the observation still executed the third-party plugin CLI. Executing that CLI
// is itself a write — `claude` creates $HOME/.claude and $HOME/.claude.json on
// startup — which made `install --dry-run` mutate the home directory it was
// only supposed to describe.
func TestSyncDryRunNeverExecutesThePluginHostCLI(t *testing.T) {
	plugins := &fakePluginHost{available: true, installed: []string{"eli5@claude-community"}, markets: []string{"claude-community"}}
	skills := &fakeSkillStore{installed: []string{"diagram-design"}}
	service := Service{Plugins: plugins, Skills: skills}

	report, err := service.Sync(context.Background(), testConfig(), true)
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if plugins.listCalls != 0 {
		t.Fatalf("dry-run queried the plugin host CLI %d times; running it writes to the home directory", plugins.listCalls)
	}
	// Skills are observed through a plain directory read, which has no side
	// effects, so an already-present skill is still reported as skipped.
	assertStatuses(t, report, map[string]string{
		"eli5@claude-community":             upstreamcontract.StatusPlanned,
		"open-code-review@open-code-review": upstreamcontract.StatusPlanned,
		"cua-driver":                        upstreamcontract.StatusPlanned,
		"diagram-design":                    upstreamcontract.StatusSkipped,
	})
}
