package commandparse

import (
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	commandparsecontract "agent-harness/internal/contract/commandparse"
	"agent-harness/internal/domain/shelltoken"
)

// 셸 토큰 판정은 도메인 규칙이므로 shelltoken이 소유한다. 아래 별칭은 이 파일이
// 원래 같은 패키지에서 쓰던 이름을 그대로 유지해, 분리가 호출부 문법을 바꾸지
// 않게 한다.
var (
	SplitCommandTokens                 = shelltoken.SplitCommandTokens
	HasActiveShellSpecialQuoting       = shelltoken.HasActiveShellSpecialQuoting
	HasActiveShellComment              = shelltoken.HasActiveShellComment
	HasActiveZshEqualsExpansion        = shelltoken.HasActiveZshEqualsExpansion
	HasUnquotedControlOperator         = shelltoken.HasUnquotedControlOperator
	HasUnquotedBackgroundOperator      = shelltoken.HasUnquotedBackgroundOperator
	HasActiveCommandSubstitution       = shelltoken.HasActiveCommandSubstitution
	HasActiveOutputRedirect            = shelltoken.HasActiveOutputRedirect
	HasActiveInputRedirect             = shelltoken.HasActiveInputRedirect
	HasActiveParameterOrTildeExpansion = shelltoken.HasActiveParameterOrTildeExpansion
	HasActivePathnameExpansion         = shelltoken.HasActivePathnameExpansion
)

// ExactIssueOpsCommand은 파싱된 정확한 `agent-harness issueops …` 명령이다.
// subcommand path, 전체 token slice, 그리고 flag가 시작되는 인덱스를 담는다.
type ExactIssueOpsCommand struct {
	Path   string
	Tokens []string
	Start  int
}

// ParseExactIssueOpsCommand은 명령을 ExactIssueOpsCommand으로 파싱하며, 활성
// shell control/expansion을 담은 명령은 모두 거부한다(fail closed). bare
// `agent-harness`, `bin/agent-harness`, `./bin/agent-harness issueops …`
// 호출과 provenance envelope의 executable과 exact 일치하는 absolute 호출만
// 파싱되고, 지원되는 두 단어 subcommand는 Path로 합쳐진다.
func ParseExactIssueOpsCommand(command string) (ExactIssueOpsCommand, bool) {
	command = strings.TrimSpace(command)
	if command == "" || HasUnquotedControlOperator(command) || HasActiveCommandSubstitution(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveZshEqualsExpansion(command) {
		return ExactIssueOpsCommand{}, false
	}
	return parseExactIssueOpsTokens(SplitCommandTokens(command))
}

// ParseExactIssueOpsArgs parses the argv slice received after the top-level
// `issueops` command. Unlike ParseExactIssueOpsCommand, argv values are already
// separated by the operating system and therefore do not need shell quoting.
func ParseExactIssueOpsArgs(args []string) (ExactIssueOpsCommand, bool) {
	tokens := make([]string, 0, len(args)+2)
	tokens = append(tokens, "agent-harness", "issueops")
	tokens = append(tokens, args...)
	return parseExactIssueOpsTokens(tokens)
}

func parseExactIssueOpsTokens(tokens []string) (ExactIssueOpsCommand, bool) {
	if len(tokens) < 3 || !exactIssueOpsExecutable(tokens) || tokens[1] != "issueops" {
		return ExactIssueOpsCommand{}, false
	}
	parts := []string{tokens[2]}
	start := 3
	if len(tokens) > 3 {
		switch tokens[2] {
		case "compatibility", "execution", "devils-advocate", "feedback", "remote", "cleanup", "ai-slop-clean", "artifact", "implementation-review", "branch", "decision", "child",
			"intent", "domain-review", "design", "plan-prep":
			if strings.HasPrefix(tokens[3], "--") {
				return ExactIssueOpsCommand{}, false
			}
			parts = append(parts, tokens[3])
			start = 4
		}
	}
	return ExactIssueOpsCommand{Path: strings.Join(parts, " "), Tokens: tokens, Start: start}, true
}

// ExactIssueOpsOwnerMutation returns the validated flags for a lifecycle-owner
// mutation. Read-only child status remains excluded unless --repair is present.
func ExactIssueOpsOwnerMutation(command ExactIssueOpsCommand) (map[string][]string, bool) {
	switch command.Path {
	case "link-plan", "link-worktree", "compatibility review", "devils-advocate review", "phase",
		"decision add", "ai-slop-clean record", "feedback mark-issue-updated", "feedback resolve",
		"implementation-review record", "branch prepare", "intent record", "domain-review record", "design review", "regress",
		"plan-prep record",
		"link-child", "link-related", "feedback add",
		"child start", "child status", "child accept", "child reject", "child drop",
		"remote create-child", "remote create-pr", "remote verify-artifact", "remote reflect-devils-advocate":
	default:
		return nil, false
	}
	values, booleans, repeatable, ok := IssueOpsCommandSpec(command.Path)
	if !ok {
		return nil, false
	}
	flags, ok := ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return nil, false
	}
	if command.Path == "child status" {
		if _, repair := flags["--repair"]; !repair {
			return nil, false
		}
	}
	idFlag := IssueOpsLifecycleIDFlag(command.Path)
	for _, name := range []string{idFlag, "--host", "--session-id", "--cwd"} {
		values := flags[name]
		if len(values) != 1 || strings.TrimSpace(values[0]) == "" {
			return nil, false
		}
	}
	return flags, true
}

// IssueOpsLifecycleIDFlag identifies the durable lifecycle selected by a
// parsed command. Delegation commands address their parent lifecycle.
func IssueOpsLifecycleIDFlag(path string) string {
	if strings.HasPrefix(path, "child ") {
		return "--parent"
	}
	return "--id"
}

func exactIssueOpsExecutable(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	switch tokens[0] {
	case "agent-harness", "bin/agent-harness", "./bin/agent-harness":
		return true
	}
	if !filepath.IsAbs(tokens[0]) {
		return false
	}
	_, provenance, present, err := commandparsecontract.ConsumeGeneratedCommandProvenance(tokens[1:])
	return err == nil && present && provenance.ExecutablePath == tokens[0]
}

// ExactFlags는 ExactIssueOpsCommand의 flag를 value/boolean/repeatable spec에
// 대조해 검증하고 수집하며, 알 수 없는 flag, non-repeatable flag의 중복, 값이
// 빠진 경우를 거부한다(fail closed).
func ExactFlags(command ExactIssueOpsCommand, values, booleans, repeatable map[string]bool) (map[string][]string, bool) {
	parsed := map[string][]string{}
	for i := command.Start; i < len(command.Tokens); i++ {
		token := command.Tokens[i]
		if !strings.HasPrefix(token, "--") {
			return nil, false
		}
		name, value, hasValue := token, "", false
		if at := strings.Index(token, "="); at >= 0 {
			name, value, hasValue = token[:at], token[at+1:], true
		}
		switch {
		case booleans[name]:
			if hasValue || len(parsed[name]) > 0 {
				return nil, false
			}
			parsed[name] = []string{"true"}
		case values[name] || generatedCommandProvenanceValueFlag(name):
			if !hasValue {
				if i+1 >= len(command.Tokens) || strings.HasPrefix(command.Tokens[i+1], "--") {
					return nil, false
				}
				i++
				value = command.Tokens[i]
			}
			if !repeatable[name] && len(parsed[name]) > 0 {
				return nil, false
			}
			parsed[name] = append(parsed[name], value)
		default:
			return nil, false
		}
	}
	return parsed, true
}

func generatedCommandProvenanceValueFlag(name string) bool {
	switch name {
	case commandparsecontract.GeneratedByExecutableFlag,
		commandparsecontract.GeneratedBySHA256Flag,
		commandparsecontract.GeneratedForGenerationFlag:
		return true
	default:
		return false
	}
}

// IssueOpsCommandSpec은 정확한 issueops subcommand path에 대한 (values,
// booleans, repeatable, ok) flag spec을 반환한다. 알 수 없는 path면 ok는 false다.
func IssueOpsCommandSpec(path string) (map[string]bool, map[string]bool, map[string]bool, bool) {
	v := func(names ...string) map[string]bool {
		out := map[string]bool{}
		for _, name := range names {
			out[name] = true
		}
		return out
	}
	b := func(names ...string) map[string]bool { return v(names...) }
	r := map[string]bool{}
	switch path {
	case "status":
		return v("--id"), b("--json"), r, true
	case "list":
		return v("--repo"), b("--json"), r, true
	case "pr-readiness":
		return v("--id"), b("--strict", "--json"), r, true
	case "execution status":
		return v("--id"), b("--json"), r, true
	case "execution prepare":
		return v("--id", "--mode", "--owner-host", "--owner-model", "--owner-effort", "--issue-snapshot-file", "--direct-reason", "--expected-readiness-fingerprint", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--confirm", "--json"), r, true
	case "execution claim":
		return v("--id", "--generation", "--claim-token-file", "--issue-body-sha256", "--context-packet-sha256", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--json"), r, true
	case "execution release":
		return v("--id", "--generation", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--json"), r, true
	case "execution replace":
		return v("--id", "--expected-generation", "--completion-generation", "--inventory-fingerprint", "--quiescence-fingerprint", "--reason", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--preview", "--revoke", "--finalize-preview", "--finalize", "--reseed", "--confirm", "--json"), r, true
	case "execution resume":
		return v("--id", "--expected-generation", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--confirm", "--json"), r, true
	case "execution whoami":
		return v(), b("--json"), r, true
	case "branch prepare":
		return v("--id", "--provider", "--issue-url", "--branch", "--base-branch", "--base-sha", "--parent-worktree", "--remote-branch-url", "--host", "--session-id", "--agent-id", "--cwd"), b("--link-verified", "--json"), r, true
	case "child start":
		values := v("--parent", "--branch", "--title", "--scope", "--acceptance", "--child-issue-url", "--host", "--session-id", "--agent-id", "--cwd")
		r["--acceptance"] = true
		return values, b("--json"), r, true
	case "child status":
		return v("--parent", "--host", "--session-id", "--agent-id", "--cwd"), b("--repair", "--json"), r, true
	case "child list":
		return v("--parent", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "child accept":
		values := v("--parent", "--child", "--evidence", "--host", "--session-id", "--agent-id", "--cwd")
		r["--evidence"] = true
		return values, b("--json"), r, true
	case "child reject", "child drop":
		return v("--parent", "--child", "--reason", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "intent record":
		values := v("--id", "--raw-request", "--interpreted-intent", "--success-criteria", "--constraint", "--ambiguity", "--non-goal", "--intent-class", "--host", "--session-id", "--agent-id", "--cwd")
		for _, name := range []string{"--success-criteria", "--constraint", "--ambiguity", "--non-goal"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "domain-review record":
		values := v("--id", "--model-fit", "--terminology", "--risk", "--uncertainty", "--host", "--session-id", "--agent-id", "--cwd")
		for _, name := range []string{"--terminology", "--risk", "--uncertainty"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "design review":
		values := v("--id", "--problem-summary", "--proposed-design", "--refactor-plan", "--alternative", "--risk", "--verification", "--open-question", "--host", "--session-id", "--agent-id", "--cwd")
		for _, name := range []string{"--alternative", "--risk", "--verification", "--open-question"} {
			r[name] = true
		}
		return values, b("--approved", "--json"), r, true
	case "plan-prep record":
		values := v("--id", "--decisions-evidence", "--decisions-waive", "--related-score-ref", "--related-waive", "--web-research-evidence", "--web-research-waive", "--codebase-survey-evidence", "--codebase-survey-waive", "--host", "--session-id", "--agent-id", "--cwd")
		for _, name := range []string{"--decisions-evidence", "--related-score-ref", "--web-research-evidence", "--codebase-survey-evidence"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "regress":
		return v("--id", "--reason", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "execution reconcile":
		return v("--id", "--operation-id", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--preview", "--confirm", "--json"), r, true
	case "execution complete":
		return v("--id", "--generation", "--final-head", "--turing-report", "--remote-artifact-url", "--verification", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--confirm", "--json"), map[string]bool{"--verification": true}, true
	case "execution sync-base":
		return v("--id", "--completion-generation", "--fingerprint", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--preview", "--apply", "--finalize", "--abort", "--confirm", "--json"), r, true
	case "execution switch-mode":
		return v("--id", "--mode", "--fingerprint", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--apply", "--confirm", "--json"), r, true
	case "link-plan":
		return v("--id", "--plan-path", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "link-worktree":
		return v("--id", "--worktree-path", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "link-child":
		return v("--id", "--child-url", "--title", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "link-related":
		return v("--id", "--type", "--related-url", "--title", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "feedback add":
		return v("--id", "--source", "--body", "--classification", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "compatibility review":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--backward-compatibility", "--side-effect", "--rollback-plan", "--verification", "--blocker")
		for _, name := range []string{"--backward-compatibility", "--side-effect", "--verification", "--blocker"} {
			r[name] = true
		}
		return values, b("--approved", "--json"), r, true
	case "devils-advocate review":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--verdict", "--finding", "--waiver-rationale")
		r["--finding"] = true
		return values, b("--waive", "--json"), r, true
	case "phase":
		return v("--id", "--to", "--host", "--session-id", "--agent-id", "--cwd"), b("--force", "--json"), r, true
	case "decision add":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--title", "--body", "--kind", "--rationale", "--alternative", "--affected-link", "--affected-artifact")
		for _, name := range []string{"--alternative", "--affected-link", "--affected-artifact"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "ai-slop-clean record":
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--category", "--verification")
		r["--category"] = true
		r["--verification"] = true
		return values, b("--json"), r, true
	case "feedback mark-issue-updated":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "feedback resolve":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--index", "--resolution"), b("--json"), r, true
	case "remote create-pr":
		values := v("--id", "--expected-generation", "--title", "--body", "--body-file", "--template", "--provider", "--score-file", "--head", "--base", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd", "--label", "--assignee", "--field")
		for _, name := range []string{"--label", "--assignee", "--field"} {
			r[name] = true
		}
		return values, b("--confirm", "--json"), r, true
	case "remote create-child":
		values := v("--id", "--title", "--body", "--body-file", "--template", "--provider", "--score-file", "--host", "--session-id", "--agent-id", "--cwd", "--label", "--assignee", "--field")
		for _, name := range []string{"--label", "--assignee", "--field"} {
			r[name] = true
		}
		return values, b("--confirm", "--json"), r, true
	case "remote verify-artifact":
		values := v("--id", "--provider", "--kind", "--url", "--target-branch", "--label", "--labels", "--assignee", "--assignees", "--host", "--session-id", "--agent-id", "--cwd")
		for _, name := range []string{"--label", "--labels", "--assignee", "--assignees"} {
			r[name] = true
		}
		return values, b("--json"), r, true
	case "remote score":
		return v("--input", "--judge", "--judge-file"), b("--json"), r, true
	case "implementation-review record":
		values := v("--id", "--verdict", "--finding", "--evidence", "--reviewer-host", "--reviewer-model", "--reviewer-effort", "--host", "--session-id", "--agent-id", "--cwd")
		r["--finding"] = true
		r["--evidence"] = true
		return values, b("--json"), r, true
	case "artifact unstage":
		return v("--id", "--name"), b("--json"), r, true
	case "artifact stage":
		return v("--id", "--name", "--file", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "cleanup status":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--merged", "--json"), r, true
	case "cleanup close-children":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--merged", "--confirm", "--json"), r, true
	case "cleanup orphan":
		return v("--id", "--repo", "--worktree", "--branch", "--provider", "--kind", "--artifact-url", "--fingerprint", "--host", "--session-id", "--agent-id", "--cwd"), b("--apply", "--confirm", "--json"), r, true
	case "cleanup finish":
		return v("--id", "--provider", "--fingerprint"), b("--preview", "--apply", "--confirm", "--json"), r, true
	// cleanup remote-branch는 source checkout 전용 표면이라 워크트리 cwd에서는
	// 가드가 미분류 셸로 차단한다. 이 등록은 typed 통과 권한이 아니라 usage와
	// spec의 관례 parity 목적이다(brooks B2 — spec-only 등록은 가드에 무효).
	case "cleanup remote-branch":
		return v("--id", "--fingerprint"), b("--preview", "--apply", "--confirm", "--json"), r, true
	case "cleanup abandon":
		return v("--id", "--reason", "--fingerprint"), b("--preview", "--apply", "--confirm", "--json"), r, true
	case "remote reflect-completion":
		return v("--id", "--provider"), b("--confirm", "--json"), r, true
	case "remote reflect-devils-advocate":
		return v("--id", "--provider", "--host", "--session-id", "--agent-id", "--cwd"), b("--confirm", "--json"), r, true
	case "remote close-issue":
		return v("--id", "--provider"), b("--confirm", "--json"), r, true
	default:
		return nil, nil, nil, false
	}
}

// ExactReadOnlyShellCommand은 non-issueops shell 명령이 정확한 read-only
// 관찰(pwd, safe rg, read-only git, read-only orca terminal/orchestration)인지
// 보고한다. exact read-only 조각과 고정 CodeGraph probe로만 구성된 제한된
// 세미콜론·개행·&& 시퀀스 외의 활성 shell control/expansion은 거부한다.
// IssueOps의 read-only 권한은 record identity가 필요하므로 호출자가 처리한다.
func ExactReadOnlyShellCommand(command string) bool {
	if HasActiveCommandSubstitution(command) || HasActiveInputRedirect(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveShellComment(command) || HasActiveZshEqualsExpansion(command) {
		return false
	}
	if exactReadOnlyCommandLookup(command) {
		return true
	}
	if HasUnquotedControlOperator(command) {
		return exactReadOnlyShellSequence(command)
	}
	return exactReadOnlySimpleShellCommand(command)
}

func exactReadOnlyCommandLookup(command string) bool {
	const prefix = "command -v "
	command = strings.TrimSpace(command)
	name, matched := strings.CutPrefix(command, prefix)
	if !matched || name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		character := name[index]
		asciiAlphaNumeric := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9'
		if asciiAlphaNumeric || character == '_' ||
			(index > 0 && (character == '.' || character == '+' || character == '-')) {
			continue
		}
		return false
	}
	return true
}

func exactReadOnlySimpleShellCommand(command string) bool {
	tokens := SplitCommandTokens(strings.TrimSpace(command))
	if len(tokens) == 0 {
		return false
	}
	switch readerCommandName(tokens[0]) {
	case "pwd":
		return len(tokens) == 1
	case "cat":
		return exactReadOnlyCat(tokens[1:])
	case "bash", "sh", "zsh":
		return exactReadOnlyShellSyntaxCheck(tokens[1:])
	case "ps":
		return exactReadOnlyProcessStatus(tokens[1:])
	case "pgrep":
		return exactReadOnlyProcessGrep(tokens[1:])
	case "head":
		return exactReadOnlyHeadOrTail(tokens[1:])
	case "tail":
		return exactReadOnlyHeadOrTail(tokens[1:])
	case "ls":
		return exactReadOnlyLS(tokens[1:])
	case "find":
		return exactReadOnlyFind(tokens[1:])
	case "stat":
		return exactReadOnlyStat(tokens[1:])
	case "file":
		return exactReadOnlyFile(tokens[1:])
	case "shasum":
		return exactReadOnlyDigestCommand(tokens[1:], true)
	case "sha256sum":
		return exactReadOnlyDigestCommand(tokens[1:], false)
	case "wc":
		return exactReadOnlyWCCommand(tokens[1:])
	case "sed":
		return exactReadOnlySedCommand(tokens[1:])
	case "jq":
		return exactReadOnlyJQCommand(tokens[1:])
	case "gofmt":
		return exactReadOnlyGofmtDiff(tokens[1:])
	case "go":
		return exactReadOnlyGoDoc(tokens[1:])
	case "codegraph":
		return len(tokens) == 3 && tokens[1] == "explore" && strings.TrimSpace(tokens[2]) != "" && !strings.HasPrefix(tokens[2], "-")
	case "rg":
		return SafeRipgrepArgs(tokens[1:])
	case "agent-harness", "bin/agent-harness", "./bin/agent-harness":
		return exactReadOnlyStateRead(tokens[1:])
	case "git":
		i := CommandAfterDirectoryOption(tokens, 1)
		if i < 0 || i >= len(tokens) {
			return false
		}
		switch tokens[i] {
		case "status", "diff", "log", "show", "rev-parse":
			for _, token := range tokens[i+1:] {
				if token == "-o" || strings.HasPrefix(token, "--output") || token == "--ext-diff" || token == "--textconv" || strings.HasPrefix(token, "--exec") {
					return false
				}
			}
			return true
		case "branch":
			return len(tokens) == i+2 && tokens[i+1] == "--show-current"
		case "ls-files":
			// Shannon 측정이 Git이 추적하지 않는 파일 목록만 읽는 고정 형태다.
			// 추가 exclude 파일이나 format 표면은 이 reader 계약에 열지 않는다.
			return len(tokens) == i+3 &&
				tokens[i+1] == "--others" &&
				tokens[i+2] == "--exclude-standard"
		case "ls-remote":
			return exactReadOnlyGitLSRemote(tokens[i+1:])
		case "merge-base":
			return exactReadOnlyGitMergeBase(tokens[i+1:])
		}
	case "gh":
		return exactReadOnlyGHCommand(tokens)
	case "orca":
		return ExactReadOnlyOrcaTerminalCommand(tokens) ||
			(len(tokens) == 4 && tokens[1] == "orchestration" && tokens[2] == "task-list" && tokens[3] == "--json")
	}
	return false
}

func exactReadOnlyStateRead(tokens []string) bool {
	if len(tokens) < 3 || tokens[0] != "state" || tokens[1] != "read" {
		return false
	}
	keySeen, jsonSeen := false, false
	for i := 2; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "--key":
			if keySeen || i+1 >= len(tokens) || strings.TrimSpace(tokens[i+1]) == "" || strings.HasPrefix(tokens[i+1], "--") {
				return false
			}
			keySeen = true
			i++
		case strings.HasPrefix(token, "--key="):
			if keySeen || strings.TrimSpace(strings.TrimPrefix(token, "--key=")) == "" {
				return false
			}
			keySeen = true
		case token == "--json":
			if jsonSeen {
				return false
			}
			jsonSeen = true
		default:
			return false
		}
	}
	return keySeen
}

func exactReadOnlyGofmtDiff(tokens []string) bool {
	// -d는 파일을 덮어쓰지 않고 stdout에 formatting diff만 출력한다.
	// 쓰기·profile·rewrite 표면을 열지 않도록 명시적 .go 파일만 받는다.
	if len(tokens) < 2 || tokens[0] != "-d" {
		return false
	}
	for _, operand := range tokens[1:] {
		if operand == "" || operand == "-" || strings.HasPrefix(operand, "-") || path.Ext(operand) != ".go" {
			return false
		}
	}
	return true
}

func exactReadOnlyGoDoc(tokens []string) bool {
	if len(tokens) == 0 || tokens[0] != "doc" {
		return false
	}
	operands := 0
	for _, token := range tokens[1:] {
		if strings.HasPrefix(token, "-") {
			switch token {
			case "-all", "-c", "-cmd", "-short", "-src", "-u":
				if operands != 0 {
					return false
				}
				continue
			default:
				// -http는 서버를 열고 -C는 관찰 범위를 바꾸므로 승인하지 않는다.
				return false
			}
		}
		operands++
		if operands > 2 {
			return false
		}
	}
	return true
}

func exactReadOnlyJQCommand(tokens []string) bool {
	// Turing 증거 JSON의 구문 검증과 전체 관찰에 필요한 고정 filter 및 .json
	// 파일 하나만 허용한다. -e는 전체 문서가 truthy인지 exit status로 확인하는
	// 실제 dogfood 경로다. 그 밖의 option/filter/stdin은 넓은 표면을 열지 않는다.
	if len(tokens) == 3 && tokens[0] == "-e" && tokens[1] == "." {
		tokens = tokens[1:]
	}
	return len(tokens) == 2 &&
		(tokens[0] == "empty" || tokens[0] == ".") &&
		exactReadOnlyJSONOperand(tokens[1])
}

func exactReadOnlyJSONOperand(token string) bool {
	return token != "" &&
		token != "-" &&
		!strings.HasPrefix(token, "-") &&
		strings.HasSuffix(token, ".json")
}

func exactReadOnlyDigestCommand(tokens []string, algorithmRequired bool) bool {
	if algorithmRequired {
		if len(tokens) < 3 || (tokens[0] != "-a" && tokens[0] != "--algorithm") || tokens[1] != "256" {
			return false
		}
		tokens = tokens[2:]
	}
	if len(tokens) == 0 {
		return false
	}
	for _, operand := range tokens {
		if operand == "" || operand == "-" || strings.HasPrefix(operand, "-") {
			return false
		}
	}
	return true
}

func exactReadOnlyCat(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	operands := 0
	options := true
	longOptions := map[string]bool{
		"--show-all": true, "--number-nonblank": true, "--show-ends": true,
		"--number": true, "--squeeze-blank": true, "--show-tabs": true,
		"--show-nonprinting": true,
	}
	for _, token := range tokens {
		if options && token == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(token, "--") {
			if !longOptions[token] {
				return false
			}
			continue
		}
		if options && strings.HasPrefix(token, "-") && token != "-" {
			for _, flag := range strings.TrimPrefix(token, "-") {
				if !strings.ContainsRune("AbeEnstTuv", flag) {
					return false
				}
			}
			continue
		}
		if token == "-" || strings.TrimSpace(token) == "" {
			return false
		}
		operands++
	}
	return operands > 0
}

func exactReadOnlyHeadOrTail(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	operands := 0
	options := true
	limitSeen := false
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if options && token == "--" {
			options = false
			continue
		}
		if options {
			switch token {
			case "-q", "--quiet", "--silent", "-v", "--verbose":
				continue
			case "-n", "--lines":
				if limitSeen || i+1 >= len(tokens) || !boundedLineCount(tokens[i+1]) {
					return false
				}
				limitSeen = true
				i++
				continue
			}
			if strings.HasPrefix(token, "--lines=") {
				if limitSeen || !boundedLineCount(strings.TrimPrefix(token, "--lines=")) {
					return false
				}
				limitSeen = true
				continue
			}
			if strings.HasPrefix(token, "-n") && len(token) > 2 {
				if limitSeen || !boundedLineCount(strings.TrimPrefix(token, "-n")) {
					return false
				}
				limitSeen = true
				continue
			}
			// head/tail이 지원하는 `-80` 숫자 축약형도 `-n 80`과 같은
			// 상한으로 제한해 active IssueOps worktree의 안전한 판독을 허용한다.
			if strings.HasPrefix(token, "-") && len(token) > 1 && boundedLineCount(strings.TrimPrefix(token, "-")) {
				if limitSeen {
					return false
				}
				limitSeen = true
				continue
			}
			if strings.HasPrefix(token, "-") {
				return false
			}
		}
		if token == "-" || strings.TrimSpace(token) == "" {
			return false
		}
		operands++
	}
	return operands > 0
}

func boundedLineCount(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	count, err := strconv.ParseUint(value, 10, 14)
	return err == nil && count <= 10_000
}

func exactReadOnlyLS(tokens []string) bool {
	options := true
	for _, token := range tokens {
		if options && token == "--" {
			options = false
			continue
		}
		if !options || !strings.HasPrefix(token, "-") || token == "-" {
			continue
		}
		if token == "--recursive" || !strings.HasPrefix(token, "--") && strings.ContainsRune(strings.TrimPrefix(token, "-"), 'R') {
			return false
		}
	}
	return true
}

func exactReadOnlyFind(tokens []string) bool {
	if len(tokens) < 3 {
		return false
	}
	i := 0
	for i < len(tokens) && !strings.HasPrefix(tokens[i], "-") && tokens[i] != "!" && tokens[i] != "(" {
		if strings.TrimSpace(tokens[i]) == "" {
			return false
		}
		i++
	}
	if i == 0 {
		return false
	}
	maxDepth := false
	valuePredicates := map[string]bool{
		"-type": true, "-name": true, "-iname": true, "-path": true,
		"-ipath": true, "-regex": true, "-iregex": true, "-size": true,
		"-mtime": true, "-mmin": true, "-newer": true, "-user": true,
		"-group": true, "-perm": true, "-links": true, "-inum": true,
	}
	noValuePredicates := map[string]bool{
		"-print": true, "-print0": true, "-ls": true, "-empty": true,
		"-readable": true, "-writable": true, "-executable": true,
		"-true": true, "-false": true, "!": true, "-not": true,
		"-a": true, "-and": true, "-o": true, "-or": true,
	}
	for i < len(tokens) {
		token := tokens[i]
		switch token {
		case "-delete", "-exec", "-execdir", "-ok", "-okdir", "-fls", "-fprint", "-fprint0", "-fprintf":
			return false
		case "-maxdepth":
			if maxDepth || i+1 >= len(tokens) || !boundedFindDepth(tokens[i+1]) {
				return false
			}
			maxDepth = true
			i += 2
			continue
		case "-mindepth":
			if i+1 >= len(tokens) || !boundedFindDepth(tokens[i+1]) {
				return false
			}
			i += 2
			continue
		}
		if valuePredicates[token] {
			if i+1 >= len(tokens) {
				return false
			}
			i += 2
			continue
		}
		if noValuePredicates[token] {
			i++
			continue
		}
		return false
	}
	return maxDepth
}

func boundedFindDepth(value string) bool {
	depth, err := strconv.ParseUint(value, 10, 8)
	return err == nil && depth <= 20
}

func exactReadOnlyStat(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	operands := 0
	options := true
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if options && token == "--" {
			options = false
			continue
		}
		if options {
			switch token {
			case "-L", "--dereference", "-t", "--terse", "-x", "-s", "-r", "-l":
				continue
			case "-c", "--format", "--printf", "-f":
				if i+1 >= len(tokens) {
					return false
				}
				i++
				continue
			}
			if strings.HasPrefix(token, "--format=") || strings.HasPrefix(token, "--printf=") {
				continue
			}
			if strings.HasPrefix(token, "-") {
				return false
			}
		}
		operands++
	}
	return operands > 0
}

func exactReadOnlyFile(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	operands := 0
	options := true
	allowed := map[string]bool{
		"-b": true, "--brief": true, "-i": true, "--mime": true,
		"--mime-type": true, "--mime-encoding": true, "-L": true,
		"--dereference": true, "-h": true, "--no-dereference": true,
		"-k": true, "--keep-going": true, "-z": true, "--uncompress": true,
		"-0": true, "--print0": true,
	}
	for _, token := range tokens {
		if options && token == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(token, "-") {
			if token == "-C" || token == "--compile" || !allowed[token] {
				return false
			}
			continue
		}
		if token == "-" || strings.TrimSpace(token) == "" {
			return false
		}
		operands++
	}
	return operands > 0
}

func exactReadOnlyGitLSRemote(tokens []string) bool {
	if len(tokens) < 2 {
		return false
	}
	allowedOptions := map[string]bool{
		"--heads": true, "--tags": true, "--refs": true, "--quiet": true,
		"-q": true, "--exit-code": true, "--symref": true,
	}
	i := 0
	for i < len(tokens) && strings.HasPrefix(tokens[i], "-") {
		if !allowedOptions[tokens[i]] {
			return false
		}
		i++
	}
	if i >= len(tokens) || tokens[i] != "origin" {
		return false
	}
	i++
	if i >= len(tokens) {
		return false
	}
	for _, ref := range tokens[i:] {
		if (!strings.HasPrefix(ref, "refs/heads/") && !strings.HasPrefix(ref, "refs/tags/")) || strings.ContainsAny(ref, "*?[]") {
			return false
		}
	}
	return true
}

func exactReadOnlyGitMergeBase(tokens []string) bool {
	if len(tokens) == 3 && tokens[0] == "--is-ancestor" {
		tokens = tokens[1:]
	}
	if len(tokens) != 2 {
		return false
	}
	for _, token := range tokens {
		if len(token) != 40 && len(token) != 64 {
			return false
		}
		for _, r := range token {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return false
			}
		}
	}
	return true
}

func exactReadOnlyGHCommand(tokens []string) bool {
	if len(tokens) < 3 || tokens[0] != "gh" {
		return false
	}
	switch tokens[1] {
	case "issue":
		if len(tokens) < 4 {
			return false
		}
		return exactReadOnlyGHIssueCommand(tokens)
	case "pr":
		if len(tokens) < 4 {
			return false
		}
		return exactReadOnlyGHPRCommand(tokens)
	case "run":
		if len(tokens) < 4 {
			return false
		}
		return exactReadOnlyGHRunCommand(tokens)
	case "api":
		return exactReadOnlyGHAPICommand(tokens)
	default:
		return false
	}
}

func exactReadOnlyGHAPICommand(tokens []string) bool {
	if len(tokens) == 3 {
		// Governed PR 생성 뒤 canonical artifact 전체를 독립적으로 재검증하는 GET이다.
		parts := strings.Split(tokens[2], "/")
		if len(parts) != 5 || parts[0] != "repos" || parts[3] != "pulls" ||
			!safeGHRepository(parts[1]+"/"+parts[2]) {
			return false
		}
		number, err := strconv.Atoi(parts[4])
		return err == nil && number > 0
	}
	return exactReadOnlyGHAPIIssueMetadataCommand(tokens)
}

func exactReadOnlyGHAPIIssueMetadataCommand(tokens []string) bool {
	// PR 게시 시 원격 이슈의 label과 assignee를 그대로 이어받기 위한 두 projection만
	// 허용한다. gh api의 다른 flag나 endpoint를 열지 않아 GET reader 경계를 유지한다.
	if len(tokens) != 5 || tokens[3] != "--jq" ||
		(tokens[4] != ".labels[].name" && tokens[4] != ".assignees[].login") {
		return false
	}
	parts := strings.Split(tokens[2], "/")
	if len(parts) != 5 || parts[0] != "repos" || parts[3] != "issues" ||
		!safeGHRepository(parts[1]+"/"+parts[2]) {
		return false
	}
	number, err := strconv.Atoi(parts[4])
	return err == nil && number > 0
}

func exactReadOnlyGHIssueCommand(tokens []string) bool {
	switch tokens[2] {
	case "develop":
		return exactReadOnlyGHIssueDevelopCommand(tokens)
	case "view":
	default:
		return false
	}
	number, err := strconv.Atoi(tokens[3])
	if err != nil || number <= 0 {
		return false
	}
	flags, ok := ExactFlags(
		ExactIssueOpsCommand{Tokens: tokens, Start: 4},
		map[string]bool{"--json": true, "--repo": true},
		map[string]bool{"--comments": true},
		map[string]bool{},
	)
	if !ok {
		return false
	}
	if fields, exists := flags["--json"]; exists && !safeGHJSONFields(fields[0]) {
		return false
	}
	if repo, exists := flags["--repo"]; exists && !safeGHRepository(repo[0]) {
		return false
	}
	return true
}

func exactReadOnlyGHIssueDevelopCommand(tokens []string) bool {
	listSeen := false
	numberSeen := false
	repoSeen := false
	for i := 3; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "--list":
			if listSeen {
				return false
			}
			listSeen = true
		case token == "--repo":
			if repoSeen || i+1 >= len(tokens) || !safeGHRepository(tokens[i+1]) {
				return false
			}
			repoSeen = true
			i++
		case strings.HasPrefix(token, "--repo="):
			if repoSeen || !safeGHRepository(strings.TrimPrefix(token, "--repo=")) {
				return false
			}
			repoSeen = true
		case strings.HasPrefix(token, "-"):
			return false
		default:
			number, err := strconv.Atoi(token)
			if numberSeen || err != nil || number <= 0 {
				return false
			}
			numberSeen = true
		}
	}
	return listSeen && numberSeen
}

func exactReadOnlyGHPRCommand(tokens []string) bool {
	if !safeGHPRSelector(tokens[3]) {
		return false
	}
	values := map[string]bool{"--json": true, "--repo": true}
	booleans := map[string]bool{}
	switch tokens[2] {
	case "view":
		booleans["--comments"] = true
	case "checks":
		booleans["--required"] = true
	default:
		return false
	}
	flags, ok := ExactFlags(ExactIssueOpsCommand{Tokens: tokens, Start: 4}, values, booleans, map[string]bool{})
	if !ok {
		return false
	}
	if fields, exists := flags["--json"]; exists && !safeGHJSONFields(fields[0]) {
		return false
	}
	if repo, exists := flags["--repo"]; exists && !safeGHRepository(repo[0]) {
		return false
	}
	return true
}

func safeGHPRSelector(value string) bool {
	if number, err := strconv.Atoi(value); err == nil {
		return number > 0
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawPath != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" || parsed.Path != "/"+strings.Join(parts, "/") ||
		!safeGHRepository(parts[0]+"/"+parts[1]) {
		return false
	}
	number, err := strconv.Atoi(parts[3])
	return err == nil && number > 0
}

func exactReadOnlyGHRunCommand(tokens []string) bool {
	if tokens[2] != "view" {
		return false
	}
	runID, err := strconv.ParseUint(tokens[3], 10, 64)
	if err != nil || runID == 0 {
		return false
	}
	flags, ok := ExactFlags(
		ExactIssueOpsCommand{Tokens: tokens, Start: 4},
		map[string]bool{"--json": true, "--job": true, "--attempt": true, "--repo": true},
		map[string]bool{"--log": true, "--log-failed": true, "--verbose": true},
		map[string]bool{},
	)
	if !ok {
		return false
	}
	if fields, exists := flags["--json"]; exists && !safeGHJSONFields(fields[0]) {
		return false
	}
	if repo, exists := flags["--repo"]; exists && !safeGHRepository(repo[0]) {
		return false
	}
	for _, name := range []string{"--job", "--attempt"} {
		if values, exists := flags[name]; exists {
			value, parseErr := strconv.ParseUint(values[0], 10, 64)
			if parseErr != nil || value == 0 {
				return false
			}
		}
	}
	return true
}

func safeGHRepository(value string) bool {
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 100 {
			return false
		}
		for _, r := range part {
			if r != '-' && r != '_' && r != '.' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
		}
	}
	return true
}

func safeGHJSONFields(value string) bool {
	parts := strings.Split(value, ",")
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r != '_' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
		}
	}
	return true
}

func exactReadOnlyWCCommand(tokens []string) bool {
	if len(tokens) == 0 {
		return false
	}
	longOptions := map[string]bool{
		"--bytes": true, "--chars": true, "--lines": true,
		"--max-line-length": true, "--words": true,
	}
	operands, optionsDone := 0, false
	for _, token := range tokens {
		if token == "" {
			return false
		}
		if !optionsDone && token == "--" {
			optionsDone = true
			continue
		}
		if !optionsDone && strings.HasPrefix(token, "--") {
			if !longOptions[token] {
				return false
			}
			continue
		}
		if !optionsDone && strings.HasPrefix(token, "-") {
			if token == "-" || len(token) == 1 {
				return false
			}
			for _, flag := range token[1:] {
				if !strings.ContainsRune("cmlwL", flag) {
					return false
				}
			}
			continue
		}
		if token == "-" {
			return false
		}
		operands++
	}
	return operands > 0
}

func exactReadOnlySedCommand(tokens []string) bool {
	if len(tokens) < 3 || tokens[0] != "-n" || !boundedSedPrintRange(tokens[1]) {
		return false
	}
	for _, operand := range tokens[2:] {
		if operand == "" || operand == "-" || strings.HasPrefix(operand, "-") || knownStdinAlias(operand) {
			return false
		}
	}
	return true
}

func knownStdinAlias(operand string) bool {
	clean := path.Clean(operand)
	if strings.HasSuffix(clean, "/dev/stdin") {
		return true
	}
	const marker = "/fd/"
	index := strings.LastIndex(clean, marker)
	if index < 0 {
		return false
	}
	fd, err := strconv.Atoi(clean[index+len(marker):])
	return err == nil && fd == 0
}

func boundedSedPrintRange(script string) bool {
	if !strings.HasSuffix(script, "p") {
		return false
	}
	parts := strings.Split(strings.TrimSuffix(script, "p"), ",")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	// 문서 끝까지 읽는 `<positive-line>,$p`만 마지막 줄 표식 범위로 인정한다.
	// `$p` 단독이나 다른 sed 명령은 기존처럼 fail closed로 둔다.
	if len(parts) == 2 && parts[1] == "$" {
		start, err := strconv.Atoi(parts[0])
		return err == nil && start > 0
	}
	lines := make([]int, len(parts))
	for i, part := range parts {
		line, err := strconv.Atoi(part)
		if err != nil || line <= 0 {
			return false
		}
		lines[i] = line
	}
	return len(lines) == 1 || lines[0] <= lines[1]
}

// ExactReadOnlyOrcaTerminalCommand은 token들이 bounded flag를 가진 정확한
// read-only `orca terminal list|show|read|wait` 호출인지 보고한다.
func ExactReadOnlyOrcaTerminalCommand(tokens []string) bool {
	if len(tokens) < 4 || tokens[0] != "orca" || tokens[1] != "terminal" {
		return false
	}
	values := map[string]bool{}
	booleans := map[string]bool{"--json": true}
	switch tokens[2] {
	case "list":
		values = map[string]bool{"--worktree": true, "--limit": true}
	case "show":
		values = map[string]bool{"--terminal": true}
	case "read":
		values = map[string]bool{"--terminal": true, "--cursor": true, "--limit": true}
	case "wait":
		values = map[string]bool{"--terminal": true, "--for": true, "--timeout-ms": true}
	default:
		return false
	}
	flags, ok := ExactFlags(ExactIssueOpsCommand{Tokens: tokens, Start: 3}, values, booleans, map[string]bool{})
	if !ok || len(flags["--json"]) != 1 {
		return false
	}
	for name, entries := range flags {
		if name == "--json" {
			continue
		}
		if len(entries) != 1 || strings.TrimSpace(entries[0]) == "" {
			return false
		}
	}
	if value, exists := flags["--for"]; exists && value[0] != "exit" && value[0] != "tui-idle" {
		return false
	}
	for _, name := range []string{"--cursor", "--limit", "--timeout-ms"} {
		if value, exists := flags[name]; exists {
			n, err := strconv.Atoi(value[0])
			if err != nil || n < 0 || (name == "--limit" && n == 0) {
				return false
			}
		}
	}
	return tokens[2] != "wait" || len(flags["--for"]) == 1
}

// SafeRipgrepArgs는 모든 rg 인자가 read-only value/boolean allowlist에 있는지
// 보고한다(알 수 없는 flag는 fail closed).
func SafeRipgrepArgs(tokens []string) bool {
	valueOptions := map[string]bool{
		"-g": true, "--glob": true, "-t": true, "--type": true, "-T": true, "--type-not": true,
		"-m": true, "--max-count": true, "-A": true, "--after-context": true, "-B": true, "--before-context": true,
		"-C": true, "--context": true, "--color": true, "--sort": true, "--sortr": true,
	}
	boolOptions := map[string]bool{
		"-n": true, "--line-number": true, "--files": true, "--hidden": true, "--no-ignore": true,
		"-F": true, "--fixed-strings": true, "--json": true, "-l": true, "--files-with-matches": true,
		"-c": true, "--stats": true, "--pcre2": true, "-U": true, "--multiline": true, "--no-heading": true,
		"--column": true, "--count": true, "--count-matches": true, "--no-messages": true,
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if !strings.HasPrefix(token, "-") || token == "-" {
			continue
		}
		name := token
		if at := strings.Index(token, "="); at >= 0 {
			name = token[:at]
			if !valueOptions[name] {
				return false
			}
			continue
		}
		if boolOptions[name] {
			continue
		}
		if exactRipgrepCompactNumericOption(token) {
			continue
		}
		if valueOptions[name] {
			if i+1 >= len(tokens) || strings.HasPrefix(tokens[i+1], "-") {
				return false
			}
			i++
			continue
		}
		return false
	}
	return true
}

// exactRipgrepCompactNumericOption은 ripgrep가 허용하는 `-A5` 같은 결합형
// 표기를 기존 숫자형 관찰 옵션과 제한된 값에만 허용한다.
func exactRipgrepCompactNumericOption(token string) bool {
	if len(token) < 3 {
		return false
	}
	switch token[:2] {
	case "-A", "-B", "-C", "-m":
		return boundedLineCount(token[2:])
	default:
		return false
	}
}

// CommandAfterDirectoryOption은 start부터 시작해 처음 나오는 non-`-C`(directory
// 옵션) token의 인덱스를 반환하며, -C 옵션이 잘못된 경우(값이 없거나 비어 있음)
// -1을 반환한다.
func CommandAfterDirectoryOption(tokens []string, start int) int {
	for start < len(tokens) {
		token := tokens[start]
		if token == "-C" {
			if start+1 >= len(tokens) || strings.HasPrefix(tokens[start+1], "-") {
				return -1
			}
			start += 2
			continue
		}
		if strings.HasPrefix(token, "-C=") {
			if strings.TrimPrefix(token, "-C=") == "" {
				return -1
			}
			start++
			continue
		}
		return start
	}
	return -1
}

// ContainsASCIITerminalControl은 value에 ASCII C0 control이나 DEL 문자(PTY를
// 조종하거나 comment marker를 지울 수 있음)가 들어 있는지 보고한다.
func ContainsASCIITerminalControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
