package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"agent-harness/internal/core"
)

const selfVerificationCandidateExportKind = "self_verification_candidate_export"

type SelfVerificationCandidateExportResult struct {
	OK                    bool                        `json:"ok"`
	Kind                  string                      `json:"kind"`
	LoopKind              string                      `json:"loop_kind"`
	KoreanName            string                      `json:"korean_name"`
	HarnessRoot           string                      `json:"harness_root"`
	GeneratedAt           string                      `json:"generated_at"`
	SourcePath            string                      `json:"source_path"`
	SourceExists          bool                        `json:"source_exists"`
	CandidateCount        int                         `json:"candidate_count"`
	OpenCandidateIDs      []string                    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                    `json:"satisfied_candidate_ids"`
	SelectedCandidate     *SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []SelfVerificationCandidate `json:"candidates"`
	StateCheckpoint       *SelfAugmentStateCheckpoint `json:"state_checkpoint,omitempty"`
	Warnings              []string                    `json:"warnings"`
}

type SelfVerificationCandidate struct {
	Priority             int      `json:"priority"`
	ID                   string   `json:"id"`
	Category             string   `json:"category"`
	Status               string   `json:"status"`
	Score                float64  `json:"score"`
	WhyNow               []string `json:"why_now"`
	VerifyWith           []string `json:"verify_with"`
	SatisfactionEvidence []string `json:"satisfaction_evidence,omitempty"`
}

type SelfVerificationCandidateExportStateSnapshot struct {
	SchemaVersion         int                         `json:"schema_version"`
	Kind                  string                      `json:"kind"`
	LoopKind              string                      `json:"loop_kind"`
	KoreanName            string                      `json:"korean_name"`
	OK                    bool                        `json:"ok"`
	HarnessRoot           string                      `json:"harness_root"`
	GeneratedAt           string                      `json:"generated_at"`
	SourcePath            string                      `json:"source_path"`
	CandidateCount        int                         `json:"candidate_count"`
	OpenCandidateIDs      []string                    `json:"open_candidate_ids"`
	SatisfiedCandidateIDs []string                    `json:"satisfied_candidate_ids"`
	SelectedCandidate     *SelfVerificationCandidate  `json:"selected_candidate,omitempty"`
	Candidates            []SelfVerificationCandidate `json:"candidates"`
}

func runSelfVerifyCandidates(args []string) error {
	fs := flag.NewFlagSet("self-verify candidates", flag.ContinueOnError)
	saveState := fs.Bool("save-state", false, "save candidate export snapshot to harness state")
	stateKey := fs.String("state-key", "self-verify-candidates-latest", "state key for --save-state")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result := exportSelfVerificationCandidates()
	if *saveState {
		if err := saveSelfVerificationCandidateExport(&result, *stateKey); err != nil {
			return err
		}
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("%s candidates: %d candidate(s), selected=%s\n", result.KoreanName, result.CandidateCount, selectedSelfVerificationCandidateID(result.SelectedCandidate))
	for _, candidate := range result.Candidates {
		fmt.Printf("- %s %s score=%.1f status=%s\n", candidate.ID, candidate.Category, candidate.Score, candidate.Status)
	}
	return nil
}

func exportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	root := harnessRoot()
	sourcePath := filepath.Join(root, "agent_docs", "SELF_VERIFICATION_CANDIDATES.md")
	sourceExists := exists(sourcePath)
	candidates := selfVerificationCandidateCatalog()
	openIDs := selfVerificationCandidateIDsByStatus(candidates, selfAugmentCandidateStatusOpen)
	satisfiedIDs := selfVerificationCandidateIDsByStatus(candidates, selfAugmentCandidateStatusSatisfied)
	var selected *SelfVerificationCandidate
	for _, candidate := range candidates {
		if candidate.Status != selfAugmentCandidateStatusOpen {
			continue
		}
		if selected == nil || candidate.Score > selected.Score || (candidate.Score == selected.Score && candidate.Priority < selected.Priority) {
			copyCandidate := candidate
			selected = &copyCandidate
		}
	}
	warnings := []string{}
	if !sourceExists {
		warnings = append(warnings, "agent_docs/SELF_VERIFICATION_CANDIDATES.md not found; using built-in candidate export catalog")
	}
	return SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            selfVerificationKoreanName,
		HarnessRoot:           root,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		SourcePath:            sourcePath,
		SourceExists:          sourceExists,
		CandidateCount:        len(candidates),
		OpenCandidateIDs:      openIDs,
		SatisfiedCandidateIDs: satisfiedIDs,
		SelectedCandidate:     selected,
		Candidates:            candidates,
		Warnings:              warnings,
	}
}

func saveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	if key == "" {
		key = "self-verify-candidates-latest"
	}
	snapshot := SelfVerificationCandidateExportStateSnapshot{
		SchemaVersion:         1,
		Kind:                  selfVerificationCandidateExportKind,
		LoopKind:              result.LoopKind,
		KoreanName:            result.KoreanName,
		OK:                    result.OK,
		HarnessRoot:           result.HarnessRoot,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		SourcePath:            result.SourcePath,
		CandidateCount:        result.CandidateCount,
		OpenCandidateIDs:      result.OpenCandidateIDs,
		SatisfiedCandidateIDs: result.SatisfiedCandidateIDs,
		SelectedCandidate:     result.SelectedCandidate,
		Candidates:            result.Candidates,
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, Error: err.Error()}
		return err
	}
	state, err := core.StateWrite(key, string(b))
	if err != nil {
		result.StateCheckpoint = &SelfAugmentStateCheckpoint{OK: false, Key: key, StateDir: core.StateDir(), Error: err.Error()}
		return err
	}
	result.StateCheckpoint = &SelfAugmentStateCheckpoint{
		OK:       true,
		Key:      state.Record.Key,
		StateDir: state.StateDir,
		Path:     state.Path,
		Bytes:    state.Record.Bytes,
	}
	return nil
}

func selfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return []SelfVerificationCandidate{
		selfVerificationCandidate(1, "self-verify-progress-heartbeat", "observability", 81, selfAugmentCandidateStatusSatisfied, []string{"self-verify can look hung in redirected output without progress events"}, []string{"self-verify --progress=jsonl", "progress reporter test"}, []string{"--progress=jsonl emits loop and step JSONL events without breaking stdout JSON"}),
		selfVerificationCandidate(2, "self-verify-secret-redaction-audit", "security", 79, selfAugmentCandidateStatusSatisfied, []string{"verification artifacts must not become a secret leak path"}, []string{"redaction audit step", "secret-like fixture scan"}, []string{"redaction audit scans docs, skill metadata, golden responses, and synthetic command output"}),
		selfVerificationCandidate(3, "self-verify-coverage-gap-report", "coverage", 78, selfAugmentCandidateStatusSatisfied, []string{"self-verify claims need machine-readable evidence mapping"}, []string{"summary.coverage", "summary.coverage_gaps"}, []string{"summary includes coverage matrix and fails termination when claims are missing evidence labels"}),
		selfVerificationCandidate(4, "self-verify-llm-wiki-fixture-guard", "MCP/state", 78, selfAugmentCandidateStatusSatisfied, []string{"MCP smoke must prove it uses temp wiki fixtures, not durable user memory"}, []string{"llm-wiki fixture guard", "MCP smoke root leak assertion"}, []string{"llm-wiki fixture guard covers temp LLM_WIKI_ROOT and isolated HOME default-root negative checks"}),
		selfVerificationCandidate(5, "self-verify-failure-rerun-recipe", "reproducibility", 78, selfAugmentCandidateStatusSatisfied, []string{"failed steps need copy-paste rerun commands"}, []string{"summary.rerun_commands", "failure fixture"}, []string{"failure summaries include rerun_commands keyed by failed label"}),
		selfVerificationCandidate(6, "self-verify-candidate-export", "curriculum", 77, selfAugmentCandidateStatusSatisfied, []string{"self-augment candidates can all be satisfied, so self-verify needs its own next-candidate export"}, []string{"self-verify candidates --json", "state save/read", "response contract golden"}, []string{"self-verify candidates exports open/satisfied candidate IDs and can save a state snapshot"}),
		selfVerificationCandidate(7, "self-verify-step-budget-baseline", "performance", 76, selfAugmentCandidateStatusOpen, []string{"slowest_steps top 5 does not catch gradual per-label budget drift"}, []string{"baseline compare fixture", "label budget regression"}, nil),
		selfVerificationCandidate(8, "self-verify-install-dry-run-smoke", "native integration", 76, selfAugmentCandidateStatusOpen, []string{"install-native --dry-run should be an independent no-write evidence label"}, []string{"temp HOME smoke", "dry-run no-write assertion"}, nil),
		selfVerificationCandidate(9, "self-verify-policy-path-fuzz-plus", "policy/security", 76, selfAugmentCandidateStatusSatisfied, []string{"path policy needs seeded edge cases beyond simple outside paths"}, []string{"preflight fuzz", "policy path fixtures"}, []string{"policy fuzz covers symlink escape, ~/path, remote URL/ref exceptions, and outside workspace assertions"}),
		selfVerificationCandidate(10, "self-verify-json-schema-contract", "contract", 76, selfAugmentCandidateStatusSatisfied, []string{"summary schema drift should be visible without hand-reading golden diffs"}, []string{"summary.contract", "response contract golden"}, []string{"summary.contract includes version, hash, required fields, goals, and coverage claims"}),
		selfVerificationCandidate(11, "self-verify-flake-classifier", "reliability", 75, selfAugmentCandidateStatusSatisfied, []string{"intermittent seed failures need deterministic vs flaky classification"}, []string{"failure_class", "failure_clusters"}, []string{"summary classifies failure patterns and clusters failed seeds by step"}),
		selfVerificationCandidate(12, "self-verify-output-size-budget", "operability", 73, selfAugmentCandidateStatusSatisfied, []string{"large command output can bloat JSON and state snapshots"}, []string{"bounded stdout/stderr", "truncation metadata"}, []string{"StepResult output is budgeted with byte counts and truncation flags"}),
		selfVerificationCandidate(13, "self-verify-history-retention-budget", "state operations", 71, selfAugmentCandidateStatusSatisfied, []string{"history checkpoints need retention planning before state gets slow"}, []string{"history --retention-limit", "prune dry-run/confirm"}, []string{"history retention computes dry-run candidates and requires --confirm to delete"}),
		selfVerificationCandidate(14, "self-verify-parallel-temp-isolation", "concurrency", 70, selfAugmentCandidateStatusSatisfied, []string{"parallel self-verify runs must not collide in temp state, daemon, wiki, or artifacts"}, []string{"parallel isolation step", "race tier"}, []string{"parallel isolation probes unique temp roots for state, daemon, llm-wiki, and artifact paths"}),
		selfVerificationCandidate(15, "self-verify-duplicate-mcp-warning", "native integration", 70, selfAugmentCandidateStatusSatisfied, []string{"host MCP duplicate-scope warnings can pass smoke tests while hurting UX"}, []string{"Claude MCP warning fixture", "native integration step"}, []string{"native integration classifies conflicting user/project MCP scope warnings from fixtures"}),
		selfVerificationCandidate(16, "self-verify-daemon-restart-resilience", "daemon", 68, selfAugmentCandidateStatusSatisfied, []string{"daemon-backed MCP proxy must recover from stale lock/socket state"}, []string{"daemon resilience step", "stale lock recovery test"}, []string{"daemon resilience verifies stale lock/socket recovery, start/status/stop, and socket permissions"}),
	}
}

func selfVerificationCandidate(priority int, id, category string, score float64, status string, whyNow []string, verifyWith []string, evidence []string) SelfVerificationCandidate {
	return SelfVerificationCandidate{
		Priority:             priority,
		ID:                   id,
		Category:             category,
		Status:               status,
		Score:                score,
		WhyNow:               whyNow,
		VerifyWith:           verifyWith,
		SatisfactionEvidence: evidence,
	}
}

func selfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	ids := []string{}
	for _, candidate := range candidates {
		if candidate.Status == status {
			ids = append(ids, candidate.ID)
		}
	}
	return ids
}

func selectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	if candidate == nil {
		return "none"
	}
	return candidate.ID
}
