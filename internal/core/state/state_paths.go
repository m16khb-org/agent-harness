package state

import (
	"os"
	"path/filepath"
)

func StateDir() string {
	if env := os.Getenv("HARNESS_STATE_DIR"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "agent-harness-state")
	}
	return filepath.Join(home, ".local", "state", "agent-harness")
}

func statePath(dir, key string) string {
	return filepath.Join(dir, key+".json")
}
