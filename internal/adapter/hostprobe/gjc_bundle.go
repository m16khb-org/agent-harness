package hostprobe

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

type gjcPluginManifest struct {
	Kind    string         `json:"kind"`
	Name    string         `json:"name"`
	Version string         `json:"version"`
	MCPs    []gjcPluginMCP `json:"mcps"`
}

type gjcPluginMCP struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Cwd       string   `json:"cwd"`
}

func writeGJCBundle(bundleRoot string, serve []string) error {
	manifest, err := json.MarshalIndent(gjcPluginManifest{
		Kind:    "gajae-code-plugin",
		Name:    "agent-harness-conformance",
		Version: "0.0.0",
		MCPs: []gjcPluginMCP{{
			Name:      "agent_harness_probe",
			Transport: "stdio",
			Command:   "bun",
			Args:      []string{"./launcher.ts"},
			Cwd:       ".",
		}},
	}, "", "  ")
	if err != nil {
		return err
	}
	serveJSON, err := json.Marshal(serve)
	if err != nil {
		return err
	}
	launcher := fmt.Sprintf(`import { spawn } from "node:child_process";

const argv = JSON.parse(%s) as string[];
const child = spawn(argv[0], argv.slice(1), {
	stdio: ["inherit", "inherit", "inherit"],
});
child.on("error", () => process.exit(1));
child.on("exit", code => process.exit(code ?? 1));
`, jsonString(string(serveJSON)))
	if err := writePrivateFile(filepath.Join(bundleRoot, "gajae-plugin.json"), append(manifest, '\n')); err != nil {
		return err
	}
	return writePrivateFile(filepath.Join(bundleRoot, "launcher.ts"), []byte(launcher))
}
