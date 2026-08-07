package remoteartifact

import (
	"os"
	"path/filepath"
	"strings"

	"agent-harness/internal/domain/commandparse"
)

func readRemoteArtifactBodyFile(repo string, path string) string {
	p := strings.TrimSpace(path)
	if p == "" || p == "-" {
		return ""
	}
	if !filepath.IsAbs(p) {
		base := cleanAbsPath(repo)
		if base != "" {
			p = filepath.Join(base, p)
		}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func fillRemoteArtifactInlineBodyFile(artifact *remoteArtifactCommand, command string) {
	if strings.TrimSpace(artifact.body) != "" || strings.TrimSpace(artifact.bodyFilePath) == "" {
		return
	}
	artifact.body = extractInlineHereDocBodyForTarget(command, artifact.bodyFilePath)
}

func extractInlineHereDocBodyForTarget(command string, target string) string {
	targets := remoteArtifactBodyFileTargetAliases(target)
	if len(targets) == 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(command, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(line, "<<") {
			continue
		}
		if !lineWritesHereDocToAnyTarget(line, targets) {
			continue
		}
		marker := hereDocMarkerFromLine(line)
		if marker == "" {
			continue
		}
		body := []string{}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == marker {
				return strings.Join(body, "\n")
			}
			body = append(body, lines[j])
		}
		return ""
	}
	return ""
}

func remoteArtifactBodyFileTargetAliases(target string) []string {
	target = strings.TrimSpace(target)
	if target == "" || target == "-" {
		return nil
	}
	aliases := []string{target}
	if strings.HasPrefix(target, "$") {
		name := strings.TrimPrefix(target, "$")
		name = strings.TrimPrefix(name, "{")
		name = strings.TrimSuffix(name, "}")
		if name != "" {
			aliases = append(aliases, "$"+name, "${"+name+"}", name)
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, alias := range aliases {
		alias = strings.Trim(alias, `"'`)
		if alias != "" && !seen[alias] {
			seen[alias] = true
			out = append(out, alias)
		}
	}
	return out
}

func lineWritesHereDocToAnyTarget(line string, targets []string) bool {
	tokens := commandparse.SplitCommandTokens(line)
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == ">" || token == "1>":
			if i+1 < len(tokens) && tokenMatchesAnyRemoteArtifactTarget(tokens[i+1], targets) {
				return true
			}
		case strings.HasPrefix(token, ">") && len(token) > 1:
			if tokenMatchesAnyRemoteArtifactTarget(strings.TrimPrefix(token, ">"), targets) {
				return true
			}
		}
	}
	return false
}

func tokenMatchesAnyRemoteArtifactTarget(token string, targets []string) bool {
	token = strings.TrimSpace(strings.Trim(token, `"'`))
	for _, target := range targets {
		if token == target {
			return true
		}
	}
	return false
}

func hereDocMarkerFromLine(line string) string {
	index := strings.Index(line, "<<")
	if index < 0 {
		return ""
	}
	rest := strings.TrimSpace(line[index+2:])
	rest = strings.TrimPrefix(rest, "-")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	fields := commandparse.SplitCommandTokens(rest)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Trim(fields[0], `"'`))
}
