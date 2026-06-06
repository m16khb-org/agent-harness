package codex

import (
	"errors"
	"strings"
)

func shellQuote(value string) string {
	if value == "" {
		return "harness"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func joinErrors(errs []error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
