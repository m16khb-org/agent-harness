package installcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/adapter/installutil"
	"agent-harness/internal/port"
)

const shellPathRCMarker = "# agent-harness: add user-local bin to PATH"

func applyInstallPathPlan(result *port.NativeInstallResult, req port.NativeInstallRequest, mode string) error {
	userBin := filepath.Join(req.Home, ".local", "bin")
	commandPath := filepath.Join(userBin, "agent-harness")
	link, err := installutil.EnsureSymlinkPlan(req.BinPath, commandPath, req.DryRun)
	result.Links = append(result.Links, link)
	if err != nil {
		return err
	}
	shortCommandPath := filepath.Join(userBin, "ah")
	shortLink, err := ensureShortCommandShimPlan(commandPath, shortCommandPath, req.DryRun)
	result.Links = append(result.Links, shortLink)
	if err != nil {
		return err
	}
	if mode == "manual" {
		result.Messages = append(result.Messages, `path-mode=manual: command shims are planned at `+commandPath+` and `+shortCommandPath+`; run export PATH="$HOME/.local/bin:$PATH" for this shell or add it to your shell rc`)
		return nil
	}
	if mode == "skip" {
		result.Messages = append(result.Messages, "path-mode=skip: shell rc PATH update skipped; command shims still use "+commandPath+" and "+shortCommandPath)
		return nil
	}
	if localBinInPath(req.Home) {
		return nil
	}
	rcPath := preferredShellRC(req.Home)
	if shellRCAlreadyAddsLocalBin(rcPath, req.Home) {
		return nil
	}
	file, err := appendShellPathLinePlan(rcPath, req.DryRun)
	if file.Path != "" && (file.WouldWrite || file.Written) {
		result.Files = append(result.Files, file)
	}
	if err != nil {
		return err
	}
	if file.WouldWrite {
		result.Messages = append(result.Messages, "dry-run: would add ~/.local/bin to PATH in "+rcPath)
	} else if file.Written {
		result.Messages = append(result.Messages, `added ~/.local/bin to PATH in `+rcPath+`; restart shell or run: export PATH="$HOME/.local/bin:$PATH"`)
	}
	return nil
}

func ensureShortCommandShimPlan(target, path string, dryRun bool) (port.InstallLink, error) {
	link := port.InstallLink{Path: path, Target: target}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return installutil.EnsureSymlinkPlan(target, path, dryRun)
	}
	if err != nil {
		return link, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return link, fmt.Errorf("refusing to replace existing ah command: %s", path)
	}
	current, err := os.Readlink(path)
	if err != nil {
		return link, err
	}
	if current != target {
		return link, fmt.Errorf("refusing to replace existing ah command symlink %s -> %s", path, current)
	}
	return link, nil
}

func preferredShellRC(home string) string {
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	}
	for _, name := range []string{".zshrc", ".bashrc"} {
		path := filepath.Join(home, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(home, ".profile")
}

func localBinInPath(home string) bool {
	localBin := filepath.Clean(filepath.Join(home, ".local", "bin"))
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(entry) == localBin {
			return true
		}
	}
	return false
}

func shellRCAlreadyAddsLocalBin(path, home string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(b)
	return strings.Contains(text, shellPathRCMarker) ||
		strings.Contains(text, `export PATH="$HOME/.local/bin:$PATH"`) ||
		strings.Contains(text, `export PATH="`+filepath.Join(home, ".local", "bin")+`:$PATH"`)
}

func appendShellPathLinePlan(path string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "shell_path_rc"}
	if dryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return file, err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n%s\n%s\n", shellPathRCMarker, `export PATH="$HOME/.local/bin:$PATH"`); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}
