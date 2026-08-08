package draftwiki

import (
	draftwikicontract "agent-harness/internal/contract/draftwiki"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func InitDraftWiki(req draftwikicontract.DraftWikiInitRequest) (draftwikicontract.DraftWikiInitResult, error) {
	root, err := NormalizeRepoRoot(req.RepoRoot)
	if err != nil {
		return draftwikicontract.DraftWikiInitResult{}, err
	}
	files := []ProjectDocsPlannedFile{}
	for rel, content := range draftWikiSeedFiles() {
		path := filepath.Join(root, filepath.FromSlash(rel))
		action := plannedFileAction(path, content)
		if req.Write && action == "create" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return draftwikicontract.DraftWikiInitResult{}, err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return draftwikicontract.DraftWikiInitResult{}, err
			}
		}
		if req.Write && action == "update" {
			action = "preserve"
		}
		files = append(files, ProjectDocsPlannedFile{
			RelPath: rel,
			Path:    path,
			Action:  action,
			Bytes:   len([]byte(content)),
			SHA256:  sha256Hex(content),
			Reason:  "repo-local draft wiki review staging",
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return draftwikicontract.DraftWikiInitResult{
		OK:          true,
		Kind:        "draft_wiki_init",
		RepoRoot:    root,
		DraftDir:    filepath.Join(root, filepath.FromSlash(DraftWikiDir)),
		Write:       req.Write,
		DryRun:      !req.Write,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Files:       files,
	}, nil
}

func draftWikiSeedFiles() map[string]string {
	files := map[string]string{
		filepath.ToSlash(filepath.Join(DraftWikiDir, "README.md")): draftWikiREADME(),
	}
	for _, status := range draftWikiStatusDirs {
		files[filepath.ToSlash(filepath.Join(DraftWikiDir, status, ".gitkeep"))] = ""
	}
	return files
}

func draftWikiREADME() string {
	return `# Draft Wiki

이 디렉토리는 agent-harness가 제안한 wiki 후보를 사용자가 검토하는 repo-local staging area다.

- ` + "`draft/`" + `: 에이전트 노트 등에서 선별된 후보. 아직 승인되지 않았다.
- ` + "`approved/`" + `: 사용자가 로컬 export를 승인한 후보.
- ` + "`rejected/`" + `: 사용자가 거절한 후보.
- ` + "`exported/`" + `: 승인 후 promote된 Markdown 산출물. 사용자가 원하는 저장소나 지식베이스로 직접 옮긴다.

주의: 이 디렉토리는 검토와 export를 위한 repo-local staging area다. promote는 외부 wiki나 companion tool에 쓰지 않는다.
`
}
