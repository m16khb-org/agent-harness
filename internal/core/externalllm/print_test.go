package externalllm

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunExternalLLMPrintRequiresPromptAndDefaultsCommand(t *testing.T) {
	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Prompt: "   "})
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected prompt error, got err=%v result=%+v", err, result)
	}
	if result.Command != defaultExternalLLMCommand {
		t.Fatalf("Command=%q, want %q", result.Command, defaultExternalLLMCommand)
	}
}

func TestRunExternalLLMPrintRunsCommandWithPromptAndWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-only")
	}
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(dir, "fake-agy.sh")
	script := `#!/bin/sh
printf 'cwd=%s\n' "$PWD"
printf 'args=%s\n' "$*"
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agy: %v", err)
	}

	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Command: fake,
		WorkDir: workDir,
		Prompt:  "return json",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunExternalLLMPrint() error = %v; output=%s", err, result.Output)
	}
	wantArgv := []string{"--dangerously-skip-permissions", "-p", "return json"}
	if !reflect.DeepEqual(result.Argv, wantArgv) {
		t.Fatalf("Argv=%#v, want %#v", result.Argv, wantArgv)
	}
	output := string(result.Output)
	if !strings.Contains(output, "cwd="+workDir) {
		t.Fatalf("output %q does not contain working directory %q", output, workDir)
	}
	if !strings.Contains(output, "args=--dangerously-skip-permissions -p return json") {
		t.Fatalf("output %q does not contain argv", output)
	}
}

func TestRunExternalLLMPrintReturnsCommandErrorWithOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is POSIX-only")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-agy-fail.sh")
	script := `#!/bin/sh
echo provider-failed
exit 7
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agy: %v", err)
	}

	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Command: fake,
		Prompt:  "return json",
		Timeout: 5 * time.Second,
	})
	if err == nil {
		t.Fatalf("expected command error, got result=%+v", result)
	}
	if !strings.Contains(string(result.Output), "provider-failed") {
		t.Fatalf("Output=%q, want provider failure text", result.Output)
	}
}

func TestExternalLLMPrintCommandPreview(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		want    string
	}{
		{name: "default command", command: "   ", want: defaultExternalLLMCommand},
		{name: "custom command", command: " custom-agy ", want: "custom-agy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ExternalLLMPrintCommandPreview(tc.command)
			for _, want := range []string{tc.want, "--dangerously-skip-permissions", "-p", "<prompt>"} {
				if !strings.Contains(got, want) {
					t.Fatalf("preview %q does not contain %q", got, want)
				}
			}
		})
	}
}

func TestRunExternalLLMPrintTimeoutKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script timeout fixture is POSIX-only")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-agy.sh")
	script := `#!/bin/sh
(sleep 2; echo late-child-output) &
wait
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agy: %v", err)
	}

	started := time.Now()
	result, err := RunExternalLLMPrint(ExternalLLMPrintRequest{
		Command: fake,
		Prompt:  "return json",
		Timeout: 50 * time.Millisecond,
	})
	elapsed := time.Since(started)

	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got err=%v result=%+v", err, result)
	}
	if elapsed > 750*time.Millisecond {
		t.Fatalf("timeout should not wait for child process pipe close; elapsed=%s", elapsed)
	}
}
