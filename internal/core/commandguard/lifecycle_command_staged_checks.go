package commandguard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/searchrouting"
)

func StagedCheckDecision(tool, repo, command string) (string, string) {
	if !searchrouting.IsShellTool(tool) {
		return "", ""
	}
	for _, command := range ExpandPackageScriptCommands(repo, command) {
		if BroadBiomeCheckCommand(command) {
			return "ask", "Broad lint/format checks can fail on unrelated existing debt. Prefer staged or explicit changed-file checks such as `biome check --staged`, `biome format --staged`, lint-staged, or a file list for this diff; ask the user before running a repo-wide apps/libs check."
		}
	}
	return "", ""
}

func ExpandPackageScriptCommands(repo string, command string) []string {
	commands := []string{command}
	tokens := commandparse.SplitCommandTokens(command)
	for i := 0; i+2 < len(tokens); i++ {
		cli := searchrouting.SearchTokenName(tokens[i])
		if cli != "npm" {
			continue
		}
		action := strings.ToLower(searchrouting.SearchTokenName(tokens[i+1]))
		if action != "run" && action != "run-script" {
			continue
		}
		scriptName := strings.TrimSpace(tokens[i+2])
		if script := PackageScript(repo, scriptName); script != "" {
			commands = append(commands, script)
		}
	}
	return commands
}

func PackageScript(repo string, scriptName string) string {
	if strings.TrimSpace(repo) == "" || strings.TrimSpace(scriptName) == "" {
		return ""
	}
	body, err := os.ReadFile(filepath.Join(repo, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil {
		return ""
	}
	return strings.TrimSpace(pkg.Scripts[scriptName])
}

func BroadBiomeCheckCommand(command string) bool {
	tokens := commandparse.SplitCommandTokens(command)
	for i := 0; i+1 < len(tokens); i++ {
		if searchrouting.SearchTokenName(tokens[i]) != "biome" {
			continue
		}
		subcommand := strings.ToLower(searchrouting.SearchTokenName(tokens[i+1]))
		if subcommand != "check" && subcommand != "format" && subcommand != "ci" {
			continue
		}
		args := tokens[i+2:]
		if BiomeArgsAreScoped(args) {
			continue
		}
		if BiomeArgsIncludeBroadRepoDirs(args) {
			return true
		}
	}
	return false
}

func BiomeArgsAreScoped(args []string) bool {
	for _, arg := range args {
		name := strings.TrimSpace(arg)
		if name == "--staged" || name == "--changed" || strings.HasPrefix(name, "--since") {
			return true
		}
	}
	return false
}

func BiomeArgsIncludeBroadRepoDirs(args []string) bool {
	for _, arg := range args {
		name := strings.Trim(strings.TrimSpace(arg), `"'`)
		if name == "apps" || name == "libs" || name == "apps/" || name == "libs/" {
			return true
		}
	}
	return false
}
