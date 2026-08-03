package installutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareManagedCommandPathRefusesRegularFileWithoutApproval(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)

	_, plan, err := prepareManagedCommandPathWithDeps(target, command, false, false, validCommandDeps())
	if err == nil || !strings.Contains(err.Error(), "refusing to adopt regular command file without --adopt-command-file") {
		t.Fatalf("error = %v, plan = %+v", err, plan)
	}
	assertRegularCommand(t, command, "old", 0o751)
}

func TestPrepareManagedCommandPathDryRunReportsAdoptionWithoutWriting(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)

	transaction, plan, err := prepareManagedCommandPathWithDeps(target, command, true, true, validCommandDeps())
	if err != nil {
		t.Fatal(err)
	}
	if transaction != nil || !plan.AdoptionApproved || !plan.WouldAdopt || plan.BackupPath == "" || !plan.RollbackAvailable {
		t.Fatalf("dry-run transaction=%v plan=%+v", transaction, plan)
	}
	assertRegularCommand(t, command, "old", 0o751)
	if _, err := os.Lstat(plan.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run backup exists: %v", err)
	}
}

func TestManagedCommandPathApplyFinalizeAtomicallyAdoptsRegularFile(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)

	transaction, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, validCommandDeps())
	if err != nil {
		t.Fatal(err)
	}
	applied, err := transaction.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Adopted || !applied.BackupRetained {
		t.Fatalf("apply plan = %+v", applied)
	}
	assertSymlinkTarget(t, command, target)
	backupInfo, err := os.Lstat(applied.BackupPath)
	if err != nil || !backupInfo.Mode().IsRegular() || backupInfo.Mode().Perm() != 0o600 {
		t.Fatalf("backup info=%v err=%v", backupInfo, err)
	}
	finalized, err := transaction.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	if !finalized.Committed || finalized.BackupRetained || finalized.RollbackAvailable {
		t.Fatalf("finalized = %+v", finalized)
	}
	if _, err := os.Lstat(applied.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("finalized backup still exists: %v", err)
	}
}

func TestManagedCommandPathRollbackRestoresOriginalBytesAndMode(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)

	transaction, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, validCommandDeps())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Apply(); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := transaction.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if !rolledBack.RolledBack || rolledBack.BackupRetained || rolledBack.RollbackAvailable {
		t.Fatalf("rollback = %+v", rolledBack)
	}
	assertRegularCommand(t, command, "old", 0o751)
}

func TestPrepareManagedCommandPathRejectsInvalidIdentityBeforeMutation(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)

	invalidBuild := validCommandDeps()
	invalidBuild.readBuildInfo = func(path string) (managedBuildInfo, error) {
		if path == command {
			return managedBuildInfo{MainPath: "cmd/echo", ModulePath: "example.invalid"}, nil
		}
		return managedBuildInfo{MainPath: managedCommandMainPath, ModulePath: managedCommandModulePath}, nil
	}
	if _, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, invalidBuild); err == nil || !strings.Contains(err.Error(), "managed agent-harness build identity") {
		t.Fatalf("wrong build identity error = %v", err)
	}
	assertRegularCommand(t, command, "old", 0o751)
	invalidCandidate := validCommandDeps()
	invalidCandidate.readBuildInfo = func(path string) (managedBuildInfo, error) {
		if path == target {
			return managedBuildInfo{MainPath: "agent-harness/cmd/other", ModulePath: managedCommandModulePath}, nil
		}
		return managedBuildInfo{MainPath: managedCommandMainPath, ModulePath: managedCommandModulePath}, nil
	}
	if _, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, invalidCandidate); err == nil || !strings.Contains(err.Error(), "command candidate") {
		t.Fatalf("wrong candidate build identity error = %v", err)
	}

	for name, mutate := range map[string]func(string){
		"not executable": func(path string) { mustChmod(t, path, 0o600) },
		"empty":          func(path string) { mustTruncate(t, path, 0) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := writeCommandFixture(t, root, strings.ReplaceAll(name, " ", "-"), "old", 0o751)
			mutate(candidate)
			if _, _, err := prepareManagedCommandPathWithDeps(target, candidate, true, false, validCommandDeps()); err == nil {
				t.Fatalf("invalid command accepted")
			}
		})
	}
}

func TestPrepareManagedCommandPathRejectsUnmanagedBinaryAndInvalidFileMatrix(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	echoCopy := filepath.Join(root, "echo")
	body, err := os.ReadFile("/bin/echo")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(echoCopy, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PrepareManagedCommandPath(target, echoCopy, true, true); err == nil || !strings.Contains(err.Error(), "managed agent-harness build identity") {
		t.Fatalf("/bin/echo copy error = %v", err)
	}

	directory := filepath.Join(root, "directory")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	multi := writeCommandFixture(t, root, "multi", "old", 0o755)
	if err := os.Link(multi, filepath.Join(root, "multi-second")); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"directory": directory, "symlink": link, "multiple hard links": multi} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := prepareManagedCommandPathWithDeps(target, path, true, true, validCommandDeps()); err == nil {
				t.Fatalf("invalid path was accepted: %s", path)
			}
		})
	}
}

func TestPrepareManagedCommandPathEnforcesExactSizeBoundary(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	boundary := writeCommandFixture(t, root, "boundary", "x", 0o755)
	if err := os.Truncate(boundary, managedCommandMaxSize); err != nil {
		t.Fatal(err)
	}
	if _, plan, err := prepareManagedCommandPathWithDeps(target, boundary, true, true, validCommandDeps()); err != nil || !plan.WouldAdopt {
		t.Fatalf("boundary plan=%+v err=%v", plan, err)
	}
	oversized := writeCommandFixture(t, root, "oversized", "x", 0o755)
	if err := os.Truncate(oversized, managedCommandMaxSize+1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareManagedCommandPathWithDeps(target, oversized, true, true, validCommandDeps()); err == nil {
		t.Fatal("oversized command was accepted")
	}
}

func TestManagedCommandPathApplyRejectsIdentityDrift(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)
	transaction, plan, err := prepareManagedCommandPathWithDeps(target, command, true, false, validCommandDeps())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(command, []byte("drift"), 0o751); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Apply(); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("drift error = %v", err)
	}
	if _, err := os.Lstat(plan.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("backup created before drift rejection: %v", err)
	}
}

func TestManagedCommandPathApplyRejectsCandidateIdentityDrift(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)
	transaction, plan, err := prepareManagedCommandPathWithDeps(target, command, true, false, validCommandDeps())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("candidate drift"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Apply(); err == nil || !strings.Contains(err.Error(), "candidate identity changed") {
		t.Fatalf("candidate drift error = %v", err)
	}
	if _, err := os.Lstat(plan.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("backup created before candidate drift rejection: %v", err)
	}
}

func TestManagedCommandPathApplyDoesNotOverwriteReplacementAtExchange(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)
	concurrent := writeCommandFixture(t, root, "concurrent", "concurrent", 0o755)
	deps := validCommandDeps()
	exchangePaths := deps.exchangePaths
	exchangeCalls := 0
	deps.exchangePaths = func(left, right string) error {
		exchangeCalls++
		if exchangeCalls == 1 {
			if err := os.Rename(concurrent, right); err != nil {
				return err
			}
		}
		return exchangePaths(left, right)
	}
	transaction, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, deps)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := transaction.Apply()
	if err == nil || !strings.Contains(err.Error(), "identity changed during replacement") {
		t.Fatalf("apply plan=%+v err=%v", plan, err)
	}
	assertRegularCommand(t, command, "concurrent", 0o755)
	backupInfo, statErr := os.Lstat(plan.BackupPath)
	if statErr != nil || backupInfo.Mode().Perm() != 0o600 || !plan.BackupRetained {
		t.Fatalf("recovery backup info=%v statErr=%v plan=%+v", backupInfo, statErr, plan)
	}
}

func TestManagedCommandPathRollbackDoesNotOverwriteReplacementAtExchange(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)
	concurrent := writeCommandFixture(t, root, "concurrent", "concurrent", 0o755)
	deps := validCommandDeps()
	exchangePaths := deps.exchangePaths
	exchangeCalls := 0
	deps.exchangePaths = func(left, right string) error {
		exchangeCalls++
		if exchangeCalls == 2 {
			if err := os.Rename(concurrent, right); err != nil {
				return err
			}
		}
		return exchangePaths(left, right)
	}
	transaction, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, deps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Apply(); err != nil {
		t.Fatal(err)
	}
	plan, err := transaction.Rollback()
	if err == nil || !strings.Contains(err.Error(), "changed during rollback") {
		t.Fatalf("rollback plan=%+v err=%v", plan, err)
	}
	assertRegularCommand(t, command, "concurrent", 0o755)
	backupInfo, statErr := os.Lstat(plan.BackupPath)
	if statErr != nil || backupInfo.Mode().Perm() != 0o600 || !plan.BackupRetained {
		t.Fatalf("recovery backup info=%v statErr=%v plan=%+v", backupInfo, statErr, plan)
	}
}

func TestManagedCommandPathApplyKeepsRecoveryBackupWhenDirectorySyncFails(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)
	deps := validCommandDeps()
	syncCalls := 0
	deps.syncDir = func(string) error {
		syncCalls++
		if syncCalls == 2 {
			return errors.New("injected directory sync failure")
		}
		return nil
	}
	transaction, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := transaction.Apply()
	if err == nil || !strings.Contains(err.Error(), "directory sync failure") || !result.Adopted || !result.BackupRetained {
		t.Fatalf("apply result=%+v err=%v", result, err)
	}
	if _, rollbackErr := transaction.Rollback(); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	assertRegularCommand(t, command, "old", 0o751)
}

func TestManagedCommandPathFinalizeKeepsCommittedRecoveryBackupOnCleanupFailure(t *testing.T) {
	root := t.TempDir()
	target := writeCommandFixture(t, root, "target", "new", 0o755)
	command := writeCommandFixture(t, root, "agent-harness", "old", 0o751)
	transaction, _, err := prepareManagedCommandPathWithDeps(target, command, true, false, validCommandDeps())
	if err != nil {
		t.Fatal(err)
	}
	applied, err := transaction.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	finalized, finalizeErr := transaction.Finalize()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if finalizeErr == nil || !finalized.Committed || !finalized.BackupRetained || finalized.RollbackAvailable {
		t.Fatalf("finalized=%+v err=%v", finalized, finalizeErr)
	}
	if _, err := os.Lstat(applied.BackupPath); err != nil {
		t.Fatalf("recovery backup missing: %v", err)
	}
}

func validCommandDeps() managedCommandPathDeps {
	deps := defaultManagedCommandPathDeps()
	deps.readBuildInfo = func(string) (managedBuildInfo, error) {
		return managedBuildInfo{MainPath: managedCommandMainPath, ModulePath: managedCommandModulePath}, nil
	}
	return deps
}

func writeCommandFixture(t *testing.T, root, name, content string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertRegularCommand(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || string(data) != content || info.Mode().Perm() != mode {
		t.Fatalf("command info=%v content=%q", info.Mode(), data)
	}
}

func assertSymlinkTarget(t *testing.T, path, target string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 || actual != target {
		t.Fatalf("symlink info=%v target=%q", info.Mode(), actual)
	}
}

func mustChmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func mustTruncate(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
}
