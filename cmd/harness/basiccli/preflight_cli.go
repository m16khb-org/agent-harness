package basiccli

import (
	preflight "agent-harness/internal/adapter/preflight"
	"flag"
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
	result := preflight.GitPreflight(deps.ResolveTarget(target), deps.HarnessRoot())
	if *jsonOut {
		return printJSON(result)
	}
	return printJSON(result)
}
