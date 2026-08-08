package hookfailure

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"time"

	"agent-harness/cmd/harness/hookcli/hookinput"
	hookfailurecontract "agent-harness/internal/contract/hookfailure"
)

func Record(args []string, stdin []byte, hookErr error) {
	// Help requests are not failures; recording them buried real defects
	// under noise (16 of the first 38 logged "failures" were ErrHelp).
	if errors.Is(hookErr, flag.ErrHelp) {
		return
	}
	hook := "unknown"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		hook = strings.TrimSpace(args[0])
	}
	cwd, _ := os.Getwd()
	repo := ArgValue(args, "--repo")
	if repo == "" {
		repo = hookinput.RepoFromHookInput(stdin)
	}
	_, _ = RecordHookFailureEvent(hookfailurecontract.HookFailureEvent{
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
	if len(args) > 0 && args[0] == "prune" {
		return RunPrune(args[1:])
	}
	if len(args) > 0 && args[0] == "stats" {
		return RunStats(args[1:])
	}
	fs := flag.NewFlagSet("hook failures", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "maximum recent hook failure events to return")
	jsonOut := fs.Bool("json", false, "print hook failure events as JSON")
	pruneFlag := fs.Duration("prune", 0, "prune entries older than this duration before listing (e.g. 720h)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pruneFlag > 0 {
		pruneResult, err := PruneHookFailureLog(*pruneFlag)
		if err != nil {
			return err
		}
		if *jsonOut {
			return printJSON(pruneResult)
		}
		return printJSON(pruneResult)
	}
	result, err := ListHookFailureEvents(*limit)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}

// RunMetrics prints aggregated hook latency/gate telemetry (Q2 phase 2).
func RunMetrics(args []string) error {
	fs := flag.NewFlagSet("hook metrics", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print metrics as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = jsonOut
	stats, err := SummarizeHookMetricsLog()
	if err != nil {
		return err
	}
	return printJSON(stats)
}

// RunStats prints aggregated hook-failure metrics (quality program Q2): the
// first measurable failure-rate signal for the hook surface.
func RunStats(args []string) error {
	fs := flag.NewFlagSet("hook failures stats", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print stats as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = jsonOut
	stats, err := SummarizeHookFailureStats()
	if err != nil {
		return err
	}
	return printJSON(stats)
}

func RunPrune(args []string) error {
	fs := flag.NewFlagSet("hook failures prune", flag.ContinueOnError)
	maxAge := fs.Duration("max-age", 720*time.Hour, "maximum age of entries to keep (e.g. 720h)")
	jsonOut := fs.Bool("json", false, "print result as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := PruneHookFailureLog(*maxAge)
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
