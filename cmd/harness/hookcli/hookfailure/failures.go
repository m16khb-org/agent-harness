package hookfailure

import (
	"encoding/json"
	"flag"
	"os"
	"strings"

	"agent-harness/cmd/harness/hookcli/hookinput"
	"agent-harness/internal/core"
)

func Record(args []string, stdin []byte, hookErr error) {
	hook := "unknown"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		hook = strings.TrimSpace(args[0])
	}
	cwd, _ := os.Getwd()
	repo := ArgValue(args, "--repo")
	if repo == "" {
		repo = hookinput.RepoFromHookInput(stdin)
	}
	_, _ = core.RecordHookFailureEvent(core.HookFailureEvent{
		Hook:           hook,
		Host:           ArgValue(args, "--host"),
		Repo:           repo,
		CWD:            cwd,
		Tool:           hookinput.ToolNameFromHookInput(stdin),
		Argv:           args,
		CommandSnippet: hookinput.CommandFromHookInput(stdin),
		Error:          hookErr.Error(),
	})
}

func Run(args []string) error {
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

func ArgValue(args []string, flagName string) string {
	prefix := flagName + "="
	for i, arg := range args {
		if arg == flagName && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func printJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
