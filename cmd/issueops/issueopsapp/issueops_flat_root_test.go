package issueopsapp

import (
	"testing"
)

func TestFlatLifecycleCommandsReachRootDispatch(t *testing.T) {
	command := rootCommand()
	for _, name := range []string{"start", "next", "list", "status", "execution", "remote", "cleanup", "system-status", "update"} {
		if command.Runners[name] == nil {
			t.Errorf("root has no %s command", name)
		}
	}
	if command.Runners["issueops"] != nil {
		t.Error("root retained the duplicated issueops namespace")
	}
}
