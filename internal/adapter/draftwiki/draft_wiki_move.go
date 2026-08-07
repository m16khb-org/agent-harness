package draftwiki

import (
	"fmt"
	"os"
	"path/filepath"

	"agent-harness/internal/adapter/repopath"
)

func ApproveDraftWiki(req DraftWikiMoveRequest) (DraftWikiMoveResult, error) {
	return moveDraftWiki(req, "draft", "approved", "draft_wiki_approve")
}

func RejectDraftWiki(req DraftWikiMoveRequest) (DraftWikiMoveResult, error) {
	return moveDraftWiki(req, "", "rejected", "draft_wiki_reject")
}

func moveDraftWiki(req DraftWikiMoveRequest, requiredStatus, targetStatus, kind string) (DraftWikiMoveResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiMoveResult{}, err
	}
	from, err := resolveDraftWikiDraft(root, req.Path)
	if err != nil {
		return DraftWikiMoveResult{}, err
	}
	if requiredStatus != "" && from.Status != requiredStatus {
		return DraftWikiMoveResult{}, fmt.Errorf("draft %s has status %q; %s requires %s", from.RelPath, from.Status, kind, requiredStatus)
	}
	to, err := moveDraftWikiFile(root, from, targetStatus)
	if err != nil {
		return DraftWikiMoveResult{}, err
	}
	return DraftWikiMoveResult{
		OK:       true,
		Kind:     kind,
		RepoRoot: root,
		DraftDir: filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		From:     from,
		To:       to,
	}, nil
}

func moveDraftWikiFile(root string, from DraftWikiDraft, targetStatus string) (DraftWikiDraft, error) {
	if !isDraftWikiStatus(targetStatus) {
		return DraftWikiDraft{}, fmt.Errorf("unsupported draft wiki status %q", targetStatus)
	}
	targetPath := filepath.Join(root, filepath.FromSlash(DraftWikiDir), targetStatus, filepath.Base(from.Path))
	if _, err := os.Stat(targetPath); err == nil {
		return DraftWikiDraft{}, fmt.Errorf("target draft already exists: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return DraftWikiDraft{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return DraftWikiDraft{}, err
	}
	if err := os.Rename(from.Path, targetPath); err != nil {
		return DraftWikiDraft{}, err
	}
	return readDraftWikiDraft(root, targetPath, targetStatus)
}
