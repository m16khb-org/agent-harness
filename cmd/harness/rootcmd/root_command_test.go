package rootcmd

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestRunRootCommandUsageAndVersionSurface(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "no args prints usage to stderr",
			args:     []string{},
			wantCode: 2,
			wantErr:  "agent-harness 0.1.0",
		},
		{
			name:     "help prints usage to stderr",
			args:     []string{"help"},
			wantCode: 0,
			wantErr:  "agent-harness 0.1.0",
		},
		{
			name:     "version prints stdout",
			args:     []string{"version"},
			wantCode: 0,
			wantOut:  "agent-harness 0.1.0\n",
		},
		{
			name:     "unknown prints usage to stderr",
			args:     []string{"unknown-command"},
			wantCode: 2,
			wantErr:  "agent-harness 0.1.0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errText, code := captureRootCommandOutput(tc.args)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("stdout = %q, want containing %q", out, tc.wantOut)
			}
			if !strings.Contains(errText, tc.wantErr) {
				t.Fatalf("stderr = %q, want containing %q", errText, tc.wantErr)
			}
		})
	}
}

func TestRunRootCommandDispatchesRunner(t *testing.T) {
	var gotArgs []string
	cmd := testCommand(&bytes.Buffer{}, &bytes.Buffer{})
	cmd.Runners["status"] = func(args []string) error {
		gotArgs = append([]string{}, args...)
		return nil
	}

	if code := cmd.Run([]string{"status", "--json"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "--json" {
		t.Fatalf("runner args = %#v, want --json", gotArgs)
	}
}

func TestRunRootCommandUsesCustomErrorExitCode(t *testing.T) {
	var stderr bytes.Buffer
	cmd := testCommand(&bytes.Buffer{}, &stderr)
	cmd.Runners["guard"] = func([]string) error {
		return errors.New("blocked")
	}
	cmd.ErrorExitCode = func(name string, err error) int {
		if name == "guard" && err != nil {
			return 3
		}
		return 1
	}

	if code := cmd.Run([]string{"guard"}); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "guard: blocked") {
		t.Fatalf("stderr = %q, want guard error", stderr.String())
	}
}

// A subcommand answering --help returns flag.ErrHelp after printing its own
// usage. That is a successful request: exit 0 and no "name: flag: help
// requested" line on stderr.
func TestRunRootCommandTreatsHelpRequestAsSuccess(t *testing.T) {
	var stderr bytes.Buffer
	cmd := testCommand(&bytes.Buffer{}, &stderr)
	cmd.Runners["hook"] = func([]string) error { return flag.ErrHelp }

	if code := cmd.Run([]string{"hook", "--help"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing for a help request", stderr.String())
	}
}

func captureRootCommandOutput(args []string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	cmd := testCommand(&stdout, &stderr)
	code := cmd.Run(args)
	return stdout.String(), stderr.String(), code
}

func testCommand(stdout, stderr *bytes.Buffer) Command {
	return Command{
		Version: "0.1.0",
		Usage: func() {
			stderr.WriteString("agent-harness 0.1.0\n")
		},
		Stdout:  stdout,
		Stderr:  stderr,
		Runners: map[string]Runner{},
		ErrorExitCode: func(string, error) int {
			return 1
		},
	}
}
