package daemonlog

import (
	"log/slog"
	"strings"
	"testing"
)

func TestRoutineSessionEventsDemotedToDebug(t *testing.T) {
	var buf strings.Builder
	handler := NewFilteringHandler(slog.NewTextHandler(&buf, nil))
	logger := slog.New(handler)
	for event := range RoutineSessionEvents {
		logger.Info(event)
	}
	logger.Info("tool call completed")
	logger.Error("handler panicked")
	out := buf.String()
	for event := range RoutineSessionEvents {
		if strings.Contains(out, "level=INFO msg=\""+event+"\"") {
			t.Fatalf("routine event %q must not stay INFO:\n%s", event, out)
		}
	}
	if !strings.Contains(out, `level=INFO msg="tool call completed"`) {
		t.Fatalf("ordinary INFO must pass through:\n%s", out)
	}
	if !strings.Contains(out, `level=ERROR msg="handler panicked"`) {
		t.Fatalf("ERROR must pass through:\n%s", out)
	}
}

func TestIsRoutineShutdownError(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"close unix /x.sock->: use of closed network connection", true},
		{"server is closing: EOF", true},
		{"connection refused", false},
		{"broken pipe", false},
	}
	for _, tc := range cases {
		if got := IsRoutineShutdownError(errText(tc.text)); got != tc.want {
			t.Fatalf("IsRoutineShutdownError(%q) = %v want %v", tc.text, got, tc.want)
		}
	}
	if IsRoutineShutdownError(nil) {
		t.Fatal("nil error must not be routine")
	}
}

type errText string

func (e errText) Error() string { return string(e) }

func TestRoutineShutdownErrorRecordsAreDemoted(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(NewFilteringHandler(slog.NewTextHandler(&buf, nil)))
	logger.Error("server session ended with error", "error", errText("server is closing: EOF"))
	logger.Error("server session ended with error", "error", errText("use of closed network connection"))
	logger.Error("server session ended with error", "error", errText("connection refused by peer"))
	out := buf.String()
	if strings.Count(out, "level=ERROR") != 1 {
		t.Fatalf("routine shutdown errors must be demoted, exactly one real ERROR must remain:\n%s", out)
	}
	if !strings.Contains(out, "connection refused by peer") {
		t.Fatalf("genuine session error must stay ERROR:\n%s", out)
	}
}
