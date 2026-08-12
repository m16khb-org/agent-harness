package installcli

import (
	"os"
	"path/filepath"
	"testing"

	"agent-harness/internal/port"
)

func TestPrepareInstallHostTransactionStopsAtCachedExistingParent(t *testing.T) {
	root := t.TempDir()
	plan := port.NativeInstallResult{
		Files: []port.InstallFile{
			{Path: filepath.Join(root, "a", "one.json")},
			{Path: filepath.Join(root, "b", "two.json")},
			{Path: filepath.Join(root, "c", "three.json")},
		},
	}

	transaction, err := prepareInstallHostTransaction(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, snapshot := range transaction.parents {
		rel, relErr := filepath.Rel(root, snapshot.path)
		if relErr != nil || rel == ".." || filepath.IsAbs(rel) {
			t.Fatalf("parent traversal escaped existing root: %+v", snapshot)
		}
	}
}

func TestCaptureInstallHostParentAcceptsDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "link")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	snapshot, err := captureInstallHostParent(link)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.existed {
		t.Fatalf("directory symlink was not accepted: %+v", snapshot)
	}
}

func TestInstallHostTransactionRestoresManagedFileSymlinkReferent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "managed.json")
	link := filepath.Join(root, "config", "managed.json")
	if err := os.WriteFile(target, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	transaction, err := prepareInstallHostTransaction(port.NativeInstallResult{
		Files: []port.InstallFile{{Path: link}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := transaction.rollback(); err != nil {
		t.Fatal(err)
	}
	gotTarget, err := os.Readlink(link)
	if err != nil || gotTarget != target {
		t.Fatalf("managed file symlink was not restored: target=%q err=%v", gotTarget, err)
	}
	body, err := os.ReadFile(target)
	info, statErr := os.Stat(target)
	if err != nil || statErr != nil || string(body) != "before\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("managed file referent was not restored: body=%q mode=%v readErr=%v statErr=%v", body, info.Mode(), err, statErr)
	}
}
