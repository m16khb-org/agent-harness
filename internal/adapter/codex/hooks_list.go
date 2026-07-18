package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agent-harness/internal/core/audit"
	"agent-harness/internal/core/policy"
)

const (
	hooksListTimeout      = 15 * time.Second
	hooksListStdoutLimit  = 1 << 20
	hooksListStderrLimit  = 32 << 10
	hooksListLineLimit    = 512 << 10
	hooksListObjectLimit  = 256
	hooksListHookLimit    = 1024
	hooksListMessageLimit = 256
	hooksListStringLimit  = 4096
	hooksListDepthLimit   = 32
	hooksListNodeLimit    = 32 << 10
	hooksListEncodedLimit = 1 << 20
	hooksListAuditIDSpace = 128
	initializeMessage     = `{"method":"initialize","id":1,"params":{"clientInfo":{"name":"agent_harness","title":"agent-harness","version":"1"}}}`
	initializedMessage    = `{"method":"initialized","params":{}}`
)

// HooksListResult is the bounded, read-only trust inventory returned by the
// Codex app server for one persisted IssueOps worker root.
type HooksListResult struct {
	OK         bool           `json:"ok"`
	AuditLogID string         `json:"audit_log_id,omitempty"`
	Data       []HooksListCWD `json:"data"`
}

type HooksListCWD struct {
	CWD      string           `json:"cwd"`
	Hooks    []map[string]any `json:"hooks"`
	Warnings []string         `json:"warnings"`
	Errors   []string         `json:"errors"`
}

// ListHooks owns the complete Codex app-server transport. Callers provide only
// the persisted worker root; executable, argv, protocol messages, timeout, and
// stdin are fixed here.
func ListHooks(parent context.Context, workerRoot string) (result HooksListResult, retErr error) {
	if !filepath.IsAbs(workerRoot) || filepath.Clean(workerRoot) != workerRoot {
		return HooksListResult{}, fmt.Errorf("Codex hooks/list requires an exact absolute worker root")
	}
	ctx, cancel := context.WithTimeout(parent, hooksListTimeout)
	defer cancel()
	started := time.Now().UTC()
	executable := ""
	environment := hooksListEnvironment(os.Environ())
	defer func() {
		record, err := audit.AuditProcessExecution(audit.ProcessExecutionRequest{
			Name:       "codex-hooks-list",
			Executable: executable,
			Argv:       hooksListArgv(executable, workerRoot),
			CWD:        workerRoot,
			Timeout:    hooksListTimeout,
			EnvPolicy:  "codex_hooks_list_v1",
			EnvKeys:    hooksListEnvironmentKeys(environment),
			Outcome:    hooksListAuditOutcome(ctx.Err(), retErr),
			Diagnostic: hooksListErrorString(retErr),
			StartedAt:  started,
		})
		if err != nil {
			if retErr == nil {
				retErr = fmt.Errorf("write Codex hooks/list process audit: %s", boundedHooksListError(err))
			} else {
				retErr = fmt.Errorf("%w; Codex hooks/list process audit failed", retErr)
			}
			return
		}
		result.AuditLogID = record.AuditLogID
		if err := validateHooksListEncodedSize(result); err != nil {
			result = HooksListResult{}
			retErr = err
		}
	}()

	var err error
	executable, err = resolveCodexExecutable()
	if err != nil {
		return HooksListResult{}, err
	}

	process, err := startHooksListProcess(ctx, executable, workerRoot, environment)
	if err != nil {
		return HooksListResult{}, err
	}
	defer func() {
		retErr = process.finish(ctx.Err(), retErr)
	}()

	exchange, err := newHooksListExchange(process, workerRoot)
	if err != nil {
		return HooksListResult{}, err
	}
	result, err = exchange.run()
	if err != nil {
		return HooksListResult{}, err
	}
	if err := process.wait(); err != nil {
		return HooksListResult{}, err
	}
	return result, nil
}

func hooksListArgv(executable, workerRoot string) []string {
	if executable == "" {
		executable = "codex"
	}
	return []string{executable, "-C", workerRoot, "app-server", "--stdio"}
}

func resolveCodexExecutable() (string, error) {
	path, err := exec.LookPath("codex")
	if err != nil {
		return "", fmt.Errorf("resolve installed Codex executable: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute Codex executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve Codex executable target: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func hooksListEnvironment(environ []string) []string {
	allowed := map[string]bool{
		"CODEX_HOME": true, "HOME": true, "LANG": true, "LC_ALL": true,
		"LC_CTYPE": true, "LOGNAME": true, "NO_COLOR": true, "TERM": true,
		"TMP": true, "TEMP": true, "TMPDIR": true, "USER": true,
		"XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_STATE_HOME": true,
	}
	values := map[string]string{}
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			values[name] = value
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, name+"="+values[name])
	}
	return out
}

func hooksListEnvironmentKeys(environment []string) []string {
	keys := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		keys = append(keys, name)
	}
	return keys
}

func hooksListAuditOutcome(ctxErr, retErr error) string {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return "timeout"
	}
	if retErr != nil {
		return "failed"
	}
	return "success"
}

func hooksListErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func boundedHooksListError(err error) string {
	value := policy.RedactDiagnostic(err.Error())
	if len(value) > 1024 {
		return value[:1024] + "..."
	}
	return value
}

type hooksListProcess struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderrCapture *boundedCapture
	stderrDone    chan struct{}
	waited        bool
	stdinClosed   bool
	stderrRead    bool
}

func startHooksListProcess(ctx context.Context, executable, workerRoot string, environment []string) (*hooksListProcess, error) {
	argv := hooksListArgv(executable, workerRoot)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	configureHooksListProcess(cmd)
	cmd.Cancel = func() error { return terminateHooksListProcess(cmd) }
	cmd.WaitDelay = time.Second
	cmd.Dir = workerRoot
	cmd.Env = append([]string(nil), environment...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open Codex app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open Codex app-server stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("open Codex app-server stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	process := &hooksListProcess{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderrCapture: &boundedCapture{limit: hooksListStderrLimit},
		stderrDone:    make(chan struct{}),
	}
	go func() {
		_, _ = io.Copy(process.stderrCapture, stderr)
		close(process.stderrDone)
	}()
	return process, nil
}

func (process *hooksListProcess) closeInput() error {
	if err := process.stdin.Close(); err != nil {
		return fmt.Errorf("close Codex app-server stdin: %w", err)
	}
	process.stdinClosed = true
	return nil
}

func (process *hooksListProcess) wait() error {
	err := process.cmd.Wait()
	process.waited = true
	process.awaitStderr()
	if err != nil {
		return fmt.Errorf("Codex app-server exited unsuccessfully: %w", err)
	}
	return nil
}

func (process *hooksListProcess) awaitStderr() {
	if !process.stderrRead {
		<-process.stderrDone
		process.stderrRead = true
	}
}

func (process *hooksListProcess) finish(ctxErr error, retErr error) error {
	if !process.stdinClosed {
		_ = process.stdin.Close()
	}
	if !process.waited {
		_ = terminateHooksListProcess(process.cmd)
		_ = process.cmd.Wait()
	}
	process.awaitStderr()
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		retErr = fmt.Errorf("Codex hooks/list timed out after %s", hooksListTimeout)
	}
	return process.appendStderr(retErr)
}

func (process *hooksListProcess) appendStderr(retErr error) error {
	if retErr == nil || process.stderrCapture.Len() == 0 {
		return retErr
	}
	diagnostic := policy.RedactDiagnostic(strings.TrimSpace(process.stderrCapture.String()))
	if process.stderrCapture.truncated {
		diagnostic += " [truncated at 32768 bytes]"
	}
	return fmt.Errorf("%w: Codex stderr: %s", retErr, diagnostic)
}

type hooksListExchange struct {
	process        *hooksListProcess
	workerRoot     string
	hooksRequest   string
	reader         *bufio.Reader
	stdoutBytes    int
	objects        int
	seenInitialize bool
	seenHooks      bool
	result         HooksListResult
}

func newHooksListExchange(process *hooksListProcess, workerRoot string) (*hooksListExchange, error) {
	encodedWorkerRoot, err := json.Marshal(workerRoot)
	if err != nil {
		return nil, fmt.Errorf("encode persisted Codex worker root: %w", err)
	}
	return &hooksListExchange{
		process:      process,
		workerRoot:   workerRoot,
		hooksRequest: `{"method":"hooks/list","id":2,"params":{"cwds":[` + string(encodedWorkerRoot) + `]}}`,
		reader:       bufio.NewReaderSize(process.stdout, 64<<10),
	}, nil
}

func (exchange *hooksListExchange) run() (HooksListResult, error) {
	if err := writeHooksListMessage(exchange.process.stdin, initializeMessage); err != nil {
		return HooksListResult{}, err
	}
	for {
		done, err := exchange.consumeNext()
		if err != nil {
			return HooksListResult{}, err
		}
		if done {
			break
		}
	}
	if !exchange.seenInitialize || !exchange.seenHooks {
		return HooksListResult{}, fmt.Errorf("Codex app-server closed before both hooks/list responses arrived")
	}
	return exchange.result, nil
}

func (exchange *hooksListExchange) consumeNext() (bool, error) {
	line, err := readHooksListLine(exchange.reader, &exchange.stdoutBytes)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	exchange.objects++
	if exchange.objects > hooksListObjectLimit {
		return false, fmt.Errorf("Codex hooks/list response object limit exceeded (%d)", hooksListObjectLimit)
	}
	envelope, err := decodeHooksListEnvelope(line)
	if err != nil {
		return false, err
	}
	id, response, err := hooksListResponseID(envelope)
	if err != nil || !response {
		return false, err
	}
	return false, exchange.acceptResponse(id, envelope)
}

func (exchange *hooksListExchange) acceptResponse(id int, envelope map[string]json.RawMessage) error {
	if rawError, exists := envelope["error"]; exists && string(bytes.TrimSpace(rawError)) != "null" {
		return fmt.Errorf("Codex JSON-RPC error: %s", boundedRPCError(rawError))
	}
	rawResult, hasResult := envelope["result"]
	if !hasResult || string(bytes.TrimSpace(rawResult)) == "null" {
		return fmt.Errorf("Codex response id %d is missing a successful result", id)
	}
	switch id {
	case 1:
		return exchange.acceptInitialize()
	case 2:
		return exchange.acceptHooks(rawResult)
	default:
		return fmt.Errorf("unexpected response id %d from Codex app-server", id)
	}
}

func (exchange *hooksListExchange) acceptInitialize() error {
	if exchange.seenInitialize {
		return fmt.Errorf("duplicate response id 1 from Codex app-server")
	}
	if exchange.seenHooks {
		return fmt.Errorf("response id 1 arrived after hooks/list")
	}
	exchange.seenInitialize = true
	if err := writeHooksListMessage(exchange.process.stdin, initializedMessage); err != nil {
		return err
	}
	return writeHooksListMessage(exchange.process.stdin, exchange.hooksRequest)
}

func (exchange *hooksListExchange) acceptHooks(rawResult json.RawMessage) error {
	if !exchange.seenInitialize {
		return fmt.Errorf("Codex response id 2 arrived before initialize succeeded")
	}
	if exchange.seenHooks {
		return fmt.Errorf("duplicate response id 2 from Codex app-server")
	}
	parsed, err := parseHooksListResult(rawResult, exchange.workerRoot)
	if err != nil {
		return err
	}
	exchange.result = parsed
	exchange.seenHooks = true
	return exchange.process.closeInput()
}

func writeHooksListMessage(w io.Writer, message string) error {
	if _, err := io.WriteString(w, message+"\n"); err != nil {
		return fmt.Errorf("write fixed Codex app-server JSONL message: %w", err)
	}
	return nil
}

func readHooksListLine(reader *bufio.Reader, total *int) ([]byte, error) {
	line := make([]byte, 0, 1024)
	for {
		fragment, err := reader.ReadSlice('\n')
		*total += len(fragment)
		if *total > hooksListStdoutLimit {
			return nil, fmt.Errorf("Codex hooks/list stdout limit exceeded (%d bytes)", hooksListStdoutLimit)
		}
		line = append(line, fragment...)
		if len(line) > hooksListLineLimit {
			return nil, fmt.Errorf("Codex hooks/list JSONL line limit exceeded (%d bytes)", hooksListLineLimit)
		}
		switch {
		case err == nil:
			line = bytes.TrimSuffix(line, []byte{'\n'})
			line = bytes.TrimSuffix(line, []byte{'\r'})
			if len(bytes.TrimSpace(line)) == 0 {
				return nil, fmt.Errorf("malformed Codex app-server JSONL: empty line")
			}
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return nil, io.EOF
			}
			if len(bytes.TrimSpace(line)) == 0 {
				return nil, fmt.Errorf("malformed Codex app-server JSONL: empty trailing line")
			}
			return line, nil
		default:
			return nil, fmt.Errorf("read Codex app-server stdout: %w", err)
		}
	}
}

func decodeHooksListEnvelope(line []byte) (map[string]json.RawMessage, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, fmt.Errorf("malformed Codex app-server JSONL: %w", err)
	}
	if envelope == nil {
		return nil, fmt.Errorf("malformed Codex app-server JSONL: expected an object")
	}
	return envelope, nil
}

func hooksListResponseID(envelope map[string]json.RawMessage) (int, bool, error) {
	raw, exists := envelope["id"]
	if !exists || string(bytes.TrimSpace(raw)) == "null" {
		method, ok := envelope["method"]
		if !ok {
			return 0, false, fmt.Errorf("malformed Codex app-server notification without method")
		}
		var name string
		if err := json.Unmarshal(method, &name); err != nil || strings.TrimSpace(name) == "" {
			return 0, false, fmt.Errorf("malformed Codex app-server notification method")
		}
		if _, hasResult := envelope["result"]; hasResult {
			return 0, false, fmt.Errorf("malformed Codex app-server notification contains result")
		}
		if _, hasError := envelope["error"]; hasError {
			return 0, false, fmt.Errorf("malformed Codex app-server notification contains error")
		}
		return 0, false, nil
	}
	switch string(bytes.TrimSpace(raw)) {
	case "1":
		return 1, true, nil
	case "2":
		return 2, true, nil
	default:
		return 0, true, fmt.Errorf("unexpected response id from Codex app-server")
	}
}

func boundedRPCError(raw json.RawMessage) string {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "malformed error object"
	}
	message, _ := value["message"].(string)
	if len(message) > hooksListStringLimit {
		message = message[:hooksListStringLimit] + " [truncated]"
	}
	message = policy.RedactDiagnostic(message)
	if strings.TrimSpace(message) == "" {
		return "unspecified app-server error"
	}
	return message
}

func parseHooksListResult(raw json.RawMessage, workerRoot string) (HooksListResult, error) {
	var rawPayload any
	if err := json.Unmarshal(raw, &rawPayload); err != nil {
		return HooksListResult{}, fmt.Errorf("malformed Codex hooks/list result: %w", err)
	}
	if _, err := sanitizeHooksListValue(rawPayload); err != nil {
		return HooksListResult{}, fmt.Errorf("Codex hooks/list result: %w", err)
	}
	var payload struct {
		Data []HooksListCWD `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return HooksListResult{}, fmt.Errorf("malformed Codex hooks/list result: %w", err)
	}
	if len(payload.Data) != 1 {
		return HooksListResult{}, fmt.Errorf("Codex hooks/list result requires exactly one cwd entry")
	}
	entry := &payload.Data[0]
	if err := validateHooksListEntry(entry, workerRoot); err != nil {
		return HooksListResult{}, err
	}
	if err := sanitizeHooksListEntry(entry); err != nil {
		return HooksListResult{}, err
	}
	result := HooksListResult{OK: true, Data: payload.Data}
	if err := validateHooksListEncodedSize(result); err != nil {
		return HooksListResult{}, err
	}
	return result, nil
}

func validateHooksListEntry(entry *HooksListCWD, workerRoot string) error {
	if entry.CWD != workerRoot {
		return fmt.Errorf("Codex hooks/list result does not report the exact worker cwd")
	}
	if len(entry.Hooks) > hooksListHookLimit {
		return fmt.Errorf("Codex hooks/list hook limit exceeded (%d)", hooksListHookLimit)
	}
	if len(entry.Warnings) > hooksListMessageLimit {
		return fmt.Errorf("Codex hooks/list warning limit exceeded (%d)", hooksListMessageLimit)
	}
	if len(entry.Errors) > hooksListMessageLimit {
		return fmt.Errorf("Codex hooks/list error limit exceeded (%d)", hooksListMessageLimit)
	}
	if len(entry.CWD) > hooksListStringLimit {
		return fmt.Errorf("Codex hooks/list string limit exceeded (%d bytes)", hooksListStringLimit)
	}
	return nil
}

func sanitizeHooksListEntry(entry *HooksListCWD) error {
	for i, hook := range entry.Hooks {
		clean, err := sanitizeHooksListValue(hook)
		if err != nil {
			return fmt.Errorf("Codex hooks/list hook %d: %w", i, err)
		}
		entry.Hooks[i] = clean.(map[string]any)
	}
	if err := sanitizeHooksListMessages(entry.Warnings); err != nil {
		return err
	}
	return sanitizeHooksListMessages(entry.Errors)
}

func sanitizeHooksListMessages(messages []string) error {
	for i, message := range messages {
		if len(message) > hooksListStringLimit {
			return fmt.Errorf("Codex hooks/list string limit exceeded (%d bytes)", hooksListStringLimit)
		}
		messages[i] = policy.RedactFreeform(message)
	}
	return nil
}

func sanitizeHooksListValue(value any) (any, error) {
	nodes := 0
	return sanitizeHooksListValueAt(value, 0, &nodes)
}

func sanitizeHooksListValueAt(value any, depth int, nodes *int) (any, error) {
	if depth > hooksListDepthLimit {
		return nil, fmt.Errorf("nesting depth limit exceeded (%d)", hooksListDepthLimit)
	}
	*nodes++
	if *nodes > hooksListNodeLimit {
		return nil, fmt.Errorf("node limit exceeded (%d)", hooksListNodeLimit)
	}
	switch typed := value.(type) {
	case string:
		if len(typed) > hooksListStringLimit {
			return nil, fmt.Errorf("string limit exceeded (%d bytes)", hooksListStringLimit)
		}
		return policy.RedactFreeform(typed), nil
	case []any:
		for i, item := range typed {
			clean, err := sanitizeHooksListValueAt(item, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			typed[i] = clean
		}
		return typed, nil
	case map[string]any:
		for key, item := range typed {
			if len(key) > hooksListStringLimit {
				return nil, fmt.Errorf("string limit exceeded (%d bytes)", hooksListStringLimit)
			}
			if policy.RedactFreeform(key) != key {
				return nil, fmt.Errorf("unsafe map key")
			}
			if hooksListSecretBearingKey(key) {
				return nil, fmt.Errorf("secret-bearing map key")
			}
			clean, err := sanitizeHooksListValueAt(item, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			typed[key] = clean
		}
		return typed, nil
	case nil, bool, float64:
		return typed, nil
	default:
		return nil, fmt.Errorf("unsupported JSON value type %T", value)
	}
}

func hooksListSecretBearingKey(key string) bool {
	var normalized strings.Builder
	for _, r := range strings.ToLower(key) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			normalized.WriteRune(r)
		}
	}
	value := normalized.String()
	for _, marker := range []string{"token", "password", "passwd", "secret", "apikey", "credential", "authorization"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func validateHooksListEncodedSize(result HooksListResult) error {
	if result.AuditLogID == "" {
		result.AuditLogID = strings.Repeat("x", hooksListAuditIDSpace)
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("encode bounded Codex hooks/list result: %w", err)
	}
	if len(encoded)+1 > hooksListEncodedLimit {
		return fmt.Errorf("Codex hooks/list encoded size limit exceeded (%d bytes)", hooksListEncodedLimit)
	}
	return nil
}

type boundedCapture struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (w *boundedCapture) Write(p []byte) (int, error) {
	original := len(p)
	remaining := w.limit - w.buffer.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = w.buffer.Write(p)
	}
	if original > remaining {
		w.truncated = true
	}
	return original, nil
}

func (w *boundedCapture) Len() int       { return w.buffer.Len() }
func (w *boundedCapture) String() string { return w.buffer.String() }
