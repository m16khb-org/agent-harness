package commandparse

import (
	"path/filepath"
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
		return v("--id", "--generation", "--claim-token-file", "--issue-body-sha256", "--context-packet-sha256", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"), b("--claim-current-token", "--json"), r, true
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
		values := v("--id", "--host", "--session-id", "--agent-id", "--cwd", "--verdict", "--reviewer-context", "--finding", "--waiver-rationale")
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
	// branch await-link는 읽기 전용 대기다. pre-link 창에서 owner가 쓸 수
	// 있어야 하므로 가드가 이 형태 하나를 명시적으로 허용한다(#319).
	case "branch await-link":
		return v("--id", "--timeout", "--host", "--session-id", "--agent-id", "--cwd"), b("--json"), r, true
	case "cleanup status":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--merged", "--json"), r, true
	case "cleanup close-children":
		return v("--id", "--host", "--session-id", "--agent-id", "--cwd"), b("--merged", "--confirm", "--json"), r, true
	case "cleanup orphan":
		return v("--id", "--repo", "--worktree", "--branch", "--provider", "--kind", "--artifact-url", "--fingerprint", "--host", "--session-id", "--agent-id", "--cwd"), b("--apply", "--confirm", "--json"), r, true
	case "cleanup finish":
		return v("--id", "--provider", "--fingerprint", "--superseded-by"), b("--preview", "--apply", "--confirm", "--json"), r, true
	// cleanup remote-branch는 source checkout 전용 표면이라 워크트리 cwd에서는
	// 가드가 미분류 셸로 차단한다. 이 등록은 typed 통과 권한이 아니라 usage와
	// spec의 관례 parity 목적이다(brooks B2 — spec-only 등록은 가드에 무효).
	case "cleanup remote-branch":
		return v("--id", "--fingerprint", "--superseded-by"), b("--preview", "--apply", "--confirm", "--json"), r, true
	// cleanup linked-branch도 source checkout 전용이다. remote-branch와 같은
	// 이유로 여기 등록은 usage/spec parity 목적이며 가드 통과 권한이 아니다.
	case "cleanup linked-branch":
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
