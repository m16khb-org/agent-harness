package draftwiki

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
	"os"
	"path/filepath"
	"strings"
)

func SuggestDraftWiki(req draftwikicontract.DraftWikiSuggestRequest) (draftwikicontract.DraftWikiSuggestResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return draftwikicontract.DraftWikiSuggestResult{}, err
	}
	inputPath, err := ResolveRepoFile(root, req.InputPath)
	if err != nil {
		return draftwikicontract.DraftWikiSuggestResult{}, err
	}
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return draftwikicontract.DraftWikiSuggestResult{}, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	prompt := buildDraftWikiSuggestPrompt(req, string(input), targetType)
	result := draftwikicontract.DraftWikiSuggestResult{
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
