package draftwikicli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func QueueMaterial(repo, input, material string, stdinFlag bool) (string, error) {
	count := 0
	if strings.TrimSpace(input) != "" {
		count++
	}
	if strings.TrimSpace(material) != "" {
		count++
	}
	if stdinFlag {
		count++
	}
	if count != 1 {
		return "", fmt.Errorf("exactly one of --input, --material, or --stdin is required")
	}
	if strings.TrimSpace(material) != "" {
		return material, nil
	}
	if stdinFlag {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, filepath.FromSlash(path))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
