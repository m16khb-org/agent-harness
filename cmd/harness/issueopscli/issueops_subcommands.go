package issueopscli

import (
	"flag"
	"fmt"
	"strings"

	"agent-harness/internal/core"
)

// This file holds the individual `issueops <subcommand>` handlers. runIssueOps
// (issueops.go) stays a thin dispatcher that routes to one handler per
// subcommand, so adding or changing a subcommand is local to its own function
// instead of growing the router's branch count.

func runIssueOpsStart(args []string) error {
	fs := flag.NewFlagSet("issueops start", flag.ContinueOnError)
	repo := fs.String("repo", "", "repository path")
	branch := fs.String("branch", "", "working branch")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsStatus(args []string) error {
	fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.IssueOpsStatus(core.IssueOpsStateRoot(), *id)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkIssue(args []string) error {
	fs := flag.NewFlagSet("issueops link-issue", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsIssueWithActor(core.IssueOpsStateRoot(), *id, *issueURL, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkPlan(args []string) error {
	fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	planPath := fs.String("plan-path", "", "issue-driven plan path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsPlanWithActor(core.IssueOpsStateRoot(), *id, *planPath, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkWorktree(args []string) error {
	fs := flag.NewFlagSet("issueops link-worktree", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	worktreePath := fs.String("worktree-path", "", "issue-driven worktree path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsWorktreeWithActor(core.IssueOpsStateRoot(), *id, *worktreePath, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkChild(args []string) error {
	fs := flag.NewFlagSet("issueops link-child", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	childURL := fs.String("child-url", "", "GitHub sub-issue or GitLab child item URL")
	title := fs.String("title", "", "optional child issue title")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	if err := verifyIssueOpsChildIssueBeforeLink(*childURL); err != nil {
		return printIssueOpsResult(core.IssueOpsRecord{OK: false}, *jsonOut, err)
	}
	record, err := core.LinkIssueOpsChildWithActor(core.IssueOpsStateRoot(), *id, *childURL, *title, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkRelated(args []string) error {
	fs := flag.NewFlagSet("issueops link-related", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	linkType := fs.String("type", "", "link type: depends-on, blocks, supersedes, follows-up, duplicates, splits-from, implements")
	relatedURL := fs.String("related-url", "", "related issue URL")
	title := fs.String("title", "", "optional related issue title")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsRelatedWithActor(core.IssueOpsStateRoot(), *id, *linkType, *relatedURL, *title, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsChild(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(issueOpsChildUsage)
		return nil
	}
	switch args[0] {
	case "start":
		return runIssueOpsChildStart(args[1:])
	case "status":
		return runIssueOpsChildStatus(args[1:], false)
	case "list":
		return runIssueOpsChildStatus(args[1:], false)
	case "accept":
		return runIssueOpsChildAccept(args[1:])
	case "reject":
		return runIssueOpsChildReject(args[1:])
	case "drop":
		return runIssueOpsChildDrop(args[1:])
	default:
		return fmt.Errorf("unknown issueops child subcommand %q", args[0])
	}
}

func runIssueOpsChildStart(args []string) error {
	fs := flag.NewFlagSet("issueops child start", flag.ContinueOnError)
	parentID := fs.String("parent", "", "parent issueops id")
	actor := addIssueOpsActorFlags(fs)
	branch := fs.String("branch", "", "child branch")
	title := fs.String("title", "", "child task title")
	scope := fs.String("scope", "", "delegated task scope")
	childIssueURL := fs.String("child-issue-url", "", "optional child issue URL")
	jsonOut := fs.Bool("json", false, "print JSON")
	var acceptance repeatedFlag
	fs.Var(&acceptance, "acceptance", "acceptance criterion; repeatable")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.StartIssueOpsChildWithActor(core.IssueOpsStateRoot(), core.IssueOpsChildStartRequest{
		ParentID:           *parentID,
		Branch:             *branch,
		Title:              *title,
		TaskScope:          *scope,
		AcceptanceCriteria: []string(acceptance),
		ChildIssueURL:      *childIssueURL,
	}, actor.actor())
	return printIssueOpsChildValue(result, *jsonOut, err)
}

func runIssueOpsChildStatus(args []string, repairDefault bool) error {
	fs := flag.NewFlagSet("issueops child status", flag.ContinueOnError)
	parentID := fs.String("parent", "", "parent issueops id")
	actor := addIssueOpsActorFlags(fs)
	repair := fs.Bool("repair", repairDefault, "append scanned children missing from the parent index")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.IssueOpsChildStatusWithActor(core.IssueOpsStateRoot(), *parentID, *repair, actor.actor())
	if *jsonOut {
		return printIssueOpsChildValue(result, true, err)
	}
	if err != nil {
		return err
	}
	for _, child := range result.Children {
		fmt.Printf("%s %s %s verdict=%s\n", child.CycleID, child.Phase, child.Branch, child.ValidationVerdict)
	}
	return nil
}

func runIssueOpsChildAccept(args []string) error {
	fs := flag.NewFlagSet("issueops child accept", flag.ContinueOnError)
	parentID := fs.String("parent", "", "parent issueops id")
	actor := addIssueOpsActorFlags(fs)
	childID := fs.String("child", "", "child issueops id")
	jsonOut := fs.Bool("json", false, "print JSON")
	var evidence repeatedFlag
	fs.Var(&evidence, "evidence", "validation evidence; repeatable")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.AcceptIssueOpsChildWithActor(core.IssueOpsStateRoot(), *parentID, *childID, []string(evidence), actor.actor())
	return printIssueOpsChildValue(result, *jsonOut, err)
}

func runIssueOpsChildReject(args []string) error {
	fs := flag.NewFlagSet("issueops child reject", flag.ContinueOnError)
	parentID := fs.String("parent", "", "parent issueops id")
	actor := addIssueOpsActorFlags(fs)
	childID := fs.String("child", "", "child issueops id")
	reason := fs.String("reason", "", "rejection reason")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.RejectIssueOpsChildWithActor(core.IssueOpsStateRoot(), *parentID, *childID, *reason, nil, actor.actor())
	return printIssueOpsChildValue(result, *jsonOut, err)
}

func runIssueOpsChildDrop(args []string) error {
	fs := flag.NewFlagSet("issueops child drop", flag.ContinueOnError)
	parentID := fs.String("parent", "", "parent issueops id")
	actor := addIssueOpsActorFlags(fs)
	childID := fs.String("child", "", "child issueops id")
	reason := fs.String("reason", "", "drop reason")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.DropIssueOpsChildWithActor(core.IssueOpsStateRoot(), *parentID, *childID, *reason, actor.actor())
	return printIssueOpsChildValue(result, *jsonOut, err)
}

func printIssueOpsChildValue(value any, jsonOut bool, err error) error {
	if err != nil {
		if jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if jsonOut {
		return printJSON(value)
	}
	fmt.Printf("%+v\n", value)
	return nil
}

func runIssueOpsRoutingScore(args []string) error {
	fs := flag.NewFlagSet("issueops routing-score", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	expect := fs.String("expect", "", "expected routing as comma-separated phase:skill pairings (e.g. plan:codd,implement:dijkstra)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	expected, err := parseExpectedRouting(*expect)
	if err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			return printIssueOpsErrorJSON(err)
		}
		return err
	}
	result := core.ScoreLiveRoutingFidelity(record, expected)
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("routing fidelity: ok=%v (observed %d pairings)\n", result.OK, len(record.RoutingTrace))
	for _, m := range result.Missing {
		fmt.Printf("- missing: %s@%s\n", m.Skill, m.Phase)
	}
	return nil
}

func parseExpectedRouting(spec string) ([]core.SkillRouting, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("--expect is required as comma-separated phase:skill pairings")
	}
	var out []core.SkillRouting
	for _, p := range strings.Split(spec, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		phase, skill, ok := strings.Cut(p, ":")
		phase, skill = strings.TrimSpace(phase), strings.TrimSpace(skill)
		if !ok || phase == "" || skill == "" {
			return nil, fmt.Errorf("invalid --expect pairing %q; want phase:skill", p)
		}
		out = append(out, core.SkillRouting{Phase: phase, Skill: skill})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--expect produced no pairings")
	}
	return out, nil
}

func runIssueOpsRecordRouting(args []string) error {
	fs := flag.NewFlagSet("issueops record-routing", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	phase := fs.String("phase", "", "lifecycle phase at which the skill fired")
	skill := fs.String("skill", "", "skill that fired (codd, dijkstra, hopper, shannon, ...)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.RecordIssueOpsRoutingWithActor(core.IssueOpsStateRoot(), *id, *phase, *skill, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsPhase(args []string) error {
	fs := flag.NewFlagSet("issueops phase", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	to := fs.String("to", "", "target phase: problem, grill, plan, compatibility-review, implement, ai-slop-clean, feedback, pr, done")
	force := fs.Bool("force", false, "bypass remote artifact verification when advancing to done")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	var record core.IssueOpsRecord
	var err error
	if *force && *to == "done" {
		record, err = core.ForceDoneIssueOps(core.IssueOpsStateRoot(), *id)
	} else {
		record, err = core.AdvanceIssueOpsPhaseWithActor(core.IssueOpsStateRoot(), *id, *to, actor.actor())
	}
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsPRReadiness(args []string) error {
	fs := flag.NewFlagSet("issueops pr-readiness", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	strict := fs.Bool("strict", false, "verify git cleanliness, upstream sync, plan path, and linked worktree path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	readiness := core.IssueOpsPRReadiness(record)
	if *strict {
		readiness = core.IssueOpsStrictPRReadinessWithState(core.IssueOpsStateRoot(), record)
	}
	if *jsonOut {
		return printJSON(readiness)
	}
	fmt.Printf("ready: %v\n", readiness.Ready)
	for _, missing := range readiness.Missing {
		fmt.Printf("- missing: %s\n", missing)
	}
	return nil
}

func runIssueOpsForceRelease(args []string) error {
	fs := flag.NewFlagSet("issueops force-release", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	reason := fs.String("reason", "", "reason for force-release")
	expectedRawSHA256 := fs.String("expected-raw-sha256", "", "sealed raw record SHA-256 (requires expected-canonical-sha256)")
	expectedCanonicalSHA256 := fs.String("expected-canonical-sha256", "", "sealed canonical record SHA-256 (requires expected-raw-sha256)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	var rawDigestProvided, canonicalDigestProvided bool
	fs.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "expected-raw-sha256":
			rawDigestProvided = true
		case "expected-canonical-sha256":
			canonicalDigestProvided = true
		}
	})
	casRequested := rawDigestProvided || canonicalDigestProvided
	if casRequested && (!rawDigestProvided || !canonicalDigestProvided) {
		err := fmt.Errorf("expected-raw-sha256 and expected-canonical-sha256 must be provided together")
		if *jsonOut {
			_ = printIssueOpsErrorJSON(err)
		}
		return err
	}
	if casRequested {
		result, err := core.ForceReleaseIssueOpsCAS(core.IssueOpsStateRoot(), *id, *reason, core.ForceReleaseCASRequest{
			ExpectedRawSHA256:       *expectedRawSHA256,
			ExpectedCanonicalSHA256: *expectedCanonicalSHA256,
		})
		if err != nil {
			if *jsonOut {
				_ = printIssueOpsErrorJSON(err)
			}
			return err
		}
		if *jsonOut {
			return printJSON(result)
		}
		fmt.Printf("%s %s %s\n", result.Record.ID, result.Record.Phase, result.Record.Repo)
		return nil
	}
	record, err := core.ForceReleaseIssueOps(core.IssueOpsStateRoot(), *id, *reason)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsResume(args []string) error {
	fs := flag.NewFlagSet("issueops resume", flag.ContinueOnError)
	repo := fs.String("repo", "", "repository path")
	id := fs.String("id", "", "issueops id")
	bind := fs.Bool("bind", false, "bind the session to the resumed cycle")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result := core.IssueOpsResume(*repo, *id)
	if *bind && result.OK && result.CycleID != "" {
		if result.ExecutionHandoff != nil {
			err := fmt.Errorf("resume bind is read-only and refused for a supervised handoff; use the exact handoff claim command")
			if *jsonOut {
				_ = printIssueOpsErrorJSON(err)
			}
			return err
		}
		if err := core.BindIssueOpsSessionForCycle(result.Repo, result.CycleID); err != nil {
			return printIssueOpsResult(core.IssueOpsRecord{OK: false}, *jsonOut, fmt.Errorf("resume bind: %w", err))
		}
	}
	if *jsonOut {
		return printJSON(result)
	}
	if !result.OK {
		return fmt.Errorf("no active IssueOps cycle to resume")
	}
	if result.Bound {
		fmt.Printf("bound: %s %s %s\nworktree: %s\nbranch: %s\n",
			result.CycleID, result.Phase, result.Repo, result.WorktreePath, result.Branch)
		if result.Readiness != nil && !result.Readiness.Ready {
			fmt.Printf("readiness: not ready\nmissing: %s\n", strings.Join(result.Readiness.Missing, ", "))
		} else {
			fmt.Printf("readiness: ready\n")
		}
		if result.Guidance != "" {
			fmt.Printf("guidance: %s\n", result.Guidance)
		}
	} else {
		fmt.Printf("not bound. suggested cycles: %s\n", strings.Join(result.SuggestedCycles, ", "))
	}
	return nil
}

func runIssueOpsHeartbeat(args []string) error {
	fs := flag.NewFlagSet("issueops heartbeat", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	attempt := fs.Int("attempt", 0, "handoff attempt")
	epoch := fs.String("ownership-epoch", "", "handoff ownership epoch")
	contextSHA := fs.String("context-sha256", "", "handoff context sha256")
	host := fs.String("host", "", "claimed worker host")
	sessionID := fs.String("session-id", "", "claimed worker session id")
	agentID := fs.String("agent-id", "", "claimed worker agent id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.RecordIssueOpsHeartbeatWithRequest(core.IssueOpsStateRoot(), core.IssueOpsHeartbeatRequest{
		ID: *id, Attempt: *attempt, OwnershipEpoch: *epoch, ContextSHA256: *contextSHA, Host: *host, SessionID: *sessionID, AgentID: *agentID,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}
