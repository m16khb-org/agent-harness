package mcp

func IssueOpsLifecycleTools() []Tool {
	return []Tool{
		{
			Name:        "issueops_add_decision",
			Description: "Record an explicit decision for an IssueOps loop with kind, rationale, alternatives, affected issue links, and affected artifacts. Unlike feedback (which captures external input), decisions capture internal product/architecture/implementation/test/review/scope choices.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "title", "body", "kind"}, "properties": map[string]any{
				"id":                   map[string]any{"type": "string", "description": "IssueOps id."},
				"title":                map[string]any{"type": "string", "description": "Decision title."},
				"body":                 map[string]any{"type": "string", "description": "Decision body."},
				"kind":                 map[string]any{"type": "string", "description": "Decision kind.", "enum": []string{"product", "architecture", "implementation", "test", "review", "scope", "follow-up"}},
				"rationale":            map[string]any{"type": "string", "description": "Decision rationale."},
				"alternatives":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Alternatives considered."},
				"affected_issue_links": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Affected issue URLs from the issue graph."},
				"affected_artifacts":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Affected artifacts: issue, plan, test, implementation, review, pr_mr, follow-up."},
			}},
		},
		{
			Name:        "issueops_record_routing",
			Description: "Record that a pioneer/CS skill (codd, dijkstra, hopper, shannon, etc.) actually fired at the current IssueOps phase during this run. Captures a live (phase, skill) routing pairing so skill_routing_fidelity can be scored against observed activation instead of a synthesized trace. Idempotent per (phase, skill).",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "phase", "skill"}, "properties": map[string]any{
				"id":    map[string]any{"type": "string", "description": "IssueOps id."},
				"phase": map[string]any{"type": "string", "description": "Lifecycle phase at which the skill fired, such as plan, implement, or grill."},
				"skill": map[string]any{"type": "string", "description": "Skill that fired, such as codd, dijkstra, hopper, shannon, karpathy, turing, von-neumann, or berners-lee."},
			}},
		},
		{
			Name:        "issueops_add_feedback",
			Description: "Record user or review feedback for an IssueOps loop and move it to the feedback phase.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "source", "body"}, "properties": map[string]any{
				"id":             map[string]any{"type": "string", "description": "IssueOps id."},
				"source":         map[string]any{"type": "string", "description": "Feedback source, such as user, review, CI, or QA."},
				"body":           map[string]any{"type": "string", "description": "Feedback body."},
				"classification": map[string]any{"type": "string", "description": "Optional feedback classification, such as contract_change, defect, question, or noise."},
			}},
		},
		{
			Name:        "issueops_mark_issue_updated",
			Description: "Record that unresolved contract_change feedback has been reflected in the remote issue body, unblocking PR readiness.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
		{
			Name:        "issueops_set_phase",
			Description: "Advance an IssueOps loop to a named lifecycle phase (problem, grill, plan, compatibility-review, implement, ai-slop-clean, feedback, pr, done). The plan phase requires a linked issue and recorded intent contract; compatibility-review requires linked issue, provider-linked branch, linked worktree, approved design review, and linked plan; implement additionally requires approved compatibility review, durable worktree preparation, and execution decision; ai-slop-clean additionally requires implementation changes; pr requires strict PR readiness; done requires prior pr phase plus verified remote PR/MR artifact state (unless force=true).",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":    map[string]any{"type": "string", "description": "IssueOps id."},
				"phase": map[string]any{"type": "string", "description": "Target phase: problem, grill, plan, compatibility-review, implement, ai-slop-clean, feedback, pr, or done.", "enum": []string{"problem", "grill", "plan", "compatibility-review", "implement", "ai-slop-clean", "feedback", "pr", "done"}},
				"to":    map[string]any{"type": "string", "description": "Compatibility alias for phase, matching the CLI --to flag.", "enum": []string{"problem", "grill", "plan", "compatibility-review", "implement", "ai-slop-clean", "feedback", "pr", "done"}},
				"force": map[string]any{"type": "boolean", "description": "When true and phase is done, bypass remote artifact verification requirement. The skip reason is recorded in force_release_reason."},
			}},
		},
		{
			Name:        "issueops_verify_remote_artifact",
			Description: "Record that the created PR/MR was verified remotely with URL, labels, and assignees before the IssueOps loop may enter done.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "provider", "kind", "url", "labels", "assignees"}, "properties": map[string]any{
				"id":        map[string]any{"type": "string", "description": "IssueOps id."},
				"provider":  map[string]any{"type": "string", "description": "Remote provider.", "enum": []string{"github", "gitlab"}},
				"kind":      map[string]any{"type": "string", "description": "Remote artifact kind.", "enum": []string{"pr", "mr"}},
				"url":       map[string]any{"type": "string", "description": "Verified PR/MR URL."},
				"labels":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verified remote labels."},
				"assignees": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Verified remote assignees."},
			}},
		},
		{
			Name:        "issueops_pr_readiness",
			Description: "Report whether an IssueOps loop has the issue, plan, exact worktree, clean git state, and upstream sync needed before drafting a PR or MR.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "IssueOps id."},
				"strict": map[string]any{"type": "boolean", "description": "When true, require linked worktree path, clean git status, matching branch, existing plan path, upstream, and 0/0 divergence."},
			}},
		},
		{
			Name:        "issueops_cleanup_status",
			Description: "Report whether a merged IssueOps PR/MR is ready for local cleanup before deleting worktrees or branches. Linked child tasks must have verified child-only close evidence; parent issues remain open umbrella issues.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "IssueOps id."},
				"merged": map[string]any{"type": "boolean", "description": "Whether the remote PR/MR merge was verified before cleanup."},
			}},
		},
		{
			Name:        "issueops_cleanup_close_children",
			Description: "Dry-run or execute child-only cleanup after a child PR/MR has been verified merged into its parent work branch. Closes linked GitHub sub-issues or GitLab child work items only; the parent issue stays open as the umbrella. Set confirm=true to mutate remote state and record close verification evidence.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "merged"}, "properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": "IssueOps id."},
				"merged":  map[string]any{"type": "boolean", "description": "Whether the child PR/MR merge into the parent work branch was verified before closing child tasks."},
				"confirm": map[string]any{"type": "boolean", "description": "When true, close linked child tasks remotely and record verified close evidence. Defaults to false dry-run preview."},
			}},
		},
		{
			Name:        "issueops_cleanup_stale",
			Description: "Scan a repo's non-done IssueOps cycles and classify abandoned ones using multi-signal liveness (confirmed-stale: worktree deleted/reused; likely-done: remote branch merged/absent; needs-review: idle past max-age). Reports only by default; set apply to force-release confirmed-stale and likely-done cycles. Maintenance tool; runs off the hot path and may consult git/remote.",
			InputSchema: map[string]any{"type": "object", "required": []string{"repo"}, "properties": map[string]any{
				"repo":       map[string]any{"type": "string", "description": "Source repository path whose cycles are scanned."},
				"max_age":    map[string]any{"type": "number", "description": "Age in days after which an idle non-done cycle is flagged needs-review. Defaults to 14."},
				"apply":      map[string]any{"type": "boolean", "description": "When true, force-release confirmed-stale and likely-done cycles. needs-review is always report-only. Defaults to false (report only)."},
				"prune_done": map[string]any{"type": "string", "description": "Go duration (e.g. 720h) after which done cycles are pruned; only takes effect together with apply. Defaults to 720h (30 days), matching the CLI --prune-done flag."},
			}},
		},
		{
			Name:        "issueops_remote_score",
			Description: "Deterministically score related issue and label candidates for a new IssueOps issue and select only those at/above the threshold. Read-only background_join gate; join before any remote artifact write.",
			InputSchema: map[string]any{"type": "object", "required": []string{"issue"}, "properties": map[string]any{
				"provider":  map[string]any{"type": "string", "description": "Remote provider: github or gitlab. Defaults to github.", "enum": []string{"github", "gitlab"}},
				"threshold": map[string]any{"type": "number", "description": "Selection cutoff from 0 to 1. Defaults to 0.70."},
				"issue": map[string]any{"type": "object", "description": "The new issue artifact being scored.", "required": []string{"title", "body"}, "properties": map[string]any{
					"provider": map[string]any{"type": "string"},
					"title":    map[string]any{"type": "string"},
					"body":     map[string]any{"type": "string"},
				}},
				"issue_candidates": map[string]any{"type": "array", "description": "Existing related issue candidates to score.", "items": map[string]any{"type": "object", "properties": map[string]any{
					"id":     map[string]any{"type": "string"},
					"title":  map[string]any{"type": "string"},
					"body":   map[string]any{"type": "string"},
					"url":    map[string]any{"type": "string"},
					"state":  map[string]any{"type": "string"},
					"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"score":  map[string]any{"type": "number"},
				}}},
				"label_candidates": map[string]any{"type": "array", "description": "Label candidates to score.", "items": map[string]any{"type": "object", "properties": map[string]any{
					"name":        map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"aliases":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"score":       map[string]any{"type": "number"},
				}}},
			}},
		},
		{
			Name:        "issueops_child_start",
			Description: "Start a delegated child IssueOps cycle from an implement-ready parent when the parent recorded a net-positive sub-agent plan. Writes the child record plus the parent child_cycles index; args mirror `issueops child start`; result is the same IssueOpsChildStartResult JSON shape as the CLI.",
			InputSchema: map[string]any{"type": "object", "required": []string{"parent", "branch", "title", "scope", "acceptance"}, "properties": map[string]any{
				"parent":          map[string]any{"type": "string", "description": "Parent IssueOps cycle id."},
				"branch":          map[string]any{"type": "string", "description": "Delegated child branch name."},
				"title":           map[string]any{"type": "string", "description": "Delegated child task title recorded on the parent reference."},
				"scope":           map[string]any{"type": "string", "description": "Delegated task scope for the child intent contract."},
				"acceptance":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Acceptance criteria for the delegated child cycle."},
				"child_issue_url": map[string]any{"type": "string", "description": "Optional provider child issue/work-item URL to associate with the delegated child."},
			}},
		},
		{
			Name:        "issueops_child_status",
			Description: "List delegated child cycles for a parent and reconcile child phase/verdict state. Read-only unless repair=true appends missing child refs found by scan; args mirror `issueops child status`; result is the same IssueOpsChildStatusResult JSON shape as the CLI.",
			InputSchema: map[string]any{"type": "object", "required": []string{"parent"}, "properties": map[string]any{
				"parent": map[string]any{"type": "string", "description": "Parent IssueOps cycle id."},
				"repair": map[string]any{"type": "boolean", "description": "When true, append scanned child records missing from the parent index."},
			}},
		},
		{
			Name:        "issueops_child_accept",
			Description: "Accept a done delegated child after parent-side validation. Writes the parent-owned accepted verdict and evidence; args mirror `issueops child accept`; result is the same IssueOpsChildValidationResult JSON shape as the CLI.",
			InputSchema: map[string]any{"type": "object", "required": []string{"parent", "child", "evidence"}, "properties": map[string]any{
				"parent":   map[string]any{"type": "string", "description": "Parent IssueOps cycle id."},
				"child":    map[string]any{"type": "string", "description": "Delegated child IssueOps cycle id."},
				"evidence": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Parent validation evidence. At least one entry is required."},
			}},
		},
		{
			Name:        "issueops_child_reject",
			Description: "Reject a delegated child result when parent validation finds redo work. Writes the parent-owned rejected verdict and reason; args mirror `issueops child reject`; result is the same IssueOpsChildValidationResult JSON shape as the CLI.",
			InputSchema: map[string]any{"type": "object", "required": []string{"parent", "child", "reason"}, "properties": map[string]any{
				"parent": map[string]any{"type": "string", "description": "Parent IssueOps cycle id."},
				"child":  map[string]any{"type": "string", "description": "Delegated child IssueOps cycle id."},
				"reason": map[string]any{"type": "string", "description": "Rejection reason. Must be at least 10 characters."},
			}},
		},
		{
			Name:        "issueops_child_drop",
			Description: "Drop a delegated child from the parent gate when the work is deliberately abandoned. Writes the parent-owned dropped verdict and reason; args mirror `issueops child drop`; result is the same IssueOpsChildValidationResult JSON shape as the CLI.",
			InputSchema: map[string]any{"type": "object", "required": []string{"parent", "child", "reason"}, "properties": map[string]any{
				"parent": map[string]any{"type": "string", "description": "Parent IssueOps cycle id."},
				"child":  map[string]any{"type": "string", "description": "Delegated child IssueOps cycle id."},
				"reason": map[string]any{"type": "string", "description": "Drop reason. Must be at least 10 characters."},
			}},
		},
		{
			Name:        "issueops_force_release",
			Description: "Force-release a stuck IssueOps cycle to done, bypassing phase gate requirements. Use only when a cycle is deadlocked and cannot complete normally (e.g. missing remote_artifact verification). Requires an explicit reason recorded in the cycle state.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "reason"}, "properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "IssueOps id of the stuck cycle."},
				"reason": map[string]any{"type": "string", "description": "Reason for force-release, recorded in the cycle state for audit."},
			}},
		},
		{
			Name:        "issueops_remote_render_template",
			Description: "Render and validate a canonical Korean IssueOps remote issue, child task, or PR/MR body from the shared core template policy without mutating provider state.",
			InputSchema: map[string]any{"type": "object", "required": []string{"kind", "template", "title"}, "properties": map[string]any{
				"kind":          map[string]any{"type": "string", "description": "Artifact kind.", "enum": []string{"issue", "child", "pr"}},
				"template":      map[string]any{"type": "string", "description": "Template kind.", "enum": []string{"bug", "feature", "proposal", "implementation_task", "child_task", "pull_request"}},
				"title":         map[string]any{"type": "string", "description": "Artifact title."},
				"provider":      map[string]any{"type": "string", "description": "Remote provider for provider-specific body rules.", "enum": []string{"github", "gitlab"}},
				"fields":        map[string]any{"type": "object", "description": "Template field values keyed by canonical field name or documented alias."},
				"score_summary": map[string]any{"type": "string", "description": "Related issue and label scoring summary to include in the rendered artifact."},
				"score_result":  map[string]any{"type": "object", "description": "Optional structured score result; score_summary takes precedence when both are supplied."},
			}},
		},
		{
			Name:        "issueops_remote_create_issue",
			Description: "Create a remote issue (GitHub/GitLab) for the IssueOps cycle. Dry-run by default; pass confirm=true to execute. Returns the URL of the created issue or a preview of what would be created.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "title"}, "properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "IssueOps id. The provider is inferred from the cycle's issue_url or branch_prepare records."},
				"title":         map[string]any{"type": "string", "description": "Issue title."},
				"body":          map[string]any{"type": "string", "description": "Issue body (markdown)."},
				"provider":      map[string]any{"type": "string", "description": "Optional remote provider override.", "enum": []string{"github", "gitlab"}},
				"template":      map[string]any{"type": "string", "description": "Optional canonical template kind to render or validate.", "enum": []string{"bug", "feature", "proposal", "implementation_task"}},
				"fields":        map[string]any{"type": "object", "description": "Template field values keyed by canonical field name or documented alias."},
				"score_summary": map[string]any{"type": "string", "description": "Related issue and label scoring summary."},
				"score_result":  map[string]any{"type": "object", "description": "Optional structured score result."},
				"labels":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels to apply."},
				"assignees":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Assignee usernames."},
				"confirm":       map[string]any{"type": "boolean", "description": "Set true to execute; omit for dry-run preview."},
			}},
		},
		{
			Name:        "issueops_remote_reflect_devils_advocate",
			Description: "Reflect the recorded devil's-advocate findings into the linked issue's managed body section (GitHub/GitLab). Dry-run by default; pass confirm=true to write and stamp issue_reflected_at, which the regress precondition requires before re-planning a stop verdict.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":       map[string]any{"type": "string", "description": "IssueOps id. The provider is inferred from the cycle's issue_url or branch_prepare records."},
				"provider": map[string]any{"type": "string", "description": "Optional remote provider override.", "enum": []string{"github", "gitlab"}},
				"confirm":  map[string]any{"type": "boolean", "description": "Set true to write to the remote issue; omit for dry-run preview."},
			}},
		},
		{
			Name:        "issueops_remote_create_child",
			Description: "Create a provider-native child work item under the linked parent issue, verify hierarchy/labels/assignees, then record the child link in IssueOps state when confirm=true. Dry-run by default.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "title", "labels", "assignees"}, "properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "IssueOps id. The parent issue URL and provider are inferred from the cycle state."},
				"title":         map[string]any{"type": "string", "description": "Child task title."},
				"body":          map[string]any{"type": "string", "description": "Child task body (markdown)."},
				"provider":      map[string]any{"type": "string", "description": "Optional remote provider override.", "enum": []string{"github", "gitlab"}},
				"template":      map[string]any{"type": "string", "description": "Optional canonical template kind to render or validate.", "enum": []string{"child_task"}},
				"fields":        map[string]any{"type": "object", "description": "Template field values keyed by canonical field name or documented alias."},
				"score_summary": map[string]any{"type": "string", "description": "Related issue and label scoring summary."},
				"score_result":  map[string]any{"type": "object", "description": "Optional structured score result."},
				"labels":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels to apply and verify on the child."},
				"assignees":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Assignee usernames to apply and verify on the child."},
				"confirm":       map[string]any{"type": "boolean", "description": "Set true to create, attach, verify, and record the child; omit for dry-run preview."},
			}},
		},
		{
			Name:        "issueops_remote_create_pr",
			Description: "Create a remote pull request / merge request for the IssueOps cycle. Dry-run by default; pass confirm=true to execute. Provider and branch info are inferred from the cycle state (branch_prepare, branch, remote_artifact).",
			InputSchema: map[string]any{"type": "object", "required": []string{"id", "title"}, "properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "IssueOps id."},
				"title":         map[string]any{"type": "string", "description": "PR/MR title."},
				"body":          map[string]any{"type": "string", "description": "PR/MR body (markdown)."},
				"head":          map[string]any{"type": "string", "description": "Source branch (defaults to cycle branch)."},
				"base":          map[string]any{"type": "string", "description": "Target branch (defaults to branch_prepare.base_branch)."},
				"provider":      map[string]any{"type": "string", "description": "Optional remote provider override.", "enum": []string{"github", "gitlab"}},
				"template":      map[string]any{"type": "string", "description": "Optional canonical template kind to render or validate.", "enum": []string{"pull_request"}},
				"fields":        map[string]any{"type": "object", "description": "Template field values keyed by canonical field name or documented alias."},
				"score_summary": map[string]any{"type": "string", "description": "Related issue and label scoring summary."},
				"score_result":  map[string]any{"type": "object", "description": "Optional structured score result."},
				"labels":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels to apply."},
				"assignees":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Assignee usernames."},
				"confirm":       map[string]any{"type": "boolean", "description": "Set true to execute; omit for dry-run preview."},
			}},
		},
		{
			Name:        "issueops_remote_sync_graph",
			Description: "Sync the local IssueOps issue graph (typed related-issue links) to the remote issue as a comment listing each link. Dry-run by default; pass confirm=true to post the comment. Helps collaborators see the decision structure directly on the remote issue.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": "IssueOps id."},
				"confirm": map[string]any{"type": "boolean", "description": "Set true to post the comment; omit for dry-run preview."},
			}},
		},
		{
			Name:        "issueops_resume",
			Description: "Read an IssueOps cycle by id or read the session-to-cycle binding for a repo, returning cycle state, worktree path, branch, readiness, and HARNESS_EXPECTED_WORKTREE guidance. When repo is unbound and id is omitted, suggests active cycles for the repo. Optionally bind the session with bind=true.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"repo": map[string]any{"type": "string", "description": "Repository path."},
				"id":   map[string]any{"type": "string", "description": "Optional IssueOps id to resume directly."},
				"bind": map[string]any{"type": "boolean", "description": "When true and a cycle is found, bind the session to it."},
			}},
		},
		{
			Name:        "issueops_heartbeat",
			Description: "Update the liveness heartbeat for an active IssueOps cycle without otherwise mutating phase, links, or work artifacts.",
			InputSchema: map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "IssueOps id."},
			}},
		},
	}
}
