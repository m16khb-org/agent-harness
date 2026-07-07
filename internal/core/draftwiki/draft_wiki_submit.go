package draftwiki

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/repopath"
)

type DraftWikiSubmitRequest struct {
	RepoRoot   string `json:"repo_root"`
	DraftPath  string `json:"draft_path"`
	Title      string `json:"title,omitempty"`
	TargetWiki string `json:"target_wiki,omitempty"`
	TargetType string `json:"target_type,omitempty"`
}

type DraftWikiSubmitResult struct {
	OK        bool           `json:"ok"`
	Kind      string         `json:"kind"`
	RepoRoot  string         `json:"repo_root"`
	InputPath string         `json:"input_path"`
	Draft     DraftWikiDraft `json:"draft"`
}

func SubmitDraftWiki(req DraftWikiSubmitRequest) (DraftWikiSubmitResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiSubmitResult{}, err
	}
	inputPath, err := repopath.ResolveFile(root, req.DraftPath)
	if err != nil {
		return DraftWikiSubmitResult{}, err
	}
	body, err := os.ReadFile(inputPath)
	if err != nil {
		return DraftWikiSubmitResult{}, err
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = "notes"
	}
	draftPath, err := writeSuggestedDraft(root, req.Title, req.TargetWiki, targetType, string(body))
	if err != nil {
		return DraftWikiSubmitResult{}, err
	}
	draft, err := readDraftWikiDraft(root, draftPath, "draft")
	if err != nil {
		return DraftWikiSubmitResult{}, err
	}
	return DraftWikiSubmitResult{
		OK:        true,
		Kind:      "draft_wiki_submit",
		RepoRoot:  root,
		InputPath: filepath.Clean(inputPath),
		Draft:     draft,
	}, nil
}
