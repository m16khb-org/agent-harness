package apidoc

import "agent-harness/cmd/harness/apidoc/staticcheck"

func CheckNestControllerStatic(file, text string) []StaticViolation {
	return staticcheck.CheckNestController(file, text)
}
