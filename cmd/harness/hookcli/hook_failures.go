package hookcli

import (
	"flag"
	"os"
	"strings"

	"agent-harness/internal/core"
)

func recordHookFailure(args []string, stdin []byte, hookErr error) {
	hook := "unknown"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		hook = strings.TrimSpace(args[0])
	}
	cwd, _ := os.Getwd()
	repo := hookArgValue(args, "--repo")
	if repo == "" {
		repo = repoFromHookInput(stdin)
	}
	_, _ = core.RecordHookFailureEvent(core.HookFailureEvent{
		Hook:           hook,
		Host:           hookArgValue(args, "--host"),
		Repo:           repo,
		CWD:            cwd,
		Tool:           toolNameFromHookInput(stdin),
		Argv:           args,
		CommandSnippet: commandFromHookInput(stdin),
		Error:          hookErr.Error(),
	})
}

func runHookFailures(args []string) error {
	fs := flag.NewFlagSet("hook failures", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "maximum recent hook failure events to return")
	jsonOut := fs.Bool("json", false, "print hook failure events as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.ListHookFailureEvents(*limit)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}
