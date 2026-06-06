package installutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTextPlanCoversDryRunWriteAndNoop(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "file.txt")

	planned, err := WriteTextPlan(path, "config", "hello\n", 0o644, true)
	if err != nil {
		t.Fatal(err)
	}
	if !planned.WouldWrite || planned.Written {
		t.Fatalf("dry-run plan = %+v, want would_write only", planned)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create file, stat err = %v", err)
	}

	written, err := WriteText(path, "config", "hello\n", 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if !written.Written || written.WouldWrite {
		t.Fatalf("write result = %+v, want written only", written)
	}
	noOp, err := WriteText(path, "config", "hello\n", 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if noOp.Written || noOp.WouldWrite {
		t.Fatalf("same content should be no-op: %+v", noOp)
	}
}

func TestWriteJSONPlanCoversMarshalAndInvalidValue(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")

	written, err := WriteJSON(path, "settings", map[string]string{"name": "agent"}, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if !written.Written {
		t.Fatalf("write json result = %+v, want written", written)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "{\n  \"name\": \"agent\"\n}\n" {
		t.Fatalf("json content = %q", got)
	}
	if _, err := WriteJSONPlan(path, "settings", make(chan int), 0o644, true); err == nil {
		t.Fatalf("invalid JSON value should return an error")
	}
}

func TestEnsureSymlinkPlanCoversCreateReplaceAndRefusal(t *testing.T) {
	root := t.TempDir()
	targetA := filepath.Join(root, "target-a")
	targetB := filepath.Join(root, "target-b")
	linkPath := filepath.Join(root, "links", "skill")
	for _, path := range []string{targetA, targetB} {
		if err := os.WriteFile(path, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	planned, err := EnsureSymlinkPlan(targetA, linkPath, true)
	if err != nil {
		t.Fatal(err)
	}
	if !planned.WouldCreate || planned.Created {
		t.Fatalf("dry-run symlink plan = %+v, want would_create only", planned)
	}
	created, err := EnsureSymlink(targetA, linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if !created.Created {
		t.Fatalf("created symlink result = %+v, want created", created)
	}
	unchanged, err := EnsureSymlink(targetA, linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Created || unchanged.WouldCreate {
		t.Fatalf("same symlink should be no-op: %+v", unchanged)
	}
	replaced, err := EnsureSymlink(targetB, linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if !replaced.Created {
		t.Fatalf("replaced symlink result = %+v, want created", replaced)
	}

	regular := filepath.Join(root, "regular")
	if err := os.WriteFile(regular, []byte("not a symlink"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureSymlink(targetA, regular); err == nil || !strings.Contains(err.Error(), "refusing to replace non-symlink path") {
		t.Fatalf("regular path error = %v", err)
	}
}

func TestPlanHostSkillLinksPartitionsAndPlans(t *testing.T) {
	root := t.TempDir()
	dest := t.TempDir()
	for _, name := range []string{"shared", "codex-only"} {
		if err := os.MkdirAll(filepath.Join(root, "skills", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "codex-only", "install.json"), []byte(`{"hosts":["codex"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	enabled, links, messages, errs := PlanHostSkillLinks(root, dest, []string{"shared", "codex-only"}, "claude", true)
	if len(errs) != 0 {
		t.Fatalf("PlanHostSkillLinks errs = %v", errs)
	}
	if got, want := strings.Join(enabled, ","), "shared"; got != want {
		t.Fatalf("enabled = %q, want %q", got, want)
	}
	if len(messages) != 1 || messages[0] != "skip skill for claude: codex-only" {
		t.Fatalf("messages = %#v", messages)
	}
	if len(links) != 1 || !links[0].WouldCreate || links[0].Path != filepath.Join(dest, "shared") {
		t.Fatalf("links = %+v", links)
	}
	if got := TOMLString("a\"b"); got != `"a\"b"` {
		t.Fatalf("TOMLString = %q", got)
	}
}
