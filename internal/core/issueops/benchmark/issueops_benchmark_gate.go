package benchmark

import (
	"fmt"
	"path/filepath"
	"strings"
)

func EvaluateIssueOpsAutoresearchGate(req IssueOpsAutoresearchGateRequest) IssueOpsAutoresearchGateResult {
	compare := CompareIssueOpsBenchmarkRuns(req.BaselineRun, req.CandidateRun)
	result := IssueOpsAutoresearchGateResult{
		OK:               true,
		KeepCandidate:    true,
		CandidateID:      strings.TrimSpace(req.Candidate.ID),
		BenchmarkCompare: compare,
	}
	if result.CandidateID == "" {
		result.DiscardReasons = append(result.DiscardReasons, "candidate id is required")
	}
	if strings.TrimSpace(req.Candidate.Hypothesis) == "" {
		result.DiscardReasons = append(result.DiscardReasons, "candidate hypothesis is required")
	}
	if len(req.Candidate.TargetDimensions) == 0 {
		result.DiscardReasons = append(result.DiscardReasons, "target dimensions are required")
	} else {
		for _, target := range req.Candidate.TargetDimensions {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			if !isKnownIssueOpsBenchmarkDimension(target) {
				result.DiscardReasons = append(result.DiscardReasons, fmt.Sprintf("invalid target dimension %q", target))
			}
		}
	}
	if len(req.Candidate.EditSurface) == 0 {
		result.DiscardReasons = append(result.DiscardReasons, "edit surface is required")
	}
	if !req.CandidateRun.OK {
		result.DiscardReasons = append(result.DiscardReasons, "candidate benchmark did not pass")
	}
	if !compare.OK {
		result.DiscardReasons = append(result.DiscardReasons, "benchmark comparison regressed")
	}
	result.EditSurfaceViolations = issueOpsEditSurfaceViolations(req.ChangedPaths, req.Candidate.EditSurface)
	if len(result.EditSurfaceViolations) > 0 {
		result.DiscardReasons = append(result.DiscardReasons, "changed paths outside declared edit surface")
	}
	result.TargetDimensionRegressions = issueOpsTargetDimensionRegressions(req.Candidate.TargetDimensions, req.BaselineRun, req.CandidateRun)
	if len(result.TargetDimensionRegressions) > 0 {
		result.DiscardReasons = append(result.DiscardReasons, "target dimensions regressed")
	}
	result.KeepCandidate = len(result.DiscardReasons) == 0
	result.OK = result.KeepCandidate
	return result
}

func compareIssueOpsDimensionRegressions(baseline, candidate IssueOpsBenchmarkRunResult) []string {
	baselineScores := issueOpsDimensionMinimums(baseline)
	candidateScores := issueOpsDimensionMinimums(candidate)
	var regressions []string
	for _, dimension := range issueOpsBenchmarkDimensions {
		if candidateScores[dimension] < baselineScores[dimension] {
			regressions = append(regressions, dimension)
		}
	}
	return regressions
}

func issueOpsTargetDimensionRegressions(targets []string, baseline, candidate IssueOpsBenchmarkRunResult) []string {
	baselineScores := issueOpsDimensionMinimums(baseline)
	candidateScores := issueOpsDimensionMinimums(candidate)
	var regressions []string
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if candidateScores[target] < baselineScores[target] {
			regressions = append(regressions, target)
		}
	}
	return regressions
}

func issueOpsEditSurfaceViolations(changedPaths, editSurface []string) []string {
	if len(changedPaths) == 0 || len(editSurface) == 0 {
		return nil
	}
	var violations []string
	for _, changedPath := range changedPaths {
		changedPath = normalizeIssueOpsPath(changedPath)
		if changedPath == "" {
			continue
		}
		if !issueOpsPathAllowed(changedPath, editSurface) {
			violations = append(violations, changedPath)
		}
	}
	return violations
}

func issueOpsPathAllowed(changedPath string, editSurface []string) bool {
	for _, pattern := range editSurface {
		pattern = normalizeIssueOpsPath(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if changedPath == prefix || strings.HasPrefix(changedPath, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(changedPath)); ok {
			return true
		}
		if changedPath == pattern {
			return true
		}
	}
	return false
}

func isKnownIssueOpsBenchmarkDimension(target string) bool {
	for _, dimension := range issueOpsBenchmarkDimensions {
		if dimension == target {
			return true
		}
	}
	return false
}

func normalizeIssueOpsPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimPrefix(path, "./")
	return strings.Trim(path, "/")
}

func issueOpsDimensionMinimums(run IssueOpsBenchmarkRunResult) map[string]float64 {
	minimums := make(map[string]float64)
	seen := make(map[string]bool)
	for _, score := range run.Scores {
		for _, dimensionScore := range score.DimensionScores {
			if dimensionScore.NotApplicable {
				continue
			}
			if !seen[dimensionScore.Dimension] || dimensionScore.Score < minimums[dimensionScore.Dimension] {
				minimums[dimensionScore.Dimension] = dimensionScore.Score
				seen[dimensionScore.Dimension] = true
			}
		}
	}
	return minimums
}
