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
	issueURL := fs.String("issue-url", "", "GitHub/GitLab issue URL")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsIssue(core.IssueOpsStateRoot(), *id, *issueURL)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkPlan(args []string) error {
	fs := flag.NewFlagSet("issueops link-plan", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	planPath := fs.String("plan-path", "", "issue-driven plan path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsPlan(core.IssueOpsStateRoot(), *id, *planPath)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkWorktree(args []string) error {
	fs := flag.NewFlagSet("issueops link-worktree", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	worktreePath := fs.String("worktree-path", "", "issue-driven worktree path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsWorktree(core.IssueOpsStateRoot(), *id, *worktreePath)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkChild(args []string) error {
	fs := flag.NewFlagSet("issueops link-child", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	childURL := fs.String("child-url", "", "GitHub sub-issue or GitLab child item URL")
	title := fs.String("title", "", "optional child issue title")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	if err := verifyIssueOpsChildIssueBeforeLink(*childURL); err != nil {
		return printIssueOpsResult(core.IssueOpsRecord{OK: false}, *jsonOut, err)
	}
	record, err := core.LinkIssueOpsChild(core.IssueOpsStateRoot(), *id, *childURL, *title)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsLinkRelated(args []string) error {
	fs := flag.NewFlagSet("issueops link-related", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	linkType := fs.String("type", "", "link type: depends-on, blocks, supersedes, follows-up, duplicates, splits-from, implements")
	relatedURL := fs.String("related-url", "", "related issue URL")
	title := fs.String("title", "", "optional related issue title")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.LinkIssueOpsRelated(core.IssueOpsStateRoot(), *id, *linkType, *relatedURL, *title)
	return printIssueOpsResult(record, *jsonOut, err)
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
	phase := fs.String("phase", "", "lifecycle phase at which the skill fired")
	skill := fs.String("skill", "", "skill that fired (codd, dijkstra, hopper, shannon, ...)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.RecordIssueOpsRouting(core.IssueOpsStateRoot(), *id, *phase, *skill)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsPhase(args []string) error {
	fs := flag.NewFlagSet("issueops phase", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
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
		record, err = core.AdvanceIssueOpsPhase(core.IssueOpsStateRoot(), *id, *to)
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
		readiness = core.IssueOpsStrictPRReadiness(record)
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
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ForceReleaseIssueOps(core.IssueOpsStateRoot(), *id, *reason)
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsResume(args []string) error {
	fs := flag.NewFlagSet("issueops resume", flag.ContinueOnError)
	repo := fs.String("repo", "", "repository path")
	bind := fs.Bool("bind", false, "bind the session to the resumed cycle")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result := core.IssueOpsResume(*repo)
	if *bind && result.OK && result.Bound {
		if err := core.BindIssueOpsSession(*repo, result.CycleID, result.Branch, result.WorktreePath); err != nil {
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
