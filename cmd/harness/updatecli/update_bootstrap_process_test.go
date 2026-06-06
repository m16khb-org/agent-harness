package updatecli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseMCPProxyProcessOnlyMatchesCurrentHarnessMCP(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "agent-harness")
	match, ok := parseMCPProxyProcess("  123 "+binary+" mcp", binary)
	if !ok || match.PID != 123 || match.Command != binary+" mcp" {
		t.Fatalf("expected MCP proxy match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 " + binary + " daemon --internal",
		"125 " + binary + " update",
		"126 /other/bin/agent-harness mcp",
		"not-a-pid " + binary + " mcp",
	} {
		if got, ok := parseMCPProxyProcess(line, binary); ok {
			t.Fatalf("unexpected match for %q: %+v", line, got)
		}
	}
}

func TestParseDaemonProcessOnlyMatchesCurrentHarnessDaemon(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "agent-harness")
	match, ok := parseDaemonProcess("  123 "+binary+" daemon --internal", binary)
	if !ok || match.PID != 123 || match.Command != binary+" daemon --internal" {
		t.Fatalf("expected daemon match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 " + binary + " mcp",
		"125 " + binary + " daemon start",
		"126 /other/bin/agent-harness daemon --internal",
		"not-a-pid " + binary + " daemon --internal",
	} {
		if got, ok := parseDaemonProcess(line, binary); ok {
			t.Fatalf("unexpected match for %q: %+v", line, got)
		}
	}
}

func TestTerminateStaleDaemonProcessesSkipsCurrentProcess(t *testing.T) {
	currentPID := os.Getpid()
	restoreList := stubDaemonProcessLister(t, func() ([]daemonProcess, error) {
		return []daemonProcess{
			{PID: currentPID, Command: "agent-harness daemon --internal"},
			{PID: 12345, Command: "agent-harness daemon --internal"},
			{PID: 12346, Command: "agent-harness daemon --internal"},
		}, nil
	})
	defer restoreList()
	var terminated []int
	restoreTerminate := stubDaemonProcessTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerminate()

	count, err := terminateStaleDaemonProcesses()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !reflect.DeepEqual(terminated, []int{12345, 12346}) {
		t.Fatalf("unexpected daemon cleanup result count=%d terminated=%v", count, terminated)
	}
}

func TestRefreshRunningMCPProxiesAfterInstallSkipsCurrentProcess(t *testing.T) {
	currentPID := os.Getpid()
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		return []mcpProxyProcess{
			{PID: currentPID, Command: "agent-harness mcp"},
			{PID: 22345, Command: "agent-harness mcp"},
			{PID: 22346, Command: "agent-harness mcp"},
		}, nil
	})
	defer restoreList()
	var terminated []int
	restoreTerminate := stubMCPProxyTerminator(t, func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	})
	defer restoreTerminate()

	count, err := refreshRunningMCPProxiesAfterInstall()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !reflect.DeepEqual(terminated, []int{22345, 22346}) {
		t.Fatalf("unexpected MCP proxy cleanup result count=%d terminated=%v", count, terminated)
	}
}
