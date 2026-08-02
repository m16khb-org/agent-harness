package policy_test

import (
	"testing"

	policydomain "agent-harness/internal/domain/policy"
)

func TestResolveTierPreservesCapabilityPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		request policydomain.Request
		want    string
	}{
		{"read only", policydomain.Request{}, policydomain.TierReadOnly},
		{"write", policydomain.Request{WriteAllowed: true}, policydomain.TierWorkspaceWrite},
		{"network", policydomain.Request{WriteAllowed: true, NetworkAllowed: true}, policydomain.TierNetworkAccess},
		{"shell", policydomain.Request{WriteAllowed: true, NetworkAllowed: true, ShellAllowed: true}, policydomain.TierShellException},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := policydomain.ResolveTier(tt.request); got.Name != tt.want {
				t.Fatalf("tier=%q want %q", got.Name, tt.want)
			}
		})
	}
}
