// 벤치마크 실행의 요청·결과·집계 DTO다. 값을 만들어내는 쪽은 I/O를 하지만
// 읽고 전달하는 쪽은 그 구현을 알 필요가 없다.
package issueops

type IssueOpsDimensionScore struct {
	Dimension     string  `json:"dimension"`
	Score         float64 `json:"score"`
	Evidence      string  `json:"evidence"`
	NotApplicable bool    `json:"not_applicable,omitempty"`
}
type IssueOpsBenchmarkScore struct {
	OK                    bool                     `json:"ok"`
	FixtureID             string                   `json:"fixture_id"`
	AverageScore          float64                  `json:"average_score"`
	MinimumScore          float64                  `json:"minimum_score"`
	DimensionScores       []IssueOpsDimensionScore `json:"dimension_scores"`
	DeterministicFailures []string                 `json:"deterministic_failures"`
	JudgeFailures         []string                 `json:"judge_failures"`
	CriticalFailures      []string                 `json:"critical_failures"`
	Passed                bool                     `json:"passed"`
}
type IssueOpsBenchmarkRunRequest struct {
	StateRoot string
	Fixtures  []IssueOpsBenchmarkFixture
	Artifacts map[string]IssueOpsBenchmarkArtifact
}
type IssueOpsBenchmarkRunResult struct {
	OK                   bool                     `json:"ok"`
	ID                   string                   `json:"id"`
	FixtureCount         int                      `json:"fixture_count"`
	AverageScore         float64                  `json:"average_score"`
	MinimumScore         float64                  `json:"minimum_score"`
	CriticalFailureCount int                      `json:"critical_failure_count"`
	Scores               []IssueOpsBenchmarkScore `json:"scores"`
}
type IssueOpsBenchmarkCompareResult struct {
	OK                   bool     `json:"ok"`
	Improved             bool     `json:"improved"`
	BaselineID           string   `json:"baseline_id"`
	CandidateID          string   `json:"candidate_id"`
	AverageScoreDelta    float64  `json:"average_score_delta"`
	MinimumScoreDelta    float64  `json:"minimum_score_delta"`
	CriticalFailureDelta int      `json:"critical_failure_delta"`
	Regressions          []string `json:"regressions"`
}
type IssueOpsAutoresearchCandidate struct {
	ID               string   `json:"id"`
	Hypothesis       string   `json:"hypothesis"`
	TargetDimensions []string `json:"target_dimensions"`
	EditSurface      []string `json:"edit_surface"`
	BaselineCommand  string   `json:"baseline_command,omitempty"`
	CandidateCommand string   `json:"candidate_command,omitempty"`
	KeepCriteria     string   `json:"keep_criteria,omitempty"`
	DiscardCriteria  string   `json:"discard_criteria,omitempty"`
}
type IssueOpsAutoresearchGateRequest struct {
	Candidate    IssueOpsAutoresearchCandidate
	BaselineRun  IssueOpsBenchmarkRunResult
	CandidateRun IssueOpsBenchmarkRunResult
	ChangedPaths []string
}
type IssueOpsAutoresearchGateResult struct {
	OK                         bool                           `json:"ok"`
	KeepCandidate              bool                           `json:"keep_candidate"`
	CandidateID                string                         `json:"candidate_id"`
	BenchmarkCompare           IssueOpsBenchmarkCompareResult `json:"benchmark_compare"`
	EditSurfaceViolations      []string                       `json:"edit_surface_violations,omitempty"`
	TargetDimensionRegressions []string                       `json:"target_dimension_regressions,omitempty"`
	DiscardReasons             []string                       `json:"discard_reasons,omitempty"`
}
type IssueOpsLLMJudgeRequest struct {
	Fixture  IssueOpsBenchmarkFixture
	Artifact IssueOpsBenchmarkArtifact
}

// A7 — judge-map provenance. The --judge file backend merges externally-produced
// judge scores into a fresh deterministic run. IssueOpsJudgeMap wraps those
// scores with the provenance of the run whose artifacts the judge evaluated, so
// a judge map cannot be silently self-attributed to the very run it scores.
//
// HONEST SCOPE (mirrors the RecordedRun provenance guard in
// issueops_reliability.go): ValidateJudgeProvenance is a SELF-REFERENCE GUARD,
// not a proof of judge independence. It rejects a judge map whose source run is
// the scored run itself and requires the named source run to actually exist on
// disk — raising the bar from "type any string" to "name a real prior run". It
// does NOT establish that an independent judge evaluated different artifacts;
// the dashboard must not claim "independent judge" on the strength of this gate.
type IssueOpsJudgeMap struct {
	SourceRunID string                            `json:"source_run_id"`
	Provenance  string                            `json:"provenance"`
	Scores      map[string]IssueOpsBenchmarkScore `json:"scores"`
}

// RecordedRun은 SUT를 offline으로 한 번 실행한 fixture별 pass/fail 결과다.
// RunID와 Provenance는 필수이고 RunID는 run마다 달라야 한다. 이는 기계적으로
// 확인하는 독립성 guard다.
type RecordedRun struct {
	RunID      string          `json:"run_id"`
	Provenance string          `json:"provenance"`
	Outcomes   map[string]bool `json:"outcomes"` // fixtureID -> passed
}

// RecordedOutcomes는 `issueops benchmark reliability`의 offline 입력 형식이다.
// fixture 집합이 정렬된 benchmark 전체 재실행 k회를 담는다.
type RecordedOutcomes struct {
	Runs []RecordedRun `json:"runs"`
}

// FixtureReliability은 fixture별 분석이다. Clopper-Pearson interval을 의도적으로
// fixture별로 계산한다. 이질적인 fixture를 하나의 (c,n)으로 합치면 단일 Bernoulli
// p를 가정하게 되어 지나치게 좁은 interval을 보고하게 된다.
type FixtureReliability struct {
	FixtureID     string  `json:"fixture_id"`
	Trials        int     `json:"trials"`
	Successes     int     `json:"successes"`
	PassAt1       float64 `json:"pass_at_1"`
	IntervalLow   float64 `json:"interval_low"`
	IntervalHigh  float64 `json:"interval_high"`
	IntervalWidth float64 `json:"interval_width"`
}

// PassPowKPoint는 suite 수준 pass^k 신뢰도 곡선의 한 점이다.
type PassPowKPoint struct {
	K        int     `json:"k"`
	PassPowK float64 `json:"pass_pow_k"`
}

// ReliabilityReport는 k개의 offline 기록 run에서 SUT 신뢰도를 요약한다.
// pass^k는 tau-bench 정의를 따른다. suite 곡선은 fixture별 C(c_i,k)/C(n_i,k)의
// 평균이며, MacroPassAt1은 시행 수가 많은 fixture가 지배하지 않도록 fixture별
// 성공률을 평균한 값이다. deterministic scorer gate에는 쓰지 않는다.
type ReliabilityReport struct {
	Runs          int                  `json:"runs"`
	Alpha         float64              `json:"alpha"`
	MacroPassAt1  float64              `json:"macro_pass_at_1"`
	MaxK          int                  `json:"max_k"` // = min_i trials_i
	PassPowKCurve []PassPowKPoint      `json:"pass_pow_k_curve"`
	Fixtures      []FixtureReliability `json:"fixtures"`
	Provenance    []string             `json:"provenance"`
}

// RoutingFidelityResult reports whether an observed routing trace covered every
// expected skill-at-phase pairing, listing the pairings that were not observed.
type RoutingFidelityResult struct {
	OK      bool           `json:"ok"`
	Missing []SkillRouting `json:"missing,omitempty"`
}

// JudgeSample is one offline-recorded judge verdict of the same artifact. SampleID
// is REQUIRED and must be DISTINCT across samples, and Provenance must be
// non-empty: this is the machine-checkable guard (ported from the RecordedRun /
// judge-provenance guards) against re-using one judge sample as N fake voters.
type JudgeSample struct {
	SampleID   string                 `json:"sample_id"`
	Provenance string                 `json:"provenance"`
	Score      IssueOpsBenchmarkScore `json:"score"`
}

// ConsensusVerdict is the self-consistency aggregation of N judge samples.
type ConsensusVerdict struct {
	Samples int `json:"samples"`
	// MajorityPassed is the majority vote over each sample's BINARIZED pass/fail
	// (the gate's own boolean) — the faithful "majority vote". Ties resolve to
	// false (fail-closed) and surface as PassAgreement 0.5.
	MajorityPassed bool    `json:"majority_passed"`
	PassAgreement  float64 `json:"pass_agreement"`
	// MedianAverageScore is the robust consensus point of the N average scores.
	// Median, NOT mean: the deterministic scorer is bimodal (0/100), so a mean
	// would land on a value no sample produced and the gate can never accept.
	MedianAverageScore float64  `json:"median_average_score"`
	ScoreMin           float64  `json:"score_min"`
	ScoreMax           float64  `json:"score_max"`
	ScoreSpread        float64  `json:"score_spread"`
	SampleVariance     float64  `json:"sample_variance"`
	Provenance         []string `json:"provenance"`
	Caveat             string   `json:"caveat"`
}
