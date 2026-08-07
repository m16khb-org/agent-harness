package failurecause

import (
	"sort"
	"strings"

	"agent-harness/internal/adapter/policy"
)

type Cause string

const (
	None               Cause = "none"
	Model              Cause = "model"
	HarnessEnvironment Cause = "harness_environment"
	Transport          Cause = "transport"
	ContractInput      Cause = "contract_input"
	Unknown            Cause = "unknown"
)

type Evidence struct {
	Cause  Cause  `json:"cause"`
	Code   string `json:"code"`
	Source string `json:"source"`
}
type Result struct {
	Cause    Cause      `json:"cause"`
	Reason   string     `json:"reason"`
	Evidence []Evidence `json:"evidence"`
}

func Classify(failed bool, items []Evidence) Result {
	if !failed {
		return Result{Cause: None, Reason: "no_failed_steps", Evidence: []Evidence{}}
	}
	out := make([]Evidence, 0, len(items))
	for _, e := range items {
		if e.Cause == Model || e.Cause == HarnessEnvironment || e.Cause == Transport || e.Cause == ContractInput || e.Cause == Unknown {
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
	cause := Unknown
	for _, want := range []Cause{Transport, HarnessEnvironment, ContractInput, Model} {
		for _, e := range out {
			if e.Cause == want {
				cause = want
				break
			}
		}
		if cause != Unknown {
			break
		}
	}
	reason := "no_typed_evidence"
	if cause != Unknown {
		codes := []string{}
		for _, e := range out {
			if e.Cause == cause && (len(codes) == 0 || codes[len(codes)-1] != e.Code) {
				codes = append(codes, e.Code)
			}
		}
		reason = string(cause) + ":" + strings.Join(codes, "+")
	}
	return Result{Cause: cause, Reason: reason, Evidence: out}
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
