package pioneerskill

var canonicalNames = []string{
	"web-research",
	"requirements-analysis",
	"design-review",
	"database-design",
	"algorithm-optimization",
	"meeting-notes",
	"issueops-debugging",
	"prompt-engineering",
	"code-quality-metrics",
	"git-operations",
	"verified-execution",
	"implementation-planning",
}

func Names() []string {
	return append([]string(nil), canonicalNames...)
}

func Missing(observed []string) []string {
	present := make(map[string]bool, len(observed))
	for _, name := range observed {
		present[name] = true
	}
	missing := make([]string, 0)
	for _, name := range canonicalNames {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	return missing
}
