package validationcli

func (s *stateRoundtripSelfVerifySession) writeCompareSnapshots(input validateStateRoundtripSelfVerifyInput, baselineCompareKey, candidateCompareKey string) StepResult {
	compareSummary := SelfAugmentSummary{
		TotalRuns:    10,
		TotalSteps:   20,
		PassedSteps:  20,
		StepLabels:   []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{{Iteration: 1, Seed: input.seed, Label: "go test", DurationMS: 1000}},
	}
	for _, snapshot := range []struct {
		key       string
		elapsedMS int64
		at        string
	}{
		{baselineCompareKey, 1000, "2000-01-01T00:00:00Z"},
		{candidateCompareKey, 1100, "2000-01-01T00:01:00Z"},
	} {
		if err := input.deps.writeSnapshot(input.tempState, snapshot.key, SelfAugmentStateSnapshot{
			SchemaVersion: 1,
			Kind:          selfVerificationSummaryKind,
			OK:            true,
			Iterations:    10,
			BaseSeed:      input.seed,
			ElapsedMS:     snapshot.elapsedMS,
			HarnessRoot:   input.root,
			GeneratedAt:   snapshot.at,
			Summary:       compareSummary,
		}); err != nil {
			return s.fail(err.Error())
		}
	}
	return StepResult{OK: true}
}
