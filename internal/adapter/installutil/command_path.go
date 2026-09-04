package installutil

import (
	"crypto/rand"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const (
	managedCommandMainPath   = "issueops/cmd/issueops"
	managedCommandModulePath = "issueops"
	managedCommandMaxSize    = int64(268435456)
)

type ManagedCommandPathTransaction struct {
	plan      ManagedCommandPathPlan
	existing  managedCommandIdentity
	candidate managedCommandIdentity
	deps      managedCommandPathDeps
	applied   bool
	closed    bool
	displaced string
}

type managedBuildInfo struct {
	MainPath   string
	ModulePath string
}

type managedCommandIdentity struct {
	path   string
	sha256 string
	mode   os.FileMode
	size   int64
	device uint64
	inode  uint64
	uid    uint32
	links  uint64
}

type managedCommandPathDeps struct {
	readBuildInfo func(string) (managedBuildInfo, error)
	randomSuffix  func() (string, error)
	syncDir       func(string) error
	exchangePaths func(string, string) error
}

func defaultManagedCommandPathDeps() managedCommandPathDeps {
	return managedCommandPathDeps{
		readBuildInfo: func(path string) (managedBuildInfo, error) {
			info, err := buildinfo.ReadFile(path)
			if err != nil {
				return managedBuildInfo{}, err
			}
			return managedBuildInfo{MainPath: info.Path, ModulePath: info.Main.Path}, nil
		},
		randomSuffix: func() (string, error) {
			var value [12]byte
			if _, err := rand.Read(value[:]); err != nil {
				return "", err
			}
			return hex.EncodeToString(value[:]), nil
		},
		syncDir:       syncManagedCommandDirectory,
		exchangePaths: exchangeManagedCommandPaths,
	}
}

func PrepareManagedCommandPath(target, path string, adopt, dryRun bool) (*ManagedCommandPathTransaction, ManagedCommandPathPlan, error) {
	return PrepareManagedCommandPathCandidate(target, target, path, adopt, dryRun)
}

func PrepareManagedCommandPathCandidate(target, candidate, path string, adopt, dryRun bool) (*ManagedCommandPathTransaction, ManagedCommandPathPlan, error) {
	return prepareManagedCommandPathCandidateWithDeps(target, candidate, path, adopt, dryRun, defaultManagedCommandPathDeps())
}

func prepareManagedCommandPathWithDeps(target, path string, adopt, dryRun bool, deps managedCommandPathDeps) (*ManagedCommandPathTransaction, ManagedCommandPathPlan, error) {
	return prepareManagedCommandPathCandidateWithDeps(target, target, path, adopt, dryRun, deps)
}

func prepareManagedCommandPathCandidateWithDeps(target, candidatePath, path string, adopt, dryRun bool, deps managedCommandPathDeps) (*ManagedCommandPathTransaction, ManagedCommandPathPlan, error) {
	plan := ManagedCommandPathPlan{Path: path, Target: target, AdoptionApproved: adopt}
	if !adopt {
		return nil, plan, fmt.Errorf("refusing to adopt regular command file without --adopt-command-file: %s", path)
	}
	if deps.readBuildInfo == nil || deps.randomSuffix == nil || deps.syncDir == nil || deps.exchangePaths == nil {
		return nil, plan, fmt.Errorf("managed command path dependencies are unavailable")
	}
	existing, err := inspectManagedCommand(path, deps.readBuildInfo)
	if err != nil {
		return nil, plan, fmt.Errorf("inspect existing command: %w", err)
	}
	candidate, err := inspectManagedCommand(candidatePath, deps.readBuildInfo)
	if err != nil {
		return nil, plan, fmt.Errorf("inspect command candidate: %w", err)
	}
	suffix, err := deps.randomSuffix()
	if err != nil || suffix == "" {
		return nil, plan, fmt.Errorf("allocate private command backup path")
	}
	plan.BackupPath = filepath.Join(filepath.Dir(path), ".issueops.command-backup-"+suffix)
	plan.RollbackAvailable = true
	if dryRun {
		plan.WouldAdopt = true
		return nil, plan, nil
	}
	return &ManagedCommandPathTransaction{plan: plan, existing: existing, candidate: candidate, deps: deps}, plan, nil
}

func (transaction *ManagedCommandPathTransaction) Apply() (ManagedCommandPathPlan, error) {
	if transaction == nil || transaction.closed || transaction.applied {
		return ManagedCommandPathPlan{}, fmt.Errorf("managed command path transaction is not applicable")
	}
	if err := transaction.revalidate(); err != nil {
		return transaction.plan, err
	}
	if err := copyManagedCommandFile(transaction.plan.Path, transaction.plan.BackupPath, 0o600, &transaction.existing); err != nil {
		return transaction.plan, fmt.Errorf("create private command backup: %w", err)
	}
	transaction.plan.BackupRetained = true
	directory := filepath.Dir(transaction.plan.Path)
	if err := transaction.deps.syncDir(directory); err != nil {
		_ = os.Remove(transaction.plan.BackupPath)
		transaction.plan.BackupRetained = false
		return transaction.plan, fmt.Errorf("sync private command backup directory: %w", err)
	}
	if err := transaction.revalidate(); err != nil {
		return transaction.plan, err
	}
	temporaryLink := transaction.plan.Path + ".activate-" + filepath.Base(transaction.plan.BackupPath)
	if err := os.Symlink(transaction.plan.Target, temporaryLink); err != nil {
		_ = os.Remove(transaction.plan.BackupPath)
		transaction.plan.BackupRetained = false
		return transaction.plan, fmt.Errorf("create temporary command symlink: %w", err)
	}
	if err := transaction.deps.exchangePaths(temporaryLink, transaction.plan.Path); err != nil {
		_ = os.Remove(temporaryLink)
		_ = os.Remove(transaction.plan.BackupPath)
		transaction.plan.BackupRetained = false
		return transaction.plan, fmt.Errorf("atomically exchange managed command path: %w", err)
	}
	displaced, inspectErr := inspectManagedCommand(temporaryLink, transaction.deps.readBuildInfo)
	if inspectErr != nil || !sameManagedCommandSnapshot(displaced, transaction.existing) {
		restoreErr := transaction.restoreDisplacedPath(temporaryLink)
		return transaction.plan, errors.Join(
			fmt.Errorf("existing managed command identity changed during replacement; recovery backup retained at %s", transaction.plan.BackupPath),
			restoreErr,
		)
	}
	if err := verifyManagedCommandSymlink(transaction.plan.Path, transaction.plan.Target); err != nil {
		_ = os.Remove(temporaryLink)
		return transaction.plan, fmt.Errorf("managed command path changed immediately after replacement; recovery backup retained at %s: %w", transaction.plan.BackupPath, err)
	}
	transaction.applied = true
	transaction.plan.Adopted = true
	if err := os.Remove(temporaryLink); err != nil {
		transaction.displaced = temporaryLink
		return transaction.plan, fmt.Errorf("remove displaced managed command after replacement; recovery retained at %s: %w", temporaryLink, err)
	}
	if err := transaction.deps.syncDir(directory); err != nil {
		return transaction.plan, fmt.Errorf("post-rename directory sync failure: %w", err)
	}
	return transaction.plan, nil
}

func (transaction *ManagedCommandPathTransaction) Rollback() (ManagedCommandPathPlan, error) {
	if transaction == nil || transaction.closed || !transaction.applied {
		return ManagedCommandPathPlan{}, fmt.Errorf("managed command path transaction cannot roll back")
	}
	if err := verifyManagedCommandSymlink(transaction.plan.Path, transaction.plan.Target); err != nil {
		return transaction.plan, fmt.Errorf("managed command path changed before rollback")
	}
	temporary, err := transaction.temporaryRegularPath("rollback")
	if err != nil {
		return transaction.plan, err
	}
	if err := copyManagedCommandFile(transaction.plan.BackupPath, temporary, transaction.existing.mode.Perm(), nil); err != nil {
		return transaction.plan, fmt.Errorf("prepare command rollback: %w", err)
	}
	if err := verifyManagedCommandCopy(temporary, transaction.existing); err != nil {
		_ = os.Remove(temporary)
		return transaction.plan, fmt.Errorf("verify command rollback: %w", err)
	}
	if err := transaction.deps.exchangePaths(temporary, transaction.plan.Path); err != nil {
		_ = os.Remove(temporary)
		return transaction.plan, fmt.Errorf("atomically exchange original command during rollback: %w", err)
	}
	if err := verifyManagedCommandSymlink(temporary, transaction.plan.Target); err != nil {
		restoreErr := transaction.restoreRollbackDisplacement(temporary)
		return transaction.plan, errors.Join(
			fmt.Errorf("managed command path changed during rollback; recovery backup retained at %s", transaction.plan.BackupPath),
			restoreErr,
		)
	}
	if err := verifyManagedCommandCopy(transaction.plan.Path, transaction.existing); err != nil {
		restoreErr := transaction.restoreRollbackDisplacement(temporary)
		return transaction.plan, errors.Join(fmt.Errorf("verify restored command after rollback: %w", err), restoreErr)
	}
	if err := os.Remove(temporary); err != nil {
		return transaction.plan, fmt.Errorf("remove displaced command symlink after rollback: %w", err)
	}
	if err := transaction.deps.syncDir(filepath.Dir(transaction.plan.Path)); err != nil {
		return transaction.plan, fmt.Errorf("sync restored command directory: %w", err)
	}
	transaction.plan.RolledBack = true
	transaction.plan.Adopted = false
	transaction.plan.RollbackAvailable = false
	transaction.closed = true
	if transaction.displaced != "" {
		if err := os.Remove(transaction.displaced); err != nil && !os.IsNotExist(err) {
			return transaction.plan, fmt.Errorf("remove retained displaced command after rollback: %w", err)
		}
		transaction.displaced = ""
	}
	if err := os.Remove(transaction.plan.BackupPath); err != nil && !os.IsNotExist(err) {
		return transaction.plan, fmt.Errorf("remove command backup after rollback: %w", err)
	}
	transaction.plan.BackupRetained = false
	if err := transaction.deps.syncDir(filepath.Dir(transaction.plan.Path)); err != nil {
		return transaction.plan, fmt.Errorf("sync rollback cleanup: %w", err)
	}
	return transaction.plan, nil
}

func (transaction *ManagedCommandPathTransaction) restoreDisplacedPath(displaced string) error {
	if err := verifyManagedCommandSymlink(transaction.plan.Path, transaction.plan.Target); err != nil {
		return fmt.Errorf("command replacement recovery retained displaced path %s: %w", displaced, err)
	}
	if err := transaction.deps.exchangePaths(displaced, transaction.plan.Path); err != nil {
		return fmt.Errorf("restore concurrently replaced command from %s: %w", displaced, err)
	}
	if err := verifyManagedCommandSymlink(displaced, transaction.plan.Target); err != nil {
		return fmt.Errorf("command replacement recovery changed during restore; retained path %s: %w", displaced, err)
	}
	if err := os.Remove(displaced); err != nil {
		return fmt.Errorf("remove recovered temporary command symlink %s: %w", displaced, err)
	}
	return nil
}

func (transaction *ManagedCommandPathTransaction) restoreRollbackDisplacement(displaced string) error {
	if err := verifyManagedCommandCopy(transaction.plan.Path, transaction.existing); err != nil {
		return fmt.Errorf("rollback recovery retained displaced path %s: %w", displaced, err)
	}
	if err := transaction.deps.exchangePaths(displaced, transaction.plan.Path); err != nil {
		return fmt.Errorf("restore concurrently replaced command from %s: %w", displaced, err)
	}
	if err := os.Remove(displaced); err != nil {
		return fmt.Errorf("remove recovered rollback temporary %s: %w", displaced, err)
	}
	return nil
}

func (transaction *ManagedCommandPathTransaction) Finalize() (ManagedCommandPathPlan, error) {
	if transaction == nil || transaction.closed || !transaction.applied {
		return ManagedCommandPathPlan{}, fmt.Errorf("managed command path transaction cannot finalize")
	}
	current, err := os.Lstat(transaction.plan.Path)
	if err != nil || current.Mode()&os.ModeSymlink == 0 {
		return transaction.plan, fmt.Errorf("managed command path changed before finalize")
	}
	target, err := os.Readlink(transaction.plan.Path)
	if err != nil || target != transaction.plan.Target {
		return transaction.plan, fmt.Errorf("managed command path changed before finalize")
	}
	transaction.plan.Committed = true
	transaction.plan.RollbackAvailable = false
	transaction.closed = true
	if err := os.Remove(transaction.plan.BackupPath); err != nil && !os.IsNotExist(err) {
		return transaction.plan, fmt.Errorf("remove committed command backup: %w", err)
	}
	transaction.plan.BackupRetained = false
	if err := transaction.deps.syncDir(filepath.Dir(transaction.plan.Path)); err != nil {
		return transaction.plan, fmt.Errorf("sync committed command cleanup: %w", err)
	}
	return transaction.plan, nil
}

func (transaction *ManagedCommandPathTransaction) revalidate() error {
	existing, err := inspectManagedCommand(transaction.plan.Path, transaction.deps.readBuildInfo)
	if err != nil || existing != transaction.existing {
		return fmt.Errorf("existing managed command identity changed after preflight")
	}
	candidate, err := inspectManagedCommand(transaction.candidate.path, transaction.deps.readBuildInfo)
	if err != nil || candidate != transaction.candidate {
		return fmt.Errorf("managed command candidate identity changed after preflight")
	}
	return nil
}

func sameManagedCommandSnapshot(actual, expected managedCommandIdentity) bool {
	return actual.sha256 == expected.sha256 && actual.mode == expected.mode && actual.size == expected.size &&
		actual.device == expected.device && actual.inode == expected.inode && actual.uid == expected.uid && actual.links == expected.links
}

func verifyManagedCommandSymlink(path, target string) error {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("managed command is not the expected symlink")
	}
	actual, err := os.Readlink(path)
	if err != nil || actual != target {
		return fmt.Errorf("managed command symlink target changed")
	}
	return nil
}

func (transaction *ManagedCommandPathTransaction) temporaryRegularPath(kind string) (string, error) {
	suffix, err := transaction.deps.randomSuffix()
	if err != nil || suffix == "" {
		return "", fmt.Errorf("allocate command %s path", kind)
	}
	return filepath.Join(filepath.Dir(transaction.plan.Path), ".issueops.command-"+kind+"-"+suffix), nil
}

func inspectManagedCommand(path string, readBuildInfo func(string) (managedBuildInfo, error)) (managedCommandIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return managedCommandIdentity{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o111 == 0 ||
		info.Size() <= 0 || info.Size() > managedCommandMaxSize || uint32(stat.Uid) != uint32(os.Geteuid()) || uint64(stat.Nlink) != 1 {
		return managedCommandIdentity{}, fmt.Errorf("managed command must be a current-user executable single-link regular file within the size limit")
	}
	identity := managedCommandIdentity{
		path: path, mode: info.Mode(), size: info.Size(), device: uint64(stat.Dev), inode: uint64(stat.Ino),
		uid: uint32(stat.Uid), links: uint64(stat.Nlink),
	}
	identity.sha256, err = hashManagedCommand(path, info, identity)
	if err != nil {
		return managedCommandIdentity{}, err
	}
	build, err := readBuildInfo(path)
	if err != nil || build.MainPath != managedCommandMainPath || build.ModulePath != managedCommandModulePath {
		return managedCommandIdentity{}, fmt.Errorf("command does not have the managed issueops build identity")
	}
	finalInfo, err := os.Lstat(path)
	if err != nil || !sameManagedCommandIdentity(finalInfo, info, identity) {
		return managedCommandIdentity{}, fmt.Errorf("managed command identity changed during build inspection")
	}
	return identity, nil
}

func hashManagedCommand(path string, expected os.FileInfo, identity managedCommandIdentity) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !sameManagedCommandIdentity(before, expected, identity) {
		return "", fmt.Errorf("managed command identity changed before hashing")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	after, err := file.Stat()
	if err != nil || !sameManagedCommandIdentity(after, expected, identity) {
		return "", fmt.Errorf("managed command identity changed during hashing")
	}
	pathAfter, err := os.Lstat(path)
	if err != nil || !sameManagedCommandIdentity(pathAfter, expected, identity) {
		return "", fmt.Errorf("managed command path changed during hashing")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sameManagedCommandIdentity(actual, expected os.FileInfo, identity managedCommandIdentity) bool {
	stat, ok := actual.Sys().(*syscall.Stat_t)
	return ok && actual.Mode() == expected.Mode() && actual.Size() == expected.Size() &&
		uint64(stat.Dev) == identity.device && uint64(stat.Ino) == identity.inode &&
		uint32(stat.Uid) == identity.uid && uint64(stat.Nlink) == identity.links
}

func copyManagedCommandFile(source, destination string, mode os.FileMode, expected *managedCommandIdentity) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	var before os.FileInfo
	if expected != nil {
		before, err = input.Stat()
		if err != nil || !sameManagedCommandIdentity(before, before, *expected) {
			return fmt.Errorf("managed command identity changed before backup")
		}
	}
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	clean := true
	defer func() {
		_ = output.Close()
		if clean {
			_ = os.Remove(destination)
		}
	}()
	writer := io.Writer(output)
	var digest hash.Hash
	if expected != nil {
		digest = sha256.New()
		writer = io.MultiWriter(output, digest)
	}
	if _, err := io.Copy(writer, input); err != nil {
		return err
	}
	if expected != nil {
		after, statErr := input.Stat()
		pathAfter, pathErr := os.Lstat(source)
		if statErr != nil || pathErr != nil || !sameManagedCommandIdentity(after, before, *expected) || !sameManagedCommandIdentity(pathAfter, before, *expected) ||
			hex.EncodeToString(digest.Sum(nil)) != expected.sha256 {
			return fmt.Errorf("managed command identity changed during backup")
		}
	}
	if err := output.Chmod(mode); err != nil {
		return err
	}
	if err := output.Sync(); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	clean = false
	return nil
}

func syncManagedCommandDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func verifyManagedCommandCopy(path string, expected managedCommandIdentity) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != expected.mode.Perm() || info.Size() != expected.size {
		return fmt.Errorf("restored command metadata does not match the backup identity")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected.sha256 {
		return fmt.Errorf("restored command bytes do not match the backup identity")
	}
	return nil
}
