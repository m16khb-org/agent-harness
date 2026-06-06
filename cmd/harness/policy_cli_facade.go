package main

import (
	"agent-harness/cmd/harness/policycli"
	"agent-harness/internal/core"
)

func init() {
	policycli.ResolveTarget = resolveTarget
}

func runPolicy(args []string) error {
	return policycli.Run(args)
}

func runPolicyCheck(args []string) error {
	return policycli.RunCheck(args)
}

func runPolicyFakeRun(args []string) error {
	return policycli.RunFakeRun(args)
}

func runPolicyRun(args []string) error {
	return policycli.RunReadOnly(args)
}

func runPolicyAudit(args []string) error {
	return policycli.RunAudit(args)
}

func parseCommandPolicyFlags(name string, args []string) (core.CommandPolicyRequest, bool, error) {
	return policycli.ParseFlags(name, args)
}

func parseCommandPolicyRunFlags(args []string) (core.CommandPolicyRequest, bool, bool, error) {
	return policycli.ParseRunFlags(args)
}
