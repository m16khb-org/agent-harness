package core

func summarizeIssueOpsDimensionScores(scores []IssueOpsDimensionScore) (float64, float64) {
	if len(scores) == 0 {
		return 0, 0
	}
	total := 0.0
	minimum := scores[0].Score
	for _, score := range scores {
		total += score.Score
		if score.Score < minimum {
			minimum = score.Score
		}
	}
	return total / float64(len(scores)), minimum
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
