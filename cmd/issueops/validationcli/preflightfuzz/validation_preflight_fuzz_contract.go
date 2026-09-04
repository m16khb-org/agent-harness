package preflightfuzz

import preflightcontract "issueops/internal/contract/preflight"

func preflightFuzzValidationErrors(preflight preflightcontract.PreflightResult) []string {
	errs := []string{}
	if !preflight.OK {
		errs = append(errs, "preflight ok=false")
	}
	if preflight.CommitStyleHints["conventional_subjects"] != float64(1) {
		errs = append(errs, "conventional subject not detected")
	}
	if preflight.CommitStyleHints["lore_bodies"] != float64(1) {
		errs = append(errs, "Lore body not detected")
	}
	if len(preflight.SecretLikePaths) == 0 {
		errs = append(errs, "secret-like path not detected")
	}
	return errs
}
