package hookcli

import (
	hookadapter "agent-harness/internal/adapter/hook"
)

// production wiring과 같은 lint 게이트를 설치한다.
func init() {
	LintEditedGoFiles = hookadapter.LintEditedGoFiles
}
