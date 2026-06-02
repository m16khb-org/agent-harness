package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type DraftWikiSuggestRequest struct {
	RepoRoot        string        `json:"repo_root"`
	InputPath       string        `json:"input_path"`
	Title           string        `json:"title"`
	TargetWiki      string        `json:"target_wiki"`
	TargetType      string        `json:"target_type"`
	AgyCommand      string        `json:"agy_command"`
	AgyModel        string        `json:"agy_model"`
	AgySettingsPath string        `json:"-"`
	Write           bool          `json:"write"`
	Timeout         time.Duration `json:"-"`
}

type DraftWikiSuggestResult struct {
	OK                   bool            `json:"ok"`
	Kind                 string          `json:"kind"`
	RepoRoot             string          `json:"repo_root"`
	DraftDir             string          `json:"draft_dir"`
	Write                bool            `json:"write"`
	DryRun               bool            `json:"dry_run"`
	Executed             bool            `json:"executed"`
	InputPath            string          `json:"input_path"`
	Command              string          `json:"command"`
	AgyCommand           string          `json:"agy_command"`
	AgyModel             string          `json:"agy_model"`
	AgySettingsPath      string          `json:"agy_settings_path"`
	ModelSelectionMethod string          `json:"model_selection_method"`
	PromptBytes          int             `json:"prompt_bytes"`
	Draft                *DraftWikiDraft `json:"draft,omitempty"`
}

type agySettingsFile struct {
	Model string `json:"model"`
}

func SuggestDraftWiki(req DraftWikiSuggestRequest) (DraftWikiSuggestResult, error) {
	root, err := normalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	inputPath, err := resolveRepoFile(root, req.InputPath)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	agyCommand := strings.TrimSpace(req.AgyCommand)
	if agyCommand == "" {
		agyCommand = "agy"
	}
	settingsPath := resolveAgySettingsPath(req.AgySettingsPath)
	configuredModel, err := readAgyConfiguredModel(settingsPath)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	agyModel := strings.TrimSpace(req.AgyModel)
	if agyModel == "" {
		agyModel = configuredModel
	}
	if configuredModel != agyModel {
		return DraftWikiSuggestResult{}, fmt.Errorf("agy model mismatch: settings %s has %q, want %q; select the model in agy with /model or update the settings model key", settingsPath, configuredModel, agyModel)
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	prompt := buildDraftWikiSuggestPrompt(req, string(input), agyModel, targetType)
	result := DraftWikiSuggestResult{
		OK:                   true,
		Kind:                 "draft_wiki_suggest",
		RepoRoot:             root,
		DraftDir:             filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		Write:                req.Write,
		DryRun:               !req.Write,
		Executed:             false,
		InputPath:            inputPath,
		Command:              joinHandoffArgs([]string{agyCommand, "-p", "<prompt>"}),
		AgyCommand:           agyCommand,
		AgyModel:             agyModel,
		AgySettingsPath:      settingsPath,
		ModelSelectionMethod: "settings_json",
		PromptBytes:          len([]byte(prompt)),
	}
	if !req.Write {
		return result, nil
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, agyCommand, "-p", prompt)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return DraftWikiSuggestResult{}, fmt.Errorf("agy print timed out after %s", timeout)
	}
	if err != nil {
		return DraftWikiSuggestResult{}, fmt.Errorf("agy print failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	draftPath, err := writeSuggestedDraft(root, req.Title, req.TargetWiki, targetType, agyModel, string(out))
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	draft, err := readDraftWikiDraft(root, draftPath, "draft")
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	result.Executed = true
	result.Draft = &draft
	return result, nil
}

func resolveRepoFile(root, candidate string) (string, error) {
	if strings.TrimSpace(candidate) == "" {
		return "", fmt.Errorf("input path is required")
	}
	path := candidate
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("input path escapes repo root: %s", candidate)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("input path is a directory: %s", candidate)
	}
	return abs, nil
}

func resolveAgySettingsPath(path string) string {
	if strings.TrimSpace(path) != "" {
		return expandLeadingTilde(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
}

func readAgyConfiguredModel(settingsPath string) (string, error) {
	b, err := os.ReadFile(settingsPath)
	if err != nil {
		return "", fmt.Errorf("read agy settings %s: %w", settingsPath, err)
	}
	var settings agySettingsFile
	if err := json.Unmarshal(b, &settings); err != nil {
		return "", fmt.Errorf("parse agy settings %s: %w", settingsPath, err)
	}
	if strings.TrimSpace(settings.Model) == "" {
		return "", fmt.Errorf("agy settings %s has no model key", settingsPath)
	}
	return settings.Model, nil
}

func buildDraftWikiSuggestPrompt(req DraftWikiSuggestRequest, input, agyModel, targetType string) string {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Draft wiki candidate"
	}
	targetWiki := strings.TrimSpace(req.TargetWiki)
	if targetWiki == "" {
		targetWiki = "dev-fundamentals"
	}
	return fmt.Sprintf(`You are agent-harness draft-wiki suggester.

Goal: turn the source material into one durable wiki draft if it contains reusable long-term knowledge.

Return exactly one Markdown document, no surrounding code fences.
Use this YAML frontmatter:
---
title: %q
source: "claude-mem"
target_wiki: %q
target_type: %q
summary: "<one sentence>"
suggester: "agy -p"
model: %q
---

Rules:
- Keep only reusable cross-session knowledge.
- Do not include secrets, credentials, transient logs, or private personal data.
- Preserve concrete commands, paths, and decisions when they are useful later.
- If the source is not worth remembering, return a short draft titled "Rejected: <reason>" explaining why.

Source material:
%s
`, title, targetWiki, targetType, agyModel, input)
}

func writeSuggestedDraft(root, title, targetWiki, targetType, agyModel, output string) (string, error) {
	body := strings.TrimSpace(stripAgyOutputPreamble(stripMarkdownFence(output)))
	if body == "" {
		return "", fmt.Errorf("agy output was empty")
	}
	if !strings.HasPrefix(body, "---\n") {
		body = generatedDraftFrontmatter(title, targetWiki, targetType, agyModel) + "\n" + body + "\n"
	}
	meta := parseDraftWikiFrontmatter(body)
	draftTitle := meta["title"]
	if draftTitle == "" {
		draftTitle = title
	}
	if draftTitle == "" {
		draftTitle = "draft wiki candidate"
	}
	filename := time.Now().Format(time.DateOnly) + "-" + slugifyDraftWiki(draftTitle) + ".md"
	path := filepath.Join(root, filepath.FromSlash(DraftWikiDir), "draft", filename)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("draft already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func generatedDraftFrontmatter(title, targetWiki, targetType, agyModel string) string {
	if title == "" {
		title = "Draft wiki candidate"
	}
	if targetWiki == "" {
		targetWiki = "dev-fundamentals"
	}
	if targetType == "" {
		targetType = "notes"
	}
	return fmt.Sprintf(`---
title: %q
source: "claude-mem"
target_wiki: %q
target_type: %q
summary: "Draft generated by agy for human review."
suggester: "agy -p"
model: %q
---`, title, targetWiki, targetType, agyModel)
}

func stripMarkdownFence(output string) string {
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[len(lines)-1]) != "```" {
		return trimmed
	}
	return strings.Join(lines[1:len(lines)-1], "\n")
}

func stripAgyOutputPreamble(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		if line == "" || strings.HasPrefix(line, "ULTRAWORK MODE ENABLED") {
			lines = lines[1:]
			continue
		}
		break
	}
	return strings.Join(lines, "\n")
}

var draftWikiSlugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyDraftWiki(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = draftWikiSlugInvalid.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "draft"
	}
	return value
}
