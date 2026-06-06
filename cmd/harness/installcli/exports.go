package installcli

import (
	"os"

	"agent-harness/internal/port"
)

const ShellPathRCMarker = shellPathRCMarker

func RunInstall(args []string) error {
	return runInstall(args)
}

func RunInstallNative(args []string) error {
	return runInstallNative(args)
}

func RunInstallCommand(commandName string, args []string) error {
	return runInstallCommand(commandName, args)
}

func ValidateInteractiveInput(stdin *os.File) error {
	return validateInteractiveInstallInput(stdin)
}

func PrintNativeResult(result port.NativeInstallResult) {
	printInstallNativeResult(result)
}

func PreferredShellRC(home string) string {
	return preferredShellRC(home)
}

func AppendShellPathLinePlan(path string, dryRun bool) (port.InstallFile, error) {
	return appendShellPathLinePlan(path, dryRun)
}

func ShellRCAlreadyAddsLocalBin(path, home string) bool {
	return shellRCAlreadyAddsLocalBin(path, home)
}
