package issueopsbasesync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	basesyncport "agent-harness/internal/port/issueopsbasesync"
)

var ErrGitRunnerRequired = errors.New("base sync git runner is required")

type GitRunner func(context.Context, string, ...string) (int, string, string, error)

type Inspector struct {
	run GitRunner
}

func NewInspector(run GitRunner) *Inspector {
	return &Inspector{run: run}
}

func (i *Inspector) Observe(ctx context.Context, request basesyncport.Request) (basesyncport.Receipt, error) {
	if i == nil || i.run == nil {
		return basesyncport.Receipt{}, ErrGitRunnerRequired
	}
	worktree := strings.TrimSpace(request.Worktree)
	if worktree == "" {
		return basesyncport.Receipt{}, fmt.Errorf("worktree is required")
	}
	baseBranch := strings.TrimSpace(request.BaseBranch)
	if baseBranch == "" {
		return basesyncport.Receipt{}, fmt.Errorf("base branch is required")
	}
	if err := i.requireSuccess(ctx, worktree, "fetch base branch", "fetch", "--quiet", "origin", baseBranch); err != nil {
		return basesyncport.Receipt{}, err
	}
	baseOID, err := i.requireOutput(ctx, worktree, "read FETCH_HEAD", "rev-parse", "FETCH_HEAD")
	if err != nil {
		return basesyncport.Receipt{}, err
	}
	workOID, err := i.requireOutput(ctx, worktree, "read HEAD", "rev-parse", "HEAD")
	if err != nil {
		return basesyncport.Receipt{}, err
	}
	code, _, stderr, err := i.run(ctx, worktree, "merge-base", "--is-ancestor", baseOID, workOID)
	if err != nil {
		return basesyncport.Receipt{}, fmt.Errorf("observe base ancestry: %w", err)
	}
	switch code {
	case 0:
		return basesyncport.Receipt{BaseOID: baseOID, WorkOID: workOID}, nil
	case 1:
		return basesyncport.Receipt{BaseOID: baseOID, WorkOID: workOID, SyncRequired: true}, nil
	default:
		return basesyncport.Receipt{}, fmt.Errorf("observe base ancestry: git exited %d: %s", code, strings.TrimSpace(stderr))
	}
}

func (i *Inspector) requireSuccess(ctx context.Context, worktree, step string, args ...string) error {
	code, _, stderr, err := i.run(ctx, worktree, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	if code != 0 {
		return fmt.Errorf("%s: git exited %d: %s", step, code, strings.TrimSpace(stderr))
	}
	return nil
}

func (i *Inspector) requireOutput(ctx context.Context, worktree, step string, args ...string) (string, error) {
	code, stdout, stderr, err := i.run(ctx, worktree, args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", step, err)
	}
	value := strings.TrimSpace(stdout)
	if code != 0 || value == "" {
		return "", fmt.Errorf("%s: git exited %d: %s", step, code, strings.TrimSpace(stderr))
	}
	return value, nil
}

func RunGit(ctx context.Context, dir string, args ...string) (int, string, string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String(), nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String(), nil
	}
	return 0, stdout.String(), stderr.String(), err
}

var _ basesyncport.Inspector = (*Inspector)(nil)
