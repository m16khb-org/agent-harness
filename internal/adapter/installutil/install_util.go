package installutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/port"
)

func WriteText(path, kind, content string, perm os.FileMode) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, []byte(content)) {
		return file, nil
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func WriteJSON(path, kind string, value any, perm os.FileMode) (port.InstallFile, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return port.InstallFile{Path: path, Kind: kind}, err
	}
	return WriteText(path, kind, string(append(b, '\n')), perm)
}

func EnsureSymlink(target, path string) (port.InstallLink, error) {
	link := port.InstallLink{Path: path, Target: target}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return link, err
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return link, fmt.Errorf("refusing to replace non-symlink path: %s", path)
		}
		current, readErr := os.Readlink(path)
		if readErr == nil && current == target {
			return link, nil
		}
		if err := os.Remove(path); err != nil {
			return link, err
		}
	} else if !os.IsNotExist(err) {
		return link, err
	}
	if err := os.Symlink(target, path); err != nil {
		return link, err
	}
	link.Created = true
	return link, nil
}

func TOMLString(value string) string {
	b, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(b)
}
