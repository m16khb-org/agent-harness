package apidoc

import "issueops/cmd/issueops/apidoc/staticcheck"

func CheckNestControllerStatic(file, text string) []StaticViolation {
	return staticcheck.CheckNestController(file, text)
}
