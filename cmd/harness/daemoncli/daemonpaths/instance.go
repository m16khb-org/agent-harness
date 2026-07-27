package daemonpaths

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// InstanceRecord는 daemon 생명주기 동작을 정확히 하나의 OS 프로세스와 정확히
// 하나의 daemon protocol 인스턴스에 묶는다. 필드는 additive이므로 구버전
// 바이너리도 legacy 정수 PID 형식을 status 용도로만 계속 읽을 수 있다.
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

// ReadInstance는 과거의 정수 PID 파일에 대해서만 legacy=true를 반환한다. 호출자는
// 그 PID를 표시할 수는 있으나 파괴적 동작에 사용해서는 안 된다.
func ReadInstance(path string) (record InstanceRecord, legacy bool, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return InstanceRecord{}, false, err
	}
	trimmed := strings.TrimSpace(string(b))
	if pid, parseErr := strconv.Atoi(trimmed); parseErr == nil {
		if pid <= 0 {
			return InstanceRecord{}, true, fmt.Errorf("legacy pid must be positive")
		}
		return InstanceRecord{PID: pid}, true, nil
	}
	if err := json.Unmarshal(b, &record); err != nil {
		return InstanceRecord{}, false, fmt.Errorf("decode daemon instance record: %w", err)
	}
	if err := record.Validate(); err != nil {
		return InstanceRecord{}, false, fmt.Errorf("invalid daemon instance record: %w", err)
	}
	return record, false, nil
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
