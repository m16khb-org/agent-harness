package issueopslease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	leaseapp "agent-harness/internal/application/issueopslease"
	leasecontract "agent-harness/internal/contract/issueopslease"
	leasedomain "agent-harness/internal/domain/issueopslease"
	"agent-harness/internal/port"
)

type ReseedInventory struct {
	owner   port.ExecutionOrcaOwnerInspector
	inspect leaseapp.ProcessInspector
}

func NewReseedInventory(owner port.ExecutionOrcaOwnerInspector, inspect leaseapp.ProcessInspector) *ReseedInventory {
	return &ReseedInventory{owner: owner, inspect: inspect}
}

func (a *ReseedInventory) Observe(ctx context.Context, record leasecontract.Record, requester leasedomain.Actor) (leaseapp.ReseedInventoryReceipt, error) {
	if a == nil || a.inspect == nil || record.Execution == nil {
		return leaseapp.ReseedInventoryReceipt{}, fmt.Errorf("reseed inventory requires an execution and process inspector")
	}
	snapshot, err := reseedWorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return leaseapp.ReseedInventoryReceipt{}, err
	}
	processStatus := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		processStatus, _, err = a.inspect(ctx, leasedomain.ProcessReceipt{PID: holder.SessionProcess.PID, StartedAt: holder.SessionProcess.StartedAt, Executable: holder.SessionProcess.Executable})
		if err != nil {
			return leaseapp.ReseedInventoryReceipt{}, err
		}
	}
	orcaInventory := port.ExecutionOrcaOwnerInventory{}
	if record.Execution.Mode == "orca" {
		if record.Execution.Orca == nil || a.owner == nil {
			return leaseapp.ReseedInventoryReceipt{}, fmt.Errorf("Orca execution requires exact owner terminal and task inventory")
		}
		binding := record.Execution.Orca
		orcaInventory, err = a.owner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
			RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
			DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
			AllowRuntimeRollover: record.Execution.Lease.Holder == nil &&
				(record.Execution.Lease.Status == "released" || record.Execution.Lease.Status == "claimable"),
		})
		if err != nil {
			return leaseapp.ReseedInventoryReceipt{}, err
		}
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Status     string                           `json:"status"`
		Holder     *leasecontract.Actor             `json:"holder,omitempty"`
		Requester  leasecontract.Actor              `json:"requester"`
		Process    string                           `json:"process_status"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, reseedInventoryActor(requester), processStatus, orcaInventory, snapshot}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return leaseapp.ReseedInventoryReceipt{}, err
	}
	sum := sha256.Sum256(encoded)
	return leaseapp.ReseedInventoryReceipt{
		Fingerprint: hex.EncodeToString(sum[:]), RuntimeID: strings.TrimSpace(orcaInventory.RuntimeID),
		Inventory: leasedomain.ResumeInventory{
			RuntimeID: orcaInventory.RuntimeID, TerminalLive: orcaInventory.TerminalLive,
			TerminalInventoryComplete: orcaInventory.TerminalInventoryComplete, TaskLive: orcaInventory.TaskLive,
			TerminalID: orcaInventory.TerminalID, TaskStatus: orcaInventory.TaskStatus, DispatchStatus: orcaInventory.DispatchStatus,
			DispatchAssigneeHandle: orcaInventory.DispatchAssigneeHandle, DispatchAssigneePresent: orcaInventory.DispatchAssigneePresent,
		},
	}, nil
}

func reseedInventoryActor(actor leasedomain.Actor) leasecontract.Actor {
	result := leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}

func reseedWorkspaceSnapshot(workspace leasecontract.Workspace) (string, error) {
	info, err := os.Lstat(workspace.Root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("canonical worktree must be a real directory")
	}
	top, err := reseedGitOutput(workspace.Root, "rev-parse", "--show-toplevel")
	if err != nil || !(FilesystemPathMatcher{}).Matches(top, workspace.Root) {
		return "", fmt.Errorf("canonical worktree root does not match Git top-level")
	}
	branch, err := reseedGitOutput(workspace.Root, "branch", "--show-current")
	if err != nil || branch != workspace.Branch {
		return "", fmt.Errorf("canonical worktree branch mismatch")
	}
	head, err := reseedGitOutput(workspace.Root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commonDir, err := reseedGitOutput(workspace.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	indexPath, err := reseedGitOutput(workspace.Root, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(workspace.Root, indexPath)
	}
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	code, tracked, stderr := reseedGitRaw(workspace.Root, "diff", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", fmt.Errorf("read tracked diff: %s", strings.TrimSpace(stderr))
	}
	code, staged, stderr := reseedGitRaw(workspace.Root, "diff", "--cached", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", fmt.Errorf("read staged diff: %s", strings.TrimSpace(stderr))
	}
	code, untrackedRaw, stderr := reseedGitRaw(workspace.Root, "ls-files", "--others", "--exclude-standard", "-z")
	if code != 0 {
		return "", fmt.Errorf("list untracked files: %s", strings.TrimSpace(stderr))
	}
	untracked := strings.Split(strings.TrimSuffix(untrackedRaw, "\x00"), "\x00")
	if len(untracked) == 1 && untracked[0] == "" {
		untracked = nil
	}
	sort.Strings(untracked)
	hash := sha256.New()
	reseedWriteFingerprintPart(hash, workspace.Root)
	reseedWriteFingerprintPart(hash, commonDir)
	reseedWriteFingerprintPart(hash, branch)
	reseedWriteFingerprintPart(hash, head)
	reseedWriteFingerprintBytes(hash, indexBytes)
	reseedWriteFingerprintPart(hash, tracked)
	reseedWriteFingerprintPart(hash, staged)
	for _, relative := range untracked {
		if filepath.IsAbs(relative) || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("unsafe untracked path %q", relative)
		}
		path := filepath.Join(workspace.Root, filepath.FromSlash(relative))
		entry, err := os.Lstat(path)
		if err != nil || entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
			return "", fmt.Errorf("untracked path must be a regular file: %s", relative)
		}
		reseedWriteFingerprintPart(hash, relative)
		reseedWriteFingerprintPart(hash, entry.Mode().String())
		if err := reseedWriteFingerprintFile(hash, path, entry); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func reseedGitOutput(root string, args ...string) (string, error) {
	code, stdout, stderr := reseedGitRaw(root, args...)
	if code != 0 {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	return strings.TrimSpace(stdout), nil
}

func reseedGitRaw(root string, args ...string) (int, string, string) {
	command := exec.Command("git", args...)
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	return 1, stdout.String(), err.Error()
}

func reseedWriteFingerprintPart(hash interface{ Write([]byte) (int, error) }, value string) {
	reseedWriteFingerprintBytes(hash, []byte(value))
}

func reseedWriteFingerprintBytes(hash interface{ Write([]byte) (int, error) }, value []byte) {
	_, _ = hash.Write([]byte(strconv.Itoa(len(value))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	_, _ = hash.Write([]byte{0})
}

func reseedWriteFingerprintFile(hash interface{ Write([]byte) (int, error) }, path string, entry os.FileInfo) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(entry, opened) || !opened.Mode().IsRegular() || opened.Size() != entry.Size() {
		return fmt.Errorf("untracked file changed while snapshotting: %s", path)
	}
	_, _ = hash.Write([]byte(strconv.FormatInt(opened.Size(), 10)))
	_, _ = hash.Write([]byte{0})
	if _, err := io.CopyN(hash, file, opened.Size()); err != nil {
		return err
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return fmt.Errorf("untracked file changed while snapshotting: %s", path)
	}
	_, _ = hash.Write([]byte{0})
	return nil
}
