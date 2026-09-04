package issueops

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"issueops/internal/contract/issueops"
)

func workspaceSnapshot(workspace issueops.Workspace) (string, error) {
	info, err := os.Lstat(workspace.Root)
	if errors.Is(err, os.ErrNotExist) {
		// 부재는 quiescence의 약한 증거가 아니라 가장 강한 증거다. 존재하지
		// 않는 디렉터리에는 프로세스가 cwd를 둘 수 없고 쓸 것도 없다.
		//
		// 예전에는 이것을 "관측 불가"로 거부했고, 그래서 worktree가 lease
		// active 상태에서 제거된 lifecycle은 어떤 typed 경로로도 회수할 수
		// 없었다 — replace는 worktree를, abandon은 terminal lease를 요구하고,
		// terminal로 만들려면 replace가 필요하다(#435).
		//
		// 부재라는 사실 자체를 경로에 결속해 봉인한다. finalize 직전에
		// worktree가 되살아나면 fingerprint가 달라져 stale로 멈춘다.
		return workspaceAbsenceSnapshot(workspace), nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		// symlink나 파일이 그 경로를 차지한 것은 부재가 아니라 정체 불명이다.
		return "", fmt.Errorf("canonical worktree must be a real directory")
	}
	top, err := gitOutput(workspace.Root, "rev-parse", "--show-toplevel")
	if err != nil || !samePath(top, workspace.Root) {
		return "", fmt.Errorf("canonical worktree root does not match Git top-level")
	}
	branch, err := gitOutput(workspace.Root, "branch", "--show-current")
	if err != nil || branch != workspace.Branch {
		return "", fmt.Errorf("canonical worktree branch mismatch")
	}
	head, err := gitOutput(workspace.Root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commonDir, err := gitOutput(workspace.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	indexPath, err := gitOutput(workspace.Root, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(workspace.Root, indexPath)
	}
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	code, tracked, stderr := GitCmdRaw(workspace.Root, "diff", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", fmt.Errorf("read tracked diff: %s", strings.TrimSpace(stderr))
	}
	code, staged, stderr := GitCmdRaw(workspace.Root, "diff", "--cached", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", fmt.Errorf("read staged diff: %s", strings.TrimSpace(stderr))
	}
	code, untrackedRaw, stderr := GitCmdRaw(workspace.Root, "ls-files", "--others", "--exclude-standard", "-z")
	if code != 0 {
		return "", fmt.Errorf("list untracked files: %s", strings.TrimSpace(stderr))
	}
	untracked := strings.Split(strings.TrimSuffix(untrackedRaw, "\x00"), "\x00")
	if len(untracked) == 1 && untracked[0] == "" {
		untracked = nil
	}
	sort.Strings(untracked)
	hash := sha256.New()
	writeFingerprintPart(hash, workspace.Root)
	writeFingerprintPart(hash, commonDir)
	writeFingerprintPart(hash, branch)
	writeFingerprintPart(hash, head)
	writeFingerprintBytes(hash, indexBytes)
	writeFingerprintPart(hash, tracked)
	writeFingerprintPart(hash, staged)
	for _, relative := range untracked {
		if filepath.IsAbs(relative) || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("unsafe untracked path %q", relative)
		}
		path := filepath.Join(workspace.Root, filepath.FromSlash(relative))
		entry, err := os.Lstat(path)
		if err != nil || entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
			return "", fmt.Errorf("untracked path must be a regular file: %s", relative)
		}
		writeFingerprintPart(hash, relative)
		writeFingerprintPart(hash, entry.Mode().String())
		if err := writeFingerprintFile(hash, path, entry); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func gitOutput(root string, args ...string) (string, error) {
	code, stdout, stderr := GitCmd(root, args...)
	if code != 0 {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	return strings.TrimSpace(stdout), nil
}

type fingerprintWriter interface {
	Write([]byte) (int, error)
}

func writeFingerprintPart(hash fingerprintWriter, value string) {
	writeFingerprintBytes(hash, []byte(value))
}

func writeFingerprintBytes(hash fingerprintWriter, value []byte) {
	_, _ = hash.Write([]byte(strconv.Itoa(len(value))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	_, _ = hash.Write([]byte{0})
}

func writeFingerprintFile(hash fingerprintWriter, path string, entry os.FileInfo) (err error) {
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

func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func claimTokenPath(record issueops.IssueOpsRecord) string {
	key := tokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}

func createClaimToken(record issueops.IssueOpsRecord) (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw)
	path := claimTokenPath(record)
	if err := secureMkdirAll(record.Execution.Workspace.Root, filepath.Dir(path)); err != nil {
		return "", "", err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", err
	}
	_, writeErr := file.WriteString(token + "\n")
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return "", "", writeErr
		}
		return "", "", closeErr
	}
	return token, path, nil
}

func readExecutionLeaseToken(record issueops.IssueOpsRecord, path string) (string, error) {
	expected := claimTokenPath(record)
	if !samePath(path, expected) {
		return "", fmt.Errorf("claim_token_file must be the deterministic current-generation path")
	}
	info, err := os.Lstat(expected)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return "", fmt.Errorf("claim token file must be a 0600 regular file")
	}
	if info.Size() > 256 {
		return "", fmt.Errorf("claim token file is oversized")
	}
	data, err := os.ReadFile(expected)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("claim token file is empty")
	}
	return token, nil
}

func tokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// cleanupReplacementGeneration은 durable lease가 아직 권한을 부여하지 않은
// target generation의 harness-owned 파일만 지운다. finalize는 revoking 세대,
// reseed는 아직 persist되지 않은 다음 세대이므로 재시도 전에 회수해도 된다.
func cleanupReplacementGeneration(record issueops.IssueOpsRecord) error {
	if record.Execution == nil {
		return fmt.Errorf("cannot clean replacement residue without an execution")
	}
	paths := []string{claimTokenPath(record)}
	if record.Execution.Mode == issueops.ExecutionModeOrca && record.Execution.Orca != nil {
		packetPath, promptPath := executionOwnerArtifactPaths(record)
		paths = append(paths, packetPath, promptPath)
	}
	for _, path := range paths {
		if err := removeReplacementRuntimeFile(record.Execution.Workspace.Root, path); err != nil {
			return fmt.Errorf("clean uncommitted replacement residue %s: %w", path, err)
		}
	}
	return nil
}

func cleanupReplacementFailure(record issueops.IssueOpsRecord, cause error) error {
	if cleanupErr := cleanupReplacementGeneration(record); cleanupErr != nil {
		return fmt.Errorf("%w; replacement residue cleanup failed: %v", cause, cleanupErr)
	}
	return cause
}

func removeReplacementRuntimeFile(root, path string) error {
	// worktree가 통째로 없으면 지울 잔여물도 없다. 부모 디렉터리를 만들려
	// 시도하면 없는 worktree를 되살리려다 실패해 finalize가 멈춘다 — 그러면
	// lease가 revoking에 갇히고 abandon은 claimable/released만 받으므로 다시
	// 막다른 길이 된다(#435).
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := secureMkdirAll(root, filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("replacement residue path is a directory")
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

func secureMkdirAll(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("runtime token directory escapes the canonical worktree")
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o700); err != nil {
				return err
			}
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("runtime token directory must contain only real directories")
		}
	}
	return nil
}

// workspaceAbsenceSnapshot은 canonical worktree가 없다는 관측을 정확한 경로에
// 결속해 봉인한다. 서로 다른 부재가 같은 값을 내면 다른 lifecycle의 증거를
// 재사용할 수 있으므로 경로와 branch를 함께 넣는다.
func workspaceAbsenceSnapshot(workspace issueops.Workspace) string {
	hash := sha256.New()
	writeFingerprintPart(hash, "workspace-absent")
	writeFingerprintPart(hash, workspace.Root)
	writeFingerprintPart(hash, workspace.Branch)
	return hex.EncodeToString(hash.Sum(nil))
}

// workspaceRootAbsent는 canonical worktree가 사라졌는지 보고한다. symlink나
// 파일이 그 경로를 차지한 경우는 부재가 아니라 정체 불명이므로 false다 —
// 그 상태는 workspaceSnapshot이 별도로 거부한다.
func workspaceRootAbsent(root string) bool {
	_, err := os.Lstat(root)
	return errors.Is(err, os.ErrNotExist)
}
