package validationcli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func forbiddenNameHits(root string) []string {
	var hits []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = name
			}
			if shouldSkipForbiddenNameScanDir(name, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".git" {
			return nil
		}
		if name == "LICENSE" {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil || bytes.Contains(b, []byte{0}) {
			return nil
		}
		text := allowCurrentOwnerHandle(string(b))
		for _, needle := range forbiddenLegacyNeedles() {
			if strings.Contains(text, needle) {
				rel, _ := filepath.Rel(root, path)
				hits = append(hits, rel+" contains "+needle)
				break
			}
		}
		return nil
	})
	sort.Strings(hits)
	if len(hits) > 20 {
		return hits[:20]
	}
	return hits
}

func shouldSkipForbiddenNameScanDir(name, rel string) bool {
	switch name {
	case ".git", "bin", "cache", ".cache", ".codex", ".codegraph", ".omc", ".omx", ".antigravitycli":
		return true
	}
	return filepath.ToSlash(rel) == ".claude/hooks/.logs"
}

func forbiddenLegacyNeedles() []string {
	return []string{"m" + "16kh", "m" + "16h", "M" + "16H", "m" + "16"}
}

func currentOwnerHandle() string {
	return "m" + "16khb"
}

func allowCurrentOwnerHandle(text string) string {
	return strings.ReplaceAll(text, currentOwnerHandle(), "$CURRENT_OWNER")
}

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	sanitized := allowCurrentOwnerHandle(text)
	replacements := []string{}
	if abs, err := filepath.Abs(root); err == nil {
		replacements = append(replacements, abs)
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements = append(replacements, home)
	}
	for _, runtimePath := range replacements {
		if runtimePath == "" || runtimePath == string(filepath.Separator) {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, runtimePath, "$RUNTIME_PATH")
	}
	for _, needle := range forbiddenLegacyNeedles() {
		if strings.Contains(sanitized, needle) {
			return true
		}
	}
	return false
}
