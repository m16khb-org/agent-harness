package preflight

import (
	"bytes"
	"os/exec"
	"strings"
)

func GitCmd(dir string, args ...string) (int, string, string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String())
	}
	return 1, strings.TrimSpace(stdout.String()), err.Error()
}

func GitOut(dir string, args ...string) string {
	code, out, _ := GitCmd(dir, args...)
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}
