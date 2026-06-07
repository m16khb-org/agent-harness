package apidoc

import (
	"agent-harness/cmd/harness/apidoc/reviewprompt"
)

func BuildReviewPrompt(files []string, diff, extraPrompt string) string {
	return reviewprompt.Build(files, diff, extraPrompt)
}

func ReviewSchema() map[string]any {
	return reviewprompt.Schema()
}
