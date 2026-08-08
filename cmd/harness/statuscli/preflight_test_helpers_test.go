package statuscli

import (
	"agent-harness/internal/adapter/projectdocs"
)

func init() {
	d := deps
	d.AnalyzeProjectSignals = projectdocs.AnalyzeProjectSignals
	Configure(d)
}
