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
