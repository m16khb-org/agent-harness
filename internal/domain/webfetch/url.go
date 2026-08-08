package webfetch

import (
	"fmt"
	"net/url"
	"strings"
)

func SanitizeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	if parsed.User != nil {
		parsed.User = url.User("<redacted>")
	}
	return parsed.String()
}

func ResolveRedirect(base, location string) (string, error) {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid redirect base: %w", err)
	}
	reference, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("invalid redirect location: %w", err)
	}
	return parsedBase.ResolveReference(reference).String(), nil
}

func IsRedirect(status int) bool {
	return status == 301 || status == 302 || status == 303 || status == 307 || status == 308
}
