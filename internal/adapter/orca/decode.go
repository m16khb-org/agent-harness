package orca

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-harness/internal/port"
)

const MaxEnvelopeBytes = 2 * 1024 * 1024

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Meta struct {
		RuntimeID string `json:"runtimeId"`
	} `json:"_meta"`
}

func decodeResult(output CommandOutput, target any) (string, error) {
	if len(output.Stdout) > MaxEnvelopeBytes {
		return "", fmt.Errorf("Orca envelope exceeds %d bytes", MaxEnvelopeBytes)
	}
	var env envelope
	if err := json.Unmarshal(output.Stdout, &env); err != nil {
		return "", fmt.Errorf("decode Orca stdout envelope: %w", err)
	}
	if !env.OK {
		code := strings.TrimSpace(env.Error.Code)
		if code == "" {
			code = "orca_rejected"
		}
		return env.Meta.RuntimeID, &port.OrcaError{Code: code, Detail: boundedDiagnostic(env.Error.Message), Invoked: output.Invoked}
	}
	if target != nil {
		if len(env.Result) == 0 || string(env.Result) == "null" {
			return env.Meta.RuntimeID, fmt.Errorf("decode Orca result: missing result")
		}
		if err := json.Unmarshal(env.Result, target); err != nil {
			return env.Meta.RuntimeID, fmt.Errorf("decode Orca result: %w", err)
		}
	}
	return env.Meta.RuntimeID, nil
}
