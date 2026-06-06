package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/agysettings"
	"agent-harness/internal/core/repopath"
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

func SuggestDraftWiki(req DraftWikiSuggestRequest) (DraftWikiSuggestResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	inputPath, err := repopath.ResolveFile(root, req.InputPath)
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
	settingsPath := agysettings.ResolvePath(req.AgySettingsPath)
	configuredModel, err := agysettings.ReadConfiguredModel(settingsPath)
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
		Command:              ExternalLLMPrintCommandPreview(agyCommand),
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
	llm, err := RunExternalLLMPrint(ExternalLLMPrintRequest{Command: agyCommand, WorkDir: root, Prompt: prompt, Timeout: timeout})
	if err != nil {
		return DraftWikiSuggestResult{}, fmt.Errorf("agy print failed: %w: %s", err, strings.TrimSpace(string(llm.Output)))
	}
	draftBody, err := decodeDraftWikiSuggestAgyOutput(llm.Output)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	draftPath, err := writeSuggestedDraft(root, req.Title, req.TargetWiki, targetType, agyModel, draftBody)
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
