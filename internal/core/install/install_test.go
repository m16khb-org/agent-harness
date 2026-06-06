package install

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/port"
)

type fakeHostInstaller struct {
	name string
}

func (f fakeHostInstaller) Name() string { return f.name }
func (f fakeHostInstaller) Install(req port.NativeInstallRequest) (port.HostInstallResult, error) {
	return port.HostInstallResult{Host: f.name, OK: true, Messages: []string{req.Root}}, nil
}

func TestInstallNativeDelegatesThroughHostInstaller(t *testing.T) {
	root := t.TempDir()
	writeInstallTestSkill(t, root, "alpha")
	writeInstallTestSkill(t, root, "beta")
	req := DefaultNativeInstallRequest(root, t.TempDir(), "", "")
	result, err := InstallNative(req, fakeHostInstaller{name: "host-a"}, fakeHostInstaller{name: "host-b"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || len(result.Hosts) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if got := result.SkillNames; len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("skill names not normalized: %+v", got)
	}
	if result.ProjectLocal != false {
		t.Fatalf("unexpected install defaults: %+v", result)
	}
}

func writeInstallTestSkill(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: "+name+"\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
