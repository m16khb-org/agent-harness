package installutil

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type skillInstallConfig struct {
	Hosts []string `json:"hosts"`
}

// SkillEnabledForHost returns whether a shared skill should be installed for a
// host. Missing or empty config keeps the historical default: install for every
// host.
func SkillEnabledForHost(root, skillName, host string) bool {
	path := filepath.Join(root, "skills", skillName, "install.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var cfg skillInstallConfig
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

// SkillNamesForHost partitions already-discovered skills by their optional
// install.json host filter while preserving input order.
func SkillNamesForHost(root string, skillNames []string, host string) (enabled []string, skipped []string) {
	for _, skillName := range skillNames {
		if SkillEnabledForHost(root, skillName, host) {
			enabled = append(enabled, skillName)
		} else {
			skipped = append(skipped, skillName)
		}
	}
	return enabled, skipped
}
