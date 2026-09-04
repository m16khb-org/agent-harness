package upstream

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	upstreamcontract "issueops/internal/contract/upstream"
)

// GitSkillStore materializes upstream skills from git into a harness-owned
// cache and links them into the host skill directory, mirroring how the harness
// links its own skills. SkillsDir is the host directory (~/.claude/skills);
// CacheDir is the harness-owned copy the links point at.
type GitSkillStore struct {
	SkillsDir string
	CacheDir  string
}

// InstalledSkills reports skill names already visible to the host, whoever
// installed them, so an existing skill is never overwritten. A missing host
// skill directory is an empty inventory, not an error.
func (s GitSkillStore) InstalledSkills() ([]string, error) {
	entries, err := os.ReadDir(s.SkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// InstallSkill fetches one declared skill directory and links it into the host
// skill directory. The fetch lands in a scratch directory first so a failed or
// manifest-less fetch leaves neither a cache copy nor a host link behind.
func (s GitSkillStore) InstallSkill(ctx context.Context, entry upstreamcontract.SkillEntry) error {
	if strings.TrimSpace(s.SkillsDir) == "" || strings.TrimSpace(s.CacheDir) == "" {
		return fmt.Errorf("upstream skill store is not configured")
	}
	if err := os.MkdirAll(s.CacheDir, 0o755); err != nil {
		return err
	}
	work := filepath.Join(s.CacheDir, ".fetch-"+entry.Name)
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()

	if err := s.fetch(ctx, entry, work); err != nil {
		return err
	}
	source := work
	if entry.Path != "" {
		source = filepath.Join(work, filepath.FromSlash(entry.Path))
	}
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); err != nil {
		return fmt.Errorf("upstream skill %s has no SKILL.md at %s/%s", entry.Name, entry.Repo, entry.Path)
	}
	if err := os.RemoveAll(filepath.Join(source, ".git")); err != nil {
		return err
	}

	dest := filepath.Join(s.CacheDir, entry.Name)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.Rename(source, dest); err != nil {
		return err
	}
	return s.link(dest, filepath.Join(s.SkillsDir, entry.Name))
}

func (s GitSkillStore) fetch(ctx context.Context, entry upstreamcontract.SkillEntry, work string) error {
	args := []string{"clone", "--quiet", "--depth", "1", "--filter=blob:none"}
	if entry.Path != "" {
		args = append(args, "--sparse")
	}
	if entry.Ref != "" {
		args = append(args, "--branch", entry.Ref)
	}
	args = append(args, entry.Repo, work)
	if err := runGit(ctx, "", args...); err != nil {
		return err
	}
	if entry.Path == "" {
		return nil
	}
	return runGit(ctx, work, "sparse-checkout", "set", entry.Path)
}

// link points the host skill name at the harness cache copy. Only a symlink is
// replaced: a real directory there belongs to someone else.
func (s GitSkillStore) link(target, path string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("refusing to replace non-symlink path: %s", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, path)
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// An upstream fetch runs inside install; a credential prompt would hang it.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
