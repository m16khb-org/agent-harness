package issueopscli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	issueopsroutingcontract "agent-harness/internal/contract/issueopsrouting"
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
	record, err := issueOpsCLIDeps.StartIssueOps(issueOpsCLIDeps.IssueOpsStateRoot(), issueopscontract.IssueOpsStartRequest{Repo: *repo, Branch: *branch})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsStatus(args []string) error {
	fs := flag.NewFlagSet("issueops status", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := issueOpsCLIDeps.IssueOpsStatus(issueOpsCLIDeps.IssueOpsStateRoot(), *id)
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
	record, err := issueOpsCLIDeps.LinkIssueOpsIssueWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *issueURL, actor.actor())
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
	record, err := issueOpsCLIDeps.LinkIssueOpsPlanWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *planPath, actor.actor())
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
	record, err := issueOpsCLIDeps.LinkIssueOpsWorktreeWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *worktreePath, actor.actor())
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
		return printIssueOpsResult(issueopscontract.IssueOpsRecord{OK: false}, *jsonOut, err)
	}
	record, err := issueOpsCLIDeps.LinkIssueOpsChildWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *childURL, *title, actor.actor())
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
	record, err := issueOpsCLIDeps.LinkIssueOpsRelatedWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *linkType, *relatedURL, *title, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsChild(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(issueOpsChildUsageText())
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
	result, err := issueOpsCLIDeps.StartIssueOpsChildWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), issueopscontract.IssueOpsChildStartRequest{
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
	result, err := issueOpsCLIDeps.IssueOpsChildStatusWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *parentID, *repair, actor.actor())
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
	result, err := issueOpsCLIDeps.AcceptIssueOpsChildWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *parentID, *childID, []string(evidence), actor.actor())
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
	result, err := issueOpsCLIDeps.RejectIssueOpsChildWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *parentID, *childID, *reason, nil, actor.actor())
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
	result, err := issueOpsCLIDeps.DropIssueOpsChildWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *parentID, *childID, *reason, actor.actor())
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
	result, observed, err := issueOpsCLIDeps.ScoreLiveRoutingFidelity(
		issueOpsCLIDeps.IssueOpsStateRoot(),
		*id,
		expected,
	)
	if err != nil {
		if *jsonOut {
			return printIssueOpsErrorJSON(err)
		}
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("routing fidelity: ok=%v (observed %d pairings)\n", result.OK, observed)
	for _, m := range result.Missing {
		fmt.Printf("- missing: %s@%s\n", m.Skill, m.Phase)
	}
	return nil
}

func parseExpectedRouting(spec string) ([]issueopsroutingcontract.Expected, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("--expect is required as comma-separated phase:skill pairings")
	}
	var out []issueopsroutingcontract.Expected
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
		out = append(out, issueopsroutingcontract.Expected{Phase: phase, Skill: skill})
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
	record, err := issueOpsCLIDeps.RecordIssueOpsRoutingWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *phase, *skill, actor.actor())
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsPhase(args []string) error {
	fs := flag.NewFlagSet("issueops phase", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	actor := addIssueOpsActorFlags(fs)
	to := fs.String("to", "", "target phase: problem, grill, plan, compatibility-review, implement, ai-slop-clean, feedback, pr, done")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	if strings.TrimSpace(*to) == "done" {
		err := fmt.Errorf("done is entered atomically by issueops execution complete")
		if *jsonOut {
			_ = printIssueOpsErrorJSON(err)
		}
		return err
	}
	record, err := advancePhaseWithActor(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *to, actor.actor())
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
	record, err := issueOpsCLIDeps.ReadIssueOps(issueOpsCLIDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		if *jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	readiness := issueOpsCLIDeps.IssueOpsPRReadiness(record)
	if *strict {
		readiness = strictPRReadinessWithState(issueOpsCLIDeps.IssueOpsStateRoot(), record)
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

// runIssueOpsArtifact는 코디네이터가 prepare 이전에 plan/spec/turing-loop
// artifact를 스테이징하는 진입점이다. materialize와 manifest 봉인은
// execution prepare가 소유한다(설계 v5 WS2).
func runIssueOpsArtifact(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage:\n  agent-harness issueops artifact stage --id ID --name plan|spec|turing-loop --file PATH [--json]\n  agent-harness issueops artifact unstage --id ID --name plan|spec|turing-loop [--json]")
		return nil
	}
	if args[0] == "unstage" {
		fs := flag.NewFlagSet("issueops artifact unstage", flag.ContinueOnError)
		id := fs.String("id", "", "issueops id")
		name := fs.String("name", "", "artifact name: plan|spec|turing-loop")
		jsonOut := fs.Bool("json", false, "print JSON")
		if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
			return err
		}
		record, err := issueOpsCLIDeps.UnstageIssueOpsArtifact(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *name)
		return printIssueOpsResult(record, *jsonOut, err)
	}
	if args[0] != "stage" {
		return fmt.Errorf("unknown issueops artifact subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops artifact stage", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	name := fs.String("name", "", "artifact name: plan|spec|turing-loop")
	file := fs.String("file", "", "artifact source file path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return fmt.Errorf("artifact stage requires --file")
	}
	content, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	record, err := issueOpsCLIDeps.StageIssueOpsArtifact(issueOpsCLIDeps.IssueOpsStateRoot(), *id, *name, content)
	if err != nil {
		if *jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	staged, err := issueOpsCLIDeps.StagedIssueOpsArtifactNames(issueOpsCLIDeps.IssueOpsStateRoot(), record.ID)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(map[string]any{"ok": true, "id": record.ID, "staged": staged})
	}
	fmt.Printf("staged artifacts for %s: %s\n", record.ID, strings.Join(staged, ", "))
	return nil
}

// runIssueOpsImplementationReview는 execution owner가 publication 전에
// planner급 brooks 리뷰 verdict를 기록하는 표면이다. reviewer 필드는 감사
// 기록이며 게이트는 verdict pass + 실질 내용만 본다(설계 v5 WS5).
func runIssueOpsImplementationReview(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: agent-harness issueops implementation-review record --id ID --verdict pass|revise|stop --finding TEXT... --evidence TEXT... [--reviewer-host codex|claude|omo] [--reviewer-model MODEL] [--reviewer-effort EFFORT] [--json]")
		return nil
	}
	if args[0] != "record" {
		return fmt.Errorf("unknown issueops implementation-review subcommand %q", args[0])
	}
	fs := flag.NewFlagSet("issueops implementation-review record", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	verdict := fs.String("verdict", "", "pass|revise|stop")
	var findings, evidence repeatedFlag
	fs.Var(&findings, "finding", "review finding (repeatable)")
	fs.Var(&evidence, "evidence", "review evidence (repeatable)")
	reviewerHost := fs.String("reviewer-host", "", "reviewer host (audit only)")
	reviewerModel := fs.String("reviewer-model", "", "reviewer model (audit only)")
	reviewerEffort := fs.String("reviewer-effort", "", "reviewer effort (audit only)")
	addIssueOpsActorFlags(fs)
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args[1:]); help || err != nil {
		return err
	}
	record, err := issueOpsCLIDeps.RecordIssueOpsImplementationReview(issueOpsCLIDeps.IssueOpsStateRoot(), *id, issueopscontract.IssueOpsImplementationReviewRequest{
		Verdict: *verdict, Findings: findings, Evidence: evidence,
		ReviewerHost: *reviewerHost, ReviewerModel: *reviewerModel, ReviewerEffort: *reviewerEffort,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

// runIssueOpsList는 다중 사이클 조망 표면이다. span lock·repair 없이 전량
// 읽고, scanned_records로 O(N) 비용을 관측 가능하게 한다(설계 v5 WS6).
func runIssueOpsList(args []string) error {
	fs := flag.NewFlagSet("issueops list", flag.ContinueOnError)
	repo := fs.String("repo", "", "filter cycles by repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := issueOpsCLIDeps.ListIssueOpsCycles(issueOpsCLIDeps.IssueOpsStateRoot(), *repo)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf(
		"cycles: %d (scanned %d records, unreadable %d at %s)\n",
		len(result.Entries),
		result.ScannedRecords,
		result.ReadErrors,
		result.GeneratedAt,
	)
	for _, entry := range result.Entries {
		flags := ""
		if entry.Claimable {
			flags += " [claimable]"
		}
		if entry.CleanupCandidate {
			flags += " [cleanup]"
		}
		if entry.CompletionUnreflected {
			flags += " [unreflected]"
		}
		if entry.PendingKind != "" {
			flags += " [pending:" + entry.PendingKind + "]"
		}
		if entry.FailureCode != "" {
			flags += " [failed:" + entry.FailureCode + "]"
		}
		if entry.CleanupFailureStep != "" {
			flags += " [cleanup-failed:" + entry.CleanupFailureStep + "]"
		}
		fmt.Printf("- %s %s phase=%s lease=%s%s\n", entry.ID, entry.Branch, entry.Phase, entry.LeaseStatus, flags)
	}
	return nil
}
