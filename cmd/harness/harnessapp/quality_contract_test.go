package harnessapp

import "agent-harness/cmd/harness/qualitycli"

func qualitycliInspectDepsForContract() qualitycli.InspectDeps {
	return qualitycli.InspectDeps{
		Now: func() string { return "2000-01-01T00:00:00Z" },
		Coverage: func(string) (string, error) {
			return "ok  \tagent-harness/internal/core/commandguard\t0.011s\tcoverage: 54.3% of statements\n", nil
		},
		SelfAugmentOpenCount: func(string) (int, error) { return 10, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
	}
}
