package commandparse

import (
	clidomain "issueops/internal/domain/cli"
	"path/filepath"
	"strings"

	commandparsecontract "issueops/internal/contract/commandparse"
	"issueops/internal/domain/shelltoken"
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

// ExactIssueOpsCommand은 파싱된 정확한 `issueops …` 명령이다.
// subcommand path, 전체 token slice, 그리고 flag가 시작되는 인덱스를 담는다.
type ExactIssueOpsCommand struct {
	Path   string
	Tokens []string
	Start  int
}

// ParseExactIssueOpsCommand은 명령을 ExactIssueOpsCommand으로 파싱하며, 활성
// shell control/expansion을 담은 명령은 모두 거부한다(fail closed). bare
// `issueops`, `bin/issueops`, `./bin/issueops …`
// 호출과 provenance envelope의 executable과 exact 일치하는 absolute 호출만
// 파싱되고, 지원되는 두 단어 subcommand는 Path로 합쳐진다.
func ParseExactIssueOpsCommand(command string) (ExactIssueOpsCommand, bool) {
	command = strings.TrimSpace(command)
	if command == "" || HasUnquotedControlOperator(command) || HasActiveCommandSubstitution(command) || HasActiveOutputRedirect(command) || HasActiveParameterOrTildeExpansion(command) || HasActivePathnameExpansion(command) || HasActiveShellSpecialQuoting(command) || HasActiveZshEqualsExpansion(command) {
		return ExactIssueOpsCommand{}, false
	}
	tokens := SplitCommandTokens(command)
	if len(tokens) < 2 || !clidomain.IsLifecycleCommand(tokens[1]) {
		return ExactIssueOpsCommand{}, false
	}
	return parseExactIssueOpsTokens(tokens)
}

// ParseExactIssueOpsArgs parses the argv slice received after the top-level
// `issueops` command. Unlike ParseExactIssueOpsCommand, argv values are already
// separated by the operating system and therefore do not need shell quoting.
func ParseExactIssueOpsArgs(args []string) (ExactIssueOpsCommand, bool) {
	tokens := make([]string, 0, len(args)+1)
	tokens = append(tokens, "issueops")
	tokens = append(tokens, args...)
	return parseExactIssueOpsTokens(tokens)
}

func parseExactIssueOpsTokens(tokens []string) (ExactIssueOpsCommand, bool) {
	if len(tokens) < 2 || !exactIssueOpsExecutable(tokens) {
		return ExactIssueOpsCommand{}, false
	}
	parts := []string{tokens[1]}
	start := 2
	if len(tokens) > 2 {
		switch tokens[1] {
		case "compatibility", "execution", "devils-advocate", "feedback", "remote", "cleanup", "ai-slop-clean", "artifact", "implementation-review", "branch", "decision", "child", "intent", "domain-review", "design", "plan-prep", "project-docs-review", "schema-evidence":
			if strings.HasPrefix(tokens[2], "--") {
				return ExactIssueOpsCommand{}, false
			}
			parts = append(parts, tokens[2])
			start = 3
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
		"implementation-review record", "project-docs-review record", "schema-evidence record",
		"branch prepare", "branch retarget", "intent record", "domain-review record", "design review", "regress",
		"plan-prep record",
		"link-child", "link-related", "feedback add",
		"child start", "child status", "child accept", "child reject", "child drop",
		"remote create-child", "remote create-pr", "remote verify-artifact", "remote reflect-devils-advocate",
		"remote sync-issue", "remote sync-pr":
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
	case "issueops", "bin/issueops", "./bin/issueops", "io", "bin/io", "./bin/io":
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

type issueOpsSpec struct {
	values     []string
	booleans   []string
	repeatable []string
}

func toBoolMap(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}

var issueOpsCommandSpecs = map[string]issueOpsSpec{
	"status": {
		values:   []string{"--id"},
		booleans: []string{"--json"},
	},
	"list": {
		values:   []string{"--repo"},
		booleans: []string{"--json"},
	},
	"pr-readiness": {
		values:   []string{"--id"},
		booleans: []string{"--strict", "--json"},
	},
	"execution status": {
		values:   []string{"--id"},
		booleans: []string{"--json"},
	},
	"execution prepare": {
		values:   []string{"--id", "--mode", "--owner-host", "--owner-model", "--owner-effort", "--issue-snapshot-file", "--direct-reason", "--expected-readiness-fingerprint", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--confirm", "--json"},
	},
	"execution claim": {
		values:   []string{"--id", "--generation", "--claim-token-file", "--issue-body-sha256", "--context-packet-sha256", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--claim-current-token", "--json"},
	},
	"execution release": {
		values:   []string{"--id", "--generation", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--json"},
	},
	"execution replace": {
		values:   []string{"--id", "--expected-generation", "--completion-generation", "--inventory-fingerprint", "--quiescence-fingerprint", "--reason", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--preview", "--revoke", "--finalize-preview", "--finalize", "--reseed", "--confirm", "--json"},
	},
	"execution resume": {
		values:   []string{"--id", "--expected-generation", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--confirm", "--json"},
	},
	"execution whoami": {
		booleans: []string{"--json"},
	},
	"branch prepare": {
		values:   []string{"--id", "--provider", "--issue-url", "--branch", "--base-branch", "--base-sha", "--parent-worktree", "--remote-branch-url", "--code-project-key", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--link-verified", "--json"},
	},
	"child start": {
		values:     []string{"--parent", "--branch", "--title", "--scope", "--acceptance", "--child-issue-url", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--acceptance"},
		booleans:   []string{"--json"},
	},
	"child status": {
		values:   []string{"--parent", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--repair", "--json"},
	},
	"child list": {
		values:   []string{"--parent", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"child accept": {
		values:     []string{"--parent", "--child", "--evidence", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--evidence"},
		booleans:   []string{"--json"},
	},
	"child reject": {
		values:   []string{"--parent", "--child", "--reason", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"child drop": {
		values:   []string{"--parent", "--child", "--reason", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"intent record": {
		values:     []string{"--id", "--raw-request", "--interpreted-intent", "--success-criteria", "--constraint", "--ambiguity", "--non-goal", "--intent-class", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--success-criteria", "--constraint", "--ambiguity", "--non-goal"},
		booleans:   []string{"--json"},
	},
	"domain-review record": {
		values:     []string{"--id", "--model-fit", "--terminology", "--risk", "--uncertainty", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--terminology", "--risk", "--uncertainty"},
		booleans:   []string{"--json"},
	},
	"design review": {
		values:     []string{"--id", "--problem-summary", "--proposed-design", "--refactor-plan", "--alternative", "--risk", "--verification", "--open-question", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--alternative", "--risk", "--verification", "--open-question"},
		booleans:   []string{"--approved", "--json"},
	},
	"plan-prep record": {
		values:     []string{"--id", "--decisions-evidence", "--decisions-waive", "--related-score-ref", "--related-waive", "--web-research-evidence", "--web-research-waive", "--codebase-survey-evidence", "--codebase-survey-waive", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--decisions-evidence", "--related-score-ref", "--web-research-evidence", "--codebase-survey-evidence"},
		booleans:   []string{"--json"},
	},
	"regress": {
		values:   []string{"--id", "--reason", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"execution reconcile": {
		values:   []string{"--id", "--operation-id", "--issue-snapshot-file", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--preview", "--confirm", "--json"},
	},
	"execution complete": {
		values:     []string{"--id", "--generation", "--final-head", "--verification-report", "--remote-artifact-url", "--verification", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		repeatable: []string{"--verification"},
		booleans:   []string{"--confirm", "--json"},
	},
	"execution sync-base": {
		values:   []string{"--id", "--completion-generation", "--fingerprint", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--preview", "--apply", "--finalize", "--abort", "--confirm", "--json"},
	},
	"execution switch-mode": {
		values:   []string{"--id", "--mode", "--fingerprint", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd"},
		booleans: []string{"--apply", "--confirm", "--json"},
	},
	"link-plan": {
		values:   []string{"--id", "--plan-path", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"link-worktree": {
		values:   []string{"--id", "--worktree-path", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"link-child": {
		values:   []string{"--id", "--child-url", "--title", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"link-related": {
		values:   []string{"--id", "--type", "--related-url", "--title", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"feedback add": {
		values:   []string{"--id", "--source", "--body", "--classification", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"compatibility review": {
		values:     []string{"--id", "--host", "--session-id", "--agent-id", "--cwd", "--backward-compatibility", "--side-effect", "--rollback-plan", "--verification", "--blocker"},
		repeatable: []string{"--backward-compatibility", "--side-effect", "--verification", "--blocker"},
		booleans:   []string{"--approved", "--json"},
	},
	"devils-advocate review": {
		values:     []string{"--id", "--host", "--session-id", "--agent-id", "--cwd", "--verdict", "--reviewer-context", "--finding", "--waiver-rationale"},
		repeatable: []string{"--finding"},
		booleans:   []string{"--waive", "--json"},
	},
	"phase": {
		values:   []string{"--id", "--to", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--force", "--json"},
	},
	"decision add": {
		values:     []string{"--id", "--host", "--session-id", "--agent-id", "--cwd", "--title", "--body", "--kind", "--rationale", "--alternative", "--affected-link", "--affected-artifact"},
		repeatable: []string{"--alternative", "--affected-link", "--affected-artifact"},
		booleans:   []string{"--json"},
	},
	"ai-slop-clean record": {
		values:     []string{"--id", "--host", "--session-id", "--agent-id", "--cwd", "--category", "--verification"},
		repeatable: []string{"--category", "--verification"},
		booleans:   []string{"--json"},
	},
	"feedback mark-issue-updated": {
		values:   []string{"--id", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"feedback resolve": {
		values:   []string{"--id", "--host", "--session-id", "--agent-id", "--cwd", "--index", "--resolution"},
		booleans: []string{"--json"},
	},
	"remote create-pr": {
		values:     []string{"--id", "--expected-generation", "--title", "--body", "--body-file", "--template", "--provider", "--score-file", "--head", "--base", "--host", "--session-id", "--agent-id", "--session-pid", "--session-started-at", "--session-executable", "--cwd", "--label", "--assignee", "--field"},
		repeatable: []string{"--label", "--assignee", "--field"},
		booleans:   []string{"--confirm", "--json"},
	},
	"remote create-child": {
		values:     []string{"--id", "--title", "--body", "--body-file", "--template", "--provider", "--score-file", "--host", "--session-id", "--agent-id", "--cwd", "--label", "--assignee", "--field"},
		repeatable: []string{"--label", "--assignee", "--field"},
		booleans:   []string{"--confirm", "--json"},
	},
	"remote verify-artifact": {
		values:     []string{"--id", "--provider", "--kind", "--url", "--target-branch", "--label", "--labels", "--assignee", "--assignees", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--label", "--labels", "--assignee", "--assignees"},
		booleans:   []string{"--json"},
	},
	"remote score": {
		values:   []string{"--input", "--judge", "--judge-file"},
		booleans: []string{"--json"},
	},
	"implementation-review record": {
		values:     []string{"--id", "--verdict", "--finding", "--evidence", "--reviewer-host", "--reviewer-model", "--reviewer-effort", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--finding", "--evidence"},
		booleans:   []string{"--json"},
	},
	"project-docs-review record": {
		values:     []string{"--id", "--verdict", "--doc", "--evidence", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--doc", "--evidence"},
		booleans:   []string{"--json"},
	},
	"schema-evidence record": {
		values:     []string{"--id", "--measurement", "--source", "--waiver-rationale", "--host", "--session-id", "--agent-id", "--cwd"},
		repeatable: []string{"--measurement", "--source"},
		booleans:   []string{"--json", "--waive"},
	},
	"artifact unstage": {
		values:   []string{"--id", "--name"},
		booleans: []string{"--json"},
	},
	"artifact stage": {
		values:   []string{"--id", "--name", "--file", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"branch await-link": {
		values:   []string{"--id", "--timeout", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"branch retarget": {
		values:   []string{"--id", "--base-branch", "--reason", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--json"},
	},
	"cleanup status": {
		values:   []string{"--id", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--merged", "--json"},
	},
	"cleanup close-children": {
		values:   []string{"--id", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--merged", "--confirm", "--json"},
	},
	"cleanup orphan": {
		values:   []string{"--id", "--repo", "--worktree", "--branch", "--provider", "--kind", "--artifact-url", "--fingerprint", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--apply", "--confirm", "--json"},
	},
	"cleanup finish": {
		values:   []string{"--id", "--provider", "--fingerprint", "--superseded-by"},
		booleans: []string{"--preview", "--apply", "--confirm", "--keep-remote-branch", "--json"},
	},
	"cleanup remote-branch": {
		values:   []string{"--id", "--fingerprint", "--superseded-by"},
		booleans: []string{"--preview", "--apply", "--confirm", "--json"},
	},
	"cleanup linked-branch": {
		values:   []string{"--id", "--fingerprint"},
		booleans: []string{"--preview", "--apply", "--confirm", "--json"},
	},
	"cleanup abandon": {
		values: []string{"--id", "--reason", "--fingerprint"},
		booleans: []string{
			"--preview", "--apply", "--confirm", "--json",
			"--close-pr", "--close-issue", "--delete-remote-branch",
		},
	},
	"remote reflect-completion": {
		values:   []string{"--id", "--provider"},
		booleans: []string{"--confirm", "--json"},
	},
	"remote reflect-devils-advocate": {
		values:   []string{"--id", "--provider", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--confirm", "--json"},
	},
	"remote close-issue": {
		values:   []string{"--id", "--provider"},
		booleans: []string{"--confirm", "--json"},
	},
	"remote sync-issue": {
		values:   []string{"--id", "--provider", "--url", "--body", "--body-file", "--expected-body-sha256", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--accept-remote-edits", "--confirm", "--json"},
	},
	"remote sync-pr": {
		values:   []string{"--id", "--provider", "--expected-generation", "--body", "--body-file", "--expected-body-sha256", "--host", "--session-id", "--agent-id", "--cwd"},
		booleans: []string{"--accept-remote-edits", "--confirm", "--json"},
	},
}

// IssueOpsCommandSpec은 정확한 issueops subcommand path에 대한 (values,
// booleans, repeatable, ok) flag spec을 반환한다. 알 수 없는 path면 ok는 false다.
func IssueOpsCommandSpec(path string) (map[string]bool, map[string]bool, map[string]bool, bool) {
	spec, ok := issueOpsCommandSpecs[path]
	if !ok {
		return nil, nil, nil, false
	}
	return toBoolMap(spec.values), toBoolMap(spec.booleans), toBoolMap(spec.repeatable), true
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
