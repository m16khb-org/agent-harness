package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (IssueOpsBenchmarkRunResult, error) {
	result := IssueOpsBenchmarkRunResult{
		ID:           "issueops-benchmark-" + time.Now().UTC().Format("20060102T150405.000000000Z"),
		FixtureCount: len(req.Fixtures),
	}
	for _, fixture := range req.Fixtures {
		artifact := req.Artifacts[fixture.ID]
		score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
		result.Scores = append(result.Scores, score)
		result.CriticalFailureCount += len(score.CriticalFailures)
	}
	result = FinalizeIssueOpsBenchmarkRunResult(result)
	if strings.TrimSpace(req.StateRoot) != "" {
		if err := SaveIssueOpsBenchmarkRun(req.StateRoot, result); err != nil {
			return IssueOpsBenchmarkRunResult{}, err
		}
	}
	return result, nil
}

func SaveIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	return persistIssueOpsBenchmarkRun(stateRoot, result)
}

func ReadIssueOpsBenchmarkRun(stateRoot, id string) (IssueOpsBenchmarkRunResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return IssueOpsBenchmarkRunResult{}, fmt.Errorf("benchmark id is required")
	}
	b, err := os.ReadFile(filepath.Join(stateRoot, "issueops-benchmarks", id+".json"))
	if err != nil {
		return IssueOpsBenchmarkRunResult{}, err
	}
	var result IssueOpsBenchmarkRunResult
	if err := json.Unmarshal(b, &result); err != nil {
		return IssueOpsBenchmarkRunResult{}, err
	}
	return result, nil
}

func FinalizeIssueOpsBenchmarkRunResult(result IssueOpsBenchmarkRunResult) IssueOpsBenchmarkRunResult {
	result.FixtureCount = len(result.Scores)
	result.CriticalFailureCount = 0
	for _, score := range result.Scores {
		result.CriticalFailureCount += len(score.CriticalFailures)
	}
	result.AverageScore, result.MinimumScore = summarizeIssueOpsRunScores(result.Scores)
	result.OK = result.CriticalFailureCount == 0
	for _, score := range result.Scores {
		if !score.Passed {
			result.OK = false
			break
		}
	}
	return result
}

func MergeIssueOpsBenchmarkScoreWithJudge(deterministic, judge IssueOpsBenchmarkScore) IssueOpsBenchmarkScore {
	merged := deterministic
	judgeByDimension := make(map[string]IssueOpsDimensionScore)
	for _, score := range judge.DimensionScores {
		judgeByDimension[score.Dimension] = score
	}
	for i, score := range merged.DimensionScores {
		if score.NotApplicable {
			// N/A dimension은 채점에서 제외한다. judge score가 적용되지 않는
			// fixture에서 이를 다시 끌어들여서는 안 된다.
			continue
		}
		judgeScore, ok := judgeByDimension[score.Dimension]
		if !ok {
			continue
		}
		if judgeScore.Score < score.Score {
			merged.DimensionScores[i].Score = judgeScore.Score
		}
		merged.DimensionScores[i].Evidence = strings.TrimSpace(score.Evidence + "; judge: " + judgeScore.Evidence)
	}
	merged.JudgeFailures = append(merged.JudgeFailures, judge.JudgeFailures...)
	merged.CriticalFailures = append(merged.CriticalFailures, judge.CriticalFailures...)
	if len(judge.DimensionScores) == 0 {
		merged.JudgeFailures = append(merged.JudgeFailures, "judge returned no dimension scores")
	}
	merged.AverageScore, merged.MinimumScore = summarizeIssueOpsDimensionScores(merged.DimensionScores)
	merged.Passed = len(merged.CriticalFailures) == 0 && len(merged.DeterministicFailures) == 0 && len(merged.JudgeFailures) == 0 && merged.MinimumScore >= issueOpsBenchmarkMaxScore
	merged.OK = merged.Passed
	return merged
}

func CompareIssueOpsBenchmarkRuns(baseline, candidate IssueOpsBenchmarkRunResult) IssueOpsBenchmarkCompareResult {
	result := IssueOpsBenchmarkCompareResult{
		OK:                   true,
		BaselineID:           baseline.ID,
		CandidateID:          candidate.ID,
		AverageScoreDelta:    candidate.AverageScore - baseline.AverageScore,
		MinimumScoreDelta:    candidate.MinimumScore - baseline.MinimumScore,
		CriticalFailureDelta: candidate.CriticalFailureCount - baseline.CriticalFailureCount,
		Regressions:          compareIssueOpsDimensionRegressions(baseline, candidate),
	}
	result.Improved = result.AverageScoreDelta > 0 &&
		result.MinimumScoreDelta >= 0 &&
		result.CriticalFailureDelta <= 0 &&
		len(result.Regressions) == 0
	result.OK = result.MinimumScoreDelta >= 0 &&
		result.CriticalFailureDelta <= 0 &&
		len(result.Regressions) == 0
	return result
}

func persistIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	dir := filepath.Join(stateRoot, "issueops-benchmarks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, result.ID+".json"), b, 0o644)
}
