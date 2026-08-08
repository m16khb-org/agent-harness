package benchmark

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// A3 — offline으로 기록한 확률적 system-under-test의 신뢰도를 보고한다
// (pass@1, tau-bench pass^k, Clopper-Pearson interval). 이는 deterministic
// scorer와 엄격히 분리한다. scorer의 안정성은 분산 지표가 아닌
// scorer-determinism 검사로 다룬다(ScoreSpread와 issueops_determinism_test.go 참조).
//
// 이 package는 SUT를 실행하지 않는다. CI에서 live-skill을 호출하는 것은 범위 밖이며,
// 다른 곳에서 기록한 결과만 집계한다. 하나의 deterministic artifact를 재채점한
// 결과로 confidence interval을 계산하면 완전히 상관되어 가짜 자유도와 무의미한
// interval이 된다. 그래서 ComputeReliability는 각 run에 서로 다른 run_id와
// 생성 방식을 나타내는 비어 있지 않은 provenance label을 요구한다.

// maxReliabilityTrials는 float64 binomial tail의 정확성과 p^j·(1-p)^(n-j)의
// underflow 방지를 위해 n을 제한한다. 실제 offline reliability run은 k≈3..8이며,
// 이 상한을 넘기면 "exact binomial tail" 보장이 조용히 약해진다. 저하된 수치를
// 반환하지 않도록 호출자를 거부한다.
const maxReliabilityTrials = 50

// ComputeReliability는 offline 기록 run을 ReliabilityReport로 집계한다.
// provenance guard와 run 간 fixture 집합 정렬을 검증하며, Inf/NaN을 만들기보다
// 잘못된 입력을 거부한다.
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

	// 정렬된 fixture 집합: 각 run은 정확히 같은 fixture를 포함해야 fixture별 시행
	// 수가 run 수와 같고 macro-average가 잘 정의된다.
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

// passPowK는 tau-bench의 fixture별 신뢰도 kernel이다. 기록된 `trials`회 중
// `successes`회가 통과했을 때, 비복원으로 k회를 뽑아 모두 통과할 확률
// C(successes,k)/C(trials,k)를 ∏_{i=0}^{k-1} (successes-i)/(trials-i)로 계산한다.
//
// 먼저 domain을 검증한다. 그렇지 않으면 곱셈식이 0으로 나눌 수 있고 Go float64의
// x/0은 panic 대신 +Inf/NaN을 만든다. Inf/NaN PassPowK는 확률이 아니며 JSON과
// threshold 비교를 오염시키므로, 조용히 저하하지 않고 잘못된 입력을 거부한다.
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
	// 0 factor가 음수 factor보다 먼저 나타난다는 가정에 기대지 않는다. 성공이 k개
	// 미만이면 k개의 성공을 뽑을 수 없으므로 명시적으로 일찍 반환한다.
	if k > successes {
		return 0, nil
	}
	r := 1.0
	for i := range k {
		r *= float64(successes-i) / float64(trials-i)
	}
	return r, nil
}

// clopperPearson은 n회 중 c회 성공의 정확한 (1-alpha) binomial confidence
// interval을 반환한다. beta function 대신 binomial tail을 이분 탐색한다. c=0과
// c=n 끝점은 반드시 하드코딩한다. P(X>=0)==1과 P(X<=n)==1은 항등식이므로
// 퇴화한 방정식을 이분 탐색하면 수렴하지 않는다.
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
		// P(X>=c | p)는 p에 따라 증가하므로 alpha/2가 되는 p를 찾는다.
		lo = bisectProb(func(p float64) float64 { return binomTailGE(n, c, p) - alpha/2 })
	}
	if c == n {
		hi = 1
	} else {
		// P(X<=c | p)는 p에 따라 감소하므로 alpha/2 - P(X<=c)는 증가한다.
		hi = bisectProb(func(p float64) float64 { return alpha/2 - binomTailLE(n, c, p) })
	}
	return lo, hi, nil
}

// bisectProb는 [0,1]에서 단조 증가하는 f의 근을 찾는다.
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

// binomTailGE는 X ~ Binomial(n,p)일 때 P(X >= c | n, p)를 반환한다.
func binomTailGE(n, c int, p float64) float64 {
	sum := 0.0
	for j := c; j <= n; j++ {
		sum += binomCoeff(n, j) * math.Pow(p, float64(j)) * math.Pow(1-p, float64(n-j))
	}
	return sum
}

// binomTailLE는 X ~ Binomial(n,p)일 때 P(X <= c | n, p)를 반환한다.
func binomTailLE(n, c int, p float64) float64 {
	sum := 0.0
	for j := 0; j <= c; j++ {
		sum += binomCoeff(n, j) * math.Pow(p, float64(j)) * math.Pow(1-p, float64(n-j))
	}
	return sum
}

// binomCoeff는 이 package가 지원하는 작은 n에서 C(n,k)를 정확히 반환한다.
// n<=maxReliabilityTrials이므로 float64의 정확한 정수 범위 안이다.
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

// ScoreSpread는 점수의 최솟값, 최댓값, 폭(max-min)을 반환한다. 폭이 0이면 모든
// 값이 같다. scorer-determinism 검사를 뒷받침하며, 확률적 SUT의 신뢰도/분산
// 지표로 쓰지 않는다.
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
