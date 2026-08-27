package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudePluginHost drives the Claude Code plugin CLI. Binary defaults to
// "claude" on PATH.
type ClaudePluginHost struct {
	Binary string
}

func (h ClaudePluginHost) binary() string {
	if strings.TrimSpace(h.Binary) != "" {
		return h.Binary
	}
	return "claude"
}

// Available reports whether the Claude Code CLI is on PATH. The harness never
// gates its own install on this: an absent CLI turns plugin entries into skips.
func (h ClaudePluginHost) Available() bool {
	_, err := exec.LookPath(h.binary())
	return err == nil
}

func (h ClaudePluginHost) InstalledPlugins(ctx context.Context) ([]string, error) {
	out, err := h.run(ctx, "plugin", "list", "--json")
	if err != nil {
		return nil, err
	}
	return parsePluginIDs(out)
}

func (h ClaudePluginHost) Marketplaces(ctx context.Context) ([]string, error) {
	out, err := h.run(ctx, "plugin", "marketplace", "list", "--json")
	if err != nil {
		return nil, err
	}
	return parseMarketplaceNames(out)
}

func (h ClaudePluginHost) AddMarketplace(ctx context.Context, source string) error {
	_, err := h.run(ctx, "plugin", "marketplace", "add", source)
	return err
}

func (h ClaudePluginHost) InstallPlugin(ctx context.Context, id string) error {
	_, err := h.run(ctx, "plugin", "install", id, "--scope", "user", "--yes")
	return err
}

func (h ClaudePluginHost) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, h.binary(), args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%s %s: %w: %s", h.binary(), strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("%s %s: %w", h.binary(), strings.Join(args, " "), err)
	}
	return out, nil
}

func parsePluginIDs(body []byte) ([]string, error) {
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("parse installed plugin list: %w", err)
	}
	return collectNonEmpty(len(rows), func(i int) string { return rows[i].ID }), nil
}

func parseMarketplaceNames(body []byte) ([]string, error) {
	var rows []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("parse marketplace list: %w", err)
	}
	return collectNonEmpty(len(rows), func(i int) string { return rows[i].Name }), nil
}

func collectNonEmpty(count int, at func(int) string) []string {
	values := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if value := strings.TrimSpace(at(i)); value != "" {
			values = append(values, value)
		}
	}
	return values
}
