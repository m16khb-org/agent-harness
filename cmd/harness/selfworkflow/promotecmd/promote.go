package promotecmd

import (
	"flag"
	"fmt"

	"agent-harness/cmd/harness/selfworkflow/model"
)

type Deps struct {
	Promote   func(fromKey, baselineKey string, confirm bool) (model.SelfAugmentPromoteResult, error)
	PrintJSON func(any) error
}

func Run(args []string, deps Deps) error {
	fs := flag.NewFlagSet("self-verify promote", flag.ContinueOnError)
	fromKey := fs.String("from-key", "", "state key containing the candidate self-verification summary snapshot")
	baselineKey := fs.String("baseline-key", "", "state key to write as the promoted baseline")
	confirm := fs.Bool("confirm", false, "write baseline-key; omitted means dry-run")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := deps.Promote(*fromKey, *baselineKey, *confirm)
	if err != nil {
		return err
	}
	if *jsonOut {
		return deps.PrintJSON(result)
	}
	action := "would promote"
	if result.Confirm {
		action = "promoted"
	}
	fmt.Printf("%s self-verification summary %q to baseline %q\n", action, result.FromKey, result.BaselineKey)
	return nil
}
