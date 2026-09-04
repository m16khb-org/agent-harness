package installutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ValidateHookConfigForMerge(config map[string]any, knownEvents []string) error {
	hooksValue, present := config["hooks"]
	if !present {
		return nil
	}
	hooks, ok := hooksValue.(map[string]any)
	if !ok {
		return fmt.Errorf("installed hook catalog is malformed")
	}
	for _, event := range knownEvents {
		if groups, present := hooks[event]; present {
			if _, ok := groups.([]any); !ok {
				return fmt.Errorf("installed hook event %s is malformed", event)
			}
		}
	}
	return nil
}

// HookGroupContainsAgentHarness reports whether a host hook group already holds
// an issueops lifecycle hook command, so installers can remove their own
// prior hook while preserving third-party groups.
func HookGroupContainsAgentHarness(group any) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && isAgentHarnessHookCommand(cmd) {
			return true
		}
	}
	return false
}

func HookGroupContainsCommand(group any, commandPrefix string) bool {
	m, ok := group.(map[string]any)
	if !ok {
		return false
	}
	hooks, ok := m["hooks"].([]any)
	if !ok {
		return false
	}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		if command, ok := hm["command"].(string); ok && strings.HasPrefix(command, commandPrefix) {
			return true
		}
	}
	return false
}

func isAgentHarnessHookCommand(command string) bool {
	executable, ok := hookCommandTarget(command)
	return ok && filepath.Base(executable) == "issueops"
}
