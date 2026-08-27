package installcli

import (
	"context"
	"errors"
	"strings"
	"testing"

	upstreamcontract "agent-harness/internal/contract/upstream"
	"agent-harness/internal/port"
)

func TestAppendUpstreamMessagesRendersEveryDeclaredEntry(t *testing.T) {
	t.Cleanup(Reset)
	Configure(Deps{SyncUpstream: func(context.Context, string, bool) (upstreamcontract.Report, error) {
		return upstreamcontract.Report{Items: []upstreamcontract.ItemResult{
			{Kind: upstreamcontract.KindPlugin, Name: "eli5@claude-community", Status: upstreamcontract.StatusInstalled},
			{Kind: upstreamcontract.KindPlugin, Name: "old@mkt", Status: upstreamcontract.StatusSkipped, Reason: "already installed on the host"},
			{Kind: upstreamcontract.KindSkill, Name: "cua-driver", Status: upstreamcontract.StatusFailed, Error: "clone failed"},
		}}, nil
	}})

	result := port.NativeInstallResult{OK: true}
	appendUpstreamMessages(&result, "/root", false)

	if len(result.Messages) != 3 {
		t.Fatalf("messages = %#v, want one per entry", result.Messages)
	}
	wants := []string{
		"upstream plugin eli5@claude-community: installed",
		"upstream plugin old@mkt: skipped (already installed on the host)",
		"upstream skill cua-driver: failed (clone failed)",
	}
	for i, want := range wants {
		if result.Messages[i] != want {
			t.Fatalf("message[%d] = %q, want %q", i, result.Messages[i], want)
		}
	}
	if !result.OK {
		t.Fatalf("a failed upstream entry must not flip install ok=false")
	}
}

func TestAppendUpstreamMessagesReportsSyncFailureWithoutFailingInstall(t *testing.T) {
	t.Cleanup(Reset)
	Configure(Deps{SyncUpstream: func(context.Context, string, bool) (upstreamcontract.Report, error) {
		return upstreamcontract.Report{}, errors.New("declaration is malformed")
	}})

	result := port.NativeInstallResult{OK: true}
	appendUpstreamMessages(&result, "/root", true)

	if len(result.Messages) != 1 || !strings.Contains(result.Messages[0], "declaration is malformed") {
		t.Fatalf("messages = %#v, want the sync failure reported", result.Messages)
	}
	if !result.OK {
		t.Fatalf("upstream sync failure must not fail the harness install")
	}
}

func TestAppendUpstreamMessagesIsSilentWithoutAnInjectedSync(t *testing.T) {
	t.Cleanup(Reset)
	Reset()

	result := port.NativeInstallResult{OK: true}
	appendUpstreamMessages(&result, "/root", false)

	if len(result.Messages) != 0 {
		t.Fatalf("messages = %#v, want none", result.Messages)
	}
}

func TestAppendUpstreamMessagesSaysNothingWhenNothingIsDeclared(t *testing.T) {
	t.Cleanup(Reset)
	Configure(Deps{SyncUpstream: func(context.Context, string, bool) (upstreamcontract.Report, error) {
		return upstreamcontract.Report{Items: []upstreamcontract.ItemResult{}}, nil
	}})

	result := port.NativeInstallResult{OK: true}
	appendUpstreamMessages(&result, "/root", false)

	if len(result.Messages) != 0 {
		t.Fatalf("messages = %#v, want none", result.Messages)
	}
}
