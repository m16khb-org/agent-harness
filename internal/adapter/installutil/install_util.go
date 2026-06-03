package installutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"agent-harness/internal/port"
)

func PlanHostSkillLinks(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error) {
	return PlanHostSkills(root, nil, destRoot, skillNames, host, dryRun)
}

// PlanHostSkills installs enabled skills for a host. When embedded is nil it
// symlinks from <root>/skills (developer/checkout mode); when embedded is set it
// copies each skill tree from the embedded FS into destRoot (packaged-binary
// mode). Host filtering reads each skill's install.json from the same source.
func PlanHostSkills(root string, embedded fs.FS, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error) {
	source := embedded
	if source == nil {
		source = os.DirFS(filepath.Join(root, "skills"))
	}
	enabledSkills, skippedSkills := skillNamesForHostFS(source, skillNames, host)
	messages := make([]string, 0, len(skippedSkills))
	for _, skillName := range skippedSkills {
		messages = append(messages, "skip skill for "+host+": "+skillName)
	}
	var links []port.InstallLink
	var errs []error
	if embedded == nil {
		links, errs = PlanSkillLinks(root, destRoot, enabledSkills, dryRun)
		return enabledSkills, links, messages, errs
	}
	for _, skillName := range enabledSkills {
		link, err := copySkillFromFS(embedded, skillName, filepath.Join(destRoot, skillName), dryRun)
		links = append(links, link)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return enabledSkills, links, messages, errs
}

// copySkillFromFS materializes one embedded skill tree at dest, replacing any
// prior symlink or copy so packaged upgrades stay clean.
func copySkillFromFS(srcFS fs.FS, skillName, dest string, dryRun bool) (port.InstallLink, error) {
	link := port.InstallLink{Path: dest, Target: "embedded:" + skillName}
	if dryRun {
		link.WouldCreate = true
		return link, nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return link, err
	}
	err := fs.WalkDir(srcFS, skillName, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(skillName, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return link, err
	}
	link.Created = true
	return link, nil
}

func skillNamesForHostFS(source fs.FS, skillNames []string, host string) (enabled, skipped []string) {
	for _, skillName := range skillNames {
		if skillEnabledForHostFS(source, skillName, host) {
			enabled = append(enabled, skillName)
		} else {
			skipped = append(skipped, skillName)
		}
	}
	return enabled, skipped
}

func skillEnabledForHostFS(source fs.FS, skillName, host string) bool {
	b, err := fs.ReadFile(source, skillName+"/install.json")
	if err != nil {
		return true
	}
	var cfg struct {
		Hosts []string `json:"hosts"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil || len(cfg.Hosts) == 0 {
		return true
	}
	for _, allowed := range cfg.Hosts {
		if allowed == host {
			return true
		}
	}
	return false
}

func PlanSkillLinks(root, destRoot string, skillNames []string, dryRun bool) ([]port.InstallLink, []error) {
	links := make([]port.InstallLink, 0, len(skillNames))
	var errs []error
	for _, skillName := range skillNames {
		link, err := EnsureSymlinkPlan(filepath.Join(root, "skills", skillName), filepath.Join(destRoot, skillName), dryRun)
		links = append(links, link)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return links, errs
}

func WriteText(path, kind, content string, perm os.FileMode) (port.InstallFile, error) {
	return WriteTextPlan(path, kind, content, perm, false)
}

func WriteTextPlan(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, []byte(content)) {
		return file, nil
	} else if err != nil && !os.IsNotExist(err) && !dryRun {
		return file, err
	}
	if dryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func WriteJSON(path, kind string, value any, perm os.FileMode) (port.InstallFile, error) {
	return WriteJSONPlan(path, kind, value, perm, false)
}

func WriteJSONPlan(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return port.InstallFile{Path: path, Kind: kind}, err
	}
	return WriteTextPlan(path, kind, string(append(b, '\n')), perm, dryRun)
}

func EnsureSymlink(target, path string) (port.InstallLink, error) {
	return EnsureSymlinkPlan(target, path, false)
}

func EnsureSymlinkPlan(target, path string, dryRun bool) (port.InstallLink, error) {
	link := port.InstallLink{Path: path, Target: target}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return link, fmt.Errorf("refusing to replace non-symlink path: %s", path)
		}
		current, readErr := os.Readlink(path)
		if readErr == nil && current == target {
			return link, nil
		}
		if dryRun {
			link.WouldCreate = true
			return link, nil
		}
		if err := os.Remove(path); err != nil {
			return link, err
		}
	} else if !os.IsNotExist(err) {
		return link, err
	}
	if dryRun {
		link.WouldCreate = true
		return link, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return link, err
	}
	if err := os.Symlink(target, path); err != nil {
		return link, err
	}
	link.Created = true
	return link, nil
}

func TOMLString(value string) string {
	b, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(b)
}
