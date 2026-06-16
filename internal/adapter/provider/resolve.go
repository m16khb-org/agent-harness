// Package provider selects a concrete remote issue provider by name. It lives in
// the adapter layer so internal/core never imports concrete provider
// implementations, preserving the hexagonal boundary: core depends only on the
// port.IssueProvider abstraction and receives a resolved provider from callers.
package provider

import (
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
