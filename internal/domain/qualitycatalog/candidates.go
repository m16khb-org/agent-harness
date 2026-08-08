package qualitycatalog

import (
	"fmt"
	"strings"
)

const CandidateStatusOpen = "open"

var resolvedCandidateIDs = map[string]bool{
	"daemon-connection-limit":        true,
	"worker-stuck-running-detection": true,
	"state-write-locking":            true,
}

// VerificationKind classifies how a candidate's change is verified externally,
// making the tool-grounded vs documentary distinction EXPLICIT instead of
// guessing it from free-text VerifyWith strings (which cannot reliably tell a
// tool signal from a doc artifact from model self-critique).
type VerificationKind string

const (
	// ToolSignalKind: a code/correctness change whose VerifyWith MUST name an
	// executable external signal (test/build/lint/contract/smoke/coverage run or
	// a CLI command) — never model self-critique.
	ToolSignalKind VerificationKind = "tool_signal"
	// DocArtifactKind: a documentation/governance change whose verification is a
	// concrete produced artifact (ADR entry, README section, checklist, matrix,
	// transcript). Explicitly EXEMPT from the executable-signal rule and labeled
	// as such, so it is not falsely claimed to be tool-gated.
	DocArtifactKind VerificationKind = "doc_artifact"
)

// toolSignalMarkers are CONCRETE executable-verification tokens (runnable
// commands and test/contract artifact kinds). They deliberately EXCLUDE generic
// verbs (verify/validate/inspect/check/review) and generic nouns
// (notes/document/criteria/references/section) that self-critique prose uses as
// readily as a real tool signal: the burden of proof is on NAMING a concrete
// external mechanism, not on dodging a forbidden-phrase denylist.
var toolSignalMarkers = []string{
	"go test", "go build", "go vet", "agent-harness", "harness ", "skill ",
	"agent-harness install", "install tests", "npm ", "./",
	"_test", "test", "golden", "contract", "fixture", "smoke", "lint",
	"coverage", "-cover", "-race", "-count", "--json", "roundtrip",
	"benchmark", "schema", "self-verify", "self-augment", "quick_validate",
	"policy_check", "policy audit", "redaction audit", "qa gate",
	"internal/", "cmd/", "mcp",
}

// docArtifactMarkers are concrete documentary deliverables a DocArtifact
// candidate must name (a file/section/record), not bare self-critique.
var docArtifactMarkers = []string{
	"adr", "readme", "checklist", "matrix", "transcript", "decision entry",
	"decision record", "notes document", "dogfooding notes", ".md",
}

// VerifyWithGrounded enforces the self-correction guardrail (inherits v1 S5/S6):
// a candidate's VerifyWith must NAME at least one external verification
// mechanism appropriate to its kind, never model self-critique. This is catalog
// hygiene (the string names a mechanism); whether the mechanism exists and
// PASSES is the separate execution gate enforced by `agent-harness self-verify`
// / CI.
func VerifyWithGrounded(kind VerificationKind, verifyWith []string) error {
	if len(verifyWith) == 0 {
		return fmt.Errorf("verify_with is empty")
	}
	for _, entry := range verifyWith {
		if strings.TrimSpace(entry) == "" {
			return fmt.Errorf("verify_with has a blank entry")
		}
	}
	switch kind {
	case ToolSignalKind:
		if !verifyWithNamesAny(verifyWith, toolSignalMarkers) {
			return fmt.Errorf("tool_signal candidate must name an executable verification (go test / golden / contract / smoke / lint / self-verify / ...), not self-critique: %v", verifyWith)
		}
	case DocArtifactKind:
		if !verifyWithNamesAny(verifyWith, docArtifactMarkers) {
			return fmt.Errorf("doc_artifact candidate must name a concrete deliverable (ADR / README / checklist / matrix / transcript), not self-critique: %v", verifyWith)
		}
	default:
		return fmt.Errorf("unknown verification kind %q", kind)
	}
	return nil
}

func verifyWithNamesAny(verifyWith, markers []string) bool {
	for _, entry := range verifyWith {
		lower := strings.ToLower(entry)
		for _, marker := range markers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}

type CandidateSpec struct {
	ID               string
	Title            string
	Category         string
	VerificationKind VerificationKind
	Impact           float64
	Feasibility      float64
	Novelty          float64
	Risk             float64
	WhyNow           []string
	ExpectedGain     []string
	VerifyWith       []string
	Evidence         []string
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
	specs := []CandidateSpec{
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
			ID: "coverage-host-judgement", Title: "Cover host-agent judgement malformed output paths", Category: "coverage",
			Impact: 82, Feasibility: 78, Novelty: 56, Risk: 18,
			WhyNow:       []string{"IssueOps and quality gates depend on strict host-agent result file decoding"},
			ExpectedGain: []string{"malformed JSON and bounded error output paths stay deterministic"},
			VerifyWith:   []string{"go test ./internal/core/judgement -count=1", "go test -cover ./internal/core/judgement"},
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
			VerifyWith:   []string{"go test ./cmd/harness/daemoncli ./internal/adapter/worker -count=1"},
			Evidence:     []string{"PROJECT_AUDIT D1 P1"},
		},
		{
			ID: "worker-stuck-running-detection", Title: "Detect worker jobs stuck running after process crash", Category: "audit-risk",
			Impact: 88, Feasibility: 74, Novelty: 58, Risk: 24,
			WhyNow:       []string{".agent-harness/PROJECT_AUDIT.md flags W1 P1 stuck running jobs"},
			ExpectedGain: []string{"worker status can classify stale running records"},
			VerifyWith:   []string{"go test ./internal/adapter/worker ./cmd/harness/workercli -count=1"},
			Evidence:     []string{"PROJECT_AUDIT W1 P1"},
		},
		{
			ID: "state-write-locking", Title: "Add write locking around state file updates", Category: "audit-risk",
			Impact: 91, Feasibility: 76, Novelty: 56, Risk: 28,
			WhyNow:       []string{".agent-harness/PROJECT_AUDIT.md flags S1 P1 no write locking"},
			ExpectedGain: []string{"concurrent state writes stop risking lost updates"},
			VerifyWith:   []string{"go test ./internal/application/state ./internal/adapter/outbound/state ./internal/adapter/outbound/sqlstore -count=1", "go test -race ./internal/application/state ./internal/adapter/outbound/state ./internal/adapter/outbound/sqlstore -count=1"},
			Evidence:     []string{"PROJECT_AUDIT S1 P1"},
		},
	}
	// Quality specs are all code/correctness candidates; default them to
	// ToolSignal unless a future spec explicitly classifies itself otherwise.
	for i := range specs {
		if specs[i].VerificationKind == "" {
			specs[i].VerificationKind = ToolSignalKind
		}
	}
	return specs
}

func Candidates() []Candidate {
	specs := CandidateSpecs()
	out := make([]Candidate, 0, len(specs))
	for _, spec := range specs {
		if resolvedCandidateIDs[spec.ID] {
			continue
		}
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
