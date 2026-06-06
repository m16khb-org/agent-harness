package trace

import (
	"fmt"
	"os"

	"agent-harness/internal/core/state"
)

type traceAnalysisInput struct {
	Source string
	Body   []byte
}

func loadTraceAnalysisInput(input string) (traceAnalysisInput, error) {
	if input == "-" {
		b, err := os.ReadFile("/dev/stdin")
		return traceAnalysisInput{Source: "stdin", Body: b}, err
	}
	if b, err := os.ReadFile(input); err == nil {
		return traceAnalysisInput{Source: "file", Body: b}, nil
	}
	state, err := state.StateRead(input)
	if err != nil {
		return traceAnalysisInput{}, fmt.Errorf("read trace input as file or state key %q: %w", input, err)
	}
	return traceAnalysisInput{Source: "state", Body: []byte(state.Record.Content)}, nil
}
