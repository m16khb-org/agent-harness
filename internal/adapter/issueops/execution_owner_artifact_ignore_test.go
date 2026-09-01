package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/preflight"
)

// TestOwnerArtifactDirectoryIsInvisibleToGitStatus는 하네스가 만든 봉인
// 아티팩트가 하네스 자신의 게이트를 막지 않는지 본다.
//
// `execution prepare`는 plan을 canonical worktree 안
// `.agent-harness/issues/<n>/artifact/`에 materialize한다. 이 경로는 대상
// 저장소에서 추적되지 않으므로 `git status --porcelain`에 `?? .agent-harness/`로
// 잡히고, strict PR readiness의 `worktree_clean`과 `cleanup finish`가 막힌다.
// 구현을 모두 마친 PR 게이트에서야 `worktree_clean` 한 단어로 드러나기 때문에
// 원인이 하네스 자신이라는 사실을 알아내기 어렵다. `ChangeFingerprint`도
// 미추적 경로를 포함하므로, 나중에 손으로 ignore하면 `ai_slop_clean_stale`이
// 뒤따른다.
//
// 아티팩트 디렉터리는 하네스가 소유하고 워크트리와 수명을 같이하므로,
// 자기 무시 `.gitignore`로 자기 흔적만 지운다. 사용자의 다른 미추적 파일은
// 그대로 보여야 한다.
func TestOwnerArtifactDirectoryIsInvisibleToGitStatus(t *testing.T) {
	repo := initIssueOpsRepo(t)
	artifactDir := filepath.Join(repo, ".agent-harness", "issues", "1", "artifact")

	if err := writeExecutionOwnerArtifact(repo, filepath.Join(artifactDir, "plan.md"), []byte("plan\n")); err != nil {
		t.Fatalf("write owner artifact: %v", err)
	}

	code, out, stderr := preflight.GitCmd(repo, "status", "--porcelain=v1")
	if code != 0 {
		t.Fatalf("git status failed: %s", stderr)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("harness-owned artifact directory must not dirty the worktree; git status:\n%s", out)
	}

	// 사용자의 다른 미추적 파일은 계속 보여야 한다. 아티팩트 무시가
	// 실제 dirt까지 가리면 게이트가 무의미해진다.
	writeIssueOpsFile(t, repo, "user-change.txt", "user\n")
	writeIssueOpsFile(t, repo, ".agent-harness/issues/1/notes.md", "notes\n")
	code, out, stderr = preflight.GitCmd(repo, "status", "--porcelain=v1", "--untracked-files=all")
	if code != 0 {
		t.Fatalf("git status failed: %s", stderr)
	}
	for _, want := range []string{"user-change.txt", ".agent-harness/issues/1/notes.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("git status must still report untracked %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "artifact/plan.md") {
		t.Errorf("owner artifact must stay ignored; got:\n%s", out)
	}

	// 무시 규칙은 아티팩트 디렉터리 안에만 있어야 한다. 대상 저장소의
	// 추적 파일(.gitignore 등)을 하네스가 대신 고치지 않는다.
	if _, err := os.Lstat(filepath.Join(repo, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("harness must not create or modify the repository .gitignore: %v", err)
	}
}
