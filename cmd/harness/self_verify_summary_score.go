package main

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	goals := selfVerificationGoalDefinitions()
	scores := make([]SelfVerificationGoalScore, 0, len(goals))
	runCount := result.Iterations
	if runCount < 1 {
		runCount = len(result.Runs)
	}
	for _, goal := range goals {
		passed := 0
		total := 0
		for iteration := 1; iteration <= runCount; iteration++ {
			steps := map[string]StepResult{}
			for _, run := range result.Runs {
				if run.Iteration != iteration {
					continue
				}
				for _, step := range run.Steps {
					steps[step.Label] = step
				}
				break
			}
			for _, label := range goal.Labels {
				total++
				if step, ok := steps[label]; ok && step.OK {
					passed++
				}
			}
		}
		score := 0.0
		if total > 0 {
			score = float64(passed) * 100 / float64(total)
		}
		scores = append(scores, SelfVerificationGoalScore{
			Name:           goal.Name,
			KoreanName:     goal.KoreanName,
			Score:          score,
			TargetScore:    targetScore,
			Passed:         score > targetScore,
			EvidenceLabels: append([]string{}, goal.Labels...),
			PassedChecks:   passed,
			TotalChecks:    total,
		})
	}
	return scores
}
