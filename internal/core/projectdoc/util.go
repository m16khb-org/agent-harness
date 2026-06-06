package projectdoc

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

type ProjectDocsPlannedFile struct {
	RelPath string `json:"rel_path"`
	Path    string `json:"path"`
	Action  string `json:"action"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
	Reason  string `json:"reason"`
}

func PlannedFileAction(path, content string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "create"
	}
	if string(b) == content {
		return "unchanged"
	}
	return "update"
}

func SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
