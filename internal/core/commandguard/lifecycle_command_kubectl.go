package commandguard

import (
	"strings"

	"agent-harness/internal/core/searchrouting"
)

func GitOpsKubectlDecision(tool string, command string) (string, string) {
	result := EvaluateGitOpsKubectl(tool, command)
	return result.Decision, result.Reason
}

func kubectlVerb(args []string) (string, string) {
	verbIndex := -1
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if KubectlFlagTakesValue(arg) && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		verbIndex = i
		break
	}
	if verbIndex == -1 {
		return "", ""
	}
	verb := strings.ToLower(searchrouting.SearchTokenName(args[verbIndex]))
	subverb := ""
	for _, arg := range args[verbIndex+1:] {
		arg = strings.TrimSpace(arg)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		if isShellSeparator(arg) {
			break
		}
		subverb = strings.ToLower(searchrouting.SearchTokenName(arg))
		break
	}
	return verb, subverb
}

func KubectlFlagTakesValue(flag string) bool {
	name := strings.TrimLeft(strings.TrimSpace(flag), "-")
	if cut, _, ok := strings.Cut(name, "="); ok {
		name = cut
	}
	switch name {
	case "n", "namespace", "context", "kubeconfig", "server", "token", "as", "as-group", "user", "cluster", "request-timeout", "field-manager", "filename", "f", "output", "o", "selector", "l":
		return true
	default:
		return false
	}
}

func kubectlMutationBlocked(verb string, subverb string, args []string) bool {
	if verb == "" {
		return false
	}
	if kubectlDryRun(args) {
		return false
	}
	switch verb {
	case "apply", "annotate", "autoscale", "cordon", "create", "delete", "drain", "edit", "expose", "label", "patch", "replace", "run", "scale", "set", "taint", "uncordon":
		return true
	case "rollout":
		return subverb == "restart" || subverb == "undo" || subverb == "pause" || subverb == "resume"
	default:
		return false
	}
}

func kubectlLiveAccessNeedsConfirmation(verb string) bool {
	switch verb {
	case "exec", "port-forward":
		return true
	default:
		return false
	}
}

func kubectlDryRun(args []string) bool {
	for i, arg := range args {
		switch {
		case arg == "--dry-run=client", arg == "--dry-run=server":
			return true
		case arg == "--dry-run" && i+1 < len(args) && (args[i+1] == "client" || args[i+1] == "server"):
			return true
		}
	}
	return false
}

func isShellSeparator(token string) bool {
	switch token {
	case "&&", "||", ";", "|":
		return true
	default:
		return false
	}
}
