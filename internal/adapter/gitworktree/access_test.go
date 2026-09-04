package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/adapter/preflight"
	"agent-harness/internal/port"
)

func TestProbeAccessReturnsHostSpecificRelaunchWithoutWorktreeMutation(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(base, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(base, 0o700) })
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-69", SourceRoot: repo, Root: filepath.Join(base, "69-access"),
		Branch: "69-access", BaseBranch: "main", BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"), Confirm: true,
	}
	// 워크트리는 아직 없으므로 relaunch는 source root로 되돌리는 형태다.
	for _, tt := range []struct{ host, wantPrefix string }{
		{"codex", "codex --cd '" + repo + "'"},
		{"claude", "cd '" + repo + "' && claude "},
	} {
		got, err := New().ProbeAccess(context.Background(), req, tt.host)
		if err != nil {
			t.Fatal(err)
		}
		if got.Allowed || got.Code != "canonical_worktree_base_inaccessible" || !strings.HasPrefix(got.RelaunchCommand, tt.wantPrefix) {
			t.Fatalf("unexpected %s access result: %#v", tt.host, got)
		}
	}
	if err := os.Chmod(base, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(req.Root); !os.IsNotExist(err) {
		t.Fatalf("access failure created a partial worktree: %v", err)
	}
}

func TestWorkspaceRelaunchCommandSupportsOmoLead(t *testing.T) {
	got, err := workspaceRelaunchCommand("omo", "/repo/owner's source", "/repo.worktrees/69-missing", "/repo.worktrees")
	if err != nil {
		t.Fatal(err)
	}
	if want := "cd '/repo/owner'\\''s source' && omo"; got != want {
		t.Fatalf("Omo relaunch command=%q want=%q", got, want)
	}
}

// 되띄운 세션은 canonical worktree 안에서 다시 시작해야 한다. source root로
// 돌려보내면 그 세션이 source checkout을 작업 대상으로 오인하고, 9단계 흐름에서
// 2단계가 이미 만들어 둔 워크트리를 지나친다. claude는 아예 cd가 없어 세션이
// 이전 디렉터리에 그대로 남아 있었다.
func TestWorkspaceRelaunchCommandLandsInExistingCanonicalWorktree(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "69-relaunch")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	quotedRoot, quotedBase := "'"+root+"'", "'"+base+"'"
	for _, tt := range []struct{ host, want string }{
		{"codex", "codex --cd " + quotedRoot + " --add-dir " + quotedBase},
		{"claude", "cd " + quotedRoot + " && claude --add-dir " + quotedBase},
		{"omo", "cd " + quotedRoot + " && omo"},
	} {
		got, err := workspaceRelaunchCommand(tt.host, "/unused/source", root, base)
		if err != nil {
			t.Fatalf("%s: %v", tt.host, err)
		}
		if got != tt.want {
			t.Fatalf("%s relaunch command=%q want=%q", tt.host, got, tt.want)
		}
	}
}

// 워크트리가 아직 없으면 cd가 실패해 host가 아예 뜨지 않는다. 그 사이클(orca
// 대안 경로, worktree를 prepare가 만드는 legacy direct)은 source root로 되돌린다.
func TestWorkspaceRelaunchCommandFallsBackToSourceRootWhenWorktreeMissing(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "69-absent")
	quotedSource, quotedBase := "'/repo/source'", "'"+base+"'"
	for _, tt := range []struct{ host, want string }{
		{"codex", "codex --cd " + quotedSource + " --add-dir " + quotedBase},
		{"claude", "cd " + quotedSource + " && claude --add-dir " + quotedBase},
		{"omo", "cd " + quotedSource + " && omo"},
	} {
		got, err := workspaceRelaunchCommand(tt.host, "/repo/source", root, base)
		if err != nil {
			t.Fatalf("%s: %v", tt.host, err)
		}
		if got != tt.want {
			t.Fatalf("%s fallback relaunch command=%q want=%q", tt.host, got, tt.want)
		}
	}
}

func TestWorkspaceRelaunchCommandRejectsUnknownHost(t *testing.T) {
	if _, err := workspaceRelaunchCommand("gemini", "/repo", "/repo.worktrees/1-x", "/repo.worktrees"); err == nil {
		t.Fatal("unknown host must be rejected")
	}
}

func initAccessRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "test@example.invalid"}, {"config", "user.name", "Test"}} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := preflight.GitCmd(repo, "add", "README.md"); code != 0 {
		t.Fatal(stderr)
	}
	if code, _, stderr := preflight.GitCmd(repo, "commit", "-q", "-m", "fixture"); code != 0 {
		t.Fatal(stderr)
	}
	return repo
}
