package invariants

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

func ForbiddenNameHits(root string) []string {
	var hits []string
	var hitsMu sync.Mutex
	var reads errgroup.Group
	reads.SetLimit(8)
	owner := []byte(currentOwnerHandle())
	needleNames := forbiddenLegacyNeedles()
	needles := make([][]byte, len(needleNames))
	for index, needle := range needleNames {
		needles[index] = []byte(needle)
	}
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
		reads.Go(func() error {
			b, readErr := os.ReadFile(path)
			if readErr != nil || bytes.Contains(b, []byte{0}) {
				return nil
			}
			if needle := firstForbiddenLegacyNeedle(b, owner, needles, needleNames); needle != "" {
				rel, _ := filepath.Rel(root, path)
				hitsMu.Lock()
				hits = append(hits, rel+" contains "+needle)
				hitsMu.Unlock()
			}
			return nil
		})
		return nil
	})
	_ = reads.Wait()
	sort.Strings(hits)
	if len(hits) > 20 {
		return hits[:20]
	}
	return hits
}

func firstForbiddenLegacyNeedle(content, owner []byte, needles [][]byte, needleNames []string) string {
	for index, needle := range needles {
		remaining := content
		for {
			offset := bytes.Index(remaining, needle)
			if offset < 0 {
				break
			}
			if !bytes.HasPrefix(remaining[offset:], owner) {
				return needleNames[index]
			}
			remaining = remaining[offset+len(owner):]
		}
	}
	return ""
}

func forbiddenNameHits(root string) []string {
	return ForbiddenNameHits(root)
}

func shouldSkipForbiddenNameScanDir(name, rel string) bool {
	switch name {
	case ".git", "bin", "cache", ".cache", ".codex", ".codegraph", ".omc", ".omx", ".antigravitycli":
		return true
	}
	switch filepath.ToSlash(rel) {
	case ".claude/hooks/.logs", ".omo/senpi-task":
		return true
	default:
		return false
	}
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

func ContainsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
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

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return ContainsForbiddenLegacyOutsideRuntimePaths(text, root)
}
