package draftwiki

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func PromoteDraftWiki(req draftwikicontract.DraftWikiPromoteRequest) (draftwikicontract.DraftWikiPromoteResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return draftwikicontract.DraftWikiPromoteResult{}, err
	}
	from, err := resolveDraftWikiDraft(root, req.Path)
	if err != nil {
		return draftwikicontract.DraftWikiPromoteResult{}, err
	}
	if from.Status != "approved" {
		return draftwikicontract.DraftWikiPromoteResult{}, fmt.Errorf("draft %s has status %q; promote requires approved", from.RelPath, from.Status)
	}
	exportPath := filepath.Join(root, filepath.FromSlash(DraftWikiDir), "exported", filepath.Base(from.Path))
	exportRel, err := filepath.Rel(root, exportPath)
	if err != nil {
		exportRel = exportPath
	}
	exportRel = filepath.ToSlash(exportRel)
	exportLogPath := filepath.Join(root, filepath.FromSlash(DraftWikiDir), "exported", "export.log")
	result := draftwikicontract.DraftWikiPromoteResult{
		OK:            true,
		Kind:          "draft_wiki_promote",
		RepoRoot:      root,
		DraftDir:      filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		DryRun:        !req.Confirm,
		Confirm:       req.Confirm,
		Executed:      false,
		From:          from,
		ExportPath:    exportPath,
		ExportRel:     exportRel,
		ExportLogPath: exportLogPath,
	}
	if !req.Confirm {
		return result, nil
	}
	to, err := moveDraftWikiFile(root, from, "exported")
	if err != nil {
		return draftwikicontract.DraftWikiPromoteResult{}, err
	}
	if err := appendDraftWikiExportLog(exportLogPath, from, to); err != nil {
		return draftwikicontract.DraftWikiPromoteResult{}, err
	}
	result.Executed = true
	result.To = &to
	result.ExportPath = to.Path
	result.ExportRel = to.RelPath
	return result, nil
}

func appendDraftWikiExportLog(logPath string, from, to draftwikicontract.DraftWikiDraft) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	entry := fmt.Sprintf("\n## [%s] export | %s\n\n- From: %s\n- To: %s\n", time.Now().UTC().Format(time.RFC3339Nano), from.Title, from.RelPath, to.RelPath)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}
