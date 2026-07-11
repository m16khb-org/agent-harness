package issueopscli

import (
	"context"
	"flag"
	"fmt"

	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
)

const issueOpsHandoffUsage = `Usage:
  agent-harness issueops handoff start --id ID [--allow-codex-hook-trust-bypass] [--expected-context-sha256 SHA] [--confirm] [--json]
  agent-harness issueops handoff claim --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --host HOST --session-id SESSION --cwd PATH --orca-worktree-id ID [--agent-id ID] [--json]
  agent-harness issueops handoff finish --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --host HOST --session-id SESSION --outcome completed|failed [evidence flags] [--json]
  agent-harness issueops handoff accept --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --final-head SHA [--json]
  agent-harness issueops handoff recover --id ID --action reconcile|abandon|cancel|finalize-cancel|retry [--confirm] [--force --reason TEXT] [--json]`

func runIssueOpsHandoff(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println(issueOpsHandoffUsage)
		return nil
	}
	switch args[0] {
	case "start":
		return runIssueOpsHandoffStart(args[1:])
	case "claim":
		return runIssueOpsHandoffClaim(args[1:])
	case "finish":
		return runIssueOpsHandoffFinish(args[1:])
	case "accept":
		return runIssueOpsHandoffAccept(args[1:])
	case "recover":
		return runIssueOpsHandoffRecover(args[1:])
	default:
		return fmt.Errorf("unknown issueops handoff subcommand %q", args[0])
	}
}

func runIssueOpsHandoffStart(args []string) error {
	fs := flag.NewFlagSet("issueops handoff start", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	confirm := fs.Bool("confirm", false, "confirm terminal, task, and dispatch mutations")
	allowCodexHookTrustBypass := fs.Bool("allow-codex-hook-trust-bypass", false, "attest that the documented Codex hooks/list trust review passed")
	expectedContextSHA256 := fs.String("expected-context-sha256", "", "reviewed sealed context sha256")
	jsonOut := fs.Bool("json", false, "print JSON")
	var criteria, docs, skills, verification, stops repeatedFlag
	fs.Var(&criteria, "criteria-id", "criterion id included in the worker packet")
	fs.Var(&docs, "required-doc", "required worker document")
	fs.Var(&skills, "required-skill", "required worker skill")
	fs.Var(&verification, "verification", "worker verification command")
	fs.Var(&stops, "stop-condition", "worker stop condition")
	scope := fs.String("worker-scope", "", "bounded worker scope")
	heartbeat := fs.String("heartbeat-cadence", "", "worker heartbeat cadence")
	resultFormat := fs.String("result-format", "", "worker result contract")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.StartIssueOpsHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffStartRequest{
		ID: *id, Confirm: *confirm, ExpectedContextSHA256: *expectedContextSHA256,
		Context: handoff.ContextOptions{
			CriteriaIDs: criteria, RequiredDocs: docs, RequiredSkills: skills, WorkerScope: *scope,
			VerificationCommands: verification, HeartbeatCadence: *heartbeat, StopConditions: stops, ResultFormat: *resultFormat,
			AllowCodexHookTrustBypass: *allowCodexHookTrustBypass,
		},
	}, orca.New(), core.IssueOpsHandoffStartClock{})
	return printIssueOpsHandoffValue(result, *jsonOut, err)
}

func runIssueOpsHandoffClaim(args []string) error {
	fs := flag.NewFlagSet("issueops handoff claim", flag.ContinueOnError)
	common := addIssueOpsHandoffFenceFlags(fs)
	host := fs.String("host", "", "native host")
	sessionID := fs.String("session-id", "", "native session id")
	agentID := fs.String("agent-id", "", "native agent id")
	cwd := fs.String("cwd", "", "canonical worker cwd")
	worktreeID := fs.String("orca-worktree-id", "", "Orca worktree id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.ClaimIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffClaimRequest{
		ID: *common.id, Attempt: *common.attempt, OwnershipEpoch: *common.epoch, ContextSHA256: *common.context,
		Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, OrcaWorktreeID: *worktreeID,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsHandoffFinish(args []string) error {
	fs := flag.NewFlagSet("issueops handoff finish", flag.ContinueOnError)
	common := addIssueOpsHandoffFenceFlags(fs)
	host := fs.String("host", "", "native host")
	sessionID := fs.String("session-id", "", "native session id")
	agentID := fs.String("agent-id", "", "native agent id")
	outcome := fs.String("outcome", "", "completed or failed")
	finalHead := fs.String("final-head", "", "final git head")
	turingReport := fs.String("turing-report", "", "Turing evidence report path")
	evidenceDigest := fs.String("evidence-digest", "", "optional evidence digest")
	taskID := fs.String("task-id", "", "Orca task id")
	dispatchID := fs.String("dispatch-id", "", "Orca dispatch id")
	var changedFiles, verification, cleanup repeatedFlag
	fs.Var(&changedFiles, "changed-file", "changed file path")
	fs.Var(&verification, "verification", "verification command/result")
	fs.Var(&cleanup, "cleanup-receipt", "worker-created temporary resource cleanup receipt")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.FinishIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffFinishRequest{
		ID: *common.id, Attempt: *common.attempt, OwnershipEpoch: *common.epoch, ContextSHA256: *common.context,
		Host: *host, SessionID: *sessionID, AgentID: *agentID, Outcome: *outcome, FinalHead: *finalHead,
		ChangedFiles: changedFiles, TuringReportPath: *turingReport, Verification: verification, CleanupReceipts: cleanup,
		EvidenceDigest: *evidenceDigest, TaskID: *taskID, DispatchID: *dispatchID,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsHandoffAccept(args []string) error {
	fs := flag.NewFlagSet("issueops handoff accept", flag.ContinueOnError)
	common := addIssueOpsHandoffFenceFlags(fs)
	finalHead := fs.String("final-head", "", "accepted final git head")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.AcceptIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffAcceptRequest{
		ID: *common.id, Attempt: *common.attempt, OwnershipEpoch: *common.epoch, ContextSHA256: *common.context, FinalHead: *finalHead,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsHandoffRecover(args []string) error {
	fs := flag.NewFlagSet("issueops handoff recover", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	action := fs.String("action", "", "reconcile, abandon, cancel, finalize-cancel, or retry")
	confirm := fs.Bool("confirm", false, "confirm abandonment, cancellation finalization, or retry")
	force := fs.Bool("force", false, "force an authoritative abandon or claimed-worker cancellation")
	reason := fs.String("reason", "", "bounded durable reason for abandonment or forced cancellation")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.RecoverIssueOpsHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffRecoverRequest{ID: *id, Action: *action, Confirm: *confirm, Force: *force, Reason: *reason}, orca.New(), core.IssueOpsHandoffPrepareClock{})
	return printIssueOpsHandoffValue(result, *jsonOut, err)
}

type issueOpsHandoffFenceFlags struct {
	id      *string
	attempt *int
	epoch   *string
	context *string
}

func addIssueOpsHandoffFenceFlags(fs *flag.FlagSet) issueOpsHandoffFenceFlags {
	return issueOpsHandoffFenceFlags{
		id: fs.String("id", "", "issueops id"), attempt: fs.Int("attempt", 0, "handoff attempt"),
		epoch: fs.String("ownership-epoch", "", "handoff ownership epoch"), context: fs.String("context-sha256", "", "handoff context sha256"),
	}
}

func printIssueOpsHandoffValue(value any, jsonOut bool, err error) error {
	if err != nil {
		if jsonOut {
			_ = printIssueOpsErrorJSON(err)
		}
		return err
	}
	if jsonOut {
		return printJSON(value)
	}
	fmt.Printf("%+v\n", value)
	return nil
}
