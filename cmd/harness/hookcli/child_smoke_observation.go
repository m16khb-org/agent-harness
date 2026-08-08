package hookcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const childSmokeHookMarkerLimit = 64 << 10

func recordChildSmokeHookEvent(subcommand string) error {
	if os.Getenv("HARNESS_CHILD_SMOKE_HOOKS") != "1" {
		return nil
	}
	event := ""
	switch subcommand {
	case "session-start":
		event = "SessionStart"
	case "pre-tool-use":
		event = "PreToolUse"
	default:
		return nil
	}
	observationPath := strings.TrimSpace(os.Getenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE"))
	if !filepath.IsAbs(observationPath) {
		return fmt.Errorf("child smoke observation path must be absolute")
	}
	markerPath := observationPath + ".hooks"
	parent, err := os.Lstat(filepath.Dir(markerPath))
	if err != nil || !parent.IsDir() || parent.Mode()&os.ModeSymlink != 0 || parent.Mode().Perm() != 0o700 {
		return fmt.Errorf("child smoke observation parent must be private")
	}
	if info, err := os.Lstat(markerPath); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || info.Size() >= childSmokeHookMarkerLimit {
			return fmt.Errorf("child smoke hook marker is invalid")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("child smoke hook marker is invalid")
	}
	line := []byte(fmt.Sprintf("{\"event\":%q}\n", event))
	file, err := os.OpenFile(markerPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open child smoke hook marker: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size()+int64(len(line)) > childSmokeHookMarkerLimit {
		return fmt.Errorf("child smoke hook marker is invalid")
	}
	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("write child smoke hook marker: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync child smoke hook marker: %w", err)
	}
	return nil
}
