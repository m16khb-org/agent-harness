package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/port"
)

// DefaultNativeInstallRequest normalizes common installation inputs while keeping
// host-specific file decisions in adapter implementations.
func DefaultNativeInstallRequest(root, home, codexHome, reasonixHome, binPath string) port.NativeInstallRequest {
	root = absClean(root)
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	home = absClean(home)
	if codexHome == "" && home != "" {
		codexHome = filepath.Join(home, ".codex")
	}
	codexHome = absClean(codexHome)
	if reasonixHome == "" && home != "" {
		reasonixHome = filepath.Join(home, ".reasonix")
	}
	reasonixHome = absClean(reasonixHome)
	if binPath == "" && root != "" {
		binPath = filepath.Join(root, "bin", "agent-harness")
	}
	binPath = absClean(binPath)
	return port.NativeInstallRequest{
		Root:         root,
		Home:         home,
		CodexHome:    codexHome,
		ReasonixHome: reasonixHome,
		BinPath:      binPath,
	}
}

// InstallNative is the host-neutral installation engine. It validates shared
// inputs, resolves shared skills once, and delegates concrete writes to host
// adapters through port.HostInstaller.
func InstallNative(req port.NativeInstallRequest, installers ...port.HostInstaller) (port.NativeInstallResult, error) {
	if req.Root == "" {
		return port.NativeInstallResult{OK: false}, fmt.Errorf("root is required")
	}
	req.Root = absClean(req.Root)
	req.Home = absClean(req.Home)
	req.CodexHome = absClean(req.CodexHome)
	req.ReasonixHome = absClean(req.ReasonixHome)
	req.BinPath = absClean(req.BinPath)
	if len(req.SkillNames) == 0 {
		skills, err := ListSkillNames(req.Root)
		if err != nil {
			return port.NativeInstallResult{OK: false, Root: req.Root}, err
		}
		req.SkillNames = skills
	} else {
		req.SkillNames = normalizeSkillNames(req.SkillNames)
	}
	result := port.NativeInstallResult{
		OK:           true,
		Root:         req.Root,
		Home:         req.Home,
		CodexHome:    req.CodexHome,
		ReasonixHome: req.ReasonixHome,
		BinPath:      req.BinPath,
		SkillNames:   append([]string{}, req.SkillNames...),
		Hosts:        []port.HostInstallResult{},
		Files:        []port.InstallFile{},
		Links:        []port.InstallLink{},
		ProjectLocal: req.ProjectLocal,
		DryRun:       req.DryRun,
	}
	if len(installers) == 0 {
		result.OK = false
		return result, fmt.Errorf("at least one host installer is required")
	}
	var errs []error
	for _, installer := range installers {
		if installer == nil {
			continue
		}
		hostResult, err := installer.Install(req)
		if hostResult.Host == "" {
			hostResult.Host = installer.Name()
		}
		if err != nil {
			hostResult.OK = false
			hostResult.Error = err.Error()
			result.OK = false
			errs = append(errs, fmt.Errorf("%s: %w", installer.Name(), err))
		} else if !hostResult.OK {
			result.OK = false
			errs = append(errs, fmt.Errorf("%s: installer reported ok=false", installer.Name()))
		}
		result.Hosts = append(result.Hosts, hostResult)
		result.Files = append(result.Files, hostResult.Files...)
		result.Links = append(result.Links, hostResult.Links...)
		result.Messages = append(result.Messages, hostResult.Messages...)
	}
	return result, errors.Join(errs...)
}
