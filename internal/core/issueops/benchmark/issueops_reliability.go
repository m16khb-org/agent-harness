package benchmark

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// A3 — reliability reporting for a STOCHASTIC system-under-test over offline-
// recorded runs (pass@1, tau-bench pass^k, Clopper-Pearson intervals), kept
// strictly separate from the DETERMINISTIC scorer (whose stability is the
// scorer-determinism check, not a variance metric — see ScoreSpread and
// issueops_determinism_test.go).
//
// This package never EXECUTES the SUT — live-skill invocation in CI is a
// non-goal. It only AGGREGATES outcomes recorded elsewhere. Because a
// confidence interval over re-scorings of ONE deterministic artifact would be
// perfectly correlated (fake degrees of freedom, a meaningless interval),
// ComputeReliability enforces a provenance guard: every run must carry a
// DISTINCT run_id and a non-empty provenance label naming how it was produced.

// maxReliabilityTrials caps n so the float64 binomial tail stays exact and
// p^j·(1-p)^(n-j) does not underflow. Offline reliability runs are k≈3..8 in
// practice; beyond this cap the "exact binomial tail" claim would silently
// degrade, so callers are rejected rather than handed a degraded number.
const maxReliabilityTrials = 50

// RecordedRun is one offline-recorded execution of the SUT: its per-fixture
// pass/fail outcomes. RunID and Provenance are REQUIRED and RunID must be
// distinct across runs (the machine-checkable independence guard).
type RecordedRun struct {
	RunID      string          `json:"run_id"`
	Provenance string          `json:"provenance"`
	Outcomes   map[string]bool `json:"outcomes"` // fixtureID -> passed
}

// RecordedOutcomes is the offline input format for `issueops benchmark
// reliability`: k whole-benchmark replays with aligned fixture sets.
type RecordedOutcomes struct {
	Runs []RecordedRun `json:"runs"`
}

// FixtureReliability is the per-fixture breakdown. Clopper-Pearson intervals
// are PER FIXTURE on purpose: pooling heterogeneous fixtures into one (c,n)
// would assume a single shared Bernoulli p and report a misleadingly narrow
// interval.
type FixtureReliability struct {
	FixtureID     string  `json:"fixture_id"`
	Trials        int     `json:"trials"`
	Successes     int     `json:"successes"`
	PassAt1       float64 `json:"pass_at_1"`
	IntervalLow   float64 `json:"interval_low"`
	IntervalHigh  float64 `json:"interval_high"`
	IntervalWidth float64 `json:"interval_width"`
}

// PassPowKPoint is one point of the suite-level pass^k reliability curve.
type PassPowKPoint struct {
	K        int     `json:"k"`
	PassPowK float64 `json:"pass_pow_k"`
}

// ReliabilityReport summarizes SUT reliability over k offline-recorded runs.
// pass^k follows the tau-bench definition: the suite curve is the MEAN over
// fixtures of C(c_i,k)/C(n_i,k), and MacroPassAt1 is the per-fixture-averaged
// (macro) success rate so a high-trial fixture does not dominate. This is NOT
// for the deterministic scorer gate.
type ReliabilityReport struct {
	Runs          int                  `json:"runs"`
	Alpha         float64              `json:"alpha"`
	MacroPassAt1  float64              `json:"macro_pass_at_1"`
	MaxK          int                  `json:"max_k"` // = min_i trials_i
	PassPowKCurve []PassPowKPoint      `json:"pass_pow_k_curve"`
	Fixtures      []FixtureReliability `json:"fixtures"`
	Provenance    []string             `json:"provenance"`
}

// ComputeReliability aggregates offline-recorded runs into a ReliabilityReport.
// It validates the provenance guard, requires aligned fixture sets across runs,
// and rejects malformed inputs rather than emitting Inf/NaN numbers.
func ComputeReliability(rec RecordedOutcomes, alpha float64) (ReliabilityReport, error) {
	runs := rec.Runs
	if len(runs) < 2 {
		return ReliabilityReport{}, fmt.Errorf("reliability: need at least 2 recorded runs (acceptance uses k>=3); a single run has no reliability to estimate")
	}
	if alpha <= 0 || alpha >= 1 {
		return ReliabilityReport{}, fmt.Errorf("reliability: alpha must be in (0,1), got %v", alpha)
	}

	seenID := make(map[string]bool, len(runs))
	provSeen := make(map[string]bool, len(runs))
	var provList []string
	for i, run := range runs {
		id := strings.TrimSpace(run.RunID)
		if id == "" {
			return ReliabilityReport{}, fmt.Errorf("reliability: run %d has an empty run_id", i)
		}
		if seenID[id] {
			return ReliabilityReport{}, fmt.Errorf("reliability: duplicate run_id %q — runs must be distinct executions, not re-scorings of one artifact", id)
		}
		seenID[id] = true
		prov := strings.TrimSpace(run.Provenance)
		if prov == "" {
			return ReliabilityReport{}, fmt.Errorf("reliability: run %q has empty provenance (name how the run was generated)", id)
		}
		if !provSeen[prov] {
			provSeen[prov] = true
			provList = append(provList, prov)
		}
		if len(run.Outcomes) == 0 {
			return ReliabilityReport{}, fmt.Errorf("reliability: run %q has no outcomes", id)
		}
	}

	// Aligned fixture set: every run must cover exactly the same fixtures so
	// each fixture's trial count equals the run count and the macro-average is
	// well defined.
	canonical := make([]string, 0, len(runs[0].Outcomes))
	for f := range runs[0].Outcomes {
		canonical = append(canonical, f)
	}
	sort.Strings(canonical)
	canonSet := make(map[string]bool, len(canonical))
	for _, f := range canonical {
		canonSet[f] = true
	}
	for _, run := range runs {
		if len(run.Outcomes) != len(canonical) {
			return ReliabilityReport{}, fmt.Errorf("reliability: run %q covers %d fixtures, expected %d (runs must be whole-benchmark replays with aligned fixtures)", run.RunID, len(run.Outcomes), len(canonical))
		}
		for f := range run.Outcomes {
			if !canonSet[f] {
				return ReliabilityReport{}, fmt.Errorf("reliability: run %q has unexpected fixture %q (fixture sets must match across runs)", run.RunID, f)
			}
		}
	}

	fixtures := make([]FixtureReliability, 0, len(canonical))
	for _, f := range canonical {
		c, n := 0, 0
		for _, run := range runs {
			n++
			if run.Outcomes[f] {
				c++
			}
		}
		lo, hi, err := clopperPearson(c, n, alpha)
		if err != nil {
			return ReliabilityReport{}, err
		}
		fixtures = append(fixtures, FixtureReliability{
			FixtureID:     f,
			Trials:        n,
			Successes:     c,
			PassAt1:       round4(float64(c) / float64(n)),
			IntervalLow:   round4(lo),
			IntervalHigh:  round4(hi),
			IntervalWidth: round4(hi - lo),
		})
	}

	maxK := len(runs) // aligned ⇒ every trials_i == len(runs)
	macro := 0.0
	for _, fr := range fixtures {
		macro += float64(fr.Successes) / float64(fr.Trials)
	}
	macro /= float64(len(fixtures))

	curve := make([]PassPowKPoint, 0, maxK)
	for k := 1; k <= maxK; k++ {
		sum := 0.0
		for _, fr := range fixtures {
			v, err := passPowK(fr.Successes, fr.Trials, k)
			if err != nil {
				return ReliabilityReport{}, err
			}
			sum += v
		}
		curve = append(curve, PassPowKPoint{K: k, PassPowK: round4(sum / float64(len(fixtures)))})
	}

	return ReliabilityReport{
		Runs:          len(runs),
		Alpha:         alpha,
		MacroPassAt1:  round4(macro),
		MaxK:          maxK,
		PassPowKCurve: curve,
		Fixtures:      fixtures,
		Provenance:    provList,
	}, nil
}

// passPowK is the tau-bench per-fixture reliability kernel: the probability
// that k trials drawn WITHOUT replacement from `trials` recorded trials (of
// which `successes` passed) all pass, i.e. C(successes,k)/C(trials,k), computed
// as the product ∏_{i=0}^{k-1} (successes-i)/(trials-i).
//
// The domain is validated UP FRONT — the product form would otherwise divide
// by zero, and Go float64 x/0 yields +Inf/NaN (never a panic). A PassPowK of
// Inf/NaN is not a probability and would poison JSON and any threshold
// comparison, so invalid inputs are rejected rather than silently degraded.
func passPowK(successes, trials, k int) (float64, error) {
	if trials <= 0 || trials > maxReliabilityTrials {
		return 0, fmt.Errorf("reliability: trials must be in [1,%d], got %d", maxReliabilityTrials, trials)
	}
	if successes < 0 || successes > trials {
		return 0, fmt.Errorf("reliability: successes must be in [0,%d], got %d", trials, successes)
	}
	if k < 0 || k > trials {
		return 0, fmt.Errorf("reliability: k must be in [0,%d], got %d", trials, k)
	}
	if k == 0 {
		return 1, nil
	}
	// Explicit early return (not a reliance on a zero factor appearing before a
	// negative one): you cannot draw k successes when fewer than k exist.
	if k > successes {
		return 0, nil
	}
	r := 1.0
	for i := range k {
		r *= float64(successes-i) / float64(trials-i)
	}
	return r, nil
}

// clopperPearson returns the exact (1-alpha) binomial confidence interval for c
// successes in n trials, computed by bisection on the binomial tail (no beta
// function). The c=0 and c=n endpoints MUST be hardcoded: P(X>=0)==1 and
// P(X<=n)==1 identically, so bisection on those degenerate equations would not
// converge.
func clopperPearson(c, n int, alpha float64) (lo, hi float64, err error) {
	if n <= 0 || n > maxReliabilityTrials {
		return 0, 0, fmt.Errorf("reliability: trials must be in [1,%d], got %d", maxReliabilityTrials, n)
	}
	if c < 0 || c > n {
		return 0, 0, fmt.Errorf("reliability: successes must be in [0,%d], got %d", n, c)
	}
	if alpha <= 0 || alpha >= 1 {
		return 0, 0, fmt.Errorf("reliability: alpha must be in (0,1), got %v", alpha)
	}
	if c == 0 {
		lo = 0
	} else {
		// P(X>=c | p) is increasing in p; find p where it equals alpha/2.
		lo = bisectProb(func(p float64) float64 { return binomTailGE(n, c, p) - alpha/2 })
	}
	if c == n {
		hi = 1
	} else {
		// P(X<=c | p) is decreasing in p, so alpha/2 - P(X<=c) is increasing.
		hi = bisectProb(func(p float64) float64 { return alpha/2 - binomTailLE(n, c, p) })
	}
	return lo, hi, nil
}

// bisectProb finds the root of a monotone-increasing f on [0,1].
func bisectProb(f func(float64) float64) float64 {
	lo, hi := 0.0, 1.0
	for range 200 {
		mid := (lo + hi) / 2
		if f(mid) < 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// binomTailGE returns P(X >= c | n, p) for X ~ Binomial(n,p).
func binomTailGE(n, c int, p float64) float64 {
	sum := 0.0
	for j := c; j <= n; j++ {
		sum += binomCoeff(n, j) * math.Pow(p, float64(j)) * math.Pow(1-p, float64(n-j))
	}
	return sum
}

// binomTailLE returns P(X <= c | n, p) for X ~ Binomial(n,p).
func binomTailLE(n, c int, p float64) float64 {
	sum := 0.0
	for j := 0; j <= c; j++ {
		sum += binomCoeff(n, j) * math.Pow(p, float64(j)) * math.Pow(1-p, float64(n-j))
	}
	return sum
}

// binomCoeff returns C(n,k) exactly for the small n this package supports
// (n<=maxReliabilityTrials, well within float64's exact-integer range).
func binomCoeff(n, k int) float64 {
	if k < 0 || k > n {
		return 0
	}
	if k > n-k {
		k = n - k
	}
	r := 1.0
	for i := range k {
		r = r * float64(n-i) / float64(i+1)
	}
	return r
}

// ScoreSpread returns the min, max, and width (max-min) of scores. A width of 0
// means every value was identical. It backs the scorer-determinism check; it is
// deliberately NOT a reliability/variance metric for a stochastic SUT.
func ScoreSpread(scores []float64) (lo, hi, width float64) {
	if len(scores) == 0 {
		return 0, 0, 0
	}
	lo, hi = scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < lo {
			lo = s
		}
		if s > hi {
			hi = s
		}
	}
	return lo, hi, hi - lo
}

func round4(f float64) float64 {
	return math.Round(f*1e4) / 1e4
}
