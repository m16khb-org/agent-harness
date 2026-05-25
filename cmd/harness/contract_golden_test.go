package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func TestCLIUsageGolden(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	usage()
	_ = w.Close()
	os.Stderr = oldStderr
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "usage.golden.txt", b)
}

func TestMCPToolsGolden(t *testing.T) {
	assertJSONGolden(t, "mcp_tools.golden.json", mcpTools())
}

func TestMCPResourcesGolden(t *testing.T) {
	assertJSONGolden(t, "mcp_resources.golden.json", mcpResources())
}

func assertJSONGolden(t *testing.T, name string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, name, append(b, '\n'))
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run go test ./cmd/harness -run Golden -update)", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, string(got), string(want))
	}
}
