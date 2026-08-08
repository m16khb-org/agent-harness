package commandguard

import (
	"strings"

	"agent-harness/internal/domain/commandparse"
	"agent-harness/internal/domain/searchrouting"
)

type KubectlLiveAccessKind string

const (
	KubectlLiveAccessNone         KubectlLiveAccessKind = ""
	KubectlLiveAccessPortForward  KubectlLiveAccessKind = "port_forward"
	KubectlLiveAccessReadOnlyExec KubectlLiveAccessKind = "readonly_exec"
	KubectlLiveAccessUnsafeExec   KubectlLiveAccessKind = "unsafe_exec"
)

const (
	kubectlLiveAccessReason = "kubectl live cluster access requires explicit user confirmation: exec and port-forward can expose live workloads or local ports. Confirm before running this command."
	kubectlMutationReason   = "GitOps is the source of truth for cluster changes: do not run direct mutating kubectl commands from the agent. Edit Kubernetes manifests in git and use the repo's GitOps review/apply path instead."
)

type KubectlExecScope struct {
	Context   string
	Namespace string
}

type KubectlEvaluation struct {
	Decision   string
	Reason     string
	LiveAccess KubectlLiveAccessKind
	ExecScope  KubectlExecScope
}

func EvaluateGitOpsKubectl(tool, command string) KubectlEvaluation {
	if !searchrouting.IsShellTool(tool) {
		return KubectlEvaluation{}
	}
	tokens := commandparse.SplitCommandTokens(command)
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "kubectl" {
			continue
		}
		verb, subverb := kubectlVerb(tokens[i+1:])
		if kubectlMutationBlocked(verb, subverb, tokens[i+1:]) {
			return KubectlEvaluation{Decision: "block", Reason: kubectlMutationReason}
		}
	}
	for i, token := range tokens {
		if searchrouting.SearchTokenName(token) != "kubectl" {
			continue
		}
		verb, _ := kubectlVerb(tokens[i+1:])
		switch verb {
		case "exec":
			scope, ok := parseReadOnlyKubectlExec(command)
			kind := KubectlLiveAccessUnsafeExec
			if ok {
				kind = KubectlLiveAccessReadOnlyExec
			}
			return KubectlEvaluation{Decision: "ask", Reason: kubectlLiveAccessReason, LiveAccess: kind, ExecScope: scope}
		case "port-forward":
			return KubectlEvaluation{Decision: "ask", Reason: kubectlLiveAccessReason, LiveAccess: KubectlLiveAccessPortForward}
		}
	}
	return KubectlEvaluation{}
}

func parseReadOnlyKubectlExec(command string) (KubectlExecScope, bool) {
	if unsafeKubectlExecShell(command) {
		return KubectlExecScope{}, false
	}
	tokens := commandparse.SplitCommandTokens(command)
	if len(tokens) < 2 || tokens[0] != "kubectl" {
		return KubectlExecScope{}, false
	}
	separator := -1
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == "--" {
			separator = i
			break
		}
	}
	if separator == -1 || separator == len(tokens)-1 {
		return KubectlExecScope{}, false
	}

	var scope KubectlExecScope
	seenContext := false
	seenNamespace := false
	seenContainer := false
	seenExec := false
	target := ""
	for i := 1; i < separator; i++ {
		arg := tokens[i]
		if strings.HasPrefix(arg, "-") {
			name, value, inline := splitKubectlApprovalFlag(arg)
			switch name {
			case "context":
				if seenContext {
					return KubectlExecScope{}, false
				}
				if !inline {
					i++
					if i >= separator {
						return KubectlExecScope{}, false
					}
					value = tokens[i]
				}
				if invalidFlagValue(value) {
					return KubectlExecScope{}, false
				}
				scope.Context = value
				seenContext = true
			case "n", "namespace":
				if seenNamespace {
					return KubectlExecScope{}, false
				}
				if !inline {
					i++
					if i >= separator {
						return KubectlExecScope{}, false
					}
					value = tokens[i]
				}
				if invalidFlagValue(value) {
					return KubectlExecScope{}, false
				}
				scope.Namespace = value
				seenNamespace = true
			case "c", "container":
				if !seenExec || seenContainer {
					return KubectlExecScope{}, false
				}
				if !inline {
					i++
					if i >= separator {
						return KubectlExecScope{}, false
					}
					value = tokens[i]
				}
				if invalidFlagValue(value) {
					return KubectlExecScope{}, false
				}
				seenContainer = true
			default:
				return KubectlExecScope{}, false
			}
			continue
		}
		if !seenExec {
			if arg != "exec" {
				return KubectlExecScope{}, false
			}
			seenExec = true
			continue
		}
		if target != "" {
			return KubectlExecScope{}, false
		}
		target = arg
	}
	if !seenExec || target == "" || !seenContext || !seenNamespace {
		return KubectlExecScope{}, false
	}
	if !readOnlyRemoteArgv(tokens[separator+1:]) {
		return KubectlExecScope{}, false
	}
	return scope, true
}

func unsafeKubectlExecShell(command string) bool {
	return commandparse.HasUnquotedControlOperator(command) ||
		commandparse.HasActiveCommandSubstitution(command) ||
		commandparse.HasActiveInputRedirect(command) ||
		commandparse.HasActiveOutputRedirect(command) ||
		commandparse.HasActiveParameterOrTildeExpansion(command) ||
		commandparse.HasActivePathnameExpansion(command) ||
		commandparse.HasActiveShellSpecialQuoting(command) ||
		commandparse.HasActiveZshEqualsExpansion(command)
}

func splitKubectlApprovalFlag(arg string) (name, value string, inline bool) {
	name = strings.TrimLeft(strings.TrimSpace(arg), "-")
	if cut, rest, ok := strings.Cut(name, "="); ok {
		return cut, rest, true
	}
	return name, "", false
}

func invalidFlagValue(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.HasPrefix(value, "-")
}

func readOnlyRemoteArgv(args []string) bool {
	switch {
	case len(args) == 3 && args[0] == "getent" && args[1] == "hosts":
		return validApprovalDNSName(args[2])
	case len(args) == 2 && args[0] == "nslookup":
		return validApprovalDNSName(args[1])
	case len(args) == 2 && args[0] == "dig":
		return validApprovalDNSName(args[1])
	case len(args) == 3 && args[0] == "dig" && args[1] == "+short":
		return validApprovalDNSName(args[2])
	case len(args) == 3 && args[0] == "dig" && (args[2] == "A" || args[2] == "AAAA"):
		return validApprovalDNSName(args[1])
	case len(args) == 4 && args[0] == "dig" && args[1] == "+short" && (args[3] == "A" || args[3] == "AAAA"):
		return validApprovalDNSName(args[2])
	case len(args) == 2 && args[0] == "cat" && args[1] == "/etc/resolv.conf":
		return true
	case len(args) == 3 && args[0] == "curl" && args[1] == "-fsS":
		return args[2] == "http://localhost:4191/metrics" || args[2] == "http://127.0.0.1:4191/metrics"
	default:
		return false
	}
}

func validApprovalDNSName(value string) bool {
	if value == "" || len(value) > 253 {
		return false
	}
	value = strings.TrimSuffix(value, ".")
	if value == "" {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || !asciiAlphaNumeric(label[0]) || !asciiAlphaNumeric(label[len(label)-1]) {
			return false
		}
		for i := 1; i < len(label)-1; i++ {
			if !asciiAlphaNumeric(label[i]) && label[i] != '-' {
				return false
			}
		}
	}
	return true
}

func asciiAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
