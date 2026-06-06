package core

import (
	coreinstall "agent-harness/internal/core/install"
	"agent-harness/internal/port"
)

func DefaultNativeInstallRequest(root, home, codexHome, binPath string) port.NativeInstallRequest {
	return coreinstall.DefaultNativeInstallRequest(root, home, codexHome, binPath)
}

func InstallNative(req port.NativeInstallRequest, installers ...port.HostInstaller) (port.NativeInstallResult, error) {
	return coreinstall.InstallNative(req, installers...)
}

func ListSkillNames(root string) ([]string, error) {
	return coreinstall.ListSkillNames(root)
}
