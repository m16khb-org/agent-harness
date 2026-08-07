package install

import (
	"os"
	"path/filepath"
	"strings"
)

func absClean(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func expandHomeWithHome(path, home string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		if home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func homeRelativePath(path, home string) string {
	if path == "" || home == "" {
		return path
	}
	path = filepath.Clean(path)
	home = filepath.Clean(home)
	if path == home {
		return "~"
	}
	if rel, err := filepath.Rel(home, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filepath.Join("~", rel))
	}
	return path
}
