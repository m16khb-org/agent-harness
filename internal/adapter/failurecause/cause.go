package failurecause

import (
	"sort"
	"strings"

	"issueops/internal/domain/policy"

	failurecausecontract "issueops/internal/contract/failurecause"
)

func Classify(failed bool, items []failurecausecontract.Evidence) failurecausecontract.Result {
	if !failed {
		return failurecausecontract.Result{Cause: failurecausecontract.None, Reason: "no_failed_steps", Evidence: []failurecausecontract.Evidence{}}
	}
	out := make([]failurecausecontract.Evidence, 0, len(items))
	for _, e := range items {
		if e.Cause == failurecausecontract.Model || e.Cause == failurecausecontract.HarnessEnvironment || e.Cause == failurecausecontract.Transport || e.Cause == failurecausecontract.ContractInput || e.Cause == failurecausecontract.Unknown {
			e.Code = token(e.Code)
			e.Source = token(e.Source)
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cause != out[j].Cause {
			return out[i].Cause < out[j].Cause
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Source < out[j].Source
	})
	cause := failurecausecontract.Unknown
	for _, want := range []failurecausecontract.Cause{failurecausecontract.Transport, failurecausecontract.HarnessEnvironment, failurecausecontract.ContractInput, failurecausecontract.Model} {
		for _, e := range out {
			if e.Cause == want {
				cause = want
				break
			}
		}
		if cause != failurecausecontract.Unknown {
			break
		}
	}
	reason := "no_typed_evidence"
	if cause != failurecausecontract.Unknown {
		codes := []string{}
		for _, e := range out {
			if e.Cause == cause && (len(codes) == 0 || codes[len(codes)-1] != e.Code) {
				codes = append(codes, e.Code)
			}
		}
		reason = string(cause) + ":" + strings.Join(codes, "+")
	}
	return failurecausecontract.Result{Cause: cause, Reason: reason, Evidence: out}
}
func token(s string) string {
	s = policy.RedactDiagnostic(s)
	if s == "<redacted>" || strings.Contains(s, "[REDACTED") {
		return "redacted"
	}
	var out strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == ':' || r == '-' {
			if separator {
				out.WriteByte('_')
			}
			out.WriteRune(r)
			separator = false
			continue
		}
		if out.Len() > 0 {
			separator = true
		}
	}
	value := strings.Trim(out.String(), "_")
	if value == "" {
		value = "unknown"
	}
	if len(value) > 96 {
		return value[:96]
	}
	return value
}
