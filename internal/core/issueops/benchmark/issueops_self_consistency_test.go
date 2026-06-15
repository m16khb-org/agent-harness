package benchmark

import (
	"math"
	"testing"
)

// sample builds a JudgeSample with the distinct id + provenance the
// independence guard requires, so the happy-path tests stay terse.
func sample(id string, avg float64, passed bool) JudgeSample {
	return JudgeSample{
		SampleID:   id,
		Provenance: "run-" + id,
		Score:      IssueOpsBenchmarkScore{AverageScore: avg, Passed: passed},
	}
}

func TestConsensusJudgeVerdict_MajorityVote(t *testing.T) {
	// 2 pass / 1 fail -> majority passes, agreement 2/3.
	v, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 100, true),
		sample("b", 100, true),
		sample("c", 60, false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.MajorityPassed {
		t.Fatalf("expected MajorityPassed=true, got false")
	}
	if math.Abs(v.PassAgreement-0.6667) > 1e-4 {
		t.Fatalf("PassAgreement = %v, want ~0.6667", v.PassAgreement)
	}
	if v.Samples != 3 {
		t.Fatalf("Samples = %d, want 3", v.Samples)
	}
}

func TestConsensusJudgeVerdict_MinorityFailsClosed(t *testing.T) {
	// 1 pass / 2 fail -> majority fails, agreement is the FAIL fraction 2/3.
	v, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 100, true),
		sample("b", 40, false),
		sample("c", 60, false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.MajorityPassed {
		t.Fatalf("expected MajorityPassed=false")
	}
	if math.Abs(v.PassAgreement-0.6667) > 1e-4 {
		t.Fatalf("PassAgreement = %v, want ~0.6667 (fail fraction)", v.PassAgreement)
	}
}

func TestConsensusJudgeVerdict_TieFailsClosed(t *testing.T) {
	// Even split: strict majority is NOT reached, so fail-closed and
	// agreement is exactly 0.5 — the gate cannot accept on a coin-flip.
	v, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 100, true),
		sample("b", 100, true),
		sample("c", 50, false),
		sample("d", 50, false),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.MajorityPassed {
		t.Fatalf("tie must fail closed, got MajorityPassed=true")
	}
	if v.PassAgreement != 0.5 {
		t.Fatalf("PassAgreement = %v, want 0.5 on a tie", v.PassAgreement)
	}
}

func TestConsensusJudgeVerdict_MedianAndSpread(t *testing.T) {
	// Scores 60, 80, 100 -> median 80, spread 40, min 60, max 100.
	v, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 60, false),
		sample("b", 80, false),
		sample("c", 100, true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.MedianAverageScore != 80 {
		t.Fatalf("MedianAverageScore = %v, want 80", v.MedianAverageScore)
	}
	if v.ScoreMin != 60 || v.ScoreMax != 100 || v.ScoreSpread != 40 {
		t.Fatalf("min/max/spread = %v/%v/%v, want 60/100/40", v.ScoreMin, v.ScoreMax, v.ScoreSpread)
	}
	// Median is robust to the bimodal scorer: with an even count it averages
	// the two middle values, never inventing an unreachable mean.
	v2, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 0, false),
		sample("b", 100, true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v2.MedianAverageScore != 50 {
		t.Fatalf("even-count median = %v, want 50", v2.MedianAverageScore)
	}
}

func TestConsensusJudgeVerdict_EmpiricalVariance(t *testing.T) {
	// Population variance of {0,100} = ((0-50)^2 + (100-50)^2)/2 = 2500.
	v, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 0, false),
		sample("b", 100, true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.SampleVariance != 2500 {
		t.Fatalf("SampleVariance = %v, want 2500", v.SampleVariance)
	}
	// Degenerate (all-equal) samples have ZERO variance — the honest signal
	// that there is nothing for self-consistency to reduce.
	vZero, err := ConsensusJudgeVerdict([]JudgeSample{
		sample("a", 100, true),
		sample("b", 100, true),
		sample("c", 100, true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vZero.SampleVariance != 0 {
		t.Fatalf("all-equal variance = %v, want 0", vZero.SampleVariance)
	}
	if vZero.Caveat == "" {
		t.Fatalf("caveat must always be populated")
	}
}

func TestConsensusJudgeVerdict_IndependenceGuard(t *testing.T) {
	tests := []struct {
		name    string
		samples []JudgeSample
	}{
		{
			name:    "fewer than two samples",
			samples: []JudgeSample{sample("a", 100, true)},
		},
		{
			name: "duplicate sample id is one judge dressed as N voters",
			samples: []JudgeSample{
				sample("dup", 100, true),
				sample("dup", 100, true),
			},
		},
		{
			name: "empty sample id",
			samples: []JudgeSample{
				{SampleID: "", Provenance: "p1", Score: IssueOpsBenchmarkScore{AverageScore: 100, Passed: true}},
				sample("b", 100, true),
			},
		},
		{
			name: "empty provenance",
			samples: []JudgeSample{
				{SampleID: "a", Provenance: "  ", Score: IssueOpsBenchmarkScore{AverageScore: 100, Passed: true}},
				sample("b", 100, true),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ConsensusJudgeVerdict(tc.samples); err == nil {
				t.Fatalf("expected error for %q, got nil", tc.name)
			}
		})
	}
}

func TestConsensusJudgeVerdict_ProvenanceDeduped(t *testing.T) {
	// Distinct samples may legitimately share a provenance label only when it
	// is the same source bucket; the reported provenance list is de-duplicated
	// but the samples themselves remain distinct votes.
	v, err := ConsensusJudgeVerdict([]JudgeSample{
		{SampleID: "a", Provenance: "offline-batch-1", Score: IssueOpsBenchmarkScore{AverageScore: 100, Passed: true}},
		{SampleID: "b", Provenance: "offline-batch-1", Score: IssueOpsBenchmarkScore{AverageScore: 100, Passed: true}},
		{SampleID: "c", Provenance: "offline-batch-2", Score: IssueOpsBenchmarkScore{AverageScore: 100, Passed: true}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v.Provenance) != 2 {
		t.Fatalf("provenance = %v, want 2 distinct labels", v.Provenance)
	}
}
