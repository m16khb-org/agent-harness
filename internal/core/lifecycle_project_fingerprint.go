package core

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func projectFingerprint(root string) ProjectFingerprint {
	gitDir := ""
	if info, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if info.IsDir() {
			gitDir = filepath.Join(root, ".git")
		} else if b, err := os.ReadFile(filepath.Join(root, ".git")); err == nil {
			gitDir = strings.TrimSpace(string(b))
		}
	}
	originHash := ""
	if origin := readGitOriginURL(root); origin != "" {
		sum := sha256.Sum256([]byte(origin))
		originHash = hex.EncodeToString(sum[:])
	}
	return ProjectFingerprint{RepoRoot: root, GitDir: gitDir, GitOriginHash: originHash}
}

func projectRepoID(fp ProjectFingerprint) string {
	parts := []string{fp.RepoRoot, fp.GitDir, fp.GitOriginHash}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:24]
}

func readGitOriginURL(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	inOrigin := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = trimmed == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(trimmed, "url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func projectFingerprintEqual(a, b ProjectFingerprint) bool {
	return a.RepoRoot == b.RepoRoot && a.GitDir == b.GitDir && a.GitOriginHash == b.GitOriginHash
}
