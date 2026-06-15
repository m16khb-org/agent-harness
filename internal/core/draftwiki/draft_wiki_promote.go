package draftwiki

import (
	"fmt"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/draftwiki/llmpromote"
	"agent-harness/internal/core/handoff"
	"agent-harness/internal/core/repopath"
)

func PromoteDraftWiki(req DraftWikiPromoteRequest) (DraftWikiPromoteResult, error) {
	root, err := repopath.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return DraftWikiPromoteResult{}, err
	}
	from, err := resolveDraftWikiDraft(root, req.Path)
	if err != nil {
		return DraftWikiPromoteResult{}, err
	}
	if from.Status != "approved" {
		return DraftWikiPromoteResult{}, fmt.Errorf("draft %s has status %q; promote requires approved", from.RelPath, from.Status)
	}
	targetWiki := strings.TrimSpace(req.TargetWiki)
	if targetWiki == "" {
		targetWiki = from.TargetWiki
	}
	if targetWiki == "" {
		return DraftWikiPromoteResult{}, fmt.Errorf("target wiki is required via --target-wiki or draft frontmatter target_wiki")
	}
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		targetType = from.TargetType
	}
	if targetType == "" {
		targetType = "notes"
	}
	args := []string{"@wiki", "ingest", from.RelPath, "--wiki", targetWiki, "--type", targetType}
	result := DraftWikiPromoteResult{
		OK:             true,
		Kind:           "draft_wiki_promote",
		RepoRoot:       root,
		DraftDir:       filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		DryRun:         !req.Confirm,
		Confirm:        req.Confirm,
		Executed:       false,
		UpstreamTool:   "m16khb/llm-wiki",
		HandoffCommand: handoff.JoinArgs(args),
		HandoffArgs:    args,
		From:           from,
	}
	if !req.Confirm {
		return result, nil
	}
	promoted, err := llmpromote.Promote(llmpromote.Request{
		RepoRoot:          root,
		Draft:             llmPromoteDraft(from),
		TargetWiki:        targetWiki,
		TargetType:        targetType,
		LLMWikiConfigPath: req.LLMWikiConfigPath,
	})
	if err != nil {
		return DraftWikiPromoteResult{}, err
	}
	result.Executed = true
	result.LLMWikiRoot = promoted.WikiRoot
	result.LLMWikiRawPath = promoted.RawPath
	result.LLMWikiRawRel = promoted.RawRel
	result.LLMWikiLogPath = promoted.LogPath
	return result, nil
}

func llmPromoteDraft(draft DraftWikiDraft) llmpromote.Draft {
	return llmpromote.Draft{
		Title:   draft.Title,
		RelPath: draft.RelPath,
		Path:    draft.Path,
		Summary: draft.Summary,
	}
}
