package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCommandStepEnvWithBudgetCoversSuccessFailureAndOutputBudget(t *testing.T) {
	root := t.TempDir()
	helperTimeout := 5 * time.Second
	wrapperBin := writeCommandStepExecutable(t, root, "wrapper.sh", "#!/bin/sh\nprintf 'wrapper stdout'\n")
	wrapper := runCommandStep(root, "helper wrapper", helperTimeout, "", wrapperBin)
	if !wrapper.OK || wrapper.Label != "helper wrapper" || !strings.Contains(wrapper.Stdout, "wrapper stdout") {
		t.Fatalf("unexpected wrapper step: %+v", wrapper)
	}

	step := runCommandStepEnvWithBudget(root, "helper success", helperTimeout, "stdin text", []string{"HARNESS_HELPER_PROCESS=1", "HARNESS_HELPER_VALUE=from-env"}, 32, os.Args[0], "-test.run=TestCommandStepHelperProcess", "--", "echo")
	if !step.OK || step.Label != "helper success" || !strings.Contains(step.Command, "-test.run=TestCommandStepHelperProcess") {
		t.Fatalf("unexpected success step: %+v", step)
	}
	if !step.StdoutTruncated || step.StdoutBytes == 0 || !strings.Contains(step.Stdout, "truncated") {
		t.Fatalf("expected truncated stdout metadata, got %+v", step)
	}
	if !strings.Contains(step.Stderr, "helper stderr") || step.StderrTruncated {
		t.Fatalf("unexpected stderr capture: %+v", step)
	}

	failed := runCommandStepEnv(root, "helper failure", helperTimeout, "", []string{"HARNESS_HELPER_PROCESS=1"}, os.Args[0], "-test.run=TestCommandStepHelperProcess", "--", "fail")
	if failed.OK || !strings.Contains(failed.Error, "exit status 7") {
		t.Fatalf("unexpected failing step: %+v", failed)
	}
}

func TestCommandStepFormattingHelpers(t *testing.T) {
	started := time.Now()
	child := StepResult{Label: "child", OK: false, Stdout: "child stdout", Stderr: "child stderr", Error: "", StderrBytes: 12}
	combined := combineFailedStep("parent", started, child, []string{"first", "second"}, []string{"cmd one", "cmd two"})
	if combined.OK || combined.Error != "child failed" || combined.Command != "cmd one && cmd two" || !strings.Contains(combined.Stdout, "first\nsecond") {
		t.Fatalf("unexpected combined failure: %+v", combined)
	}

	asserted := assertionStepWithOutput("assert", started, []string{"one", "two"}, []string{"stdout"}, []string{"cmd"})
	if asserted.OK || asserted.Error != "one; two" || asserted.Command != "cmd" || asserted.Stdout != "stdout" {
		t.Fatalf("unexpected assertion step: %+v", asserted)
	}
	if failed := failedStep("label", fmt.Errorf("boom")); failed.OK || failed.Error != "boom" {
		t.Fatalf("unexpected failed step: %+v", failed)
	}

	if out, truncated, original := budgetCommandOutput("abc", 0); out != "abc" || truncated || original != 3 {
		t.Fatalf("unexpected unbudgeted output: out=%q truncated=%v original=%d", out, truncated, original)
	}
	if got := tail("abcdef", 4); !strings.HasPrefix(got, "[") || len(got) > 4 {
		t.Fatalf("unexpected tail output: %q", got)
	}
	if got := indentLines("a\nb"); got != "  a\n  b" {
		t.Fatalf("unexpected indented lines: %q", got)
	}

	okOut := captureStatusVerifyStdout(t, func() error {
		printStep(StepResult{Label: "ok step", OK: true, DurationMS: 3})
		return nil
	})
	if !strings.Contains(okOut, "ok step ok") {
		t.Fatalf("unexpected ok print:\n%s", okOut)
	}
	failOut := captureStatusVerifyStdout(t, func() error {
		printStep(StepResult{Label: "bad step", OK: false, DurationMS: 4, Error: "bad", Stdout: "out", Stderr: "err"})
		return nil
	})
	if !strings.Contains(failOut, "bad step failed") || !strings.Contains(failOut, "stdout:") || !strings.Contains(failOut, "stderr:") {
		t.Fatalf("unexpected failed print:\n%s", failOut)
	}
}

func TestCommandStepHelperProcess(t *testing.T) {
	if os.Getenv("HARNESS_HELPER_PROCESS") != "1" {
		return
	}
	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}
	switch mode {
	case "echo":
		b, _ := io.ReadAll(os.Stdin)
		fmt.Fprintf(os.Stdout, "helper stdout %s %s %s", os.Getenv("HARNESS_HELPER_VALUE"), string(b), strings.Repeat("x", 128))
		fmt.Fprint(os.Stderr, "helper stderr")
	case "fail":
		fmt.Fprint(os.Stderr, "helper failed")
		os.Exit(7)
	}
	os.Exit(0)
}

func writeCommandStepExecutable(t *testing.T, root, name, content string) string {
	t.Helper()
	path := root + string(os.PathSeparator) + name
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
