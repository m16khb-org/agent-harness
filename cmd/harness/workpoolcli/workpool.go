package workpoolcli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"agent-harness/internal/core"
)

type repeatedFlag []string

func (r *repeatedFlag) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprint([]string(*r))
}

func (r *repeatedFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		workpoolUsage()
		return nil
	}
	switch args[0] {
	case "create":
		return runCreate(args[1:])
	case "add-task":
		return runAddTask(args[1:])
	case "claim":
		return runClaim(args[1:])
	case "heartbeat":
		return runHeartbeat(args[1:])
	case "submit":
		return runSubmit(args[1:])
	case "accept":
		return runAccept(args[1:])
	case "reject":
		return runReject(args[1:])
	case "reap":
		return runReap(args[1:])
	case "status":
		return runStatus(args[1:])
	case "close":
		return runClose(args[1:])
	default:
		workpoolUsage()
		return fmt.Errorf("unknown workpool subcommand %q", args[0])
	}
}

func workpoolUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness workpool create --repo PATH --name NAME [--parent-cycle ID] [--size N] [--lease-ttl DURATION] [--max-attempts N] [--json]
  agent-harness workpool add-task --pool ID --title TEXT [--instructions TEXT] [--scope TEXT] [--acceptance TEXT] [--json]
  agent-harness workpool claim --pool ID --worker ID [--json]
  agent-harness workpool heartbeat --pool ID --task ID --worker ID [--json]
  agent-harness workpool submit --pool ID --task ID --worker ID --evidence TEXT [--branch BRANCH] [--worktree PATH] [--json]
  agent-harness workpool accept --pool ID --task ID --evidence TEXT [--json]
  agent-harness workpool reject --pool ID --task ID --reason TEXT [--requeue] [--json]
  agent-harness workpool reap --pool ID [--json]
  agent-harness workpool status --pool ID [--json]
  agent-harness workpool close --pool ID [--force] [--reason TEXT] [--json]
`)
}

func runCreate(args []string) error {
	fs := flag.NewFlagSet("workpool create", flag.ContinueOnError)
	repo := fs.String("repo", "", "repository path")
	name := fs.String("name", "", "pool name")
	parentCycle := fs.String("parent-cycle", "", "linked parent IssueOps cycle id")
	size := fs.Int("size", 0, "maximum concurrent leases")
	leaseTTL := fs.String("lease-ttl", "", "lease duration")
	maxAttempts := fs.Int("max-attempts", 0, "maximum reject/requeue attempts")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.CreateWorkPool(core.WorkPoolCreateRequest{
		Repo:          *repo,
		Name:          *name,
		ParentCycleID: *parentCycle,
		Size:          *size,
		LeaseTTL:      *leaseTTL,
		MaxAttempts:   *maxAttempts,
	})
	return printWorkpoolResult(result, err, *jsonOut)
}

func runAddTask(args []string) error {
	fs := flag.NewFlagSet("workpool add-task", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	title := fs.String("title", "", "task title")
	instructions := fs.String("instructions", "", "task instructions")
	var scope repeatedFlag
	var acceptance repeatedFlag
	fs.Var(&scope, "scope", "task scope")
	fs.Var(&acceptance, "acceptance", "acceptance criterion")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.AddWorkPoolTask(*poolID, core.WorkPoolAddTaskRequest{
		Title:              *title,
		Instructions:       *instructions,
		Scope:              []string(scope),
		AcceptanceCriteria: []string(acceptance),
	})
	return printWorkpoolResult(result, err, *jsonOut)
}

func runClaim(args []string) error {
	fs := flag.NewFlagSet("workpool claim", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	workerID := fs.String("worker", "", "worker id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.ClaimWorkPool(*poolID, *workerID)
	return printWorkpoolResult(result, err, *jsonOut)
}

func runHeartbeat(args []string) error {
	fs := flag.NewFlagSet("workpool heartbeat", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	taskID := fs.String("task", "", "task id")
	workerID := fs.String("worker", "", "worker id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.HeartbeatWorkPool(*poolID, *taskID, *workerID)
	return printWorkpoolResult(result, err, *jsonOut)
}

func runSubmit(args []string) error {
	fs := flag.NewFlagSet("workpool submit", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	taskID := fs.String("task", "", "task id")
	workerID := fs.String("worker", "", "worker id")
	branch := fs.String("branch", "", "submission branch")
	worktreePath := fs.String("worktree", "", "submission worktree")
	var evidence repeatedFlag
	fs.Var(&evidence, "evidence", "verification evidence")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.SubmitWorkPool(*poolID, *taskID, *workerID, []string(evidence), *branch, *worktreePath)
	return printWorkpoolResult(result, err, *jsonOut)
}

func runAccept(args []string) error {
	fs := flag.NewFlagSet("workpool accept", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	taskID := fs.String("task", "", "task id")
	var evidence repeatedFlag
	fs.Var(&evidence, "evidence", "verification evidence")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.AcceptWorkPool(*poolID, *taskID, []string(evidence))
	return printWorkpoolResult(result, err, *jsonOut)
}

func runReject(args []string) error {
	fs := flag.NewFlagSet("workpool reject", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	taskID := fs.String("task", "", "task id")
	reason := fs.String("reason", "", "rejection reason")
	requeue := fs.Bool("requeue", false, "return the task to pending if attempts remain")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.RejectWorkPool(*poolID, *taskID, *reason, *requeue)
	return printWorkpoolResult(result, err, *jsonOut)
}

func runReap(args []string) error {
	fs := flag.NewFlagSet("workpool reap", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.ReapWorkPool(*poolID)
	return printWorkpoolResult(result, err, *jsonOut)
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("workpool status", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StatusWorkPool(*poolID)
	return printWorkpoolResult(result, err, *jsonOut)
}

func runClose(args []string) error {
	fs := flag.NewFlagSet("workpool close", flag.ContinueOnError)
	poolID := fs.String("pool", "", "pool id")
	force := fs.Bool("force", false, "close a non-terminal pool")
	reason := fs.String("reason", "", "force close reason")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.CloseWorkPool(*poolID, *force, *reason)
	return printWorkpoolResult(result, err, *jsonOut)
}

func printWorkpoolResult(result any, err error, jsonOut bool) error {
	if jsonOut {
		if err != nil {
			_ = printJSON(map[string]any{"ok": false, "error": err.Error()})
			return err
		}
		return printJSON(result)
	}
	if err != nil {
		return err
	}
	_ = printJSON(result)
	return nil
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
