package installcli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"issueops/internal/port"
)

type installHostPathSnapshot struct {
	path       string
	mode       os.FileMode
	content    []byte
	linkTarget string
	kind       installHostPathKind
	existed    bool
	wasSymlink bool
}

type installHostPathKind uint8

const (
	installHostFile installHostPathKind = iota
	installHostLink
)

type installHostTransaction struct {
	snapshots []installHostPathSnapshot
	parents   []installHostPathSnapshot
}

func prepareInstallHostTransaction(plan port.NativeInstallResult) (*installHostTransaction, error) {
	paths := map[string]installHostPathKind{}
	for _, file := range plan.Files {
		if file.Path != "" {
			path := filepath.Clean(file.Path)
			if kind, exists := paths[path]; exists && kind != installHostFile {
				return nil, fmt.Errorf("native install path is both file and symlink: %s", path)
			}
			paths[path] = installHostFile
		}
	}
	for _, link := range plan.Links {
		if link.Path != "" {
			path := filepath.Clean(link.Path)
			if kind, exists := paths[path]; exists && kind != installHostLink {
				return nil, fmt.Errorf("native install path is both file and symlink: %s", path)
			}
			paths[path] = installHostLink
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	transaction := &installHostTransaction{}
	parentSnapshots := map[string]installHostPathSnapshot{}
	for _, path := range ordered {
		snapshot, err := captureInstallHostPath(path, paths[path])
		if err != nil {
			return nil, err
		}
		transaction.snapshots = append(transaction.snapshots, snapshot)
		for parent := filepath.Dir(path); parent != "." && parent != string(filepath.Separator); parent = filepath.Dir(parent) {
			if parentSnapshot, seen := parentSnapshots[parent]; seen {
				if parentSnapshot.existed {
					break
				}
				continue
			}
			parentSnapshot, parentErr := captureInstallHostParent(parent)
			if parentErr != nil {
				return nil, parentErr
			}
			parentSnapshots[parent] = parentSnapshot
			transaction.parents = append(transaction.parents, parentSnapshot)
			if parentSnapshot.existed {
				break
			}
		}
	}
	sort.Slice(transaction.parents, func(left, right int) bool {
		return pathDepth(transaction.parents[left].path) > pathDepth(transaction.parents[right].path)
	})
	return transaction, nil
}

func captureInstallHostParent(path string) (installHostPathSnapshot, error) {
	snapshot := installHostPathSnapshot{path: path}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	if !info.IsDir() {
		return snapshot, fmt.Errorf("native install rollback parent is not a directory: %s", path)
	}
	snapshot.existed = true
	return snapshot, nil
}

func captureInstallHostPath(path string, kind installHostPathKind) (installHostPathSnapshot, error) {
	snapshot := installHostPathSnapshot{path: path, kind: kind}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	snapshot.existed = true
	if kind == installHostLink {
		if info.Mode()&os.ModeSymlink == 0 {
			return snapshot, fmt.Errorf("native install rollback expected symlink path: %s", path)
		}
		snapshot.linkTarget, err = os.Readlink(path)
		return snapshot, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		snapshot.wasSymlink = true
		snapshot.linkTarget, err = os.Readlink(path)
		if err != nil {
			return snapshot, err
		}
		info, err = os.Stat(path)
		if err != nil {
			return snapshot, err
		}
	}
	if !info.Mode().IsRegular() {
		return snapshot, fmt.Errorf("native install rollback cannot snapshot non-file path: %s", path)
	}
	snapshot.mode = info.Mode()
	snapshot.content, err = os.ReadFile(path)
	return snapshot, err
}

func (transaction *installHostTransaction) rollback() error {
	if transaction == nil {
		return nil
	}
	var errs []error
	for index := len(transaction.snapshots) - 1; index >= 0; index-- {
		if err := restoreInstallHostPath(transaction.snapshots[index]); err != nil {
			errs = append(errs, err)
		}
	}
	for _, snapshot := range transaction.parents {
		if snapshot.existed {
			continue
		}
		if err := removeInstallDirectoryIfEmpty(snapshot.path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func restoreInstallHostPath(snapshot installHostPathSnapshot) error {
	if !snapshot.existed {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(snapshot.path), 0o755); err != nil {
		return err
	}
	if snapshot.kind == installHostLink {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return os.Symlink(snapshot.linkTarget, snapshot.path)
	}
	if snapshot.wasSymlink {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Symlink(snapshot.linkTarget, snapshot.path); err != nil {
			return err
		}
		if err := os.WriteFile(snapshot.path, snapshot.content, snapshot.mode.Perm()); err != nil {
			return err
		}
		return os.Chmod(snapshot.path, snapshot.mode.Perm())
	}
	info, err := os.Lstat(snapshot.path)
	if err == nil && !info.Mode().IsRegular() {
		if err := os.Remove(snapshot.path); err != nil {
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(snapshot.path, snapshot.content, snapshot.mode.Perm()); err != nil {
		return err
	}
	return os.Chmod(snapshot.path, snapshot.mode.Perm())
}

func pathDepth(path string) int {
	depth := 0
	for path != "." && path != string(filepath.Separator) {
		depth++
		next := filepath.Dir(path)
		if next == path {
			break
		}
		path = next
	}
	return depth
}

func removeInstallDirectoryIfEmpty(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("native install rollback parent changed to non-directory: %s", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	return os.Remove(path)
}
