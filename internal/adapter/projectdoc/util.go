package projectdoc

import (
	"os"
)

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
