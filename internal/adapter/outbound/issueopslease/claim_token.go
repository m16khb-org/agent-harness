package issueopslease

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

func claimTokenPath(record leasecontract.Record) string {
	key := claimTokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".agent-harness", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}

func readCurrentClaimToken(record leasecontract.Record, path string) (string, error) {
	expected := claimTokenPath(record)
	if !(FilesystemPathMatcher{}).Matches(path, expected) {
		return "", fmt.Errorf("claim_token_file must be the deterministic current-generation path")
	}
	if err := requirePrivateRegularFile(record.Execution.Workspace.Root, expected, 256); err != nil {
		return "", err
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

func requirePrivateRegularFile(root, path string, maxSize int64) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("claim token file must be inside the canonical worktree")
	}
	current := root
	for index, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("claim token file must be a 0600 regular file")
		}
		if index < len(strings.Split(rel, string(filepath.Separator)))-1 && !info.IsDir() {
			return fmt.Errorf("claim token file must be a 0600 regular file")
		}
		if index == len(strings.Split(rel, string(filepath.Separator)))-1 {
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				return fmt.Errorf("claim token file must be a 0600 regular file")
			}
			if info.Size() > maxSize {
				return fmt.Errorf("claim token file is oversized")
			}
		}
	}
	return nil
}

func claimTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
