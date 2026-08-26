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

func TestPlanHostSkillLinksPrunesOnlyStaleHarnessOwnedLinks(t *testing.T) {
	root := t.TempDir()
	dest := t.TempDir()
	elsewhere := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "skills", "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "plain-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustSymlink := func(target, path string) {
		t.Helper()
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
	mustSymlink(filepath.Join(root, "skills", "removed-skill"), filepath.Join(dest, "removed-skill"))
	mustSymlink(filepath.Join(root, "skills", "renamed-skill"), filepath.Join(dest, "renamed-skill"))
	mustSymlink(filepath.Join(elsewhere, "skills", "foreign-missing"), filepath.Join(dest, "foreign-missing"))
	mustSymlink(filepath.Join(root, "skills", "shared"), filepath.Join(dest, "shared"))

	_, links, messages, errs := PlanHostSkillLinks(root, dest, []string{"shared"}, "claude", true)
	if len(errs) != 0 {
		t.Fatalf("dry-run errs = %v", errs)
	}
	var wouldRemove []string
	for _, link := range links {
		if link.WouldRemove {
			wouldRemove = append(wouldRemove, filepath.Base(link.Path))
		}
		if link.Removed {
			t.Fatalf("dry-run must not remove links: %+v", link)
		}
	}
	if got, want := strings.Join(wouldRemove, ","), "removed-skill,renamed-skill"; got != want {
		t.Fatalf("would-remove = %q, want %q (links=%+v)", got, want, links)
	}
	if !strings.Contains(strings.Join(messages, "\n"), "would prune stale skill link for claude: removed-skill") {
		t.Fatalf("messages = %#v", messages)
	}
	for _, name := range []string{"removed-skill", "renamed-skill", "foreign-missing", "shared", "plain-dir"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("dry-run touched %s: %v", name, err)
		}
	}

	_, links, messages, errs = PlanHostSkillLinks(root, dest, []string{"shared"}, "claude", false)
	if len(errs) != 0 {
		t.Fatalf("apply errs = %v", errs)
	}
	var removed []string
	for _, link := range links {
		if link.Removed {
			removed = append(removed, filepath.Base(link.Path))
		}
	}
	if got, want := strings.Join(removed, ","), "removed-skill,renamed-skill"; got != want {
		t.Fatalf("removed = %q, want %q (links=%+v)", got, want, links)
	}
	if !strings.Contains(strings.Join(messages, "\n"), "prune stale skill link for claude: renamed-skill") {
		t.Fatalf("messages = %#v", messages)
	}
	for _, name := range []string{"removed-skill", "renamed-skill"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); !os.IsNotExist(err) {
			t.Fatalf("stale harness link %s survived: %v", name, err)
		}
	}
	for _, name := range []string{"foreign-missing", "shared", "plain-dir"} {
		if _, err := os.Lstat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("non-harness or live entry %s was removed: %v", name, err)
		}
	}
	if target, err := os.Readlink(filepath.Join(dest, "shared")); err != nil || target != filepath.Join(root, "skills", "shared") {
		t.Fatalf("live link was rewritten: %q %v", target, err)
	}
}

func TestPruneStaleSkillLinksToleratesMissingDestRoot(t *testing.T) {
	links, errs := PruneStaleSkillLinks(t.TempDir(), filepath.Join(t.TempDir(), "absent"), false)
	if len(links) != 0 || len(errs) != 0 {
		t.Fatalf("missing dest root must be a no-op, got links=%+v errs=%v", links, errs)
	}
}
