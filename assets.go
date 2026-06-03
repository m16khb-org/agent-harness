// Package assets embeds the shared skills and config templates into the binary so
// agent-harness can install its native integrations without a repository checkout
// (for example after `brew install`). When a repository root is available
// (HARNESS_ROOT or a discoverable checkout), callers prefer the live files; the
// embedded copies are the fallback source for packaged binaries.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed skills configs
var embedded embed.FS

// SkillsFS returns the embedded skills tree rooted at the skills directory.
func SkillsFS() (fs.FS, error) {
	return fs.Sub(embedded, "skills")
}

// ConfigsFS returns the embedded configs tree rooted at the configs directory.
func ConfigsFS() (fs.FS, error) {
	return fs.Sub(embedded, "configs")
}

// Embedded exposes the raw embedded filesystem (skills/ and configs/ at the root).
func Embedded() fs.FS {
	return embedded
}
