package projectdoc

import (
	"crypto/sha256"
	"encoding/hex"
)

type ProjectDocCatalogEntry struct {
	RelPath     string `json:"rel_path"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func SHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
