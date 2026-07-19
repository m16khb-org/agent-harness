package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/policy"
	"agent-harness/internal/core/policy/auditid"
	corestate "agent-harness/internal/core/state"
)

const (
	processDiagnosticLimit = 1024
	processVectorLimit     = 64
	processAuditTimeout    = 2 * time.Second
)

// ProcessExecutionRequest is the host-neutral evidence required for one
// bounded external process invocation. Environment values and process output
// are deliberately excluded from this audit surface.
type ProcessExecutionRequest struct {
	Name       string
	Executable string
	Argv       []string
	CWD        string
	Timeout    time.Duration
	EnvPolicy  string
	EnvKeys    []string
	Outcome    string
	Diagnostic string
	StartedAt  time.Time
}

// ProcessExecutionRecord is an append-only, redacted process audit record.
type ProcessExecutionRecord struct {
	OK          bool     `json:"ok"`
	Kind        string   `json:"kind"`
	AuditLogID  string   `json:"audit_log_id"`
	GeneratedAt string   `json:"generated_at"`
	Name        string   `json:"name"`
	Executable  string   `json:"executable,omitempty"`
	Argv        []string `json:"argv"`
	CWD         string   `json:"cwd"`
	Timeout     string   `json:"timeout"`
	EnvPolicy   string   `json:"env_policy"`
	EnvKeys     []string `json:"env_keys"`
	Outcome     string   `json:"outcome"`
	DurationMS  int64    `json:"duration_ms"`
	Diagnostic  string   `json:"diagnostic,omitempty"`
}

// AuditProcessExecution appends one bounded process record to the harness
// state audit log. Callers must use fixed Name, EnvPolicy, and Outcome values.
func AuditProcessExecution(req ProcessExecutionRequest) (ProcessExecutionRecord, error) {
	if err := validateProcessExecutionRequest(req); err != nil {
		return ProcessExecutionRecord{}, err
	}
	now := time.Now().UTC()
	started := req.StartedAt
	if started.IsZero() {
		started = now
	}
	argv := policy.RedactArgv(req.Argv)
	for i := range argv {
		argv[i] = boundedProcessField(argv[i])
	}
	envKeys := append([]string(nil), req.EnvKeys...)
	for i := range envKeys {
		envKeys[i] = boundedProcessField(policy.RedactFreeform(envKeys[i]))
	}
	record := ProcessExecutionRecord{
		OK:          req.Outcome == "success",
		Kind:        "process_execution_audit",
		AuditLogID:  auditid.Generate(req.CWD, req.CWD, argv),
		GeneratedAt: now.Format(time.RFC3339Nano),
		Name:        boundedProcessField(req.Name),
		Executable:  boundedProcessField(policy.RedactFreeform(req.Executable)),
		Argv:        argv,
		CWD:         boundedProcessField(policy.RedactFreeform(req.CWD)),
		Timeout:     req.Timeout.String(),
		EnvPolicy:   boundedProcessField(req.EnvPolicy),
		EnvKeys:     envKeys,
		Outcome:     boundedProcessField(req.Outcome),
		DurationMS:  max(0, now.Sub(started).Milliseconds()),
		Diagnostic:  boundedProcessDiagnostic(req.Diagnostic),
	}
	path := filepath.Join(corestate.StateDir(), "audit", "process-execution.jsonl")
	if err := appendProcessAudit(path, record); err != nil {
		return record, err
	}
	return record, nil
}

func appendProcessAudit(path string, record ProcessExecutionRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), processAuditTimeout)
	defer cancel()
	return corestate.WithKeyLock(ctx, corestate.StateDir(), "process-execution-audit", func(context.Context) error {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Write(append(line, '\n'))
		return err
	})
}

func boundedProcessField(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > processDiagnosticLimit {
		return value[:processDiagnosticLimit] + "..."
	}
	return value
}

func boundedProcessDiagnostic(value string) string {
	return boundedProcessField(policy.RedactDiagnostic(value))
}

func validateProcessExecutionRequest(req ProcessExecutionRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.EnvPolicy) == "" || strings.TrimSpace(req.Outcome) == "" {
		return fmt.Errorf("process audit requires name, env policy, and outcome")
	}
	if len(req.Argv) > processVectorLimit || len(req.EnvKeys) > processVectorLimit {
		return fmt.Errorf("process audit vector limit exceeded")
	}
	return nil
}
