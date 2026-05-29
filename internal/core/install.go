package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agent-harness/internal/port"
)

// DefaultNativeInstallRequest normalizes common installation inputs while keeping
// host-specific file decisions in adapter implementations.
func DefaultNativeInstallRequest(root, home, codexHome, binPath string) port.NativeInstallRequest {
	root = absClean(root)
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	home = absClean(home)
	if codexHome == "" && home != "" {
		codexHome = filepath.Join(home, ".codex")
	}
	codexHome = absClean(codexHome)
	if binPath == "" && root != "" {
		binPath = filepath.Join(root, "bin", "agent-harness")
	}
	binPath = absClean(binPath)
	return port.NativeInstallRequest{
		Root:      root,
		Home:      home,
		CodexHome: codexHome,
		BinPath:   binPath,
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

func ListSkillNames(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && exists(filepath.Join(root, "skills", entry.Name(), "SKILL.md")) {
			names = append(names, entry.Name())
		}
	}
	return normalizeSkillNames(names), nil
}

func normalizeSkillNames(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func absClean(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func expandHomeWithHome(path, home string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home == "" {
			home, _ = os.UserHomeDir()
		}
		if home != "" {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func homeRelativePath(path, home string) string {
	if path == "" || home == "" {
		return path
	}
	path = filepath.Clean(path)
	home = filepath.Clean(home)
	if path == home {
		return "~"
	}
	if rel, err := filepath.Rel(home, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filepath.Join("~", rel))
	}
	return path
}
