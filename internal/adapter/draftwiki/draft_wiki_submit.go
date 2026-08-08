package draftwiki

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
	"os"
	"path/filepath"
	"strings"
)

func SubmitDraftWiki(req draftwikicontract.DraftWikiSubmitRequest) (draftwikicontract.DraftWikiSubmitResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return draftwikicontract.DraftWikiSubmitResult{}, err
	}
	inputPath, err := ResolveRepoFile(root, req.DraftPath)
	if err != nil {
		return draftwikicontract.DraftWikiSubmitResult{}, err
	}
	body, err := os.ReadFile(inputPath)
	if err != nil {
		return draftwikicontract.DraftWikiSubmitResult{}, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	draftPath, err := writeSuggestedDraft(root, req.Title, req.TargetWiki, targetType, string(body))
	if err != nil {
		return draftwikicontract.DraftWikiSubmitResult{}, err
	}
	draft, err := readDraftWikiDraft(root, draftPath, "draft")
	if err != nil {
		return draftwikicontract.DraftWikiSubmitResult{}, err
	}
	return draftwikicontract.DraftWikiSubmitResult{
		OK:        true,
		Kind:      "draft_wiki_submit",
		RepoRoot:  root,
		InputPath: filepath.Clean(inputPath),
		Draft:     draft,
	}, nil
}
