package apidoc

import (
	"agent-harness/cmd/harness/apidoc/reviewprompt"
)

func BuildReviewPrompt(files []string, diff, extraPrompt, evidence string) string {
	return reviewprompt.Build(files, diff, extraPrompt, evidence)
}

func ReviewSchema() map[string]any {
	return reviewprompt.Schema()
}
