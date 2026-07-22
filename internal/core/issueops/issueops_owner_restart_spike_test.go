package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/port"
)

type ownerRestartSpikeWIPSeal struct {
	Ref      string `json:"ref"`
	Commit   string `json:"commit"`
	Tree     string `json:"tree"`
	BaseHead string `json:"base_head"`
}

type ownerRestartSpikeAttempt struct {
	Number           int                         `json:"number"`
	Workspace        *IssueOpsExecutionWorkspace `json:"workspace,omitempty"`
	Handoff          *IssueOpsExecutionHandoff   `json:"handoff,omitempty"`
	InheritedWIPSeal *ownerRestartSpikeWIPSeal   `json:"inherited_wip_seal,omitempty"`
	RestartedFrom    int                         `json:"restarted_from,omitempty"`
	StartedAt        string                      `json:"started_at"`
}

type ownerRestartSpikeLedger struct {
	ActiveAttempt  int                        `json:"active_attempt,omitempty"`
	Attempts       []ownerRestartSpikeAttempt `json:"attempts,omitempty"`
	PendingRestart map[string]string          `json:"pending_restart,omitempty"`
}

type ownerRestartSpikeRecord struct {
	CycleState string                  `json:"cycle_state"`
	Ownership  ownerRestartSpikeLedger `json:"ownership"`
}

func TestOwnerRestartSpikeTransitionKeepsPredecessorImmutable(t *testing.T) {
	predecessor := ownerRestartSpikeAttempt{
		Number: 1,
		Workspace: &IssueOpsExecutionWorkspace{
			State: "ready", WorkspaceEpoch: "workspace-1", WorkerRoot: "/tmp/worker-1",
		},
		Handoff: &IssueOpsExecutionHandoff{
			State: handoff.StateClosed, ClosedDisposition: handoff.DispositionCancelled,
			Attempt: 1, OwnershipEpoch: "ownership-1", WorkspaceEpoch: "workspace-1",
		},
		StartedAt: "2026-07-22T00:00:00Z",
	}
	record := ownerRestartSpikeRecord{
		CycleState: "paused",
		Ownership: ownerRestartSpikeLedger{
			Attempts:       []ownerRestartSpikeAttempt{predecessor},
			PendingRestart: map[string]string{"state": "intent"},
		},
	}
	before := ownerRestartSpikeJSON(t, record.Ownership.Attempts[0])
	seal := &ownerRestartSpikeWIPSeal{Ref: "refs/agent-harness/issueops/io-spike/attempts/2/wip", Commit: "commit-2", Tree: "tree-2", BaseHead: "head-1"}

	restarted := ownerRestartSpikeApplyTransition(record, seal)

	after := ownerRestartSpikeJSON(t, restarted.Ownership.Attempts[0])
	if string(after) != string(before) {
		t.Fatalf("predecessor attempt changed during restart\nbefore=%s\nafter=%s", before, after)
	}
	if restarted.Ownership.PendingRestart != nil {
		t.Fatalf("successful restart must clear pending_restart: %+v", restarted.Ownership.PendingRestart)
	}
	if restarted.CycleState != "active" || restarted.Ownership.ActiveAttempt != 2 || len(restarted.Ownership.Attempts) != 2 {
		t.Fatalf("unexpected successor ledger: %+v", restarted)
	}
	successor := restarted.Ownership.Attempts[1]
	if successor.RestartedFrom != 1 || successor.InheritedWIPSeal == nil || *successor.InheritedWIPSeal != *seal {
		t.Fatalf("successor must own restart provenance and WIP seal: %+v", successor)
	}
}

func ownerRestartSpikeApplyTransition(record ownerRestartSpikeRecord, seal *ownerRestartSpikeWIPSeal) ownerRestartSpikeRecord {
	result := record
	result.Ownership.Attempts = append([]ownerRestartSpikeAttempt(nil), record.Ownership.Attempts...)
	last := result.Ownership.Attempts[len(result.Ownership.Attempts)-1]
	result.Ownership.Attempts = append(result.Ownership.Attempts, ownerRestartSpikeAttempt{
		Number: 2,
		Workspace: &IssueOpsExecutionWorkspace{
			State: "ready", WorkspaceEpoch: "workspace-2", WorkerRoot: "/tmp/worker-2",
		},
		Handoff: &IssueOpsExecutionHandoff{
			State: handoff.StateOwnershipDispatching, Attempt: 2,
			OwnershipEpoch: "ownership-2", WorkspaceEpoch: "workspace-2",
		},
		InheritedWIPSeal: seal,
		RestartedFrom:    last.Number,
		StartedAt:        "2026-07-22T01:00:00Z",
	})
	result.Ownership.ActiveAttempt = 2
	result.Ownership.PendingRestart = nil
	result.CycleState = "active"
	return result
}

func TestOwnerRestartSpikePausedResourceQuarantine(t *testing.T) {
	tests := []struct {
		name   string
		action string
		allow  bool
	}{
		{name: "ordinary unrelated source work is not fenced", action: "unrelated-source", allow: true},
		{name: "exact source preview is allowed", action: "restart-preview", allow: true},
		{name: "exact source confirm is allowed", action: "restart-confirm", allow: true},
		{name: "historical owner is quarantined", action: "historical-owner", allow: false},
		{name: "historical worker root is quarantined", action: "historical-worker-root", allow: false},
		{name: "historical terminal is quarantined", action: "historical-terminal", allow: false},
		{name: "historical task is quarantined", action: "historical-task", allow: false},
		{name: "historical dispatch is quarantined", action: "historical-dispatch", allow: false},
		{name: "raw source mutation of paused cycle is denied", action: "paused-cycle-mutation", allow: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ownerRestartSpikePausedAllows(tt.action); got != tt.allow {
				t.Fatalf("action %q allow=%v, want %v", tt.action, got, tt.allow)
			}
		})
	}
}

func ownerRestartSpikePausedAllows(action string) bool {
	switch action {
	case "unrelated-source", "restart-preview", "restart-confirm":
		return true
	default:
		return false
	}
}

func TestOwnerRestartSpikeHiddenRefPreservesDirtyWorktree(t *testing.T) {
	repo := ownerRestartSpikeGitRepo(t)
	ownerRestartSpikeWriteFile(t, filepath.Join(repo, "tracked.txt"), []byte("dirty tracked\n"), 0o644)
	ownerRestartSpikeWriteFile(t, filepath.Join(repo, "untracked.txt"), []byte("dirty untracked\n"), 0o644)
	if err := os.Chmod(filepath.Join(repo, "mode.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := ownerRestartSpikeSnapshot(t, repo)
	ref := "refs/agent-harness/issueops/io-spike/attempts/2/wip"

	seal, err := ownerRestartSpikeSeal(repo, ref, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	after := ownerRestartSpikeSnapshot(t, repo)
	if before.Head != after.Head || before.BranchRef != after.BranchRef || before.IndexSHA256 != after.IndexSHA256 || before.StatusSHA256 != after.StatusSHA256 {
		t.Fatalf("seal changed checkout metadata\nbefore=%+v\nafter=%+v", before, after)
	}
	if err := ownerRestartSpikeEqualFiles(before.Files, after.Files); err != nil {
		t.Fatalf("seal changed worktree bytes or modes: %v", err)
	}
	if seal.Ref != ref || seal.BaseHead != before.Head || seal.Commit == "" || seal.Tree == "" {
		t.Fatalf("incomplete seal receipt: %+v", seal)
	}
	restore := filepath.Join(t.TempDir(), "restore")
	ownerRestartSpikeGit(t, repo, nil, "worktree", "add", "-q", "--detach", restore, seal.Commit)
	restored := ownerRestartSpikeFiles(t, restore, []string{"tracked.txt", "untracked.txt", "mode.sh"})
	if err := ownerRestartSpikeEqualFiles(before.Files, restored); err != nil {
		t.Fatalf("hidden ref does not restore sealed WIP: %v", err)
	}
}

func TestOwnerRestartSpikeRejectsStagedBeforeGitMutation(t *testing.T) {
	repo := ownerRestartSpikeGitRepo(t)
	ownerRestartSpikeWriteFile(t, filepath.Join(repo, "tracked.txt"), []byte("staged\n"), 0o644)
	ownerRestartSpikeGit(t, repo, nil, "add", "tracked.txt")
	ref := "refs/agent-harness/issueops/io-spike/attempts/2/wip"

	if _, err := ownerRestartSpikeSeal(repo, ref, t.TempDir()); err == nil || !strings.Contains(err.Error(), "staged") {
		t.Fatalf("expected staged divergence rejection, got %v", err)
	}
	if code, _, _ := ownerRestartSpikeGitRaw(repo, nil, "show-ref", "--verify", "--quiet", ref); code != 1 {
		t.Fatalf("staged rejection must happen before hidden ref creation, show-ref exit=%d", code)
	}
}

type ownerRestartSpikeFile struct {
	Bytes []byte
	Mode  os.FileMode
}

type ownerRestartSpikeGitSnapshot struct {
	Head         string
	BranchRef    string
	IndexSHA256  string
	StatusSHA256 string
	Files        map[string]ownerRestartSpikeFile
}

func ownerRestartSpikeGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	ownerRestartSpikeGit(t, repo, nil, "init", "-q", "-b", "main")
	ownerRestartSpikeGit(t, repo, nil, "config", "user.name", "Owner Restart Spike")
	ownerRestartSpikeGit(t, repo, nil, "config", "user.email", "owner-restart-spike@example.invalid")
	ownerRestartSpikeWriteFile(t, filepath.Join(repo, "tracked.txt"), []byte("clean tracked\n"), 0o644)
	ownerRestartSpikeWriteFile(t, filepath.Join(repo, "mode.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o644)
	ownerRestartSpikeGit(t, repo, nil, "add", "tracked.txt", "mode.sh")
	ownerRestartSpikeGit(t, repo, nil, "commit", "-q", "-m", "initial")
	return repo
}

func ownerRestartSpikeSeal(repo, ref, temporaryRoot string) (ownerRestartSpikeWIPSeal, error) {
	if code, _, stderr := ownerRestartSpikeGitRaw(repo, nil, "diff", "--cached", "--quiet", "--exit-code"); code != 0 {
		if code == 1 {
			return ownerRestartSpikeWIPSeal{}, fmt.Errorf("staged index divergence is not sealable")
		}
		return ownerRestartSpikeWIPSeal{}, fmt.Errorf("inspect staged index: %s", strings.TrimSpace(stderr))
	}
	head, err := ownerRestartSpikeGitOutput(repo, nil, "rev-parse", "HEAD")
	if err != nil {
		return ownerRestartSpikeWIPSeal{}, err
	}
	code, rawPaths, stderr := ownerRestartSpikeGitRaw(repo, nil, "ls-files", "-m", "-d", "-o", "--exclude-standard", "-z")
	if code != 0 {
		return ownerRestartSpikeWIPSeal{}, fmt.Errorf("list seal paths: %s", strings.TrimSpace(stderr))
	}
	paths := ownerRestartSpikeNULFields(rawPaths)
	if len(paths) == 0 {
		return ownerRestartSpikeWIPSeal{}, fmt.Errorf("dirty seal requires at least one non-ignored path")
	}
	index := filepath.Join(temporaryRoot, "owner-restart.index")
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := ownerRestartSpikeGitOutput(repo, env, "read-tree", "HEAD"); err != nil {
		return ownerRestartSpikeWIPSeal{}, err
	}
	args := append([]string{"add", "-A", "--"}, paths...)
	if _, err := ownerRestartSpikeGitOutput(repo, env, args...); err != nil {
		return ownerRestartSpikeWIPSeal{}, err
	}
	tree, err := ownerRestartSpikeGitOutput(repo, env, "write-tree")
	if err != nil {
		return ownerRestartSpikeWIPSeal{}, err
	}
	commit, err := ownerRestartSpikeGitOutput(repo, nil, "commit-tree", tree, "-p", head, "-m", "agent-harness owner WIP seal")
	if err != nil {
		return ownerRestartSpikeWIPSeal{}, err
	}
	if _, err := ownerRestartSpikeGitOutput(repo, nil, "update-ref", ref, commit, strings.Repeat("0", 40)); err != nil {
		return ownerRestartSpikeWIPSeal{}, err
	}
	return ownerRestartSpikeWIPSeal{Ref: ref, Commit: commit, Tree: tree, BaseHead: head}, nil
}

func ownerRestartSpikeSnapshot(t *testing.T, repo string) ownerRestartSpikeGitSnapshot {
	t.Helper()
	head := ownerRestartSpikeGitMustOutput(t, repo, nil, "rev-parse", "HEAD")
	branch := ownerRestartSpikeGitMustOutput(t, repo, nil, "symbolic-ref", "--short", "HEAD")
	branchRef := ownerRestartSpikeGitMustOutput(t, repo, nil, "rev-parse", "refs/heads/"+branch)
	code, status, stderr := ownerRestartSpikeGitRaw(repo, nil, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if code != 0 {
		t.Fatalf("git status: %s", stderr)
	}
	indexPath := ownerRestartSpikeGitMustOutput(t, repo, nil, "rev-parse", "--git-path", "index")
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(repo, indexPath)
	}
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	return ownerRestartSpikeGitSnapshot{
		Head: head, BranchRef: branchRef, IndexSHA256: ownerRestartSpikeSHA256(indexBytes), StatusSHA256: ownerRestartSpikeSHA256([]byte(status)),
		Files: ownerRestartSpikeFiles(t, repo, []string{"tracked.txt", "untracked.txt", "mode.sh"}),
	}
}

func ownerRestartSpikeFiles(t *testing.T, root string, paths []string) map[string]ownerRestartSpikeFile {
	t.Helper()
	result := make(map[string]ownerRestartSpikeFile, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		result[path] = ownerRestartSpikeFile{Bytes: content, Mode: info.Mode().Perm()}
	}
	return result
}

func ownerRestartSpikeEqualFiles(want, got map[string]ownerRestartSpikeFile) error {
	if len(want) != len(got) {
		return fmt.Errorf("path count got %d, want %d", len(got), len(want))
	}
	for path, expected := range want {
		actual, ok := got[path]
		if !ok {
			return fmt.Errorf("missing path %q", path)
		}
		if string(actual.Bytes) != string(expected.Bytes) || actual.Mode != expected.Mode {
			return fmt.Errorf("path %q got mode=%#o bytes=%q, want mode=%#o bytes=%q", path, actual.Mode, actual.Bytes, expected.Mode, expected.Bytes)
		}
	}
	return nil
}

func TestOwnerRestartSpikeDispatcherReconcilesWithoutDuplicateMutation(t *testing.T) {
	for _, stage := range []string{handoff.OperationTerminalCreate, handoff.OperationTaskCreate, handoff.OperationDispatch, "record_cas"} {
		t.Run(stage, func(t *testing.T) {
			spike := newOwnerRestartSpikeDispatcher(stage)
			spike.mutateOnce(t)
			if stage == "record_cas" {
				if err := spike.resume(true); !errors.Is(err, errOwnerRestartSpikeCAS) {
					t.Fatalf("expected injected CAS loss, got %v", err)
				}
			}
			if err := spike.resume(false); err != nil {
				t.Fatal(err)
			}
			if spike.externalMutations != 1 {
				t.Fatalf("resume repeated external mutation %d times", spike.externalMutations)
			}
			if spike.pendingKind != "" {
				t.Fatalf("reconciled stage retained pending operation %q", spike.pendingKind)
			}
		})
	}
}

var errOwnerRestartSpikeCAS = errors.New("injected record CAS loss")

type ownerRestartSpikeDispatcher struct {
	stage             string
	pendingKind       string
	externalMutations int
	terminals         []port.OrcaTerminal
	tasks             []port.OrcaTask
	dispatch          port.OrcaDispatch
}

const (
	ownerRestartSpikeWorkerHandle      = "term_00000000-0000-4000-8000-000000000002"
	ownerRestartSpikeCoordinatorHandle = "term_00000000-0000-4000-8000-000000000001"
)

func newOwnerRestartSpikeDispatcher(stage string) *ownerRestartSpikeDispatcher {
	pending := stage
	if stage == "record_cas" {
		pending = handoff.OperationDispatch
	}
	return &ownerRestartSpikeDispatcher{stage: stage, pendingKind: pending}
}

func (s *ownerRestartSpikeDispatcher) mutateOnce(t *testing.T) {
	t.Helper()
	s.externalMutations++
	switch s.pendingKind {
	case handoff.OperationTerminalCreate:
		s.terminals = append(s.terminals, port.OrcaTerminal{Handle: ownerRestartSpikeWorkerHandle, PTYID: "pty-2", WorktreeID: "worktree-2", WorktreePath: "/tmp/worker-2", Connected: true, Writable: true})
	case handoff.OperationTaskCreate:
		title, display, err := issueOpsHandoffTaskIdentity("io-spike", "ownership-2", 2)
		if err != nil {
			t.Fatal(err)
		}
		s.tasks = append(s.tasks, port.OrcaTask{ID: "task-2", Title: title, DisplayName: display, Status: "ready"})
	case handoff.OperationDispatch:
		s.dispatch = port.OrcaDispatch{ID: "dispatch-2", TaskID: "task-2", AssigneeHandle: ownerRestartSpikeWorkerHandle, Status: "dispatched", Injected: true, Preamble: ownerRestartSpikePreamble()}
	default:
		t.Fatalf("unknown stage %q", s.pendingKind)
	}
}

func (s *ownerRestartSpikeDispatcher) resume(failCAS bool) error {
	switch s.pendingKind {
	case handoff.OperationTerminalCreate:
		if _, err := ReconcileIssueOpsHandoffTerminal(nil, "worktree-2", "/tmp/worker-2", s.terminals); err != nil {
			return err
		}
	case handoff.OperationTaskCreate:
		title, display, err := issueOpsHandoffTaskIdentity("io-spike", "ownership-2", 2)
		if err != nil {
			return err
		}
		if _, err := ReconcileIssueOpsHandoffTask(nil, title, display, s.tasks); err != nil {
			return err
		}
	case handoff.OperationDispatch:
		if _, err := ReconcileIssueOpsHandoffDispatch(context.Background(), "task-2", ownerRestartSpikeWorkerHandle, "inject", s, ownerRestartSpikeCoordinatorHandle); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown pending stage %q", s.pendingKind)
	}
	if failCAS {
		return errOwnerRestartSpikeCAS
	}
	s.pendingKind = ""
	return nil
}

func (s *ownerRestartSpikeDispatcher) ShowDispatchFrom(context.Context, string, string) (port.OrcaDispatch, error) {
	return s.dispatch, nil
}

func ownerRestartSpikePreamble() string {
	return strings.Join([]string{
		"Your coordinator's terminal handle is: " + ownerRestartSpikeCoordinatorHandle,
		"Your task ID is: task-2",
		"orca orchestration worker_done --dispatch-id dispatch-2",
	}, "\n")
}

func ownerRestartSpikeJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func ownerRestartSpikeWriteFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func ownerRestartSpikeSHA256(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func ownerRestartSpikeNULFields(value string) []string {
	parts := strings.Split(value, "\x00")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func ownerRestartSpikeGit(t *testing.T, repo string, env []string, args ...string) {
	t.Helper()
	if _, err := ownerRestartSpikeGitOutput(repo, env, args...); err != nil {
		t.Fatal(err)
	}
}

func ownerRestartSpikeGitMustOutput(t *testing.T, repo string, env []string, args ...string) string {
	t.Helper()
	value, err := ownerRestartSpikeGitOutput(repo, env, args...)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func ownerRestartSpikeGitOutput(repo string, env []string, args ...string) (string, error) {
	code, stdout, stderr := ownerRestartSpikeGitRaw(repo, env, args...)
	if code != 0 {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func ownerRestartSpikeGitRaw(repo string, env []string, args ...string) (int, string, string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	return -1, stdout.String(), err.Error()
}
