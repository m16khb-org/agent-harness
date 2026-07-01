package mcp

func IssueOpsBasicTools() []Tool {
	return []Tool{
		{
			Name:        "issueops_start",
			Description: "Start an IssueOps loop and persist its issue-driven workflow state under harness state.",
			InputSchema: map[string]any{"type": "object", "required": []string{"repo"}, "properties": map[string]any{
				"repo":   map[string]any{"type": "string", "description": "Repository path this IssueOps loop belongs to."},
				"branch": map[string]any{"type": "string", "description": "Optional working branch name."},
			}},
		},
		{
			Name:        "issueops_status",
			Description: "Read a persisted IssueOps loop by id.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
		{
			Name:        "issueops_record_intent",
			Description: "Record the raw user request, interpreted intent, success criteria, constraints, non-goals, and ambiguity ledger before an IssueOps loop may enter planning.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "raw_request", "interpreted_intent", "success_criteria"}, "properties": map[string]any{
				"id":                 map[string]any{"type": "string", "description": "IssueOps id."},
				"raw_request":        map[string]any{"type": "string", "description": "Original user request text."},
				"interpreted_intent": map[string]any{"type": "string", "description": "Main agent interpretation of the user's intent."},
				"success_criteria":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Concrete success criteria."},
				"constraints":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Known constraints."},
				"ambiguities":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Resolved, deferred, or blocking ambiguity ledger entries."},
				"non_goals":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Explicit non-goals."},
				"intent_class":       map[string]any{"type": "string", "description": "Intent class controlling plan-prep gate strictness: trivial skips the gate, other classes enforce it.", "enum": []string{"trivial", "standard", "refactoring", "architecture", "research"}},
			}},
		},
		{
			Name:        "issueops_plan_prep_record",
			Description: "Record the pre-plan evidence gate for an IssueOps loop: prior-decision lookup, related-issue scoring, and web research. Each item takes either evidence or a mutually-exclusive waive reason. Required before entering the plan phase for non-trivial intent classes.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":                    map[string]any{"type": "string", "description": "IssueOps id."},
				"decisions_evidence":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Prior-decision evidence such as ADR or decision links."},
				"decisions_waive":       map[string]any{"type": "string", "description": "Reason prior-decision lookup is unnecessary."},
				"related_score_ref":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "remote score result summary (selected/rejected candidates, threshold)."},
				"related_waive":         map[string]any{"type": "string", "description": "Reason related-issue scoring is unnecessary."},
				"web_research_evidence": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Web research evidence such as research file paths or sources."},
				"web_research_waive":    map[string]any{"type": "string", "description": "Reason web research is unnecessary."},
			}},
		},
		{
			Name:        "issueops_link_issue",
			Description: "Attach a GitHub/GitLab issue URL to an IssueOps loop. The loop moves to plan phase only when its intent contract is recorded.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "issue_url"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"issue_url": map[string]any{"type": "string", "description": "GitHub/GitLab issue URL."},
			}},
		},
		{
			Name:        "issueops_prepare_branch",
			Description: "Record provider-linked issue branch preparation and expose the required MCP-first, provider API fallback, fail-closed order. This must be used before creating a local worktree for an IssueOps issue branch.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "provider", "issue_url", "branch", "base_branch"}, "properties": map[string]any{
				"id":                map[string]any{"type": "string", "description": "IssueOps id."},
				"provider":          map[string]any{"type": "string", "description": "Remote provider: github or gitlab.", "enum": []string{"github", "gitlab"}},
				"issue_url":         map[string]any{"type": "string", "description": "GitHub/GitLab issue URL."},
				"branch":            map[string]any{"type": "string", "description": "Provider-linked branch name, such as 2386-title or 2387-title. For GitLab, the full branch name must start with the issue number followed by a hyphen so the issue Development section links it."},
				"base_branch":       map[string]any{"type": "string", "description": "Remote base branch or ref."},
				"base_sha":          map[string]any{"type": "string", "description": "Optional resolved base commit SHA."},
				"remote_branch_url": map[string]any{"type": "string", "description": "Optional provider branch URL after creation."},
				"link_verified":     map[string]any{"type": "boolean", "description": "Whether the provider issue UI/API was verified to show the branch link."},
			}},
		},
		{
			Name:        "issueops_link_worktree",
			Description: "Attach the exact existing issue-driven worktree path that mutating tool guards must target. Requires linked issue and verified provider branch evidence.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "worktree_path"}, "properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "IssueOps id."},
				"worktree_path": map[string]any{"type": "string", "description": "Expected isolated worktree path."},
			}},
		},
		{
			Name:        "issueops_review_design",
			Description: "Record the reviewed IssueOps design, refactor boundary, alternatives, risks, verification matrix, and approval before implementation. When approved=true, include refactor_plan, at least one alternative, at least one risk, no open_questions, and a verification item such as \"design review checked alternatives and risks\"; design_review_evidence is not a separate field.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "problem_summary", "proposed_design", "verification"}, "properties": map[string]any{
				"id":              map[string]any{"type": "string", "description": "IssueOps id."},
				"problem_summary": map[string]any{"type": "string", "description": "Reviewed problem summary."},
				"proposed_design": map[string]any{"type": "string", "description": "Reviewed design."},
				"refactor_plan":   map[string]any{"type": "string", "description": "Refactor plan or boundary decision."},
				"alternatives":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Alternatives considered."},
				"risks":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Design risks."},
				"verification":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verification steps. For approved=true, one item must be design review evidence, for example: design review checked alternatives and risks. Add normal test commands as separate items."},
				"open_questions":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Open design questions. Must be empty when approved is true."},
				"approved":        map[string]any{"type": "boolean", "description": "Whether the design is approved for implementation."},
			}},
		},
		{
			Name:        "issueops_link_plan",
			Description: "Attach the issue-driven plan path to an IssueOps loop. Requires linked issue, verified provider branch evidence, linked worktree, and approved design review. This does not enter implementation by itself; run issueops_prepare_worktree_tools to prepare dependencies and CodeGraph for the linked worktree before implementation.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "plan_path"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"plan_path": map[string]any{"type": "string", "description": "Plan file path."},
			}},
		},
		{
			Name:        "issueops_prepare_worktree_tools",
			Description: "Prepare the linked IssueOps worktree before implementation by checking dependencies, installing supported missing dependencies, and initializing CodeGraph against the exact worktree path. The result is persisted on the IssueOps record and gates implementation readiness.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
		{
			Name:        "issueops_record_execution_decision",
			Description: "Record the durable pre-implementation execution decision: auto-proceed boundaries, hook-blocked work, human gates, and whether sub-agents are not used or explicitly planned from the documented allowlist.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "auto_proceed", "hook_blocked", "human_gates", "subagent_use"}, "properties": map[string]any{
				"id":                 map[string]any{"type": "string", "description": "IssueOps id."},
				"auto_proceed":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Conditions under which the main agent may continue without asking again."},
				"hook_blocked":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Workflow actions hooks must not perform."},
				"human_gates":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Conditions that require human-in-the-loop confirmation."},
				"subagent_use":       map[string]any{"type": "string", "description": "Sub-agent usage decision.", "enum": []string{"none", "planned"}},
				"subagent_rationale": map[string]any{"type": "string", "description": "Required when subagent_use=none; optional top-level rationale when planned."},
				"subagent_plans": map[string]any{"type": "array", "description": "Required when subagent_use=planned. Each plan must use a documented sub-agent pattern and benefit, list the tradeoffs, and explain why the plan remains net-positive.", "items": map[string]any{"type": "object", "required": []string{"objective", "pattern", "benefit", "tradeoffs", "net_positive_rationale", "scope", "verification", "fallback"}, "properties": map[string]any{
					"objective":              map[string]any{"type": "string"},
					"pattern":                map[string]any{"type": "string", "enum": []string{"high-volume-exploration", "isolated-worktree-work", "forked-context-exploration", "devils-advocate-review", "cross-verification-consensus", "parallel-independent-research", "task-fan-out-coordination", "background-long-running-work", "model-specialization-cost-routing", "tool-permission-gating", "plan-then-execute-separation", "triage-specialist-routing"}},
					"benefit":                map[string]any{"type": "string", "enum": []string{"context_isolation", "parallel_speed", "fresh_review", "tool_gating", "long_running", "model_specialization", "isolated_worktree"}},
					"tradeoffs":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Known costs such as limited mid-run steering, reduced visibility, added latency/token cost, or coordination overhead."},
					"net_positive_rationale": map[string]any{"type": "string", "description": "Why the expected benefit outweighs the recorded tradeoffs for this specific task."},
					"scope":                  map[string]any{"type": "string"},
					"verification":           map[string]any{"type": "string"},
					"fallback":               map[string]any{"type": "string"},
				}}},
			}},
		},
		{
			Name:        "issueops_record_compatibility_review",
			Description: "Record the IssueOps compatibility-review phase: backward compatibility findings, side effects, rollback plan, verification evidence, unresolved blockers, and approval before implementation may proceed.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "backward_compatibility", "side_effects", "rollback_plan", "verification"}, "properties": map[string]any{
				"id":                     map[string]any{"type": "string", "description": "IssueOps id."},
				"backward_compatibility": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Findings about existing public behavior, state JSON, CLI/MCP/API, schema, or migration compatibility."},
				"side_effects":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Expected side effects and their affected surfaces."},
				"rollback_plan":          map[string]any{"type": "string", "description": "Concrete rollback or mitigation path if the compatibility review was wrong."},
				"verification":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verification evidence for compatibility and side effects."},
				"blockers":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Unresolved blockers. Must be empty when approved is true."},
				"approved":               map[string]any{"type": "boolean", "description": "Whether compatibility and side effects are resolved enough to proceed."},
			}},
		},
		{
			Name:        "issueops_record_devils_advocate_review",
			Description: "Record the brooks devil's-advocate verdict on the completed plan: pass, revise, or stop, with surfaced findings and an optional explicit waiver. A recorded pass (or a waived stop/revise) is a fail-closed precondition of implement entry; a stop is reflected into the remote issue before the cycle regresses.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "verdict"}, "properties": map[string]any{
				"id":               map[string]any{"type": "string", "description": "IssueOps id."},
				"verdict":          map[string]any{"type": "string", "description": "Devil's-advocate verdict: pass, revise, or stop."},
				"findings":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Surfaced problems. Required for a stop/revise verdict unless waived."},
				"waived":           map[string]any{"type": "boolean", "description": "Explicitly waive a stop/revise verdict to proceed."},
				"waiver_rationale": map[string]any{"type": "string", "description": "Rationale required when waived is true."},
			}},
		},
		{
			Name:        "issueops_record_domain_review",
			Description: "Record the IssueOps grill-phase domain review: terminology, how the change fits the current domain model, risks, and unresolved uncertainties. Backs the grill domain_review phase-ledger artifact.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":                 map[string]any{"type": "string", "description": "IssueOps id."},
				"model_fit":          map[string]any{"type": "string", "description": "How the change fits the current domain model. Required unless terminology is provided."},
				"terminology":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Domain terminology notes."},
				"risks":              map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Domain risks surfaced during grilling."},
				"open_uncertainties": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Unresolved uncertainties to carry into planning."},
			}},
		},
		{
			Name:        "issueops_record_ai_slop_clean_evidence",
			Description: "Record AI-slop-clean evidence: which cleanup categories were checked or cleaned and which verifications were rerun afterwards. Backs the ai-slop-clean cleanup_evidence and verification_evidence phase-ledger artifacts.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "categories", "verification"}, "properties": map[string]any{
				"id":           map[string]any{"type": "string", "description": "IssueOps id."},
				"categories":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Cleanup categories checked or cleaned."},
				"verification": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verifications rerun after cleanup."},
			}},
		},
		{
			Name:        "issueops_resolve_feedback",
			Description: "Record the resolution outcome of an IssueOps feedback item by index. Backs the feedback feedback_resolution phase-ledger artifact.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "index", "resolution"}, "properties": map[string]any{
				"id":         map[string]any{"type": "string", "description": "IssueOps id."},
				"index":      map[string]any{"type": "integer", "description": "0-based index of the feedback item to resolve."},
				"resolution": map[string]any{"type": "string", "description": "Resolution outcome.", "enum": []string{"valid-defect", "question-answered", "noise-dismissed"}},
			}},
		},
		{
			Name:        "issueops_regress_for_replan",
			Description: "Take the IssueOps feedback loop backward when the Brooks devil's advocate returns a stop verdict: regress a plan or compatibility-review cycle to grill so scope is re-investigated and the plan redone. Records the stop reason as a scope decision, clears the rejected design's approval, and marks the plan/compatibility-review ledger entries stale. Does not delete the worktree, branch, or remote artifacts.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "reason"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "IssueOps id."},
				"reason": map[string]any{"type": "string", "description": "The Brooks stop verdict / why the plan must be redone."},
			}},
		},
		{
			Name:        "issueops_link_child",
			Description: "Record an existing provider-native child work item for an IssueOps loop, such as a GitHub sub-issue or GitLab child item.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "child_url"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"child_url": map[string]any{"type": "string", "description": "GitHub sub-issue or GitLab child item URL."},
				"title":     map[string]any{"type": "string", "description": "Optional child issue title."},
			}},
		},
		{
			Name:        "issueops_link_related",
			Description: "Record a typed relationship between the current IssueOps issue and another issue. Supports depends-on, blocks, supersedes, follows-up, duplicates, splits-from, and implements link types. Unlike link-child, this does not require the linked issue to be in the same project.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "type", "related_url"}, "properties": map[string]any{
				"id":          map[string]any{"type": "string", "description": "IssueOps id."},
				"type":        map[string]any{"type": "string", "description": "Relationship type.", "enum": []string{"depends-on", "blocks", "supersedes", "follows-up", "duplicates", "splits-from", "implements"}},
				"related_url": map[string]any{"type": "string", "description": "URL of the related issue."},
				"title":       map[string]any{"type": "string", "description": "Optional title of the related issue."},
			}},
		},
	}
}
