package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IssueOpsBenchmarkFixture struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	UserPrompt        string   `json:"user_prompt"`
	RepoContext       string   `json:"repo_context"`
	ExpectedIssue     []string `json:"expected_issue"`
	ExpectedPlan      []string `json:"expected_plan"`
	ExpectedTasks     []string `json:"expected_tasks"`
	ExpectedTDD       []string `json:"expected_tdd"`
	ExpectedSubagents []string `json:"expected_subagents"`
	ExpectedPR        []string `json:"expected_pr"`
	CriticalFailures  []string `json:"critical_failures"`
}

type IssueOpsBenchmarkArtifact struct {
	ProblemSummary         string `json:"problem_summary,omitempty"`
	IssueDraft             string `json:"issue_draft,omitempty"`
	Plan                   string `json:"plan,omitempty"`
	TaskBreakdown          string `json:"task_breakdown,omitempty"`
	TDDPlan                string `json:"tdd_plan,omitempty"`
	SubagentPrompts        string `json:"subagent_prompts,omitempty"`
	ImplementationNotes    string `json:"implementation_notes,omitempty"`
	PRDraft                string `json:"pr_draft,omitempty"`
	PhaseChoices           string `json:"phase_choices,omitempty"`
	BranchName             string `json:"branch_name,omitempty"`
	WorktreePath           string `json:"worktree_path,omitempty"`
	ImplementationLocation string `json:"implementation_location,omitempty"`
	WorktreeCleanup        string `json:"worktree_cleanup,omitempty"`
	GuidelineRef           string `json:"guideline_ref,omitempty"`
}

type IssueOpsDimensionScore struct {
	Dimension string  `json:"dimension"`
	Score     float64 `json:"score"`
	Evidence  string  `json:"evidence"`
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

var issueOpsBenchmarkDimensions = []string{
	"intent_understanding",
	"issue_quality",
	"plan_quality",
	"task_decomposition",
	"tdd_quality",
	"subagent_orchestration",
	"implementation_readiness",
	"pr_mr_quality",
	"phase_control_quality",
	"branch_worktree_gate_quality",
	"isolation_compliance",
	"worktree_cleanup_quality",
}

func LoadIssueOpsBenchmarkFixtures(dir string) ([]IssueOpsBenchmarkFixture, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("fixtures path is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fixtures []IssueOpsBenchmarkFixture
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fixture IssueOpsBenchmarkFixture
		if err := json.Unmarshal(b, &fixture); err != nil {
			return nil, fmt.Errorf("parse fixture %s: %w", path, err)
		}
		if err := validateIssueOpsBenchmarkFixture(fixture); err != nil {
			return nil, fmt.Errorf("invalid fixture %s: %w", path, err)
		}
		fixtures = append(fixtures, fixture)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no issueops benchmark fixtures in %s", dir)
	}
	return fixtures, nil
}

func validateIssueOpsBenchmarkFixture(f IssueOpsBenchmarkFixture) error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(f.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(f.UserPrompt) == "" {
		return fmt.Errorf("user_prompt is required")
	}
	if strings.TrimSpace(f.RepoContext) == "" {
		return fmt.Errorf("repo_context is required")
	}
	if len(f.CriticalFailures) == 0 {
		return fmt.Errorf("critical_failures is required")
	}
	return nil
}

func ScoreIssueOpsBenchmarkArtifact(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) IssueOpsBenchmarkScore {
	checks := map[string]struct {
		ok       bool
		evidence string
		failure  string
	}{
		"intent_understanding": {
			ok:       strings.TrimSpace(artifact.ProblemSummary) != "" || strings.TrimSpace(artifact.IssueDraft) != "",
			evidence: "problem summary or issue draft is present",
			failure:  "missing problem summary or issue draft",
		},
		"issue_quality": {
			ok:       containsAllFold(artifact.IssueDraft, "problem", "current evidence", "acceptance criteria", "non-goals", "verification", "feedback log") && containsHangul(artifact.IssueDraft) && hasIssueOpsGuidelineRef(artifact),
			evidence: "issue draft includes required sections, Korean text, and guideline reference",
			failure:  "issue draft missing required sections, Korean text, or guideline reference",
		},
		"plan_quality": {
			ok:       containsAnyFold(artifact.Plan, "go test", "verify", "verification", "test"),
			evidence: "plan includes verification commands or tests",
			failure:  "plan missing verification commands",
		},
		"task_decomposition": {
			ok:       containsAllFold(artifact.TaskBreakdown, "owns") && containsAnyFold(artifact.TaskBreakdown, "worker", "task"),
			evidence: "task breakdown assigns bounded ownership",
			failure:  "task breakdown missing bounded ownership",
		},
		"tdd_quality": {
			ok:       containsAllFold(artifact.TDDPlan, "failing", "test") && containsAnyFold(artifact.TDDPlan, "before", "first"),
			evidence: "TDD plan names failing tests before implementation",
			failure:  "TDD plan missing failing-test-first evidence",
		},
		"subagent_orchestration": {
			ok:       containsAllFold(artifact.SubagentPrompts, "not alone", "do not revert", "own") && containsAnyFold(artifact.SubagentPrompts, "expected output", "report"),
			evidence: "subagent prompts include coordination and ownership guidance",
			failure:  "subagent prompts missing coordination guidance",
		},
		"implementation_readiness": {
			ok:       strings.TrimSpace(artifact.BranchName) != "" && strings.TrimSpace(artifact.WorktreePath) != "",
			evidence: "branch and worktree are recorded",
			failure:  "branch or worktree evidence missing",
		},
		"pr_mr_quality": {
			ok:       containsAllFold(artifact.PRDraft, "intent", "changes", "verification", "risk", "reviewer notes") && containsAnyFold(artifact.PRDraft, "issue:", "fixes", "closes") && containsHangul(artifact.PRDraft) && hasIssueOpsGuidelineRef(artifact),
			evidence: "PR/MR draft includes required sections, issue link, Korean text, reviewer notes, and guideline reference",
			failure:  "PR/MR draft missing required sections, issue link, Korean text, reviewer notes, or guideline reference",
		},
		"phase_control_quality": {
			ok:       containsAllFold(artifact.PhaseChoices, "proceed", "revise", "jump", "pause"),
			evidence: "phase choices include proceed, revise, jump, and pause",
			failure:  "phase choices missing required options",
		},
		"branch_worktree_gate_quality": {
			ok:       strings.HasPrefix(artifact.BranchName, "feature/") && strings.Contains(artifact.WorktreePath, ".worktrees/"),
			evidence: "issue branch and sibling worktree path are recorded",
			failure:  "issue branch or sibling worktree gate missing",
		},
		"isolation_compliance": {
			ok:       implementationInWorktree(artifact),
			evidence: "implementation location is inside the isolated worktree",
			failure:  "implementation location is outside the isolated worktree",
		},
		"worktree_cleanup_quality": {
			ok:       containsAnyFold(artifact.WorktreeCleanup, "clean") && containsAnyFold(artifact.WorktreeCleanup, "cleanup", "remove") && containsAnyFold(artifact.WorktreeCleanup, "choice", "offered", "present"),
			evidence: "worktree cleanup status and choices are recorded",
			failure:  "worktree cleanup status or choices missing",
		},
	}

	score := IssueOpsBenchmarkScore{OK: true, FixtureID: fixture.ID}
	for _, dimension := range issueOpsBenchmarkDimensions {
		check := checks[dimension]
		dimensionScore := 0.0
		evidence := check.failure
		if check.ok {
			dimensionScore = 5
			evidence = check.evidence
		} else {
			score.DeterministicFailures = append(score.DeterministicFailures, check.failure)
		}
		score.DimensionScores = append(score.DimensionScores, IssueOpsDimensionScore{
			Dimension: dimension,
			Score:     dimensionScore,
			Evidence:  evidence,
		})
	}
	score.AverageScore, score.MinimumScore = summarizeIssueOpsDimensionScores(score.DimensionScores)
	score.CriticalFailures = append(score.CriticalFailures, detectIssueOpsCriticalFailures(fixture, artifact)...)
	score.Passed = len(score.CriticalFailures) == 0 && len(score.DeterministicFailures) == 0 && score.MinimumScore >= 5
	score.OK = score.Passed
	return score
}

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (IssueOpsBenchmarkRunResult, error) {
	result := IssueOpsBenchmarkRunResult{
		ID:           "issueops-benchmark-" + time.Now().UTC().Format("20060102T150405.000000000Z"),
		FixtureCount: len(req.Fixtures),
	}
	for _, fixture := range req.Fixtures {
		artifact := req.Artifacts[fixture.ID]
		score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
		result.Scores = append(result.Scores, score)
		result.CriticalFailureCount += len(score.CriticalFailures)
	}
	result.AverageScore, result.MinimumScore = summarizeIssueOpsRunScores(result.Scores)
	result.OK = result.CriticalFailureCount == 0
	for _, score := range result.Scores {
		if !score.Passed {
			result.OK = false
			break
		}
	}
	if strings.TrimSpace(req.StateRoot) != "" {
		if err := persistIssueOpsBenchmarkRun(req.StateRoot, result); err != nil {
			return IssueOpsBenchmarkRunResult{}, err
		}
	}
	return result, nil
}

func ReadIssueOpsBenchmarkRun(stateRoot, id string) (IssueOpsBenchmarkRunResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return IssueOpsBenchmarkRunResult{}, fmt.Errorf("benchmark id is required")
	}
	b, err := os.ReadFile(filepath.Join(stateRoot, "issueops-benchmarks", id+".json"))
	if err != nil {
		return IssueOpsBenchmarkRunResult{}, err
	}
	var result IssueOpsBenchmarkRunResult
	if err := json.Unmarshal(b, &result); err != nil {
		return IssueOpsBenchmarkRunResult{}, err
	}
	return result, nil
}

func CompareIssueOpsBenchmarkRuns(baseline, candidate IssueOpsBenchmarkRunResult) IssueOpsBenchmarkCompareResult {
	result := IssueOpsBenchmarkCompareResult{
		OK:                   true,
		BaselineID:           baseline.ID,
		CandidateID:          candidate.ID,
		AverageScoreDelta:    candidate.AverageScore - baseline.AverageScore,
		MinimumScoreDelta:    candidate.MinimumScore - baseline.MinimumScore,
		CriticalFailureDelta: candidate.CriticalFailureCount - baseline.CriticalFailureCount,
		Regressions:          compareIssueOpsDimensionRegressions(baseline, candidate),
	}
	result.Improved = result.AverageScoreDelta > 0 &&
		result.MinimumScoreDelta >= 0 &&
		result.CriticalFailureDelta <= 0 &&
		len(result.Regressions) == 0
	result.OK = result.MinimumScoreDelta >= 0 &&
		result.CriticalFailureDelta <= 0 &&
		len(result.Regressions) == 0
	return result
}

func summarizeIssueOpsDimensionScores(scores []IssueOpsDimensionScore) (float64, float64) {
	if len(scores) == 0 {
		return 0, 0
	}
	total := 0.0
	minimum := scores[0].Score
	for _, score := range scores {
		total += score.Score
		if score.Score < minimum {
			minimum = score.Score
		}
	}
	return total / float64(len(scores)), minimum
}

func summarizeIssueOpsRunScores(scores []IssueOpsBenchmarkScore) (float64, float64) {
	if len(scores) == 0 {
		return 0, 0
	}
	total := 0.0
	minimum := scores[0].MinimumScore
	for _, score := range scores {
		total += score.AverageScore
		if score.MinimumScore < minimum {
			minimum = score.MinimumScore
		}
	}
	return total / float64(len(scores)), minimum
}

func persistIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	dir := filepath.Join(stateRoot, "issueops-benchmarks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, result.ID+".json"), b, 0o644)
}

func compareIssueOpsDimensionRegressions(baseline, candidate IssueOpsBenchmarkRunResult) []string {
	baselineScores := issueOpsDimensionMinimums(baseline)
	candidateScores := issueOpsDimensionMinimums(candidate)
	var regressions []string
	for _, dimension := range issueOpsBenchmarkDimensions {
		if candidateScores[dimension] < baselineScores[dimension] {
			regressions = append(regressions, dimension)
		}
	}
	return regressions
}

func issueOpsDimensionMinimums(run IssueOpsBenchmarkRunResult) map[string]float64 {
	minimums := make(map[string]float64)
	seen := make(map[string]bool)
	for _, score := range run.Scores {
		for _, dimensionScore := range score.DimensionScores {
			if !seen[dimensionScore.Dimension] || dimensionScore.Score < minimums[dimensionScore.Dimension] {
				minimums[dimensionScore.Dimension] = dimensionScore.Score
				seen[dimensionScore.Dimension] = true
			}
		}
	}
	return minimums
}

func detectIssueOpsCriticalFailures(fixture IssueOpsBenchmarkFixture, artifact IssueOpsBenchmarkArtifact) []string {
	var failures []string
	for _, rule := range fixture.CriticalFailures {
		ruleText := strings.ToLower(rule)
		switch {
		case strings.Contains(ruleText, "works in source repo") && !implementationInWorktree(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "skips branch prompt") && strings.TrimSpace(artifact.BranchName) == "":
			failures = append(failures, rule)
		case strings.Contains(ruleText, "removes dirty worktree") && containsAllFold(artifact.WorktreeCleanup, "dirty", "remove"):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "not written in korean") && (!containsHangul(artifact.IssueDraft) || !containsHangul(artifact.PRDraft)):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "missing issue/pr guideline reference") && !hasIssueOpsGuidelineRef(artifact):
			failures = append(failures, rule)
		case strings.Contains(ruleText, "excessive emoji") && hasExcessiveEmoji(artifact.IssueDraft+"\n"+artifact.PRDraft):
			failures = append(failures, rule)
		}
	}
	return failures
}

func implementationInWorktree(artifact IssueOpsBenchmarkArtifact) bool {
	worktreePath := strings.TrimSpace(artifact.WorktreePath)
	location := strings.TrimSpace(artifact.ImplementationLocation)
	return worktreePath != "" && location != "" && (location == worktreePath || strings.HasPrefix(location, worktreePath+string(os.PathSeparator)))
}

func containsAllFold(s string, needles ...string) bool {
	for _, needle := range needles {
		if !containsFold(s, needle) {
			return false
		}
	}
	return true
}

func containsAnyFold(s string, needles ...string) bool {
	for _, needle := range needles {
		if containsFold(s, needle) {
			return true
		}
	}
	return false
}

func containsFold(s, needle string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(needle))
}

func containsHangul(s string) bool {
	for _, r := range s {
		if (r >= '가' && r <= '힣') || (r >= 'ㄱ' && r <= 'ㅎ') || (r >= 'ㅏ' && r <= 'ㅣ') {
			return true
		}
	}
	return false
}

func hasIssueOpsGuidelineRef(artifact IssueOpsBenchmarkArtifact) bool {
	const guideline = "docs/superpowers/specs/issueops-issue-pr-guidelines.md"
	return containsFold(artifact.GuidelineRef, guideline) ||
		containsFold(artifact.IssueDraft, guideline) ||
		containsFold(artifact.PRDraft, guideline)
}

func hasExcessiveEmoji(s string) bool {
	count := 0
	for _, r := range s {
		if isEmojiRune(r) {
			count++
		}
	}
	return count > 3
}

func isEmojiRune(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF)
}
