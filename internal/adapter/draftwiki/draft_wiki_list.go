package draftwiki

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ListDraftWiki(req draftwikicontract.DraftWikiListRequest) (draftwikicontract.DraftWikiListResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return draftwikicontract.DraftWikiListResult{}, err
	}
	drafts := []draftwikicontract.DraftWikiDraft{}
	for _, status := range draftWikiStatusDirs {
		dir := filepath.Join(root, filepath.FromSlash(DraftWikiDir), status)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return draftwikicontract.DraftWikiListResult{}, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			draft, err := readDraftWikiDraft(root, filepath.Join(dir, entry.Name()), status)
			if err != nil {
				return draftwikicontract.DraftWikiListResult{}, err
			}
			drafts = append(drafts, draft)
		}
	}
	sort.Slice(drafts, func(i, j int) bool { return drafts[i].RelPath < drafts[j].RelPath })
	return draftwikicontract.DraftWikiListResult{
		OK:       true,
		Kind:     "draft_wiki_list",
		RepoRoot: root,
		DraftDir: filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		Drafts:   drafts,
	}, nil
}
