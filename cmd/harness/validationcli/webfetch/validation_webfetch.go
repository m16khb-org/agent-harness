package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
	corewebfetch "agent-harness/internal/core/webfetch"
)

type StepResult = commandstep.StepResult

func Validate(binary, root string, seed int64) StepResult {
	started := time.Now()
	result, err := corewebfetch.RunBenchmark(context.Background(), corewebfetch.BenchmarkRequest{
		Fixtures: corewebfetch.DeterministicFixtures(),
		Timeout:  time.Second,
	})
	if err != nil {
		return commandstep.FailedStep("web fetch battery", err)
	}
	out, _ := json.Marshal(result)
	if !result.OK {
		return commandstep.AssertionStepWithOutput("web fetch battery", started, result.HardFailures, []string{string(out)}, []string{"internal/core/webfetch deterministic benchmark"}, 8*1024)
	}
	return StepResult{
		Label:      "web fetch battery",
		Command:    fmt.Sprintf("%s web-fetch benchmark --fixtures builtin --json", binary),
		OK:         true,
		DurationMS: time.Since(started).Milliseconds(),
		Stdout:     strings.TrimSpace(string(out)),
	}
}
