package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func requireRemoteValues(kind string, required []string, actual []string) error {
	actualSet := map[string]bool{}
	for _, value := range actual {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			actualSet[value] = true
		}
	}
	missing := []string{}
	for _, value := range required {
		cleaned := strings.TrimSpace(strings.ToLower(value))
		if cleaned != "" && !actualSet[cleaned] {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("remote artifact missing verified %s(s): %s", kind, strings.Join(missing, ", "))
	}
	return nil
}

func commandOutputError(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return fmt.Errorf("%s", stderr)
		}
	}
	return err
}
