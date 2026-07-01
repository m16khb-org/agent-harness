// Package issuebody renders and idempotently splices a delimited managed
// section into a remote issue body, shared by the github and gitlab adapters.
package issuebody

import (
	"fmt"
	"strings"
)

const (
	startMarker = "<!-- issueops:devils-advocate:start -->"
	endMarker   = "<!-- issueops:devils-advocate:end -->"
)

// RenderDevilsAdvocateSection builds the delimited managed section for the
// devil's-advocate findings. The delimiters let MergeIssueBodySection replace
// the block in place on re-runs instead of appending duplicates.
func RenderDevilsAdvocateSection(findings []string, ts string) string {
	var b strings.Builder
	b.WriteString(startMarker + "\n")
	fmt.Fprintf(&b, "## Devil's-advocate findings (%s)\n", ts)
	for _, f := range findings {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", f)
	}
	b.WriteString(endMarker)
	return b.String()
}

// MergeIssueBodySection replaces the delimited managed section in body with
// section, or appends it when absent. It never touches content outside the
// delimiters, so the surrounding issue body round-trips exactly.
func MergeIssueBodySection(body, section string) string {
	s := strings.Index(body, startMarker)
	e := strings.Index(body, endMarker)
	if s >= 0 && e > s {
		return body[:s] + section + body[e+len(endMarker):]
	}
	trimmed := strings.TrimRight(body, "\n")
	if trimmed == "" {
		return section + "\n"
	}
	return trimmed + "\n\n" + section + "\n"
}
