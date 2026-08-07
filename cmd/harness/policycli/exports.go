package policycli

import "agent-harness/internal/adapter/policy"

func Run(args []string) error {
	return runPolicy(args)
}

func RunCheck(args []string) error {
	return runPolicyCheck(args)
}

func RunFakeRun(args []string) error {
	return runPolicyFakeRun(args)
}

func RunReadOnly(args []string) error {
	return runPolicyRun(args)
}

func RunAudit(args []string) error {
	return runPolicyAudit(args)
}

func ParseFlags(name string, args []string) (policy.CommandPolicyRequest, bool, error) {
	return parseCommandPolicyFlags(name, args)
}

func ParseRunFlags(args []string) (policy.CommandPolicyRequest, bool, bool, error) {
	return parseCommandPolicyRunFlags(args)
}
