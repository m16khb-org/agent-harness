package upstream

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	upstreamcontract "issueops/internal/contract/upstream"
)

func TestReadConfigTreatsAMissingDeclarationAsEmpty(t *testing.T) {
	cfg, err := ReadConfig(filepath.Join(t.TempDir(), "upstream.json"))
	if err != nil {
		t.Fatalf("ReadConfig error: %v", err)
	}
	if len(cfg.Plugins) != 0 || len(cfg.Skills) != 0 {
		t.Fatalf("config = %#v, want empty", cfg)
	}
}

func TestReadConfigParsesPluginsAndSkills(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upstream.json")
	writeFile(t, path, `{"version":1,
"plugins":[{"name":"eli5","marketplace":"claude-community","source":"anthropics/claude-plugins-community"}],
"skills":[{"name":"cua-driver","repo":"https://github.com/trycua/cua","path":"libs/cua-driver","ref":"main"}]}`)

	cfg, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig error: %v", err)
	}
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].ID() != "eli5@claude-community" {
		t.Fatalf("plugins = %#v", cfg.Plugins)
	}
	if len(cfg.Skills) != 1 || cfg.Skills[0].Path != "libs/cua-driver" || cfg.Skills[0].Ref != "main" {
		t.Fatalf("skills = %#v", cfg.Skills)
	}
}

func TestParseHostInventories(t *testing.T) {
	ids, err := parsePluginIDs([]byte(`[{"id":"a@m","enabled":true},{"id":"b@m"},{"version":"1"}]`))
	if err != nil {
		t.Fatalf("parsePluginIDs error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "a@m" || ids[1] != "b@m" {
		t.Fatalf("ids = %#v", ids)
	}
	names, err := parseMarketplaceNames([]byte(`[{"name":"official"},{"name":""},{"name":"community"}]`))
	if err != nil {
		t.Fatalf("parseMarketplaceNames error: %v", err)
	}
	if len(names) != 2 || names[0] != "official" || names[1] != "community" {
		t.Fatalf("names = %#v", names)
	}
}

func TestInstalledSkillsListsDirectoriesAndLinks(t *testing.T) {
	skillsDir := t.TempDir()
	mkdirAll(t, filepath.Join(skillsDir, "verified-execution"))
	symlink(t, filepath.Join(skillsDir, "gone"), filepath.Join(skillsDir, "dangling"))
	writeFile(t, filepath.Join(skillsDir, "notes.md"), "x")

	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: t.TempDir()}
	names, err := store.InstalledSkills()
	if err != nil {
		t.Fatalf("InstalledSkills error: %v", err)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "dangling" || names[1] != "verified-execution" {
		t.Fatalf("names = %#v, want the directory and the link", names)
	}
}

func TestInstalledSkillsTreatsAMissingDirectoryAsEmpty(t *testing.T) {
	store := GitSkillStore{SkillsDir: filepath.Join(t.TempDir(), "absent"), CacheDir: t.TempDir()}
	names, err := store.InstalledSkills()
	if err != nil {
		t.Fatalf("InstalledSkills error: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("names = %#v, want empty", names)
	}
}

func TestInstallSkillFetchesTheSubdirectoryAndLinksIt(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, "tools/demo-skill", "---\nname: demo-skill\n---\n")
	skillsDir := filepath.Join(t.TempDir(), "skills")
	cacheDir := filepath.Join(t.TempDir(), "cache")
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: cacheDir}

	entry := upstreamcontract.SkillEntry{Name: "demo-skill", Repo: repo, Path: "tools/demo-skill"}
	if err := store.InstallSkill(context.Background(), entry); err != nil {
		t.Fatalf("InstallSkill error: %v", err)
	}

	link := filepath.Join(skillsDir, "demo-skill")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("host skill must be a symlink: %v", err)
	}
	if target != filepath.Join(cacheDir, "demo-skill") {
		t.Fatalf("link target = %q, want the harness cache copy", target)
	}
	body, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
	if err != nil || len(body) == 0 {
		t.Fatalf("fetched skill must expose SKILL.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "demo-skill", "unrelated.txt")); err == nil {
		t.Fatalf("only the declared subdirectory may be materialized")
	}

	names, err := store.InstalledSkills()
	if err != nil || len(names) != 1 || names[0] != "demo-skill" {
		t.Fatalf("installed skills = %#v (%v)", names, err)
	}
}

func TestInstallSkillRejectsADirectoryWithoutSkillMarkdown(t *testing.T) {
	requireGit(t)
	repo := newSkillRepo(t, "tools/demo-skill", "")
	skillsDir := filepath.Join(t.TempDir(), "skills")
	store := GitSkillStore{SkillsDir: skillsDir, CacheDir: filepath.Join(t.TempDir(), "cache")}

	entry := upstreamcontract.SkillEntry{Name: "demo-skill", Repo: repo, Path: "tools"}
	err := store.InstallSkill(context.Background(), entry)
	if err == nil {
		t.Fatalf("a directory without SKILL.md must not be installed as a skill")
	}
	if _, statErr := os.Lstat(filepath.Join(skillsDir, "demo-skill")); statErr == nil {
		t.Fatalf("failed fetch must not leave a host link behind")
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for upstream skill fetch tests")
	}
}

// newSkillRepo builds a local git repository holding one skill directory plus
// an unrelated file, so sparse fetching is observable.
func newSkillRepo(t *testing.T, skillPath, skillBody string) string {
	t.Helper()
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "unrelated.txt"), "not a skill")
	if skillBody != "" {
		writeFile(t, filepath.Join(repo, skillPath, "SKILL.md"), skillBody)
	} else {
		writeFile(t, filepath.Join(repo, skillPath, "notes.md"), "no skill manifest")
	}
	for _, args := range [][]string{
		{"init", "--quiet", "--initial-branch=main"},
		{"config", "user.email", "harness@example.com"},
		{"config", "user.name", "issueops"},
		{"add", "-A"},
		{"commit", "--quiet", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return repo
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func symlink(t *testing.T, target, path string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %s: %v", path, err)
	}
}
