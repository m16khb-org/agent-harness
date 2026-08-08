package harnessapp

import (
	"agent-harness/cmd/harness/hookcli"
	hookadapter "agent-harness/internal/adapter/hook"
)

// configureTail6은 편집 파일 lint를 설치한다.
func configureTail6() {
	hookcli.LintEditedGoFiles = hookadapter.LintEditedGoFiles
}
