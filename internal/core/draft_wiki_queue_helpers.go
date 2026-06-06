package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func trimDraftWikiQueueMaterial(material string) string {
	material = strings.TrimSpace(material)
	if material == "" {
		return ""
	}
	lines := strings.Split(material, "\n")
	for i, line := range lines {
		lines[i] = redactFreeform(line)
	}
	material = strings.TrimSpace(strings.Join(lines, "\n"))
	const maxBytes = 12000
	if len([]byte(material)) <= maxBytes {
		return material
	}
	return string([]byte(material)[:maxBytes]) + "\n[truncated]"
}

func draftWikiQueueEventID(repoID, material, at string) string {
	sum := sha256.Sum256([]byte(repoID + "\x00" + material + "\x00" + at))
	return "dwq-" + hex.EncodeToString(sum[:])[:24]
}
