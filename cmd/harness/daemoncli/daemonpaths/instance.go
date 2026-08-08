package daemonpaths

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstanceRecord는 daemon 생명주기 동작을 정확히 하나의 OS 프로세스와 정확히
// 하나의 daemon protocol 인스턴스에 묶는다.
type InstanceRecord struct {
	PID              int    `json:"pid"`
	ProcessStartTime string `json:"process_start_time"`
	Executable       string `json:"executable"`
	InstanceNonce    string `json:"instance_nonce"`
	BuildSHA         string `json:"build_sha"`
	ProtocolVersion  string `json:"protocol_version"`
	Generation       string `json:"generation"`
}

func (r InstanceRecord) Validate() error {
	if r.PID <= 0 {
		return fmt.Errorf("pid must be positive")
	}
	for name, value := range map[string]string{
		"process_start_time": r.ProcessStartTime,
		"executable":         r.Executable,
		"instance_nonce":     r.InstanceNonce,
		"build_sha":          r.BuildSHA,
		"protocol_version":   r.ProtocolVersion,
		"generation":         r.Generation,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if !filepath.IsAbs(r.Executable) {
		return fmt.Errorf("executable must be absolute")
	}
	return nil
}

func ReadInstance(path string) (record InstanceRecord, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return InstanceRecord{}, err
	}
	if err := json.Unmarshal(b, &record); err != nil {
		return InstanceRecord{}, fmt.Errorf("decode daemon instance record: %w", err)
	}
	if err := record.Validate(); err != nil {
		return InstanceRecord{}, fmt.Errorf("invalid daemon instance record: %w", err)
	}
	return record, nil
}

func WriteInstance(path string, record InstanceRecord) error {
	if err := record.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

type ProcessIdentity struct {
	StartTime            string
	Executable           string
	ExecutablePathStable bool
}

func canonicalExecutable(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("process executable is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
