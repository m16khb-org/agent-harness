package candidateexport

func SelfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return []SelfVerificationCandidate{
		selfVerificationCandidate(1, "self-verify-progress-heartbeat", "observability", 81, selfAugmentCandidateStatusSatisfied, []string{"self-verify can look hung in redirected output without progress events"}, []string{"self-verify --progress=jsonl", "progress reporter test"}, []string{"--progress=jsonl emits loop and step JSONL events without breaking stdout JSON"}),
		selfVerificationCandidate(2, "self-verify-secret-redaction-audit", "security", 79, selfAugmentCandidateStatusSatisfied, []string{"verification artifacts must not become a secret leak path"}, []string{"redaction audit step", "secret-like fixture scan"}, []string{"redaction audit scans docs, skill metadata, golden responses, and synthetic command output"}),
		selfVerificationCandidate(3, "self-verify-coverage-gap-report", "coverage", 78, selfAugmentCandidateStatusSatisfied, []string{"self-verify claims need machine-readable evidence mapping"}, []string{"summary.coverage", "summary.coverage_gaps"}, []string{"summary includes coverage matrix and fails termination when claims are missing evidence labels"}),
		selfVerificationCandidate(4, "completion-evidence-audit", "completion evidence", 80, selfAugmentCandidateStatusSatisfied, []string{"LangChain-style harness engineering emphasizes forcing verification evidence before completion", "agent-harness has verify-work, but completion reports can still omit structured evidence"}, []string{"verify-work --json evidence_matrix", "verify-work suggested_commands", "verify_work response contract golden", "self-verify candidates --json"}, []string{"verify-work JSON includes evidence_matrix entries for git_preflight, guard_check, and read_only_command", "verify-work JSON includes project-signal suggested_commands", "response contract golden snapshots verify_work JSON output", "candidate export records completion-evidence-audit as satisfied"}),
		selfVerificationCandidate(5, "self-verify-failure-rerun-recipe", "reproducibility", 78, selfAugmentCandidateStatusSatisfied, []string{"failed steps need copy-paste rerun commands"}, []string{"summary.rerun_commands", "failure fixture"}, []string{"failure summaries include rerun_commands keyed by failed label"}),
		selfVerificationCandidate(6, "self-verify-candidate-export", "curriculum", 77, selfAugmentCandidateStatusSatisfied, []string{"self-augment candidates can all be satisfied, so self-verify needs its own next-candidate export"}, []string{"self-verify candidates --json", "state save/read", "response contract golden"}, []string{"self-verify candidates exports open/satisfied candidate IDs and can save a state snapshot"}),
		selfVerificationCandidate(7, "self-verify-step-budget-baseline", "performance", 76, selfAugmentCandidateStatusSatisfied, []string{"slowest_steps top 5 does not catch gradual per-label budget drift"}, []string{"baseline compare fixture", "label budget regression"}, []string{"summary.step_duration_stats records per-label p95 budgets and compare reports step_budget regressions independent of slowest_steps top entries"}),
		selfVerificationCandidate(8, "self-verify-install-dry-run-smoke", "native integration", 76, selfAugmentCandidateStatusSatisfied, []string{"install --dry-run should be an independent no-write evidence label"}, []string{"temp HOME smoke", "dry-run no-write assertion"}, []string{"install dry-run smoke runs install --dry-run against temp HOME/CODEX_HOME/HARNESS_ROOT and asserts planned writes/links without filesystem mutations"}),
		selfVerificationCandidate(9, "self-verify-policy-path-fuzz-plus", "policy/security", 76, selfAugmentCandidateStatusSatisfied, []string{"path policy needs seeded edge cases beyond simple outside paths"}, []string{"preflight fuzz", "policy path fixtures"}, []string{"policy fuzz covers symlink escape, ~/path, remote URL/ref exceptions, and outside workspace assertions"}),
		selfVerificationCandidate(10, "self-verify-json-schema-contract", "contract", 76, selfAugmentCandidateStatusSatisfied, []string{"summary schema drift should be visible without hand-reading golden diffs"}, []string{"summary.contract", "response contract golden"}, []string{"summary.contract includes version, hash, required fields, goals, and coverage claims"}),
		selfVerificationCandidate(11, "self-verify-flake-classifier", "reliability", 75, selfAugmentCandidateStatusSatisfied, []string{"intermittent seed failures need deterministic vs flaky classification"}, []string{"failure_class", "failure_clusters"}, []string{"summary classifies failure patterns and clusters failed seeds by step"}),
		selfVerificationCandidate(12, "self-verify-output-size-budget", "operability", 73, selfAugmentCandidateStatusSatisfied, []string{"large command output can bloat JSON and state snapshots"}, []string{"bounded stdout/stderr", "truncation metadata"}, []string{"StepResult output is budgeted with byte counts and truncation flags"}),
		selfVerificationCandidate(13, "self-verify-history-retention-budget", "state operations", 71, selfAugmentCandidateStatusSatisfied, []string{"history checkpoints need retention planning before state gets slow"}, []string{"history --retention-limit", "prune dry-run/confirm"}, []string{"history retention computes dry-run candidates and requires --confirm to delete"}),
		selfVerificationCandidate(14, "self-verify-parallel-temp-isolation", "concurrency", 70, selfAugmentCandidateStatusSatisfied, []string{"parallel self-verify runs must not collide in temp state, daemon, or artifacts"}, []string{"parallel isolation step", "race tier"}, []string{"parallel isolation probes unique temp roots for state, daemon, and artifact paths"}),
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
