package executioncmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

type Deps struct {
	StateRoot  func() string
	Direct     port.ExecutionWorkspaceProvisioner
	Orca       port.ExecutionOrcaProvisioner
	ReadIssue  issueops.ExecutionIssueSnapshotReadFunc
	RemotePR   issueops.RemotePullRequestDependencies
	PrintJSON  func(any) error
	PrintError func(error) error
}

func (deps Deps) actionDeps() issueops.ExecutionActionDependencies {
	actionDeps := issueops.ExecutionActionDependencies{Direct: deps.Direct, Orca: deps.Orca, ReadIssue: deps.ReadIssue, RemotePR: deps.RemotePR}
	if inspector, ok := deps.Orca.(port.ExecutionOrcaOwnerInspector); ok {
		actionDeps.OrcaOwner = inspector
	}
	return actionDeps
}

func execute(req issueops.ExecutionActionRequest, deps Deps) (any, error) {
	if deps.StateRoot == nil {
		return nil, fmt.Errorf("IssueOps state root is unavailable")
	}
	return issueops.ExecuteExecution(context.Background(), deps.StateRoot(), req, deps.actionDeps())
}

const Usage = `Usage:
  agent-harness issueops execution prepare --id ID --mode auto|direct|orca --owner-host codex|claude [--owner-model MODEL] [--owner-effort EFFORT] ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops execution status --id ID [--json]
  agent-harness issueops execution whoami [--json]
  agent-harness issueops execution claim --id ID --generation N --claim-token-file PATH [--issue-body-sha256 HEX --context-packet-sha256 HEX] ACTOR_FLAGS [--json]
  agent-harness issueops execution release --id ID --generation N ACTOR_FLAGS [--json]
  agent-harness issueops execution replace --id ID --expected-generation N (--preview|--revoke|--finalize-preview|--finalize|--reseed) [fingerprint/reason flags] ACTOR_FLAGS [--confirm] [--json]
  agent-harness issueops execution reconcile --id ID (--preview|--confirm) ACTOR_FLAGS [--json]
  agent-harness issueops execution complete --id ID --generation N --final-head SHA --turing-report PATH --remote-artifact-url URL --verification TEXT... ACTOR_FLAGS --confirm [--json]
  agent-harness issueops execution sync-base --id ID (--preview | --apply --confirm --fingerprint SHA256 | --finalize | --abort) ACTOR_FLAGS [--json]

ACTOR_FLAGS: --host codex|claude --session-id ID [--agent-id ID] --session-pid PID --session-started-at RFC3339 --session-executable PATH --cwd PATH`

func Run(args []string, deps Deps) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Println(Usage)
		return nil
	}
	switch args[0] {
	case "prepare":
		return runPrepare(args[1:], deps)
	case "status":
		return runStatus(args[1:], deps)
	case "whoami":
		return runWhoami(args[1:], deps)
	case "claim":
		return runClaim(args[1:], deps)
	case "release":
		return runRelease(args[1:], deps)
	case "replace":
		return runReplace(args[1:], deps)
	case "reconcile":
		return runReconcile(args[1:], deps)
	case "complete":
		return runComplete(args[1:], deps)
	case "sync-base":
		return runSyncBase(args[1:], deps)
	default:
		return fmt.Errorf("unknown issueops execution subcommand %q", args[0])
	}
}

type actorFlags struct {
	host, sessionID, agentID, startedAt, executable, cwd *string
	pid                                                  *int
}

func addActorFlags(fs *flag.FlagSet) actorFlags {
	return actorFlags{
		host:       fs.String("host", "", "native host: codex or claude"),
		sessionID:  fs.String("session-id", "", "native session id"),
		agentID:    fs.String("agent-id", "", "optional native agent id"),
		pid:        fs.Int("session-pid", 0, "native session process id"),
		startedAt:  fs.String("session-started-at", "", "native session process start identity"),
		executable: fs.String("session-executable", "", "native session executable identity"),
		cwd:        fs.String("cwd", "", "source or canonical worktree cwd"),
	}
}

func (flags actorFlags) actor() model.NativeActor {
	ancestry, _ := issueops.ObserveNativeProcessAncestry(os.Getpid())
	return model.NativeActor{
		Host: strings.TrimSpace(*flags.host), SessionID: strings.TrimSpace(*flags.sessionID), AgentID: strings.TrimSpace(*flags.agentID),
		SessionProcess:  &model.NativeProcessReceipt{PID: *flags.pid, StartedAt: strings.TrimSpace(*flags.startedAt), Executable: strings.TrimSpace(*flags.executable)},
		ProcessAncestry: ancestry,
	}
}

func runPrepare(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution prepare", flag.ContinueOnError)
	id, mode := fs.String("id", "", "IssueOps id"), fs.String("mode", "auto", "auto, direct, orca")
	ownerHost, ownerModel := fs.String("owner-host", "", "owner host"), fs.String("owner-model", "", "owner model")
	ownerEffort := fs.String("owner-effort", "", "owner effort")
	actor := addActorFlags(fs)
	confirm, jsonOut := fs.Bool("confirm", false, "confirm mutations"), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	result, err := execute(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionPrepare, ID: *id, Mode: *mode, Actor: actor.actor(), CWD: *actor.cwd,
		OwnerHost: *ownerHost, OwnerModel: *ownerModel, OwnerEffort: *ownerEffort, Confirm: *confirm,
	}, deps)
	return output(result, *jsonOut, err, deps)
}

func runStatus(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution status", flag.ContinueOnError)
	id, jsonOut := fs.String("id", "", "IssueOps id"), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	result, err := execute(issueops.ExecutionActionRequest{Action: issueops.ExecutionActionStatus, ID: *id}, deps)
	return output(result, *jsonOut, err, deps)
}

// ExecutionWhoamiResult는 호출 프로세스의 native ancestry receipt를 노출한다.
// owner가 claim identity를 shell 확장 없이 리터럴 값으로 채우기 위한 read-only
// 표면이다(이슈 #90 발견 3 — 확장이 섞인 claim은 exact 파싱이 fail-closed로
// 거부하므로, 관측 가능한 대체 경로가 없으면 부트스트랩이 교착한다).
type ExecutionWhoamiResult struct {
	OK              bool                         `json:"ok"`
	Ancestry        []model.NativeProcessReceipt `json:"ancestry"`
	ClaimActorFlags []string                     `json:"claim_actor_flags"`
}

func runWhoami(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution whoami", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	ancestry, err := issueops.ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		return output(nil, *jsonOut, err, deps)
	}
	result := ExecutionWhoamiResult{OK: true, Ancestry: ancestry}
	for _, receipt := range ancestry {
		result.ClaimActorFlags = append(result.ClaimActorFlags, fmt.Sprintf(
			"--session-pid %d --session-started-at %s --session-executable '%s'",
			receipt.PID, receipt.StartedAt, receipt.Executable))
	}
	return output(result, *jsonOut, nil, deps)
}

func runClaim(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution claim", flag.ContinueOnError)
	id, generation := fs.String("id", "", "IssueOps id"), fs.Uint64("generation", 0, "lease generation")
	claimTokenFile, actor := fs.String("claim-token-file", "", "one-time claim token file"), addActorFlags(fs)
	issueDigest := fs.String("issue-body-sha256", "", "sealed remote issue body SHA-256")
	packetDigest := fs.String("context-packet-sha256", "", "sealed owner context packet SHA-256")
	jsonOut := fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	result, err := execute(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionClaim, ID: *id, Generation: *generation,
		Actor: actor.actor(), CWD: *actor.cwd, TokenFile: *claimTokenFile,
		IssueBodySHA256: *issueDigest, ContextPacketSHA256: *packetDigest,
	}, deps)
	return output(result, *jsonOut, err, deps)
}

func runRelease(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution release", flag.ContinueOnError)
	id, generation := fs.String("id", "", "IssueOps id"), fs.Uint64("generation", 0, "lease generation")
	actor, jsonOut := addActorFlags(fs), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	result, err := execute(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionRelease, ID: *id, Generation: *generation, Actor: actor.actor(), CWD: *actor.cwd,
	}, deps)
	return output(result, *jsonOut, err, deps)
}

func runReplace(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution replace", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	generation := fs.Uint64("expected-generation", 0, "expected lease generation")
	inventory := fs.String("inventory-fingerprint", "", "preview inventory fingerprint")
	quiescence := fs.String("quiescence-fingerprint", "", "finalize-preview fingerprint")
	reason := fs.String("reason", "", "replacement reason")
	preview, revoke := fs.Bool("preview", false, "preview replacement"), fs.Bool("revoke", false, "revoke current generation")
	finalizePreview, finalize := fs.Bool("finalize-preview", false, "preview finalization"), fs.Bool("finalize", false, "finalize replacement")
	reseed, confirm := fs.Bool("reseed", false, "reseed a holderless lease"), fs.Bool("confirm", false, "confirm mutation")
	actor, jsonOut := addActorFlags(fs), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	actions := map[string]bool{
		issueops.ExecutionReplacePreview: *preview, issueops.ExecutionReplaceRevoke: *revoke,
		issueops.ExecutionReplaceFinalizePreview: *finalizePreview, issueops.ExecutionReplaceFinalize: *finalize,
		issueops.ExecutionReplaceReseed: *reseed,
	}
	action := ""
	for candidate, selected := range actions {
		if selected {
			if action != "" {
				return output(nil, *jsonOut, fmt.Errorf("execution replace requires exactly one action"), deps)
			}
			action = candidate
		}
	}
	if action == "" {
		return output(nil, *jsonOut, fmt.Errorf("execution replace requires exactly one action"), deps)
	}
	result, err := execute(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReplace, ID: *id, ReplaceAction: action, ExpectedGeneration: *generation,
		InventoryFingerprint: *inventory, QuiescenceFingerprint: *quiescence, Reason: *reason,
		Actor: actor.actor(), CWD: *actor.cwd, Confirm: *confirm,
	}, deps)
	return output(result, *jsonOut, err, deps)
}

func runReconcile(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution reconcile", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	preview, confirm := fs.Bool("preview", false, "preview reconciliation"), fs.Bool("confirm", false, "confirm reconciliation")
	actor, jsonOut := addActorFlags(fs), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	result, err := execute(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReconcile, ID: *id, Preview: *preview, Confirm: *confirm,
		Actor: actor.actor(), CWD: *actor.cwd,
	}, deps)
	return output(result, *jsonOut, err, deps)
}

type repeatedString []string

func (values *repeatedString) String() string { return strings.Join(*values, ",") }
func (values *repeatedString) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func runComplete(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution complete", flag.ContinueOnError)
	id, generation := fs.String("id", "", "IssueOps id"), fs.Uint64("generation", 0, "lease generation")
	finalHead, report := fs.String("final-head", "", "final Git HEAD"), fs.String("turing-report", "", "Turing report path")
	remoteURL := fs.String("remote-artifact-url", "", "draft PR or MR URL")
	verification := repeatedString{}
	fs.Var(&verification, "verification", "verification evidence (repeatable)")
	actor := addActorFlags(fs)
	confirm, jsonOut := fs.Bool("confirm", false, "confirm completion and release"), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	result, err := execute(issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionComplete, ID: *id, Generation: *generation, Actor: actor.actor(), CWD: *actor.cwd,
		FinalHead: *finalHead, TuringReportPath: *report, Verification: verification,
		RemoteArtifactURL: *remoteURL, Confirm: *confirm,
	}, deps)
	return output(result, *jsonOut, err, deps)
}

// runSyncBase는 completion 이후 base 재동기화를 실행한다. ExecuteExecution을
// 거치지 않고 core를 직접 호출하는 것이 계약이다 — sync-base는 CLI 전용
// 표면이고 MCP action 카탈로그·mcp golden은 변경하지 않는다(설계 v2 F15).
func runSyncBase(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops execution sync-base", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	fingerprint := fs.String("fingerprint", "", "fingerprint issued by the latest --preview")
	preview := fs.Bool("preview", false, "observe base divergence and issue a fingerprint")
	apply := fs.Bool("apply", false, "merge the fetched base into the work branch and push")
	finalize := fs.Bool("finalize", false, "commit and push a resolved merge")
	abort := fs.Bool("abort", false, "withdraw the in-progress merge")
	confirm := fs.Bool("confirm", false, "confirm the apply mutation")
	actor, jsonOut := addActorFlags(fs), fs.Bool("json", false, "print JSON")
	if done, err := parse(fs, args); done || err != nil {
		return err
	}
	modes := map[string]bool{
		issueops.ExecutionSyncBasePreview:  *preview,
		issueops.ExecutionSyncBaseApply:    *apply,
		issueops.ExecutionSyncBaseFinalize: *finalize,
		issueops.ExecutionSyncBaseAbort:    *abort,
	}
	mode := ""
	for candidate, selected := range modes {
		if selected {
			if mode != "" {
				return output(nil, *jsonOut, fmt.Errorf("execution sync-base requires exactly one mode"), deps)
			}
			mode = candidate
		}
	}
	if mode == "" {
		return output(nil, *jsonOut, fmt.Errorf("execution sync-base requires exactly one mode"), deps)
	}
	if deps.StateRoot == nil {
		return output(nil, *jsonOut, fmt.Errorf("IssueOps state root is unavailable"), deps)
	}
	result, err := issueops.SyncExecutionBase(context.Background(), deps.StateRoot(), issueops.ExecutionSyncBaseRequest{
		ID: *id, Mode: mode, Actor: actor.actor(), CWD: *actor.cwd, Confirm: *confirm, Fingerprint: *fingerprint,
	}, issueops.ExecutionSyncBaseDeps{})
	return output(result, *jsonOut, err, deps)
}

func output(value any, jsonOut bool, err error, deps Deps) error {
	if err != nil {
		if jsonOut && deps.PrintError != nil {
			_ = deps.PrintError(err)
		}
		return err
	}
	if jsonOut && deps.PrintJSON != nil {
		return deps.PrintJSON(value)
	}
	printText(value)
	return nil
}

func printText(value any) {
	switch result := value.(type) {
	case issueops.ExecutionPrepareResult:
		fmt.Printf("%s %s %s generation=%d\n", result.ID, result.ResolvedMode, result.Workspace.Root, executionGeneration(result.Execution))
	case issueops.ExecutionResult:
		fmt.Printf("%s %s generation=%d\n", result.ID, result.Execution.Lease.Status, result.Execution.Lease.Generation)
	case issueops.ExecutionReplaceResult:
		fmt.Printf("%s %s generation=%d next=%s\n", result.ID, result.Execution.Lease.Status, result.Execution.Lease.Generation, result.NextCommand)
	case issueops.ExecutionReconcileResult:
		fmt.Printf("%s %s pending=%t\n", result.ID, result.Code, result.Pending != nil)
	case issueops.ExecutionSyncBaseResult:
		fmt.Printf("%s %s merged=%t pushed=%t conflicts=%d next=%s\n",
			result.ID, result.Mode, result.Merged, result.Pushed, len(result.ConflictFiles), result.NextCommand)
	default:
		fmt.Printf("%v\n", value)
	}
}

func executionGeneration(execution *model.Execution) uint64 {
	if execution == nil {
		return 0
	}
	return execution.Lease.Generation
}

func parse(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func isHelp(value string) bool { return value == "help" || value == "-h" || value == "--help" }
