// Package provider selects a concrete remote issue provider by name. It lives in
// the adapter layer so internal/core never imports concrete provider
// implementations, preserving the hexagonal boundary: core depends only on the
// port.IssueProvider abstraction and receives a resolved provider from callers.
package provider

import (
	"context"
	"fmt"

	"agent-harness/internal/adapter/provider/github"
	"agent-harness/internal/adapter/provider/gitlab"
	"agent-harness/internal/port"
)

// Resolve returns the issue provider registered under name, or an error naming
// the supported providers.
func Resolve(name string) (port.IssueProvider, error) {
	switch name {
	case "github":
		return github.NewProvider(), nil
	case "gitlab":
		return gitlab.NewProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q; supported: github, gitlab", name)
	}
}

func ReadExecutionIssueSnapshot(ctx context.Context, name string, req port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	resolved, err := Resolve(name)
	if err != nil {
		return port.ExecutionIssueSnapshot{}, err
	}
	reader, ok := resolved.(port.ExecutionIssueSnapshotReader)
	if !ok {
		return port.ExecutionIssueSnapshot{}, fmt.Errorf("provider %q cannot read issue snapshots", name)
	}
	return reader.ReadIssueSnapshot(ctx, req)
}
