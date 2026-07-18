package issueopscli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/codex"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
)

func runIssueOpsHandoffCodexHooksList(args []string) error {
	fs := flag.NewFlagSet("issueops handoff codex-hooks-list", flag.ContinueOnError)
	id := fs.String("id", "", "exact supervised IssueOps lifecycle id")
	jsonOut := fs.Bool("json", false, "print bounded hook trust evidence as JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return printCodexHooksListResult(codex.HooksListResult{}, *jsonOut, fmt.Errorf("issueops handoff codex-hooks-list rejects positional arguments"))
	}
	if *id == "" {
		return printCodexHooksListResult(codex.HooksListResult{}, *jsonOut, fmt.Errorf("issueops handoff codex-hooks-list requires --id"))
	}
	if !*jsonOut {
		return fmt.Errorf("issueops handoff codex-hooks-list requires --json")
	}
	if !validCodexHooksListID(*id) {
		return printCodexHooksListResult(codex.HooksListResult{}, true, fmt.Errorf("codex-hooks-list requires a canonical IssueOps id"))
	}
	record, err := readCodexHooksListRecord(*id)
	if err != nil {
		return printCodexHooksListResult(codex.HooksListResult{}, true, err)
	}
	if err := requireCodexHooksListSource(record.Repo); err != nil {
		return printCodexHooksListResult(codex.HooksListResult{}, true, err)
	}
	result, err := codex.ListHooks(context.Background(), record.ExecutionHandoff.WorkerRoot)
	return printCodexHooksListResult(result, true, err)
}

func validCodexHooksListID(id string) bool {
	if len(id) != len("io-")+12 || !strings.HasPrefix(id, "io-") {
		return false
	}
	for _, r := range id[len("io-"):] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func readCodexHooksListRecord(id string) (core.IssueOpsRecord, error) {
	record, err := core.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		return core.IssueOpsRecord{}, err
	}
	if record.Invalid {
		return core.IssueOpsRecord{}, fmt.Errorf("invalid supervised IssueOps durable record: %s", record.InvalidReason)
	}
	if err := handoff.ValidateEnvelope(record); err != nil {
		return core.IssueOpsRecord{}, fmt.Errorf("invalid supervised IssueOps handoff envelope: %w", err)
	}
	h := record.ExecutionHandoff
	if h == nil || h.Agent != "codex" || h.State != handoff.StateCoordinatorPreparing || h.PendingOperation != nil || h.CleanupOnly != nil || h.WorkerSession != nil || h.Result != nil {
		return core.IssueOpsRecord{}, fmt.Errorf("codex-hooks-list requires a pristine codex coordinator_preparing handoff")
	}
	return record, nil
}

func requireCodexHooksListSource(repo string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current source checkout: %w", err)
	}
	if !sameCodexHooksListPath(cwd, repo) {
		return fmt.Errorf("codex-hooks-list must run from the exact source checkout %s", repo)
	}
	return nil
}

func sameCodexHooksListPath(left, right string) bool {
	canonical := func(path string) (string, bool) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", false
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return "", false
		}
		return filepath.Clean(resolved), true
	}
	l, lok := canonical(left)
	r, rok := canonical(right)
	return lok && rok && l == r
}

func printCodexHooksListResult(result codex.HooksListResult, jsonOut bool, err error) error {
	if err != nil {
		if jsonOut {
			if printErr := printIssueOpsErrorJSON(err); printErr != nil {
				return printErr
			}
		}
		return err
	}
	if jsonOut {
		return printJSON(result)
	}
	fmt.Printf("Codex hooks/list: %s (%d hooks)\n", result.Data[0].CWD, len(result.Data[0].Hooks))
	return nil
}
