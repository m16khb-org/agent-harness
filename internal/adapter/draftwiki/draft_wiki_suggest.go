package draftwiki

import (
	"os"
	"path/filepath"
	"strings"
)

type DraftWikiSuggestRequest struct {
	RepoRoot   string `json:"repo_root"`
	InputPath  string `json:"input_path"`
	Title      string `json:"title"`
	TargetWiki string `json:"target_wiki"`
	TargetType string `json:"target_type"`
}

type DraftWikiSuggestResult struct {
	OK          bool            `json:"ok"`
	Kind        string          `json:"kind"`
	RepoRoot    string          `json:"repo_root"`
	DraftDir    string          `json:"draft_dir"`
	Executed    bool            `json:"executed"`
	InputPath   string          `json:"input_path"`
	Command     string          `json:"command"`
	PromptBytes int             `json:"prompt_bytes"`
	Prompt      string          `json:"prompt,omitempty"`
	Draft       *DraftWikiDraft `json:"draft,omitempty"`
}

func SuggestDraftWiki(req DraftWikiSuggestRequest) (DraftWikiSuggestResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	inputPath, err := ResolveRepoFile(root, req.InputPath)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return DraftWikiSuggestResult{}, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	prompt := buildDraftWikiSuggestPrompt(req, string(input), targetType)
	result := DraftWikiSuggestResult{
		OK:          true,
		Kind:        "draft_wiki_suggest",
		RepoRoot:    root,
		DraftDir:    filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		Executed:    false,
		InputPath:   inputPath,
		Command:     "host-agent judgement result file",
		PromptBytes: len([]byte(prompt)),
		Prompt:      prompt,
	}
	return result, nil
}
