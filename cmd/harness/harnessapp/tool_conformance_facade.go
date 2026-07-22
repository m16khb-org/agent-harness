package harnessapp

import (
	"context"
	"fmt"
	"os"
	"strings"

	"agent-harness/cmd/harness/contractcli"
	"agent-harness/internal/adapter/hostprobe"
	mcpadapter "agent-harness/internal/adapter/mcp"
	"agent-harness/internal/core/toolconformance"
	"agent-harness/internal/port"
)

func runToolConformanceLive(ctx context.Context, request contractcli.LiveRequest) (toolconformance.BenchmarkReport, error) {
	binary, err := os.Executable()
	if err != nil {
		return toolconformance.BenchmarkReport{}, err
	}
	models, err := conformanceModelOverrides(request.Models)
	if err != nil {
		return toolconformance.BenchmarkReport{}, err
	}
	descriptors := make([]toolconformance.ToolDescriptor, 0, len(mcpadapter.AdvertisedTools()))
	for _, tool := range mcpadapter.AdvertisedTools() {
		descriptors = append(descriptors, toolconformance.ToolDescriptor{Name: tool.Name, InputSchema: tool.InputSchema})
	}
	runners := map[string]port.HostProbeRunner{
		"codex":  hostprobe.NewCodexRunner(binary, hostprobe.Dependencies{}),
		"claude": hostprobe.NewClaudeRunner(binary, hostprobe.Dependencies{}),
	}
	return toolconformance.RunLiveBenchmark(ctx, toolconformance.LiveBenchmarkRequest{
		Hosts: request.Hosts, Models: models,
		Profile: request.Profile, Only: request.Only, TargetCompleted: request.TargetCompleted,
		MaxAttemptsPerCase: request.MaxAttemptsPerCase, HarnessBinary: binary, Previous: request.Previous,
	}, descriptors, toolconformance.LiveBenchmarkDependencies{Runners: runners})
}

func conformanceModelOverrides(values []string) (map[string]string, error) {
	models := map[string]string{}
	for _, value := range values {
		host, model, found := strings.Cut(value, "=")
		host, model = strings.TrimSpace(host), strings.TrimSpace(model)
		if !found || host == "" || model == "" {
			return nil, fmt.Errorf("invalid model override %q", value)
		}
		if _, exists := models[host]; exists {
			return nil, fmt.Errorf("duplicate model override for %s", host)
		}
		models[host] = model
	}
	return models, nil
}
