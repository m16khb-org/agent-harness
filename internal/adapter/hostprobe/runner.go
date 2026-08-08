package hostprobe

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
	"strings"
	"time"

	toolconformancecontract "agent-harness/internal/contract/toolconformance"
	"agent-harness/internal/domain/policy"
	"agent-harness/internal/port"
)

const (
	MaxOutputBytes = 64 << 10
	EpisodeTimeout = 5 * time.Minute
	VersionTimeout = 10 * time.Second
)

type CommandRequest struct {
	Cwd     string
	Argv    []string
	Env     []string
	Timeout time.Duration
}

type CommandOutput struct {
	Stdout          []byte
	Stderr          []byte
	ExitCode        int
	StdoutTruncated bool
	StderrTruncated bool
}

type CommandRunner interface {
	Run(context.Context, CommandRequest) (CommandOutput, error)
}

type Dependencies struct {
	Process  CommandRunner
	LookPath func(string) (string, error)
	Now      func() time.Time
	TempDir  func(string, string) (string, error)
	Getenv   func(string) string
	Environ  func() []string
}

type ExecRunner struct{}

type boundedBuffer struct {
	bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || written > 0
		return written, nil
	}
	if len(value) > remaining {
		value = value[:remaining]
		b.truncated = true
	}
	_, err := b.Buffer.Write(value)
	return written, err
}

func (ExecRunner) Run(ctx context.Context, request CommandRequest) (CommandOutput, error) {
	if len(request.Argv) == 0 {
		return CommandOutput{}, fmt.Errorf("empty_argv")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = EpisodeTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.Command(request.Argv[0], request.Argv[1:]...)
	configureProcessGroup(cmd)
	cmd.Dir = request.Cwd
	if request.Env != nil {
		cmd.Env = append([]string(nil), request.Env...)
	}
	stdout := &boundedBuffer{limit: MaxOutputBytes}
	stderr := &boundedBuffer{limit: MaxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return CommandOutput{}, fmt.Errorf("command_failed")
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var err error
	select {
	case err = <-waited:
	case <-runCtx.Done():
		_ = terminateProcessTree(cmd)
		err = <-waited
	}
	out := CommandOutput{
		Stdout:          append([]byte(nil), stdout.Bytes()...),
		Stderr:          append([]byte(nil), stderr.Bytes()...),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}
	if cmd.ProcessState != nil {
		out.ExitCode = cmd.ProcessState.ExitCode()
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return out, fmt.Errorf("command_timeout")
	}
	if runCtx.Err() != nil {
		return out, fmt.Errorf("command_cancelled")
	}
	if err != nil {
		return out, fmt.Errorf("command_failed")
	}
	if out.StdoutTruncated || out.StderrTruncated {
		return out, fmt.Errorf("command_output_truncated")
	}
	return out, nil
}

func normalizeDependencies(deps Dependencies) Dependencies {
	if deps.Process == nil {
		deps.Process = ExecRunner{}
	}
	if deps.LookPath == nil {
		deps.LookPath = exec.LookPath
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.TempDir == nil {
		deps.TempDir = os.MkdirTemp
	}
	if deps.Getenv == nil {
		deps.Getenv = os.Getenv
	}
	if deps.Environ == nil {
		deps.Environ = os.Environ
	}
	return deps
}

func preflight(ctx context.Context, deps Dependencies, host, executable string, request port.HostProbeRequest, env []string) port.HostProbePreflight {
	result := port.HostProbePreflight{Host: host, RequestedModel: request.Model}
	path, err := deps.LookPath(executable)
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "executable_not_found"
		result.EvidenceSource = host + "_preflight"
		return result
	}
	output, err := deps.Process.Run(ctx, CommandRequest{Argv: []string{path, "--version"}, Env: env, Timeout: VersionTimeout})
	if err != nil {
		result.Cause = "harness_environment"
		result.Code = "version_probe_failed"
		result.EvidenceSource = host + "_preflight"
		return result
	}
	result.Ready = true
	result.Version = boundedVersion(string(output.Stdout))
	return result
}

func boundedVersion(value string) string {
	value = strings.TrimSpace(policy.RedactFreeform(value))
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		value = value[:index]
	}
	if len(value) > 256 {
		value = value[:256]
	}
	return value
}

type episodeCapture struct {
	FixtureID          string                                 `json:"fixture_id"`
	CallCount          int                                    `json:"call_count"`
	RawSHA256          string                                 `json:"raw_sha256"`
	CanonicalArguments any                                    `json:"canonical_arguments"`
	SchemaSHA256       string                                 `json:"schema_sha256"`
	RunTokenSHA256     string                                 `json:"run_token_sha256"`
	Classification     toolconformancecontract.Classification `json:"classification"`
	AdvertisedValid    bool                                   `json:"advertised_valid"`
	CanonicalValid     bool                                   `json:"canonical_valid"`
	Diagnostics        []toolconformancecontract.Diagnostic   `json:"diagnostics"`
}

type hostStreamObservation struct {
	SessionStartObserved bool   `json:"session_start_observed"`
	PreToolUseObserved   bool   `json:"pre_tool_use_observed"`
	MCPCallCount         int    `json:"mcp_call_count"`
	ResponseSHA256       string `json:"response_sha256"`
	ExitCode             int    `json:"exit_code"`
	DurationMS           int64  `json:"duration_ms"`
}

func observeHostStream(data []byte) (hostStreamObservation, error) {
	if len(data) == 0 || len(data) > MaxOutputBytes {
		return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	observation := hostStreamObservation{}
	results := make([]any, 0, 1)
	claudeProbeCalls := map[string]bool{}
	events := 0
	for {
		var event any
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
		}
		events++
		observeHookEvent(event, &observation)
		observeClaudeProbeCalls(event, claudeProbeCalls)
		for _, result := range observedClaudeProbeResults(event, claudeProbeCalls) {
			observation.MCPCallCount++
			results = append(results, result)
		}
		if result, ok := observedMCPResult(event); ok {
			observation.MCPCallCount++
			results = append(results, result)
		}
	}
	if events == 0 {
		return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
	}
	if len(results) > 0 {
		canonical, err := json.Marshal(results)
		if err != nil {
			return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
		}
		digest := sha256.Sum256(canonical)
		observation.ResponseSHA256 = hex.EncodeToString(digest[:])
	}
	return observation, nil
}

func observeClaudeProbeCalls(value any, calls map[string]bool) {
	event, ok := value.(map[string]any)
	if !ok || event["type"] != "assistant" {
		return
	}
	message, _ := event["message"].(map[string]any)
	content, _ := message["content"].([]any)
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		name, _ := block["name"].(string)
		id, _ := block["id"].(string)
		if block["type"] == "tool_use" && id != "" && strings.Contains(name, "agent_harness_probe") {
			calls[id] = true
		}
	}
}

func observedClaudeProbeResults(value any, calls map[string]bool) []any {
	event, ok := value.(map[string]any)
	if !ok || event["type"] != "user" {
		return nil
	}
	message, _ := event["message"].(map[string]any)
	content, _ := message["content"].([]any)
	results := make([]any, 0, 1)
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		id, _ := block["tool_use_id"].(string)
		if block["type"] != "tool_result" || !calls[id] {
			continue
		}
		delete(calls, id)
		if result, exists := event["tool_use_result"]; exists && result != nil {
			results = append(results, result)
		} else {
			results = append(results, block)
		}
	}
	return results
}

func observeHookEvent(value any, observation *hostStreamObservation) {
	event, ok := value.(map[string]any)
	if !ok {
		return
	}
	var name string
	switch event["type"] {
	case "hook.completed":
		hook, _ := event["hook"].(map[string]any)
		name, _ = hook["event_name"].(string)
	case "system":
		if event["subtype"] == "hook_response" {
			name, _ = event["hook_event"].(string)
		}
	}
	switch strings.ToLower(strings.ReplaceAll(name, "_", "")) {
	case "sessionstart":
		observation.SessionStartObserved = true
	case "pretooluse":
		observation.PreToolUseObserved = true
	}
}

func observedMCPResult(value any) (any, bool) {
	event, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	typeName, _ := event["type"].(string)
	switch typeName {
	case "item.completed":
		item, _ := event["item"].(map[string]any)
		server, _ := item["server"].(string)
		status, _ := item["status"].(string)
		if item["type"] != "mcp_tool_call" || !strings.Contains(server, "agent_harness_probe") || status != "completed" {
			return nil, false
		}
		result, exists := item["result"]
		return result, exists && result != nil
	case "tool_result":
		tool, _ := event["tool"].(map[string]any)
		name, _ := tool["name"].(string)
		if !strings.Contains(name, "agent_harness_probe") {
			return nil, false
		}
		result, exists := event["result"]
		return result, exists && result != nil
	default:
		return nil, false
	}
}

func observeRecordedHookEvents(deps Dependencies) (hostStreamObservation, error) {
	observationPath := strings.TrimSpace(deps.Getenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE"))
	if !filepath.IsAbs(observationPath) {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	markerPath := observationPath + ".hooks"
	info, err := os.Lstat(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return hostStreamObservation{}, nil
		}
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	data, err := readBoundedEvidenceFile(markerPath)
	if err != nil {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	defer func() { _ = os.Remove(markerPath) }()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	observation := hostStreamObservation{}
	events := 0
	for {
		var marker struct {
			Event string `json:"event"`
		}
		if err := decoder.Decode(&marker); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
		}
		events++
		switch marker.Event {
		case "SessionStart":
			observation.SessionStartObserved = true
		case "PreToolUse":
			observation.PreToolUseObserved = true
		default:
			return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
		}
	}
	if events == 0 {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	return observation, nil
}

func mergeHookObservation(observation *hostStreamObservation, recorded hostStreamObservation) {
	observation.SessionStartObserved = observation.SessionStartObserved || recorded.SessionStartObserved
	observation.PreToolUseObserved = observation.PreToolUseObserved || recorded.PreToolUseObserved
}

func newEpisodeRoot(deps Dependencies, host string) (string, error) {
	root, err := deps.TempDir("", "agent-harness-conformance-"+host+"-")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		_ = os.RemoveAll(root)
		return "", err
	}
	return root, nil
}

func serveArgv(request port.HostProbeRequest, resultPath string) []string {
	return []string{
		request.HarnessBinary,
		"contract", "conformance", "serve",
		"--fixture-id", request.FixtureID,
		"--result-file", resultPath,
		"--run-token", request.RunToken,
	}
}

func decodeEpisodeCapture(resultPath string, request port.HostProbeRequest) (episodeCapture, error) {
	data, err := readBoundedEvidenceFile(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			return episodeCapture{}, fmt.Errorf("probe_result_missing")
		}
		if err.Error() == "evidence_file_too_large" {
			return episodeCapture{}, fmt.Errorf("probe_result_too_large")
		}
		return episodeCapture{}, fmt.Errorf("probe_result_read_failed")
	}
	var capture episodeCapture
	if err := decodeStrictJSON(data, &capture); err != nil {
		return episodeCapture{}, fmt.Errorf("probe_result_invalid")
	}
	want := sha256.Sum256([]byte(request.RunToken))
	wantToken := hex.EncodeToString(want[:])
	if capture.RunTokenSHA256 != wantToken {
		return episodeCapture{}, fmt.Errorf("stale_run_token")
	}
	if capture.FixtureID != request.FixtureID || capture.SchemaSHA256 != request.SchemaSHA256 {
		return episodeCapture{}, fmt.Errorf("probe_result_identity_mismatch")
	}
	if capture.CallCount != 1 || capture.CanonicalArguments == nil || !validSHA256(capture.RawSHA256) || !validSHA256(capture.RunTokenSHA256) {
		return episodeCapture{}, fmt.Errorf("probe_result_invalid")
	}
	if _, err := toolconformancecontract.ParseClassification(string(capture.Classification)); err != nil {
		return episodeCapture{}, fmt.Errorf("probe_result_invalid")
	}
	multiplePath := resultPath + ".multiple"
	multipleData, markerErr := readBoundedEvidenceFile(multiplePath)
	if markerErr == nil {
		var marker struct {
			CallCount      int    `json:"call_count"`
			RunTokenSHA256 string `json:"run_token_sha256"`
		}
		if decodeStrictJSON(multipleData, &marker) != nil || marker.RunTokenSHA256 != capture.RunTokenSHA256 || marker.CallCount <= capture.CallCount {
			return episodeCapture{}, fmt.Errorf("multiple_call_marker_invalid")
		}
		capture.CallCount = marker.CallCount
		capture.Classification = toolconformancecontract.Classification(toolconformancecontract.MultipleCalls)
		capture.AdvertisedValid = false
		capture.CanonicalValid = false
		capture.Diagnostics = []toolconformancecontract.Diagnostic{}
	} else if !os.IsNotExist(markerErr) {
		if markerErr.Error() == "evidence_file_too_large" {
			return episodeCapture{}, fmt.Errorf("multiple_call_marker_too_large")
		}
		return episodeCapture{}, fmt.Errorf("multiple_call_marker_read_failed")
	}
	return capture, nil
}

func readBoundedEvidenceFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxOutputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxOutputBytes {
		return nil, fmt.Errorf("evidence_file_too_large")
	}
	return data, nil
}

func decodeStrictJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("trailing_json")
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func completedResult(host, version string, request port.HostProbeRequest, started time.Time, deps Dependencies, capture episodeCapture) port.HostProbeResult {
	arguments, _ := json.Marshal(capture.CanonicalArguments)
	diagnostics, _ := json.Marshal(capture.Diagnostics)
	return port.HostProbeResult{
		Completed:              true,
		Host:                   host,
		HostVersion:            version,
		RequestedModel:         request.Model,
		FixtureID:              request.FixtureID,
		SchemaSHA256:           capture.SchemaSHA256,
		Profile:                request.Profile,
		Attempt:                request.Attempt,
		DurationMS:             deps.Now().Sub(started).Milliseconds(),
		AmbientToolCount:       1,
		CallCount:              capture.CallCount,
		RawArgumentsSHA256:     capture.RawSHA256,
		CanonicalArgumentsJSON: string(arguments),
		EvidenceID:             capture.RunTokenSHA256,
		Classification:         string(capture.Classification),
		AdvertisedValid:        capture.AdvertisedValid,
		CanonicalValid:         capture.CanonicalValid,
		DiagnosticsJSON:        string(diagnostics),
	}
}

func applyHostStreamObservation(result *port.HostProbeResult, observation hostStreamObservation, exitCode int) {
	result.SessionStartObserved = observation.SessionStartObserved
	result.PreToolUseObserved = observation.PreToolUseObserved
	result.ResponseSHA256 = observation.ResponseSHA256
	result.ExitCode = exitCode
}

func persistChildSmokeObservation(deps Dependencies, result port.HostProbeResult, mcpCallCount int) error {
	path := strings.TrimSpace(deps.Getenv("HARNESS_CHILD_SMOKE_OBSERVATION_FILE"))
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("child_smoke_observation_path_invalid")
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 || parentInfo.Mode().Perm() != 0o700 {
		return fmt.Errorf("child_smoke_observation_path_invalid")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("child_smoke_observation_path_invalid")
	}
	observation := hostStreamObservation{
		SessionStartObserved: result.SessionStartObserved,
		PreToolUseObserved:   result.PreToolUseObserved,
		MCPCallCount:         mcpCallCount,
		ResponseSHA256:       result.ResponseSHA256,
		ExitCode:             result.ExitCode,
		DurationMS:           result.DurationMS,
	}
	data, err := json.Marshal(observation)
	if err != nil {
		return fmt.Errorf("child_smoke_observation_encode_failed")
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("child_smoke_observation_write_failed")
	}
	return nil
}

func failedResult(host, version string, request port.HostProbeRequest, started time.Time, deps Dependencies, cause, code string) port.HostProbeResult {
	return port.HostProbeResult{
		Host:             host,
		HostVersion:      version,
		RequestedModel:   request.Model,
		FixtureID:        request.FixtureID,
		SchemaSHA256:     request.SchemaSHA256,
		Profile:          request.Profile,
		Attempt:          request.Attempt,
		DurationMS:       deps.Now().Sub(started).Milliseconds(),
		Cause:            cause,
		Code:             code,
		EvidenceSource:   host + "_runner",
		AmbientToolCount: 1,
	}
}

func writePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func observedModelFromOutput(data []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var value map[string]any
		if err := decoder.Decode(&value); err != nil {
			return ""
		}
		if model, ok := value["model"].(string); ok && strings.TrimSpace(model) != "" {
			return boundedVersion(model)
		}
		if message, ok := value["message"].(map[string]any); ok {
			if model, ok := message["model"].(string); ok && strings.TrimSpace(model) != "" {
				return boundedVersion(model)
			}
		}
	}
}

func normalizedProcessFailure(err error, defaultCode string) (string, string) {
	switch err.Error() {
	case "command_timeout", "command_cancelled":
		return "harness_environment", err.Error()
	case "command_output_truncated":
		return "transport", "command_output_truncated"
	default:
		return "harness_environment", defaultCode
	}
}

func normalizedCaptureFailure(err error) (string, string) {
	switch err.Error() {
	case "stale_run_token":
		return "harness_environment", "stale_run_token"
	case "probe_result_missing":
		return "unknown", "no_call"
	case "probe_result_read_failed", "probe_result_invalid", "probe_result_too_large", "probe_result_identity_mismatch",
		"multiple_call_marker_invalid", "multiple_call_marker_too_large", "multiple_call_marker_read_failed":
		return "transport", err.Error()
	default:
		return "transport", "probe_result_decode_failed"
	}
}
func isolatedHostEnv(deps Dependencies, additionalNames ...string) []string {
	allowed := map[string]bool{}
	for _, name := range additionalNames {
		allowed[name] = true
	}
	values := map[string]string{}
	for _, entry := range deps.Environ() {
		name, value, found := strings.Cut(entry, "=")
		if found && (isHostAmbientEnvironment(name) || allowed[name]) {
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

func isHostAmbientEnvironment(name string) bool {
	switch name {
	case "PATH", "HOME", "USER", "TMPDIR", "LANG", "LANGUAGE", "LC_ALL", "LC_ADDRESS", "LC_COLLATE", "LC_CTYPE", "LC_IDENTIFICATION", "LC_MEASUREMENT", "LC_MESSAGES", "LC_MONETARY", "LC_NAME", "LC_NUMERIC", "LC_PAPER", "LC_TELEPHONE", "LC_TIME":
		return true
	default:
		return false
	}
}
