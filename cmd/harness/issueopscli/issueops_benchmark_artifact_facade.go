package issueopscli

import (
	"agent-harness/cmd/harness/issueopscli/benchmarkartifact"
	issueopscontract "agent-harness/internal/contract/issueops"
)

func benchmarkArtifactFromFixture(fixture issueopscontract.IssueOpsBenchmarkFixture) issueopscontract.IssueOpsBenchmarkArtifact {
	return benchmarkartifact.FromFixture(fixture)
}

func issueOpsBenchmarkBullets(items []string) string {
	return benchmarkartifact.Bullets(items)
}

func issueOpsBenchmarkOwnedTasks(items []string) string {
	return benchmarkartifact.OwnedTasks(items)
}
