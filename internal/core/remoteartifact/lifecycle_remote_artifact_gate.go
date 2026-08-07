package remoteartifact

import (
	"fmt"
	"regexp"
	"strings"

	"agent-harness/internal/domain/searchrouting"
)

var (
	hangulRe       = regexp.MustCompile(`[가-힣]`)
	asciiWordRe    = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9_+-]*\b`)
	codeFenceRe    = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe   = regexp.MustCompile("`[^`]*`")
	urlRe          = regexp.MustCompile(`https?://\S+`)
	pathLikeTextRe = regexp.MustCompile(`(?:^|\s)[./~]?[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+`)
)

func KoreanBlockReason(tool, command, repo string) string {
	if !remoteArtifactGateAppliesToTool(tool) {
		return ""
	}
	artifact, ok := parseGHRemoteArtifactCommand(command, repo)
	if !ok {
		return ""
	}
	text := strings.TrimSpace(artifact.title + "\n" + artifact.body)
	if artifact.action == "create" && (strings.TrimSpace(artifact.title) == "" || strings.TrimSpace(artifact.body) == "") {
		if artifact.createFromIssue {
			return ""
		}
		return "IssueOps remote artifact gate requires inspectable Korean title and body before issue/pr/mr create/edit; provide --title and --body-file/--body after running the Korean gate"
	}
	if artifact.action != "create" && text == "" {
		return ""
	}
	hangul, englishWords := scoreKoreanRemoteArtifactLanguage(text)
	cli := remoteArtifactCLIName(artifact)
	if hangul < 20 {
		return fmt.Sprintf("IssueOps remote artifact gate failed: expected at least 20 Hangul chars before %s %s %s, got %d", cli, artifact.kind, artifact.action, hangul)
	}
	if hangul > 0 && float64(englishWords)/float64(hangul) > 1.2 {
		return fmt.Sprintf("IssueOps remote artifact gate failed: English prose ratio too high before %s %s %s (english_words=%d, hangul_chars=%d)", cli, artifact.kind, artifact.action, englishWords, hangul)
	}
	return ""
}

func remoteArtifactCLIName(artifact remoteArtifactCommand) string {
	switch artifact.provider {
	case "gitlab":
		return "glab"
	case "github":
		return "gh"
	default:
		return "remote"
	}
}

func remoteArtifactGateAppliesToTool(tool string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	if searchrouting.IsShellTool(tool) {
		return true
	}
	if tool == "" {
		return false
	}
	if !strings.HasPrefix(tool, "mcp__") {
		return false
	}
	if !(strings.Contains(tool, "github") || strings.Contains(tool, "gitlab") || strings.Contains(tool, "glab")) {
		return false
	}
	if strings.Contains(tool, "glab") && strings.Contains(tool, "api") {
		return true
	}
	if !(strings.Contains(tool, "issue") || strings.Contains(tool, "merge_request") || strings.Contains(tool, "pull_request") || strings.Contains(tool, "_mr") || strings.Contains(tool, "_pr")) {
		return false
	}
	return strings.Contains(tool, "create") || strings.Contains(tool, "open") || strings.Contains(tool, "update") || strings.Contains(tool, "edit") || strings.HasSuffix(tool, "_for") || strings.Contains(tool, "create_for") || strings.Contains(tool, "create-for")
}

func scoreKoreanRemoteArtifactLanguage(text string) (int, int) {
	prose := codeFenceRe.ReplaceAllString(text, " ")
	prose = inlineCodeRe.ReplaceAllString(prose, " ")
	prose = urlRe.ReplaceAllString(prose, " ")
	prose = pathLikeTextRe.ReplaceAllString(prose, " ")
	return len(hangulRe.FindAllString(prose, -1)), len(asciiWordRe.FindAllString(prose, -1))
}
