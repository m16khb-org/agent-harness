package apidoc

import "agent-harness/cmd/harness/apidoc/staticcheck"

func CheckNestDTOStatic(file, text string) []StaticViolation {
	return staticcheck.CheckNestDTO(file, text)
}
