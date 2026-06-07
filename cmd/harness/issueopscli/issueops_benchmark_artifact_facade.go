package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	"agent-harness/internal/core"
)

func benchmarkArtifactFromFixture(fixture core.IssueOpsBenchmarkFixture) core.IssueOpsBenchmarkArtifact {
	return benchmarkartifact.FromFixture(fixture)
}

func issueOpsBenchmarkBullets(items []string) string {
	return benchmarkartifact.Bullets(items)
}

func issueOpsBenchmarkOwnedTasks(items []string) string {
	return benchmarkartifact.OwnedTasks(items)
}
