package harnessapp

import "testing"

func TestRootCommandIncludesWebFetchRunner(t *testing.T) {
	cmd := rootCommand()
	if cmd.Runners["web-fetch"] == nil {
		t.Fatalf("root command missing web-fetch runner")
	}
}
