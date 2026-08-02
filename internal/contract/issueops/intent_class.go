package issueops

import (
	"fmt"
	"strings"
)

// IntentClass values control how strict the plan-prep evidence gate is.
// trivial cycles skip the gate; every other class must satisfy it.
var knownIntentClasses = map[string]bool{
	"trivial":      true,
	"standard":     true,
	"refactoring":  true,
	"architecture": true,
	"research":     true,
}

// NormalizeIntentClass lowercases and validates an intent class. Empty input
// normalizes to "standard" so untagged cycles stay gated (safe default).
func NormalizeIntentClass(class string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(class))
	if c == "" {
		return "standard", nil
	}
	if !knownIntentClasses[c] {
		return "", fmt.Errorf("unknown intent_class %q; want one of trivial, standard, refactoring, architecture, research", c)
	}
	return c, nil
}
