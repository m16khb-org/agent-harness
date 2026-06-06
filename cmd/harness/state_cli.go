package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"agent-harness/internal/core"
)

func runStateWrite(args []string) error {
	fs := flag.NewFlagSet("state write", flag.ContinueOnError)
	key := fs.String("key", "", "state key; [A-Za-z0-9._-], max 128 chars")
	value := fs.String("value", "", "state content")
	input := fs.String("input", "", "read state content from file")
	stdin := fs.Bool("stdin", false, "read state content from stdin")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *key == "" && fs.NArg() > 0 {
		*key = fs.Arg(0)
	}
	valueProvided := flagProvided(args, "value")
	sourceCount := 0
	if valueProvided {
		sourceCount++
	}
	if *input != "" {
		sourceCount++
	}
	if *stdin {
		sourceCount++
	}
	if sourceCount != 1 {
		return fmt.Errorf("provide exactly one content source: --value, --input, or --stdin")
	}
	var content string
	switch {
	case valueProvided:
		content = *value
	case *input != "":
		b, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		content = string(b)
	case *stdin:
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		content = string(b)
	}
	result, err := core.StateWrite(*key, content)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("state %q written (%d bytes) to %s\n", result.Record.Key, result.Record.Bytes, result.StateDir)
	return nil
}

func runStateRead(args []string) error {
	fs := flag.NewFlagSet("state read", flag.ContinueOnError)
	key := fs.String("key", "", "state key")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *key == "" && fs.NArg() > 0 {
		*key = fs.Arg(0)
	}
	result, err := core.StateRead(*key)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Print(result.Record.Content)
	return nil
}

func runStateList(args []string) error {
	fs := flag.NewFlagSet("state list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StateList()
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	for _, key := range result.Keys {
		fmt.Println(key)
	}
	return nil
}

func runStatePrune(args []string) error {
	fs := flag.NewFlagSet("state prune", flag.ContinueOnError)
	maxAge := fs.Duration("max-age", 0, "prune records older than this duration, e.g. 720h")
	confirm := fs.Bool("confirm", false, "delete matching records; omitted means dry-run")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StatePrune(*maxAge, *confirm)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	action := "would prune"
	if result.Confirm {
		action = "pruned"
	}
	fmt.Printf("%s %d state records older than %s\n", action, len(result.DeletedKeys), result.MaxAge)
	for _, key := range result.DeletedKeys {
		fmt.Println(key)
	}
	return nil
}

func runStateDoctor(args []string) error {
	fs := flag.NewFlagSet("state doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StateDoctor()
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Healthy {
		fmt.Printf("state doctor healthy: %d valid records in %s\n", len(result.ValidKeys), result.StateDir)
		return nil
	}
	fmt.Printf("state doctor found %d issues in %s\n", len(result.Issues), result.StateDir)
	for _, issue := range result.Issues {
		fmt.Printf("%s %s %s\n", issue.Severity, issue.Code, issue.Path)
	}
	return nil
}

func runStateMigrate(args []string) error {
	fs := flag.NewFlagSet("state migrate", flag.ContinueOnError)
	confirm := fs.Bool("confirm", false, "rewrite legacy records; omitted means dry-run")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := core.StateMigrate(*confirm)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Confirm {
		fmt.Printf("migrated %d state records from schema %d to %d\n", len(result.MigratedKeys), result.FromSchema, result.ToSchema)
		for _, key := range result.MigratedKeys {
			fmt.Println(key)
		}
		return nil
	}
	fmt.Printf("would migrate %d state records from schema %d to %d\n", len(result.CandidateKeys), result.FromSchema, result.ToSchema)
	for _, key := range result.CandidateKeys {
		fmt.Println(key)
	}
	return nil
}
