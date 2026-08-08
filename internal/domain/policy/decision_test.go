package policy_test

import (
	policycontract "agent-harness/internal/contract/policy"
	"testing"

	policydomain "agent-harness/internal/domain/policy"
)

func TestResolveTierPreservesCapabilityPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		request policycontract.Request
		want    string
	}{
		{"read only", policycontract.Request{}, policycontract.TierReadOnly},
		{"write", policycontract.Request{WriteAllowed: true}, policycontract.TierWorkspaceWrite},
		{"network", policycontract.Request{WriteAllowed: true, NetworkAllowed: true}, policycontract.TierNetworkAccess},
		{"shell", policycontract.Request{WriteAllowed: true, NetworkAllowed: true, ShellAllowed: true}, policycontract.TierShellException},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := policydomain.ResolveTier(tt.request); got.Name != tt.want {
				t.Fatalf("tier=%q want %q", got.Name, tt.want)
			}
		})
	}
}
