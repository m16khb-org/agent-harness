package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func runUpdate(args []string) error {
	return runInstallScriptCommand("update", args)
}

func runBootstrap(args []string) error {
	return runInstallScriptCommand("bootstrap", args)
}

func runInstallScriptCommand(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	withUpstream := fs.Bool("with-upstream-tools", true, "install/update upstream llm-wiki, codegraph, and claude-mem integrations")
	skipUpstream := fs.Bool("skip-upstream-tools", false, "skip upstream companion tool setup")
	withoutUpstream := fs.Bool("without-upstream-tools", false, "skip upstream companion tool setup")
	projectLocal := fs.Bool("project-local", false, "also write explicit project-local files")
	dryRun := fs.Bool("dry-run", false, "show install plan without writing")
	jsonOut := fs.Bool("json", false, "print JSON from install-native")
	skipBuild := fs.Bool("skip-build", false, "do not rebuild bin/agent-harness")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	script := filepath.Join(harnessRoot(), "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}

	scriptArgs := make([]string, 0, 5)
	if *skipUpstream || *withoutUpstream || !*withUpstream {
		scriptArgs = append(scriptArgs, "--skip-upstream-tools")
	} else {
		scriptArgs = append(scriptArgs, "--with-upstream-tools")
	}
	if *projectLocal {
		scriptArgs = append(scriptArgs, "--project-local")
	}
	if *dryRun {
		scriptArgs = append(scriptArgs, "--dry-run")
	}
	if *jsonOut {
		scriptArgs = append(scriptArgs, "--json")
	}
	if *skipBuild {
		scriptArgs = append(scriptArgs, "--skip-build")
	}

	cmd := exec.Command(script, scriptArgs...)
	cmd.Dir = harnessRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
