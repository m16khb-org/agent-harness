package goformat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"issueops/cmd/issueops/commandstep"
)

// Label is the self-verify step label. Command mirrors the CI "Format check
// (gofmt)" step verbatim so a failing step names exactly what CI runs.
const (
	Label   = "gofmt"
	Command = "gofmt -l $(git ls-files '*.go')"

	timeout                    = 2 * time.Minute
	aggregateOutputBudgetBytes = 8 * 1024
)

type StepResult = commandstep.StepResult

// Deps is the narrow process boundary so unit tests can replace git and gofmt
// without a repository or the Go toolchain.
type Deps struct {
	ListTrackedGoFiles func(ctx context.Context, root string) ([]string, error)
	ListUnformatted    func(ctx context.Context, root string, files []string) ([]string, error)
}

func (d Deps) withDefaults() Deps {
	if d.ListTrackedGoFiles == nil {
		d.ListTrackedGoFiles = listTrackedGoFiles
	}
	if d.ListUnformatted == nil {
		d.ListUnformatted = listUnformatted
	}
	return d
}

// Validate reports whether every git-tracked .go file under root is
// gofmt-clean. It is the local twin of the CI format gate: same file set
// (`git ls-files '*.go'`), same tool, same pass condition (empty `gofmt -l`
// output). `gofmt -l` exits 0 even when it lists files, so the verdict is
// output-based rather than exit-code-based.
func Validate(root string) StepResult {
	return ValidateWithDeps(root, Deps{})
}

func ValidateWithDeps(root string, deps Deps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	files, err := deps.ListTrackedGoFiles(ctx, root)
	if err != nil {
		return failed(started, fmt.Errorf("list tracked .go files: %w", err))
	}
	if len(files) == 0 {
		return failed(started, errors.New("no tracked .go files found; gofmt parity cannot be verified"))
	}
	unformatted, err := deps.ListUnformatted(ctx, root, files)
	if err != nil {
		return failed(started, fmt.Errorf("gofmt -l: %w", err))
	}
	errs := []string{}
	if len(unformatted) > 0 {
		errs = append(errs, fmt.Sprintf("%d tracked .go file(s) are not gofmt-clean; run gofmt -w on: %s", len(unformatted), strings.Join(unformatted, " ")))
	}
	stdout := []string{fmt.Sprintf("checked %d tracked .go file(s)", len(files))}
	return commandstep.AssertionStepWithOutput(Label, started, errs, stdout, []string{Command}, aggregateOutputBudgetBytes)
}

func failed(started time.Time, err error) StepResult {
	step := commandstep.FailedStep(Label, err)
	step.Command = Command
	step.DurationMS = time.Since(started).Milliseconds()
	return step
}

func listTrackedGoFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--", "*.go")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, commandError(err, stderr.String())
	}
	files := []string{}
	for _, file := range strings.Split(string(out), "\x00") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		// A tracked file that is already deleted in the working tree (a pending
		// `git rm`) has nothing to format, and CI checks the committed tree where
		// it is gone; passing it to gofmt would only produce an lstat error.
		if _, err := os.Lstat(filepath.Join(root, file)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func listUnformatted(ctx context.Context, root string, files []string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gofmt", append([]string{"-l"}, files...)...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, commandError(err, stderr.String())
	}
	return nonEmptyLines(string(out)), nil
}

func commandError(err error, stderr string) error {
	if detail := strings.TrimSpace(stderr); detail != "" {
		return fmt.Errorf("%w: %s", err, detail)
	}
	return err
}

func nonEmptyLines(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
