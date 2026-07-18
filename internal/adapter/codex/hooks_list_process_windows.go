//go:build windows

package codex

import (
	"errors"
	"os"
	"os/exec"
)

func configureHooksListProcess(_ *exec.Cmd) {}

func terminateHooksListProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
