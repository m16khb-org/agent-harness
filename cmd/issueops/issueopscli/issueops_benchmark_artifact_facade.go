package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/benchmarkartifact"
	issueopscontract "issueops/internal/contract/issueops"
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
