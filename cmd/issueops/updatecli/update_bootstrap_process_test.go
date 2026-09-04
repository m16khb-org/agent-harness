package updatecli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseMCPProxyProcessOnlyMatchesCurrentHarnessMCP(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "issueops")
	match, ok := parseMCPProxyProcess("  123 "+binary+" mcp", binary)
	if !ok || match.PID != 123 || match.Command != binary+" mcp" {
		t.Fatalf("expected MCP proxy match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 " + binary + " daemon --internal",
		"125 " + binary + " update",
		"126 /other/bin/issueops mcp",
		"not-a-pid " + binary + " mcp",
	} {
		if got, ok := parseMCPProxyProcess(line, binary); ok {
			t.Fatalf("unexpected match for %q: %+v", line, got)
		}
	}
}

func TestParseMCPProxyProcessSnapshotRequiresExactHarnessCommand(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "issueops")
	process, ok := parseMCPProxyProcessSnapshot("123 1 "+binary+" mcp", binary)
	if !ok || process.PID != 123 || process.ParentPID != 1 || process.Command != binary+" mcp" {
		t.Fatalf("exact snapshot parse = %#v ok=%v", process, ok)
	}
	for _, line := range []string{
		"124 1 npm exec @upstash/context7-mcp",
		"125 1 node /tmp/node_modules/.bin/dbhub",
		"126 900 " + binary + " daemon --internal",
		"127 1 /other/bin/issueops mcp",
	} {
		if process, ok := parseMCPProxyProcessSnapshot(line, binary); ok {
			t.Fatalf("external or non-proxy process matched %q: %#v", line, process)
		}
	}
}

func TestParseDaemonProcessOnlyMatchesCurrentHarnessDaemon(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "bin", "issueops")
	match, ok := parseDaemonProcess("  123 "+binary+" daemon --internal", binary)
	if !ok || match.PID != 123 || match.Command != binary+" daemon --internal" {
		t.Fatalf("expected daemon match, got match=%+v ok=%v", match, ok)
	}
	for _, line := range []string{
		"124 " + binary + " mcp",
		"125 " + binary + " daemon start",
		"126 /other/bin/issueops daemon --internal",
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
			{PID: currentPID, Command: "issueops daemon --internal"},
			{PID: 12345, Command: "issueops daemon --internal"},
			{PID: 12346, Command: "issueops daemon --internal"},
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

func TestRefreshRunningMCPProxiesAfterInstallPreservesAllActiveProcesses(t *testing.T) {
	restoreList := stubMCPProxyProcessLister(t, func() ([]mcpProxyProcess, error) {
		t.Fatal("post-install refresh must not enumerate host-owned MCP processes")
		return nil, nil
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
	if count != 0 {
		t.Fatalf("post-install MCP refresh count = %d, want 0; terminated=%v", count, terminated)
	}
}
