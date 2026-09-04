package qualitycli

import "testing"

func BenchmarkQualityInspectSignals(benchmark *testing.B) {
	root := resolveRoot("")
	benchmarks := []struct {
		name string
		run  func() error
	}{
		{name: "coverage_fingerprint", run: func() error { _, err := coverageFingerprint(root); return err }},
		{name: "self_augment_open", run: func() error { _, err := collectSelfAugmentOpenCount(root); return err }},
		{name: "self_verify_open", run: func() error { _, err := collectSelfVerifyOpenCount(root); return err }},
		{name: "quality_candidates", run: func() error { _ = collectQualityCandidates(root); return nil }},
		{name: "branch_functions", run: func() error { _, _ = collectBranchFunctions(root); return nil }},
		{name: "code_snr", run: func() error { _, err := computeCodeSNR(root); return err }},
	}
	for _, item := range benchmarks {
		benchmark.Run(item.name, func(benchmark *testing.B) {
			for range benchmark.N {
				if err := item.run(); err != nil {
					benchmark.Fatal(err)
				}
			}
		})
	}
}
