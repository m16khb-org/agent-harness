package projectdocs

import (
	projectdocscontract "agent-harness/internal/contract/projectdocs"
	"fmt"
	"strings"
	"time"
)

// renderProjectDocsAppendRecordFile renders a standalone modular record file
// with canonical frontmatter (folder-first layout, one file per record).
func renderProjectDocsAppendRecordFile(kind, name, description string, req projectdocscontract.ProjectDocsAppendRequest, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n\n", name, description)
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimSpace(req.Title))
	fmt.Fprintf(&b, "- Date: %s\n", now.Format("2006-01-02"))
	b.WriteString(renderProjectDocsAppendFields(kind, req))
	return b.String()
}

func renderProjectDocsAppendFields(kind string, req projectdocscontract.ProjectDocsAppendRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- Kind: `%s`\n", kind)
	if source := strings.TrimSpace(req.Source); source != "" {
		fmt.Fprintf(&b, "- Source: %s\n", source)
	}
	fmt.Fprintf(&b, "- Summary: %s\n", strings.TrimSpace(req.Summary))
	if v := strings.TrimSpace(req.Context); v != "" {
		fmt.Fprintf(&b, "- Context: %s\n", v)
	}
	if kind == "caution" {
		if v := strings.TrimSpace(req.Resolution); v != "" {
			fmt.Fprintf(&b, "- Resolution: %s\n", v)
		}
	} else {
		if v := strings.TrimSpace(req.Decision); v != "" {
			fmt.Fprintf(&b, "- Decision: %s\n", v)
		}
		if v := strings.TrimSpace(req.Consequences); v != "" {
			fmt.Fprintf(&b, "- Consequences: %s\n", v)
		}
	}
	if len(req.Evidence) > 0 {
		b.WriteString("- Evidence:\n")
		for _, ev := range req.Evidence {
			if ev = strings.TrimSpace(ev); ev != "" {
				fmt.Fprintf(&b, "  - %s\n", ev)
			}
		}
	}
	if len(req.Alternatives) > 0 {
		b.WriteString("- Alternatives / rejected options:\n")
		for _, alt := range req.Alternatives {
			if alt = strings.TrimSpace(alt); alt != "" {
				fmt.Fprintf(&b, "  - %s\n", alt)
			}
		}
	}
	return b.String()
}