package apidoc

import (
	"encoding/json"
	"errors"
	"os"
)

var (
	ErrReviewGateFailed     = errors.New("api documentation review gate failed")
	ErrReviewResultRequired = errors.New("api documentation host-agent review result required")
	ErrStaticGateFailed     = errors.New("api documentation static check gate failed")
)

var ResolveTarget = func(target string) string {
	if target != "" {
		return target
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
