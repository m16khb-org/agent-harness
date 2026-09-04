package apidoc

import "issueops/cmd/issueops/apidoc/staticcheck"

func CheckNestDTOStatic(file, text string) []StaticViolation {
	return staticcheck.CheckNestDTO(file, text)
}
