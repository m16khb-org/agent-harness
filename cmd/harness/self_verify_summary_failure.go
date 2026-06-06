package main

import (
	"sort"
)

func classifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	clusters := selfVerificationFailureClusters(result)
	if summary.FailedSteps == 0 {
		return "", "", nil
	}
	if len(clusters) == 0 {
		return "unknown", "summary reports failed steps but no failed step details were captured", nil
	}
	if summary.FailedSteps < summary.TotalRuns {
		return "intermittent", "only some completed seeds failed", clusters
	}
	if len(clusters) == 1 && clusters[0].Count == 1 {
		return "single_failure_observation", "self-verify is fail-fast; rerun the same seed before calling the failure flaky or deterministic", clusters
	}
	if len(clusters) == 1 {
		return "deterministic", "all completed failing seeds failed at the same step", clusters
	}
	return "mixed", "multiple failure steps were observed across completed seeds", clusters
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	byStep := map[string][]int64{}
	for _, run := range result.Runs {
		for _, step := range run.Steps {
			if step.OK {
				continue
			}
			byStep[step.Label] = append(byStep[step.Label], run.Seed)
		}
	}
	steps := make([]string, 0, len(byStep))
	for step := range byStep {
		steps = append(steps, step)
	}
	sort.Strings(steps)
	clusters := []SelfVerificationFailureCluster{}
	for _, step := range steps {
		seeds := append([]int64{}, byStep[step]...)
		sort.Slice(seeds, func(i, j int) bool { return seeds[i] < seeds[j] })
		clusters = append(clusters, SelfVerificationFailureCluster{
			Step:  step,
			Seeds: seeds,
			Count: len(seeds),
		})
	}
	return clusters
}
