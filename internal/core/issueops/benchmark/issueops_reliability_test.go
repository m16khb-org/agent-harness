package benchmark

import (
	"math"
	"testing"
)

func approxEq(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// Clopper-Pearson bounds are pinned to values independently reproduced from R's
// binom.test (and a reference beta-quantile computation): the bisection-on-
// binomial-tail method must match exact published intervals.
func TestClopperPearsonMatchesPublishedValues(t *testing.T) {
	cases := []struct {
		c, n           int
		wantLo, wantHi float64
	}{
		{8, 10, 0.4439, 0.9748},
		{10, 10, 0.6915, 1.0},
		{0, 10, 0.0, 0.3085},
		{5, 8, 0.2449, 0.9148},
		{3, 3, 0.2924, 1.0},
		{2, 3, 0.0943, 0.9916},
	}
	for _, tc := range cases {
		lo, hi, err := clopperPearson(tc.c, tc.n, 0.05)
		if err != nil {
			t.Fatalf("clopperPearson(%d,%d) error: %v", tc.c, tc.n, err)
		}
		if !approxEq(lo, tc.wantLo, 5e-4) || !approxEq(hi, tc.wantHi, 5e-4) {
			t.Fatalf("clopperPearson(%d,%d)=[%.4f,%.4f], want [%.4f,%.4f]", tc.c, tc.n, lo, hi, tc.wantLo, tc.wantHi)
		}
	}
}

func TestClopperPearsonRejectsBadDomain(t *testing.T) {
	for _, tc := range []struct {
		c, n  int
		alpha float64
	}{
		{1, 0, 0.05},                          // n<=0
		{5, 4, 0.05},                          // c>n
		{-1, 4, 0.05},                         // c<0
		{1, 4, 0},                             // alpha<=0
		{1, 4, 1},                             // alpha>=1
		{1, maxReliabilityTrials + 1, 0.05},   // n over cap
	} {
		if _, _, err := clopperPearson(tc.c, tc.n, tc.alpha); err == nil {
			t.Fatalf("clopperPearson(%d,%d,%v) must error on bad domain", tc.c, tc.n, tc.alpha)
		}
	}
}

func TestPassPowKKernel(t *testing.T) {
	cases := []struct {
		c, n, k int
		want    float64
	}{
		{8, 10, 0, 1.0},       // k=0 -> 1
		{8, 10, 1, 0.8},       // = c/n
		{8, 10, 3, 0.46667},   // = C(8,3)/C(10,3)
		{8, 10, 8, 0.02222},   // = C(8,8)/C(10,8)
		{8, 10, 9, 0.0},       // k>successes -> 0
		{0, 5, 1, 0.0},        // no successes
		{3, 3, 3, 1.0},        // all pass, all drawn
	}
	for _, tc := range cases {
		got, err := passPowK(tc.c, tc.n, tc.k)
		if err != nil {
			t.Fatalf("passPowK(%d,%d,%d) error: %v", tc.c, tc.n, tc.k, err)
		}
		if !approxEq(got, tc.want, 1e-4) {
			t.Fatalf("passPowK(%d,%d,%d)=%.5f, want %.5f", tc.c, tc.n, tc.k, got, tc.want)
		}
	}
}

// The product form would divide by zero for k>trials or trials=0 and yield
// +Inf/NaN (Go float division never panics); the domain must be rejected.
func TestPassPowKRejectsInvalidDomainInsteadOfNaN(t *testing.T) {
	for _, tc := range []struct{ c, n, k int }{
		{8, 10, 11}, // k>trials
		{5, 0, 1},   // trials=0
		{0, 0, 1},   // trials=0 (would be 0/0=NaN)
		{11, 10, 1}, // successes>trials
		{8, 10, -1}, // k<0
		{-1, 10, 1}, // successes<0
	} {
		got, err := passPowK(tc.c, tc.n, tc.k)
		if err == nil {
			t.Fatalf("passPowK(%d,%d,%d) must error, got %v", tc.c, tc.n, tc.k, got)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Fatalf("passPowK(%d,%d,%d) leaked NaN/Inf: %v", tc.c, tc.n, tc.k, got)
		}
	}
}

func fixtureRel(report ReliabilityReport, id string) (FixtureReliability, bool) {
	for _, f := range report.Fixtures {
		if f.FixtureID == id {
			return f, true
		}
	}
	return FixtureReliability{}, false
}

func curveAt(report ReliabilityReport, k int) (float64, bool) {
	for _, p := range report.PassPowKCurve {
		if p.K == k {
			return p.PassPowK, true
		}
	}
	return 0, false
}

func TestComputeReliabilityMacroAggregatesPerFixture(t *testing.T) {
	rec := RecordedOutcomes{Runs: []RecordedRun{
		{RunID: "r1", Provenance: "recorded-holdout", Outcomes: map[string]bool{"A": true, "B": true}},
		{RunID: "r2", Provenance: "recorded-holdout", Outcomes: map[string]bool{"A": true, "B": false}},
		{RunID: "r3", Provenance: "recorded-holdout", Outcomes: map[string]bool{"A": true, "B": true}},
	}}
	report, err := ComputeReliability(rec, 0.05)
	if err != nil {
		t.Fatalf("ComputeReliability error: %v", err)
	}
	if report.Runs != 3 || report.MaxK != 3 {
		t.Fatalf("runs/maxK = %d/%d, want 3/3", report.Runs, report.MaxK)
	}
	// macro pass@1 = mean(3/3, 2/3) = 0.8333
	if !approxEq(report.MacroPassAt1, 0.8333, 5e-4) {
		t.Fatalf("macro pass@1 = %.4f, want 0.8333", report.MacroPassAt1)
	}
	// pass^k curve must be the per-fixture average, NOT a pooled (5,6) bag.
	wantCurve := map[int]float64{1: 0.8333, 2: 0.6667, 3: 0.5}
	for k, want := range wantCurve {
		got, ok := curveAt(report, k)
		if !ok || !approxEq(got, want, 5e-4) {
			t.Fatalf("pass^%d = %.4f (ok=%v), want %.4f", k, got, ok, want)
		}
	}
	// per-fixture Clopper-Pearson (NOT pooled across fixtures).
	a, _ := fixtureRel(report, "A")
	if a.Successes != 3 || a.Trials != 3 || !approxEq(a.IntervalLow, 0.2924, 5e-4) || !approxEq(a.IntervalHigh, 1.0, 5e-4) {
		t.Fatalf("fixture A reliability wrong: %+v", a)
	}
	b, _ := fixtureRel(report, "B")
	if b.Successes != 2 || b.Trials != 3 || !approxEq(b.IntervalLow, 0.0943, 5e-4) || !approxEq(b.IntervalHigh, 0.9916, 5e-4) {
		t.Fatalf("fixture B reliability wrong: %+v", b)
	}
}

// Pooling would compute C(5,3)/C(6,3) = 10/20 = 0.5 for k=3; the macro form is
// mean(C(3,3)/C(3,3), C(2,3)/C(3,3)) = mean(1, 0) = 0.5 here by coincidence at
// k=3, but they diverge at k=2: pooled C(5,2)/C(6,2)=10/15=0.667 equals macro
// 0.667 too... use an asymmetric case where pooled != macro to lock the unit.
func TestComputeReliabilityIsNotPooled(t *testing.T) {
	// A=2/2 (perfect), B=2/4 -> macro and pooled diverge.
	rec := RecordedOutcomes{Runs: []RecordedRun{
		{RunID: "r1", Provenance: "p", Outcomes: map[string]bool{"A": true, "B": true}},
		{RunID: "r2", Provenance: "p", Outcomes: map[string]bool{"A": true, "B": true}},
		{RunID: "r3", Provenance: "p", Outcomes: map[string]bool{"A": true, "B": false}},
		{RunID: "r4", Provenance: "p", Outcomes: map[string]bool{"A": true, "B": false}},
	}}
	report, err := ComputeReliability(rec, 0.05)
	if err != nil {
		t.Fatalf("ComputeReliability error: %v", err)
	}
	// macro pass@1 = mean(4/4, 2/4) = mean(1.0, 0.5) = 0.75.
	// pooled would be 6/8 = 0.75 (coincides) -> use pass^2 to separate:
	// macro pass^2 = mean(C(4,2)/C(4,2), C(2,2)/C(4,2)) = mean(1, 6/... )
	//   passPowK(4,4,2)=1; passPowK(2,4,2)=(2/4)(1/3)=0.16667 -> mean=0.58333.
	// pooled pass^2 = C(6,2)/C(8,2) = 15/28 = 0.5357 -> DIFFERENT.
	got, ok := curveAt(report, 2)
	if !ok || !approxEq(got, 0.5833, 5e-4) {
		t.Fatalf("macro pass^2 = %.4f (ok=%v), want 0.5833 (pooled would be 0.5357)", got, ok)
	}
}

func TestComputeReliabilityEnforcesProvenanceGuard(t *testing.T) {
	base := func() []RecordedRun {
		return []RecordedRun{
			{RunID: "r1", Provenance: "p", Outcomes: map[string]bool{"A": true}},
			{RunID: "r2", Provenance: "p", Outcomes: map[string]bool{"A": false}},
		}
	}
	// duplicate run_id (re-scoring of one artifact dressed as two runs)
	dup := base()
	dup[1].RunID = "r1"
	if _, err := ComputeReliability(RecordedOutcomes{Runs: dup}, 0.05); err == nil {
		t.Fatal("duplicate run_id must be rejected")
	}
	// empty provenance
	noProv := base()
	noProv[0].Provenance = "  "
	if _, err := ComputeReliability(RecordedOutcomes{Runs: noProv}, 0.05); err == nil {
		t.Fatal("empty provenance must be rejected")
	}
	// empty run_id
	noID := base()
	noID[0].RunID = ""
	if _, err := ComputeReliability(RecordedOutcomes{Runs: noID}, 0.05); err == nil {
		t.Fatal("empty run_id must be rejected")
	}
	// fewer than 2 runs
	if _, err := ComputeReliability(RecordedOutcomes{Runs: base()[:1]}, 0.05); err == nil {
		t.Fatal("single run must be rejected")
	}
	// misaligned fixture sets
	misaligned := base()
	misaligned[1].Outcomes = map[string]bool{"A": false, "B": true}
	if _, err := ComputeReliability(RecordedOutcomes{Runs: misaligned}, 0.05); err == nil {
		t.Fatal("misaligned fixture sets must be rejected")
	}
	// bad alpha
	if _, err := ComputeReliability(RecordedOutcomes{Runs: base()}, 0); err == nil {
		t.Fatal("alpha=0 must be rejected")
	}
}

func TestScoreSpread(t *testing.T) {
	if _, _, w := ScoreSpread([]float64{100, 100, 100}); w != 0 {
		t.Fatalf("identical scores must have width 0, got %v", w)
	}
	if lo, hi, w := ScoreSpread([]float64{100, 50, 75}); lo != 50 || hi != 100 || w != 50 {
		t.Fatalf("spread = [%v,%v] w=%v, want [50,100] w=50", lo, hi, w)
	}
	if _, _, w := ScoreSpread(nil); w != 0 {
		t.Fatalf("empty spread width must be 0, got %v", w)
	}
}
