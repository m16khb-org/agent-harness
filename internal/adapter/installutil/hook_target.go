package installutil

import (
	"fmt"
	"sort"
	"strings"
)

// HookTargetDriftMessages reports installer-owned hook commands whose executable
// differs from the canonical runtime while ignoring co-resident third-party hooks.
func HookTargetDriftMessages(config map[string]any, host, expected string) []string {
	targets := hookTargets(config)
	observed := make([]string, 0, len(targets))
	for _, target := range targets {
		if target != expected {
			observed = append(observed, target)
		}
	}
	if len(observed) == 0 {
		return nil
	}
	messages := make([]string, 0, len(observed))
	for _, target := range observed {
		messages = append(messages, fmt.Sprintf(
			"%s native hook target is stale: observed=%s expected=%s; reinstall hooks and restart the %s session",
			host, target, expected, host,
		))
	}
	return messages
}

func hookTargets(config map[string]any) []string {
	hooks, _ := config["hooks"].(map[string]any)
	targets := map[string]bool{}
	for _, groupsValue := range hooks {
		groups, ok := groupsValue.([]any)
		if !ok {
			continue
		}
		for _, group := range groups {
			if !HookGroupContainsAgentHarness(group) {
				continue
			}
			for _, target := range hookGroupTargets(group) {
				targets[target] = true
			}
		}
	}
	observed := make([]string, 0, len(targets))
	for target := range targets {
		observed = append(observed, target)
	}
	sort.Strings(observed)
	return observed
}

func hookGroupTargets(group any) []string {
	m, _ := group.(map[string]any)
	hooks, _ := m["hooks"].([]any)
	targets := []string{}
	for _, hook := range hooks {
		hm, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		command, ok := hm["command"].(string)
		if !ok {
			continue
		}
		target, ok := hookCommandTarget(command)
		if ok {
			targets = append(targets, target)
		}
	}
	return targets
}

func hookCommandTarget(command string) (string, bool) {
	executable, remaining, ok := shellCommandWord(strings.TrimSpace(command))
	if !ok {
		return "", false
	}
	subcommand, _, ok := shellCommandWord(strings.TrimLeft(remaining, " \t\r\n"))
	if !ok || subcommand != "hook" {
		return "", false
	}
	return executable, true
}

func shellCommandWord(command string) (string, string, bool) {
	if command == "" {
		return "", "", false
	}
	var word strings.Builder
	for index := 0; index < len(command); {
		switch command[index] {
		case ' ', '\t', '\r', '\n':
			if word.Len() == 0 {
				return "", "", false
			}
			return word.String(), command[index:], true
		case '\\':
			if index+1 == len(command) {
				return "", "", false
			}
			word.WriteByte(command[index+1])
			index += 2
		case '\'', '"':
			quote := command[index]
			index++
			for index < len(command) && command[index] != quote {
				word.WriteByte(command[index])
				index++
			}
			if index == len(command) {
				return "", "", false
			}
			index++
		default:
			word.WriteByte(command[index])
			index++
		}
	}
	if word.Len() == 0 {
		return "", "", false
	}
	return word.String(), "", true
}
