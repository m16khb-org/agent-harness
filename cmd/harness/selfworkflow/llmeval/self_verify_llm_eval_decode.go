package llmeval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"agent-harness/cmd/harness/commandstep"
)

const selfVerifyLLMEvalErrorBudgetBytes = 512

func DecodeSelfVerifyLLMEval(out []byte, eval *SelfVerifyLLMEvalResult) error {
	trimmed := bytes.TrimSpace(out)
	if err := DecodeSelfVerifyLLMEvalStrict(trimmed, eval); err == nil {
		return nil
	} else if extracted, ok := ExtractSelfVerifyLLMEvalJSON(trimmed); ok {
		if extractErr := DecodeSelfVerifyLLMEvalStrict(extracted, eval); extractErr == nil {
			return nil
		}
		return err
	} else {
		return err
	}
}

func DecodeSelfVerifyLLMEvalStrict(out []byte, eval *SelfVerifyLLMEvalResult) error {
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(eval); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected extra JSON value")
	}
	return nil
}

func ExtractSelfVerifyLLMEvalJSON(out []byte) ([]byte, bool) {
	for i, b := range out {
		if b != '{' {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(out[i:]))
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil || len(raw) == 0 || raw[0] != '{' {
			continue
		}
		return raw, true
	}
	return nil, false
}

func BoundedLLMEvalError(prefix string, err error, output string) string {
	message := prefix + ": " + err.Error()
	output = strings.TrimSpace(output)
	if output != "" {
		message += ": " + output
	}
	bounded, _, _ := commandstep.TailWithBudget(message, selfVerifyLLMEvalErrorBudgetBytes)
	return bounded
}
