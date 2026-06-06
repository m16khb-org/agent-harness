package basiccli

import (
	"flag"

	"agent-harness/internal/core"
)

func runPreflight(args []string) error {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	jsonOut := fs.Bool("json", true, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	result := core.GitPreflight(ResolveTarget(target), HarnessRoot())
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}
