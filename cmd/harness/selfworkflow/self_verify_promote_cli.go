package selfworkflow

import (
	"flag"
	"fmt"
)

type SelfVerifyPromoteDeps struct {
	Promote func(fromKey, baselineKey string, confirm bool) (SelfAugmentPromoteResult, error)
}

func (deps SelfVerifyPromoteDeps) withDefaults() SelfVerifyPromoteDeps {
	if deps.Promote == nil {
		deps.Promote = PromoteSelfAugmentBaseline
	}
	return deps
}

func RunSelfVerifyPromote(args []string) error {
	return RunSelfVerifyPromoteWithDeps(args, SelfVerifyPromoteDeps{})
}

func RunSelfVerifyPromoteWithDeps(args []string, deps SelfVerifyPromoteDeps) error {
	deps = deps.withDefaults()
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
		return printJSON(result)
	}
	action := "would promote"
	if result.Confirm {
		action = "promoted"
	}
	fmt.Printf("%s self-verification summary %q to baseline %q\n", action, result.FromKey, result.BaselineKey)
	return nil
}
