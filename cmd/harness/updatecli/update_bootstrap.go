package updatecli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var installScriptCommandRunner = runInstallScriptExec

func runUpdate(args []string) error {
	return runInstallScriptCommand("update", args)
}

func runBootstrap(args []string) error {
	return runInstallScriptCommand("bootstrap", args)
}

func runInstallScriptCommand(commandName string, args []string) error {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write explicit project-local files")
	dryRun := fs.Bool("dry-run", false, "show install plan without writing")
	pathMode := fs.String("path-mode", "", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	jsonOut := fs.Bool("json", false, "print JSON from install")
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

	root := deps.HarnessRoot()
	script := filepath.Join(root, "scripts", "install-native.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found at %s: %w", script, err)
	}

	scriptArgs := make([]string, 0, 5)
	if *projectLocal {
		scriptArgs = append(scriptArgs, "--project-local")
	}
	if *dryRun {
		scriptArgs = append(scriptArgs, "--dry-run")
	}
	if *pathMode != "" {
		scriptArgs = append(scriptArgs, "--path-mode="+*pathMode)
	}
	if *interactive {
		scriptArgs = append(scriptArgs, "--interactive")
	}
	if *jsonOut {
		scriptArgs = append(scriptArgs, "--json")
	}
	if *skipBuild {
		scriptArgs = append(scriptArgs, "--skip-build")
	}

	if err := installScriptCommandRunner(script, scriptArgs...); err != nil {
		return err
	}
	if *dryRun {
		return nil
	}
	if _, err := postInstallDaemonRefresh(); err != nil {
		return err
	}
	return nil
}

func runInstallScriptExec(script string, args ...string) error {
	cmd := exec.Command(script, args...)
	cmd.Dir = deps.HarnessRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
