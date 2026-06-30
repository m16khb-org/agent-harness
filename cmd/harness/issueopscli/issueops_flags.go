package issueopscli

import "strings"

// repeatedFlag is the canonical repeatable string flag for the issueopscli
// package: each --flag occurrence appends one value and String() joins the
// collected values with "," for usage/default display.
type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// sliceFlag is a deprecated alias for repeatedFlag, retained so the existing
// callers in this package (domain-review, ai-slop-clean, compatibility, and
// execution) keep compiling without churn. It is the same type with the same
// "," String() format; new code should use repeatedFlag directly.
type sliceFlag = repeatedFlag
