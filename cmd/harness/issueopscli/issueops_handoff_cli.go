package issueopscli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/orca"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/preflight"
)

const issueOpsHandoffUsage = `Usage:
  agent-harness issueops handoff start --id ID --coordinator-recipient TERM --coordinator-host HOST --coordinator-session-id SESSION --source-cwd PATH [--coordinator-agent-id ID] [--workspace-epoch EPOCH] [--allow-codex-hook-trust-bypass] [--expected-context-sha256 SHA] [--confirm] [--json]
  agent-harness issueops handoff claim --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --host HOST --session-id SESSION --cwd PATH --orca-worktree-id ID [--agent-id ID] [--json]
  agent-harness issueops handoff acknowledge-context --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --host HOST --session-id SESSION --cwd PATH --issue-url URL --plan-sha256 SHA --understanding TEXT --scope-confirmation TEXT [--agent-id ID] [--json]
  agent-harness issueops handoff finish --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --host HOST --session-id SESSION --outcome completed|failed [evidence flags] [--no-change --verification RESULT] [--json]
  agent-harness issueops handoff accept --id ID --attempt N --ownership-epoch EPOCH --context-sha256 SHA --final-head SHA --host HOST --session-id SESSION --source-cwd PATH [--agent-id ID] [--json]
  agent-harness issueops handoff publish --id ID --host HOST --session-id SESSION [--cwd WORKER_PATH|--source-cwd SOURCE_PATH] [--agent-id ID] [--approve-legacy-coordinator-seal] --confirm [--json]
  agent-harness issueops handoff codex-hooks-list --id ID --json
  agent-harness issueops handoff recover --id ID --action reconcile|abandon|cancel|finalize-cancel|retry|approve-cleanup|record-cleanup [--cleanup-disposition retry|remove] [--cleanup-step STEP] [--confirm] [--force --reason TEXT] [--json]`

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
	case "acknowledge-context":
		return runIssueOpsHandoffAcknowledgeContext(args[1:])
	case "finish":
		return runIssueOpsHandoffFinish(args[1:])
	case "accept":
		return runIssueOpsHandoffAccept(args[1:])
	case "publish":
		return runIssueOpsHandoffPublish(args[1:])
	case "codex-hooks-list":
		return runIssueOpsHandoffCodexHooksList(args[1:])
	case "recover":
		return runIssueOpsHandoffRecover(args[1:])
	default:
		return fmt.Errorf("unknown issueops handoff subcommand %q", args[0])
	}
}

func runIssueOpsHandoffPublish(args []string) error {
	fs := flag.NewFlagSet("issueops handoff publish", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	host := fs.String("host", "", "native actor host")
	sessionID := fs.String("session-id", "", "native actor session id")
	agentID := fs.String("agent-id", "", "native actor agent id")
	sourceCWD := fs.String("source-cwd", "", "exact source checkout cwd")
	cwd := fs.String("cwd", "", "canonical worker cwd for ownership-transfer publication")
	approveLegacyCoordinatorSeal := fs.Bool("approve-legacy-coordinator-seal", false, "explicitly seal the current source coordinator identity while re-attesting a genuine schema-v5 publication")
	confirm := fs.Bool("confirm", false, "verify and persist the exact local and remote final head")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.RecordIssueOpsHandoffPublishReceipt(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffPublishRequest{
		ID: *id, Host: *host, SessionID: *sessionID, AgentID: *agentID, SourceCWD: *sourceCWD, CWD: *cwd, Confirm: *confirm, ApproveLegacyCoordinatorSeal: *approveLegacyCoordinatorSeal,
	}, core.GitIssueOpsHandoffPublicationReader{}, orca.New(), core.IssueOpsHandoffPrepareClock{})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsHandoffStart(args []string) error {
	fs := flag.NewFlagSet("issueops handoff start", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	coordinatorRecipient := fs.String("coordinator-recipient", "", "sealed concrete Orca coordinator mailbox recipient")
	coordinatorHost := fs.String("coordinator-host", "", "native coordinator host")
	coordinatorSessionID := fs.String("coordinator-session-id", "", "native coordinator session id")
	coordinatorAgentID := fs.String("coordinator-agent-id", "", "native coordinator agent id")
	sourceCWD := fs.String("source-cwd", "", "exact source checkout cwd")
	workspaceEpoch := fs.String("workspace-epoch", "", "sealed ready workspace epoch for ownership transfer")
	confirm := fs.Bool("confirm", false, "confirm terminal, task, and dispatch mutations")
	allowCodexHookTrustBypass := fs.Bool("allow-codex-hook-trust-bypass", false, "attest that the documented Codex hooks/list trust review passed")
	expectedContextSHA256 := fs.String("expected-context-sha256", "", "reviewed sealed context sha256")
	jsonOut := fs.Bool("json", false, "print JSON")
	var criteria, docs, skills, verification, stops repeatedFlag
	fs.Var(&criteria, "criteria-id", "criterion id included in the worker packet")
	fs.Var(&docs, "required-doc", "required worker document")
	fs.Var(&skills, "required-skill", "required worker skill")
	fs.Var(&verification, "verification", "worker verification command")
	fs.Var(&verification, "verification-command", "legacy alias for worker verification command")
	fs.Var(&stops, "stop-condition", "worker stop condition")
	scope := fs.String("worker-scope", "", "bounded worker scope")
	heartbeat := fs.String("heartbeat-cadence", "", "worker heartbeat cadence")
	resultFormat := fs.String("result-format", "", "worker result contract")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.StartIssueOpsHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffStartRequest{
		ID: *id, CoordinatorRecipient: *coordinatorRecipient, Confirm: *confirm, ExpectedContextSHA256: *expectedContextSHA256,
		CoordinatorHost: *coordinatorHost, CoordinatorSessionID: *coordinatorSessionID, CoordinatorAgentID: *coordinatorAgentID, SourceCWD: *sourceCWD, WorkspaceEpoch: *workspaceEpoch,
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

func runIssueOpsHandoffAcknowledgeContext(args []string) error {
	fs := flag.NewFlagSet("issueops handoff acknowledge-context", flag.ContinueOnError)
	common := addIssueOpsHandoffFenceFlags(fs)
	host := fs.String("host", "", "native host")
	sessionID := fs.String("session-id", "", "native session id")
	agentID := fs.String("agent-id", "", "native agent id")
	cwd := fs.String("cwd", "", "canonical worker cwd")
	issueURL := fs.String("issue-url", "", "exact IssueOps issue URL")
	planSHA256 := fs.String("plan-sha256", "", "exact linked plan SHA-256")
	understanding := fs.String("understanding", "", "bounded owner understanding")
	scopeConfirmation := fs.String("scope-confirmation", "", "bounded owner scope confirmation")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.AcknowledgeIssueOpsHandoffContext(core.IssueOpsStateRoot(), core.IssueOpsHandoffAcknowledgeRequest{
		ID: *common.id, Attempt: *common.attempt, OwnershipEpoch: *common.epoch, ContextSHA256: *common.context,
		Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, IssueURL: *issueURL, PlanSHA256: *planSHA256,
		Understanding: *understanding, ScopeConfirmation: *scopeConfirmation,
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
	noChange := fs.Bool("no-change", false, "derive sealed completion evidence for a clean verification-only worker")
	var changedFiles, verification, cleanup repeatedFlag
	fs.Var(&changedFiles, "changed-file", "changed file path")
	fs.Var(&verification, "verification", "verification command/result")
	fs.Var(&cleanup, "cleanup-receipt", "worker-created temporary resource cleanup receipt")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	req := core.IssueOpsHandoffFinishRequest{
		ID: *common.id, Attempt: *common.attempt, OwnershipEpoch: *common.epoch, ContextSHA256: *common.context,
		Host: *host, SessionID: *sessionID, AgentID: *agentID, Outcome: *outcome, FinalHead: *finalHead,
		ChangedFiles: changedFiles, TuringReportPath: *turingReport, Verification: verification, CleanupReceipts: cleanup,
		EvidenceDigest: *evidenceDigest, TaskID: *taskID, DispatchID: *dispatchID,
	}
	if *noChange {
		current, err := core.ReadIssueOps(core.IssueOpsStateRoot(), req.ID)
		if err != nil {
			return err
		}
		req, err = prepareNoChangeHandoffFinish(current, req)
		if err != nil {
			return err
		}
	}
	record, err := core.FinishIssueOpsHandoffWithProjection(context.Background(), core.IssueOpsStateRoot(), req, issueOpsWorkerDoneProjectionClient())
	return printIssueOpsResult(record, *jsonOut, err)
}

func prepareNoChangeHandoffFinish(record core.IssueOpsRecord, req core.IssueOpsHandoffFinishRequest) (core.IssueOpsHandoffFinishRequest, error) {
	if record.ExecutionHandoff == nil || record.ExecutionHandoff.Orca == nil {
		return req, fmt.Errorf("no-change finish requires a dispatched handoff")
	}
	if req.Outcome != "" && req.Outcome != handoff.OutcomeCompleted {
		return req, fmt.Errorf("no-change finish requires completed outcome")
	}
	if len(req.ChangedFiles) != 0 {
		return req, fmt.Errorf("no-change finish must not include changed files")
	}
	if strings.TrimSpace(req.TuringReportPath) != "" {
		return req, fmt.Errorf("no-change finish derives the sealed plan evidence path")
	}
	if len(req.CleanupReceipts) != 0 {
		return req, fmt.Errorf("no-change finish derives the no-temp cleanup receipt")
	}
	if len(req.Verification) == 0 || strings.TrimSpace(req.Verification[0]) == "" {
		return req, fmt.Errorf("no-change finish requires verification evidence")
	}
	workerRoot := strings.TrimSpace(record.WorktreePath)
	planPath := strings.TrimSpace(record.PlanPath)
	if workerRoot == "" || planPath == "" {
		return req, fmt.Errorf("no-change finish requires sealed worker root and plan path")
	}
	relativePlan, err := filepath.Rel(workerRoot, planPath)
	if err != nil || filepath.IsAbs(relativePlan) || relativePlan == "." || relativePlan == ".." || strings.HasPrefix(filepath.ToSlash(relativePlan), "../") {
		return req, fmt.Errorf("no-change finish requires the sealed plan inside the worker root")
	}
	info, err := os.Lstat(planPath)
	if err != nil || info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return req, fmt.Errorf("no-change finish requires a regular sealed plan evidence file")
	}
	code, head, stderr := preflight.GitCmd(workerRoot, "rev-parse", "--verify", "HEAD^{commit}")
	if code != 0 {
		return req, fmt.Errorf("no-change finish cannot read worker HEAD: %s", strings.TrimSpace(stderr))
	}
	head = strings.TrimSpace(head)
	if head == "" || head != strings.TrimSpace(record.ExecutionHandoff.AttemptBaseHead) {
		return req, fmt.Errorf("no-change finish requires worker HEAD to match the attempt base head")
	}
	if strings.TrimSpace(req.FinalHead) != "" && strings.TrimSpace(req.FinalHead) != head {
		return req, fmt.Errorf("no-change finish final head differs from the worker HEAD")
	}
	if strings.TrimSpace(req.TaskID) != "" && strings.TrimSpace(req.TaskID) != record.ExecutionHandoff.Orca.TaskID {
		return req, fmt.Errorf("no-change finish task id differs from the sealed handoff")
	}
	if strings.TrimSpace(req.DispatchID) != "" && strings.TrimSpace(req.DispatchID) != record.ExecutionHandoff.Orca.DispatchID {
		return req, fmt.Errorf("no-change finish dispatch id differs from the sealed handoff")
	}
	req.Outcome = handoff.OutcomeCompleted
	req.FinalHead = head
	req.TuringReportPath = filepath.ToSlash(filepath.Clean(relativePlan))
	req.CleanupReceipts = []string{"no worker-created temporary resources"}
	req.TaskID = record.ExecutionHandoff.Orca.TaskID
	req.DispatchID = record.ExecutionHandoff.Orca.DispatchID
	return req, nil
}

func runIssueOpsHandoffAccept(args []string) error {
	fs := flag.NewFlagSet("issueops handoff accept", flag.ContinueOnError)
	common := addIssueOpsHandoffFenceFlags(fs)
	finalHead := fs.String("final-head", "", "accepted final git head")
	host := fs.String("host", "", "native coordinator host")
	sessionID := fs.String("session-id", "", "native coordinator session id")
	agentID := fs.String("agent-id", "", "native coordinator agent id")
	sourceCWD := fs.String("source-cwd", "", "exact source checkout cwd")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	record, err := core.AcceptIssueOpsHandoff(core.IssueOpsStateRoot(), core.IssueOpsHandoffAcceptRequest{
		ID: *common.id, Attempt: *common.attempt, OwnershipEpoch: *common.epoch, ContextSHA256: *common.context, FinalHead: *finalHead,
		Host: *host, SessionID: *sessionID, AgentID: *agentID, SourceCWD: *sourceCWD,
	})
	return printIssueOpsResult(record, *jsonOut, err)
}

func runIssueOpsHandoffRecover(args []string) error {
	fs := flag.NewFlagSet("issueops handoff recover", flag.ContinueOnError)
	id := fs.String("id", "", "issueops id")
	action := fs.String("action", "", "reconcile, abandon, cancel, finalize-cancel, retry, approve-cleanup, or record-cleanup")
	confirm := fs.Bool("confirm", false, "confirm abandonment, cancellation finalization, or retry")
	force := fs.Bool("force", false, "force an authoritative abandon or claimed-worker cancellation")
	reason := fs.String("reason", "", "bounded durable reason for abandonment or forced cancellation")
	cleanupDisposition := fs.String("cleanup-disposition", "", "approved cleanup disposition: retry or remove")
	cleanupStep := fs.String("cleanup-step", "", "ordered cleanup receipt: task_terminal, terminal_quiescent, or worktree_removed")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	result, err := core.RecoverIssueOpsHandoff(context.Background(), core.IssueOpsStateRoot(), core.IssueOpsHandoffRecoverRequest{
		ID: *id, Action: *action, Confirm: *confirm, Force: *force, Reason: *reason,
		CleanupDisposition: *cleanupDisposition, CleanupStep: *cleanupStep,
	}, orca.New(), core.IssueOpsHandoffPrepareClock{})
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
