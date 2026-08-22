package channelcli

import (
	"strings"
	"testing"
)

func deps() Dependencies {
	return Dependencies{Send: adapterSend, Recv: adapterRecv}
}

func TestChannelCLISendRecvRoundTrip(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	if err := Run(deps(), []string{"send", "--channel", "contract", "--from", "server", "--message", "GET /users -> 200 {id,name}"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := Run(deps(), []string{"recv", "--channel", "contract"}); err != nil {
		t.Fatalf("recv: %v", err)
	}
}

func TestChannelCLIRecvWaitTimeoutExit(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	err := Run(deps(), []string{"recv", "--channel", "quiet", "--wait", "--timeout-seconds", "1"})
	if err == nil {
		t.Fatal("timed-out wait must fail")
	}
	if _, ok := err.(TimedOutError); !ok {
		t.Fatalf("want TimedOutError, got %T %v", err, err)
	}
}

func TestChannelCLIUsageErrors(t *testing.T) {
	if err := Run(deps(), []string{"unknown"}); err == nil || !strings.Contains(err.Error(), "unknown channel subcommand") {
		t.Fatalf("unknown subcommand: %v", err)
	}
	if err := Run(deps(), []string{"send", "--channel", "c", "--message", "x"}); err == nil {
		t.Fatal("missing --from must fail")
	}
	if err := Run(deps(), []string{"recv"}); err == nil {
		t.Fatal("missing --channel must fail")
	}
}
