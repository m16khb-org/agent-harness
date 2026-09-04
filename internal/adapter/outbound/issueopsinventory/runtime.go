package issueopsinventory

import (
	"os/exec"
	"time"

	"issueops/internal/domain/repoidentity"
)

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

type CleanPath struct{}

func (CleanPath) Normalize(path string) string {
	clean := repoidentity.SourceRoot(path, "")
	if clean == "" {
		return ""
	}
	command := exec.Command("git", "rev-parse", "--path-format=relative", "--git-common-dir")
	command.Dir = clean
	commonDir, err := command.Output()
	if err != nil {
		return clean
	}
	return repoidentity.SourceRoot(clean, string(commonDir))
}
