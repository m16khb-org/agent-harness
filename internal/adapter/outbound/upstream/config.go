// Package upstream implements the host side of upstream provisioning: reading
// the declaration file, driving the Claude Code plugin CLI, and materializing
// upstream skills from git into the host skill directory.
package upstream

import (
	"encoding/json"
	"fmt"
	"os"

	upstreamcontract "issueops/internal/contract/upstream"
)

// ReadConfig loads the upstream declaration. A missing file is not an error:
// a harness checkout without a declaration simply provisions nothing.
func ReadConfig(path string) (upstreamcontract.Config, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return upstreamcontract.Config{}, nil
		}
		return upstreamcontract.Config{}, err
	}
	var cfg upstreamcontract.Config
	if err := json.Unmarshal(body, &cfg); err != nil {
		return upstreamcontract.Config{}, fmt.Errorf("parse upstream declaration %s: %w", path, err)
	}
	return cfg, nil
}
