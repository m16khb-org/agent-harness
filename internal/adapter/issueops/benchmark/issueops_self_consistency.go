package benchmark

import (
	"fmt"
	"sort"
	"strings"
)

// B6 — self-consistency (Wang 2022) over N OFFLINE-recorded judge verdicts of the
// SAME artifact. This file never invokes a judge; running judgement N times in
// CI would violate the deterministic-eval non-goal. It only aggregates samples
// recorded elsewhere.
//
// HONEST SCOPE: self-consistency reduces VERDICT VARIANCE across same-model
// samples; it does NOT reduce or detect the judge's systematic BIAS — a consensus
// of a biased judge is still biased. The spread/variance figures are DESCRIPTIVE
// statistics over the supplied samples; they are valid only if the samples are
// genuinely independent draws (distinct sample ids + non-empty provenance are
// enforced below; provenance MAY be shared across distinct samples from the same
// offline source bucket) and only meaningful where verdicts actually vary (on a
// clean 100/100 run the merged verdict is bimodally pinned and its variance is
// degenerate — see A7).

const consensusCaveat = "descriptive over the supplied samples; reduces verdict VARIANCE not BIAS (a consensus of a biased judge is still biased); valid only if samples are independent (distinct sample ids + non-empty provenance enforced) and meaningful only where verdicts actually vary"

// ConsensusJudgeVerdict aggregates N independent offline judge samples of the same
// artifact into a self-consistency consensus. It fails closed on samples that lack
// distinct ids / provenance (fake degrees of freedom) or on fewer than 2 samples.
func ConsensusJudgeVerdict(samples []JudgeSample) (ConsensusVerdict, error) {
	if len(samples) < 2 {
		return ConsensusVerdict{}, fmt.Errorf("self-consistency: need at least 2 judge samples (acceptance uses N>=3); a single sample has no consensus and no variance to reduce")
	}
	seenID := make(map[string]bool, len(samples))
	provSeen := make(map[string]bool, len(samples))
	var provenance []string
	avgScores := make([]float64, 0, len(samples))
	passes := 0
	for i, sample := range samples {
		id := strings.TrimSpace(sample.SampleID)
		if id == "" {
			return ConsensusVerdict{}, fmt.Errorf("self-consistency: sample %d has empty sample_id", i)
		}
		if seenID[id] {
			return ConsensusVerdict{}, fmt.Errorf("self-consistency: duplicate sample_id %q — votes must be distinct judge runs, not one judge dressed as N voters", id)
		}
		seenID[id] = true
		prov := strings.TrimSpace(sample.Provenance)
		if prov == "" {
			return ConsensusVerdict{}, fmt.Errorf("self-consistency: sample %q has empty provenance", id)
		}
		if !provSeen[prov] {
			provSeen[prov] = true
			provenance = append(provenance, prov)
		}
		avgScores = append(avgScores, sample.Score.AverageScore)
		if sample.Score.Passed {
			passes++
		}
	}

	majorityPassed := passes*2 > len(samples) // strict majority; tie -> false (fail-closed)
	agree := passes
	if !majorityPassed {
		agree = len(samples) - passes
	}
	lo, hi, spread := ScoreSpread(avgScores)
	return ConsensusVerdict{
		Samples:            len(samples),
		MajorityPassed:     majorityPassed,
		PassAgreement:      round4(float64(agree) / float64(len(samples))),
		MedianAverageScore: round4(medianFloat(avgScores)),
		ScoreMin:           round4(lo),
		ScoreMax:           round4(hi),
		ScoreSpread:        round4(spread),
		SampleVariance:     round4(sampleVariance(avgScores)),
		Provenance:         provenance,
		Caveat:             consensusCaveat,
	}, nil
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// sampleVariance is the DESCRIPTIVE (population) variance of the supplied scores —
// the observed spread of single-run verdicts, NOT a theoretical sigma^2/N
// reduction (the samples are not i.i.d.).
func sampleVariance(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return sum / float64(len(values))
}
