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

func TestParseRegisteredMCPProcessMatchesCodexNPXLaunchersAndChildren(t *testing.T) {
	registered := []registeredMCPCommand{
		{
			Name:    "db-bc-stg",
			Command: "npx",
			Args:    []string{"-y", "@bytebase/dbhub", "--config", "/Users/habin/workspace/infra/.dbhub/bc-stg.toml"},
		},
		{
			Name:    "context7",
			Command: "/Users/habin/Library/pnpm/npx",
			Args:    []string{"-y", "@upstash/context7-mcp"},
		},
		{
			Name:    "kordoc",
			Command: "/Users/habin/Library/pnpm/npx",
			Args:    []string{"-y", "kordoc@latest", "mcp"},
		},
		{
			Name:    "node_repl",
			Command: "/Applications/Codex.app/Contents/Resources/cua_node/bin/node_repl",
		},
	}
	for _, tc := range []struct {
		line string
		want string
	}{
		{
			line: "201 npm exec @bytebase/dbhub --config /Users/habin/workspace/infra/.dbhub/bc-stg.toml",
			want: "npm exec @bytebase/dbhub --config /Users/habin/workspace/infra/.dbhub/bc-stg.toml",
		},
		{
			line: "202 node /Users/habin/.npm/_npx/e23b/node_modules/.bin/dbhub --config /Users/habin/workspace/infra/.dbhub/bc-stg.toml",
			want: "node /Users/habin/.npm/_npx/e23b/node_modules/.bin/dbhub --config /Users/habin/workspace/infra/.dbhub/bc-stg.toml",
		},
		{
			line: "203 npm exec @upstash/context7-mcp",
			want: "npm exec @upstash/context7-mcp",
		},
		{
			line: "204 node /Users/habin/.npm/_npx/eea/node_modules/.bin/context7-mcp",
			want: "node /Users/habin/.npm/_npx/eea/node_modules/.bin/context7-mcp",
		},
		{
			line: "205 node /Users/habin/.npm/_npx/kordoc/node_modules/.bin/kordoc mcp",
			want: "node /Users/habin/.npm/_npx/kordoc/node_modules/.bin/kordoc mcp",
		},
	} {
		got, ok := parseRegisteredMCPProcess(tc.line, registered)
		if !ok || got.Command != tc.want {
			t.Fatalf("parseRegisteredMCPProcess(%q) = %+v ok=%v, want %q", tc.line, got, ok, tc.want)
		}
	}

	for _, line := range []string{
		"301 npm exec @bytebase/dbhub --config /tmp/other.toml",
		"302 node /Users/habin/.npm/_npx/e23b/node_modules/.bin/dbhub --config /tmp/other.toml",
		"303 node /Users/habin/.npm/_npx/eea/node_modules/.bin/not-context7-mcp",
		"304 node /Users/habin/.npm/_npx/eea/node_modules/.bin/context7-mcp-extra",
		"305 /Applications/Codex.app/Contents/Resources/cua_node/bin/node_repl",
		"not-a-pid npm exec @upstash/context7-mcp",
	} {
		if got, ok := parseRegisteredMCPProcess(line, registered); ok {
			t.Fatalf("unexpected registered MCP match for %q: %+v", line, got)
		}
	}
}

func TestParseCodexRegisteredMCPCommands(t *testing.T) {
	config := `
[mcp_servers.context7]
command = "/Users/habin/Library/pnpm/npx"
args = ["-y", "@upstash/context7-mcp"]
startup_timeout_sec = 60.0

[mcp_servers.context7.env]
SHOULD_NOT = "be parsed as a server"

[mcp_servers.db-bc-stg]
command = "npx"
args = ["-y", "@bytebase/dbhub", "--config", "/Users/habin/workspace/infra/.dbhub/bc-stg.toml"] # keep comments out
`
	got := parseCodexRegisteredMCPCommands(config)
	want := []registeredMCPCommand{
		{Name: "context7", Command: "/Users/habin/Library/pnpm/npx", Args: []string{"-y", "@upstash/context7-mcp"}},
		{Name: "db-bc-stg", Command: "npx", Args: []string{"-y", "@bytebase/dbhub", "--config", "/Users/habin/workspace/infra/.dbhub/bc-stg.toml"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCodexRegisteredMCPCommands = %#v, want %#v", got, want)
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
