package resourcecli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"agent-harness/internal/adapter/systemresource"
	"agent-harness/internal/core/resourcewait"
)

type Deps struct {
	Stdout io.Writer
	Stderr io.Writer
	Sample func(context.Context, string) (resourcewait.Sample, error)
	Now    func() time.Time
	Sleep  func(context.Context, time.Duration) error
}

func Run(args []string) error {
	collector := systemresource.NewCollector()
	return RunWithDeps(args, Deps{Stdout: os.Stdout, Stderr: os.Stderr, Sample: collector.Sample})
}

func RunWithDeps(args []string, deps Deps) error {
	if deps.Stdout == nil {
		deps.Stdout = io.Discard
	}
	if deps.Stderr == nil {
		deps.Stderr = io.Discard
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		_, err := fmt.Fprintln(deps.Stdout, "Usage: agent-harness resource wait [--workspace-root PATH] [--profile e2e] [--timeout DURATION] [--interval DURATION] [--progress none|jsonl] [--json]")
		return err
	}
	if args[0] != "wait" {
		return fmt.Errorf("unknown resource subcommand %q", args[0])
	}
	return runWait(args[1:], deps)
}

func runWait(args []string, deps Deps) error {
	flags := flag.NewFlagSet("resource wait", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workspaceRoot := flags.String("workspace-root", ".", "workspace root")
	profile := flags.String("profile", resourcewait.ProfileE2E, "resource profile")
	timeout := flags.Duration("timeout", 10*time.Minute, "maximum wait")
	interval := flags.Duration("interval", 5*time.Second, "sample interval")
	progress := flags.String("progress", "none", "progress mode")
	jsonOut := flags.Bool("json", false, "print JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("resource wait does not accept an E2E command")
	}
	if *profile != resourcewait.ProfileE2E {
		return errors.New("--profile only e2e is supported")
	}
	if *progress != "none" && *progress != "jsonl" {
		return errors.New("--progress must be none or jsonl")
	}
	root, err := cleanWorkspaceRoot(*workspaceRoot)
	if err != nil {
		return err
	}
	if deps.Sample == nil {
		collector := systemresource.NewCollector()
		deps.Sample = collector.Sample
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	started := time.Now()
	if deps.Now != nil {
		started = deps.Now()
	}
	if *progress == "jsonl" {
		writeProgress(deps.Stderr, map[string]any{
			"event": "wait_started", "profile": *profile, "interval_ms": interval.Milliseconds(),
			"timeout_ms": timeout.Milliseconds(), "required_stable_samples": 3,
		})
	}
	lastBlockers := ""
	result, waitErr := resourcewait.Wait(ctx, resourcewait.Request{
		WorkspaceRoot: root,
		Profile:       *profile,
		Timeout:       *timeout,
		Interval:      *interval,
	}, resourcewait.Dependencies{
		Sample: deps.Sample,
		Now:    deps.Now,
		Sleep:  deps.Sleep,
		OnSample: func(sample resourcewait.Sample, count, stable int, blockers []resourcewait.Blocker) {
			codes := make([]string, len(blockers))
			for index, blocker := range blockers {
				codes[index] = blocker.Code
			}
			if *progress == "jsonl" {
				writeProgress(deps.Stderr, map[string]any{
					"event": "sample", "elapsed_ms": sample.SampledAt.Sub(started).Milliseconds(), "sample_count": count,
					"consecutive_stable_samples": stable, "blocker_codes": codes,
				})
			}
			if !*jsonOut {
				if count == 1 {
					thresholds := resourcewait.ResolveThresholds(sample, resourcewait.E2EProfile().Thresholds)
					fmt.Fprintf(deps.Stdout, "resource wait profile=%s thresholds=%+v\n", *profile, thresholds)
				}
				current := strings.Join(codes, ",")
				if current != lastBlockers {
					if current == "" {
						current = "none"
					}
					fmt.Fprintf(deps.Stdout, "resource wait blockers=%s\n", current)
					lastBlockers = strings.Join(codes, ",")
				}
			}
		},
	})
	if *progress == "jsonl" {
		writeProgress(deps.Stderr, map[string]any{
			"event": "wait_finished", "elapsed_ms": result.WaitedMS, "status": result.Status,
		})
	}
	if *jsonOut {
		if err := json.NewEncoder(deps.Stdout).Encode(result); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(deps.Stdout, "resource wait %s profile=%s waited=%s samples=%d stable=%d\n", result.Status, result.Profile, time.Duration(result.WaitedMS)*time.Millisecond, result.SampleCount, result.ConsecutiveStableSamples)
	}
	return waitErr
}

func cleanWorkspaceRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("workspace root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("workspace root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root %q is not a directory", abs)
	}
	return filepath.Clean(abs), nil
}

func writeProgress(writer io.Writer, value any) {
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		_, _ = fmt.Fprintln(writer, `{"event":"progress_error"}`)
	}
}

func ExitCode(err error) int {
	var admissionErr *resourcewait.AdmissionError
	if errors.As(err, &admissionErr) && (admissionErr.Status == resourcewait.StatusTimedOut || admissionErr.Status == resourcewait.StatusCancelled) {
		return 3
	}
	return 1
}
