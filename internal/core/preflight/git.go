package preflight

import (
	"bytes"
	"os/exec"
	"strings"
)

func GitCmd(dir string, args ...string) (int, string, string) {
	code, stdout, stderr := GitCmdRaw(dir, args...)
	return code, strings.TrimSpace(stdout), strings.TrimSpace(stderr)
}

// GitCmdRaw preserves stdout byte-for-byte for machine-delimited Git output
// such as -z path lists. Human-facing callers should continue using GitCmd.
func GitCmdRaw(dir string, args ...string) (int, string, string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	return 1, stdout.String(), err.Error()
}

func GitOut(dir string, args ...string) string {
	code, out, _ := GitCmd(dir, args...)
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}
