package executioncmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"issueops/internal/port"
)

const executionIssueSnapshotFileLimit = 1 << 20

func readExecutionIssueSnapshotFile(path string) (*port.ExecutionIssueSnapshotEvidence, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect issue snapshot file: %w", err)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("issue snapshot file must be a regular non-symlink file")
	}
	if before.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("issue snapshot file mode must be 0600")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open issue snapshot file: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat opened issue snapshot file: %w", err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("issue snapshot file changed between inspection and open")
	}
	data, err := io.ReadAll(io.LimitReader(file, executionIssueSnapshotFileLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read issue snapshot file: %w", err)
	}
	if len(data) > executionIssueSnapshotFileLimit {
		return nil, fmt.Errorf("issue snapshot file exceeds %d bytes", executionIssueSnapshotFileLimit)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var evidence *port.ExecutionIssueSnapshotEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return nil, fmt.Errorf("decode issue snapshot file: %w", err)
	}
	if evidence == nil {
		return nil, fmt.Errorf("issue snapshot file must contain one JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("issue snapshot file must contain exactly one JSON object")
		}
		return nil, fmt.Errorf("decode trailing issue snapshot data: %w", err)
	}
	return evidence, nil
}
