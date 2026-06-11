package benchmark

func summarizeIssueOpsDimensionScores(scores []IssueOpsDimensionScore) (float64, float64) {
	total := 0.0
	counted := 0
	minimum := 0.0
	for _, score := range scores {
		if score.NotApplicable {
			continue
		}
		if counted == 0 || score.Score < minimum {
			minimum = score.Score
		}
		total += score.Score
		counted++
	}
	if counted == 0 {
		return 0, 0
	}
	return total / float64(counted), minimum
}

func summarizeIssueOpsRunScores(scores []IssueOpsBenchmarkScore) (float64, float64) {
	if len(scores) == 0 {
		return 0, 0
	}
	total := 0.0
	minimum := scores[0].MinimumScore
	for _, score := range scores {
		total += score.AverageScore
		if score.MinimumScore < minimum {
			minimum = score.MinimumScore
		}
	}
	return total / float64(len(scores)), minimum
}
