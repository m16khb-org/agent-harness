package harnessapp

import (
	"os"
	"testing"
)

// TestMain wires the leaf CLI dependencies before running the package tests,
// mirroring what RunRootCommand does in production. Several tests call command
// runners (runInspect, runStatus, runDoctor, ...) directly without going through
// RunRootCommand and rely on the injected harness implementations.
func TestMain(m *testing.M) {
	wireDependencies()
	os.Exit(m.Run())
}
