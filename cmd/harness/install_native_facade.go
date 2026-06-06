package main

import (
	"os"

	"agent-harness/cmd/harness/installcli"
	"agent-harness/internal/port"
)

const shellPathRCMarker = installcli.ShellPathRCMarker

func init() {
	installcli.HarnessRoot = harnessRoot
}

func runInstall(args []string) error {
	return installcli.RunInstall(args)
}

func runInstallNative(args []string) error {
	return installcli.RunInstallNative(args)
}

func runInstallCommand(commandName string, args []string) error {
	return installcli.RunInstallCommand(commandName, args)
}

func validateInteractiveInstallInput(stdin *os.File) error {
	return installcli.ValidateInteractiveInput(stdin)
}

func printInstallNativeResult(result port.NativeInstallResult) {
	installcli.PrintNativeResult(result)
}

func preferredShellRC(home string) string {
	return installcli.PreferredShellRC(home)
}

func appendShellPathLinePlan(path string, dryRun bool) (port.InstallFile, error) {
	return installcli.AppendShellPathLinePlan(path, dryRun)
}

func shellRCAlreadyAddsLocalBin(path, home string) bool {
	return installcli.ShellRCAlreadyAddsLocalBin(path, home)
}
