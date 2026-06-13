package qualitycatalog

const CandidateStatusOpen = "open"

type CandidateSpec struct {
	ID           string
	Title        string
	Category     string
	Impact       float64
	Feasibility  float64
	Novelty      float64
	Risk         float64
	WhyNow       []string
	ExpectedGain []string
	VerifyWith   []string
	Evidence     []string
}

type Candidate struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Status      string   `json:"status"`
	Score       float64  `json:"score"`
	Impact      float64  `json:"impact"`
	Feasibility float64  `json:"feasibility"`
	Risk        float64  `json:"risk"`
	VerifyWith  []string `json:"verify_with"`
	Evidence    []string `json:"evidence"`
}

func CandidateSpecs() []CandidateSpec {
	return []CandidateSpec{
		{
			ID: "quality-signal-harvester", Title: "Add quality inspect signal harvesting CLI", Category: "quality",
			Impact: 94, Feasibility: 92, Novelty: 78, Risk: 14,
			WhyNow:       []string{"테스트 통과 여부보다 다음 결함 후보를 좁히는 측정 표면이 필요하다"},
			ExpectedGain: []string{"coverage, branch complexity, audit risk를 한 JSON 계약으로 수집", "self-augment 후보 refill의 입력을 재현 가능하게 만든다"},
			VerifyWith:   []string{"go test ./cmd/harness/qualitycli -count=1", "agent-harness quality inspect --json"},
			Evidence:     []string{"quality inspect CLI", "coverage/complexity/audit signal output"},
		},
		{
			ID: "self-augment-signal-table", Title: "Table-drive self-augment repository signal collection", Category: "refactor",
			Impact: 88, Feasibility: 90, Novelty: 64, Risk: 12,
			WhyNow:       []string{"CollectSelfAugmentRepoSignals branch count is high for a read-only detector"},
			ExpectedGain: []string{"signal additions become table rows", "branch count drops below the planned threshold"},
			VerifyWith:   []string{"go test ./cmd/harness/selfworkflow/augmentcatalog -count=1"},
			Evidence:     []string{"CollectSelfAugmentRepoSignals table-driven refactor"},
		},
		{
			ID: "coverage-commandguard", Title: "Raise commandguard coverage for boundary and denial paths", Category: "coverage",
			Impact: 82, Feasibility: 84, Novelty: 52, Risk: 14,
			WhyNow:       []string{"command policy surfaces are safety-critical and low-coverage packages should be first candidates"},
			ExpectedGain: []string{"workspace boundary regressions are caught earlier"},
			VerifyWith:   []string{"go test ./internal/core/commandguard -count=1", "go test -cover ./internal/core/commandguard"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "coverage-mcp-resources", Title: "Cover MCP resource catalog edge cases", Category: "coverage",
			Impact: 80, Feasibility: 84, Novelty: 50, Risk: 12,
			WhyNow:       []string{"MCP resource drift affects both Codex and Claude hosts"},
			ExpectedGain: []string{"resource schema and lookup regressions fail in package tests"},
			VerifyWith:   []string{"go test ./cmd/harness/mcpcli/resources -count=1", "go test -cover ./cmd/harness/mcpcli/resources"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "coverage-externalllm", Title: "Cover external LLM malformed output and timeout paths", Category: "coverage",
			Impact: 82, Feasibility: 78, Novelty: 56, Risk: 18,
			WhyNow:       []string{"IssueOps and quality gates depend on fail-closed external LLM wrappers"},
			ExpectedGain: []string{"malformed JSON, timeout, and command failure paths stay deterministic"},
			VerifyWith:   []string{"go test ./internal/core/externalllm -count=1", "go test -cover ./internal/core/externalllm"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "coverage-issueops-linking", Title: "Cover IssueOps link issue and link plan boundaries", Category: "coverage",
			Impact: 84, Feasibility: 82, Novelty: 54, Risk: 16,
			WhyNow:       []string{"IssueOps durable state gates are easy to regress with boundary paths"},
			ExpectedGain: []string{"invalid URLs, missing files, and path boundaries are pinned"},
			VerifyWith:   []string{"go test ./internal/core/issueops/linking -count=1", "go test -cover ./internal/core/issueops/linking"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "daemon-connection-limit", Title: "Add daemon connection limit protection", Category: "audit-risk",
			Impact: 90, Feasibility: 72, Novelty: 58, Risk: 26,
			WhyNow:       []string{".agent-harness/PROJECT_AUDIT.md flags D1 P1 no connection limit"},
			ExpectedGain: []string{"daemon resource exhaustion has an explicit guard and test"},
			VerifyWith:   []string{"go test ./cmd/harness/daemoncli ./internal/core/worker -count=1"},
			Evidence:     []string{"PROJECT_AUDIT D1 P1"},
		},
		{
			ID: "worker-stuck-running-detection", Title: "Detect worker jobs stuck running after process crash", Category: "audit-risk",
			Impact: 88, Feasibility: 74, Novelty: 58, Risk: 24,
			WhyNow:       []string{".agent-harness/PROJECT_AUDIT.md flags W1 P1 stuck running jobs"},
			ExpectedGain: []string{"worker status can classify stale running records"},
			VerifyWith:   []string{"go test ./internal/core/worker ./cmd/harness/workercli -count=1"},
			Evidence:     []string{"PROJECT_AUDIT W1 P1"},
		},
		{
			ID: "state-write-locking", Title: "Add write locking around state file updates", Category: "audit-risk",
			Impact: 91, Feasibility: 76, Novelty: 56, Risk: 28,
			WhyNow:       []string{".agent-harness/PROJECT_AUDIT.md flags S1 P1 no write locking"},
			ExpectedGain: []string{"concurrent state writes stop risking lost updates"},
			VerifyWith:   []string{"go test ./internal/core/state -count=1", "go test -race ./internal/core/state -count=1"},
			Evidence:     []string{"PROJECT_AUDIT S1 P1"},
		},
		{
			ID: "draftwiki-stale-lock", Title: "Detect and recover stale draft-wiki locks", Category: "audit-risk",
			Impact: 86, Feasibility: 78, Novelty: 54, Risk: 22,
			WhyNow:       []string{".agent-harness/PROJECT_AUDIT.md flags Q1 P1 stale lock detection"},
			ExpectedGain: []string{"draft-wiki queue processing does not wedge on abandoned locks"},
			VerifyWith:   []string{"go test ./internal/core/draftwiki/... ./cmd/harness/draftwikicli -count=1"},
			Evidence:     []string{"PROJECT_AUDIT Q1 P1"},
		},
	}
}

func Candidates() []Candidate {
	specs := CandidateSpecs()
	out := make([]Candidate, 0, len(specs))
	for _, spec := range specs {
		out = append(out, Candidate{
			ID:          spec.ID,
			Title:       spec.Title,
			Category:    spec.Category,
			Status:      CandidateStatusOpen,
			Score:       Score(spec.Impact, spec.Feasibility, spec.Novelty, spec.Risk),
			Impact:      spec.Impact,
			Feasibility: spec.Feasibility,
			Risk:        spec.Risk,
			VerifyWith:  append([]string{}, spec.VerifyWith...),
			Evidence:    append([]string{}, spec.Evidence...),
		})
	}
	return out
}

func Score(impact, feasibility, novelty, risk float64) float64 {
	score := impact*0.38 + feasibility*0.30 + novelty*0.20 + (100-risk)*0.12
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
