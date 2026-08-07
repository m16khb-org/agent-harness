package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	issueopscore "agent-harness/internal/adapter/issueops"
)

func benchmarkArtifactFromFixture(fixture issueopscore.IssueOpsBenchmarkFixture) issueopscore.IssueOpsBenchmarkArtifact {
	return benchmarkartifact.FromFixture(fixture)
}

func issueOpsBenchmarkBullets(items []string) string {
	return benchmarkartifact.Bullets(items)
}

func issueOpsBenchmarkOwnedTasks(items []string) string {
	return benchmarkartifact.OwnedTasks(items)
}
