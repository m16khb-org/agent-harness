package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	cliadapter "agent-harness/internal/adapter/cli"
	mcpadapter "agent-harness/internal/adapter/mcp"
)

type CompatibilityContract struct {
	OK             bool                 `json:"ok"`
	Name           string               `json:"name"`
	Version        int                  `json:"version"`
	Hash           string               `json:"hash"`
	CLICommands    []cliadapter.Command `json:"cli_commands"`
	MCPTools       []string             `json:"mcp_tools"`
	ResponseFields map[string][]string  `json:"response_fields"`
	Warnings       []string             `json:"warnings"`
	AdapterTools   []mcpadapter.Tool    `json:"adapter_tools"`
	Verification   []string             `json:"verification"`
}

func runContract(args []string) error {
	if len(args) == 0 {
		contractUsage()
		return fmt.Errorf("missing contract subcommand")
	}
	switch args[0] {
	case "schema":
		return runContractSchema(args[1:])
	case "check":
		return runContractCheck(args[1:])
	default:
		contractUsage()
		return fmt.Errorf("unknown contract subcommand %q", args[0])
	}
}

func contractUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  agent-harness contract schema [--json]
  agent-harness contract check [--json]
`)
}

func runContractSchema(args []string) error {
	fs := flag.NewFlagSet("contract schema", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	contract := compatibilityContract()
	if *jsonOut {
		return printJSON(contract)
	}
	fmt.Printf("%s v%d %s\n", contract.Name, contract.Version, contract.Hash)
	for _, warning := range contract.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return nil
}

func runContractCheck(args []string) error {
	fs := flag.NewFlagSet("contract check", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	contract := compatibilityContract()
	if *jsonOut {
		return printJSON(contract)
	}
	if contract.OK {
		fmt.Printf("contract ok: %s\n", contract.Hash)
		return nil
	}
	for _, warning := range contract.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return fmt.Errorf("contract check failed")
}

func compatibilityContract() CompatibilityContract {
	toolNames := []string{}
	for _, tool := range mcpTools() {
		if name, ok := tool["name"].(string); ok {
			toolNames = append(toolNames, name)
		}
	}
	sort.Strings(toolNames)
	contract := CompatibilityContract{
		OK:          true,
		Name:        "agent_harness_cli_mcp_compatibility",
		Version:     1,
		CLICommands: cliadapter.Commands(),
		MCPTools:    toolNames,
		ResponseFields: map[string][]string{
			"self_verification_summary": selfVerificationContract().RequiredFields,
			"harness_doctor":            {"ok", "healthy", "kind", "repo_root", "state_dir", "lifecycle_state", "checks", "issues"},
			"harness_status":            {"ok", "kind", "version", "repo", "inspect", "doctor", "daemon", "state", "workers", "self_verify", "warnings"},
			"command_policy":            {"ok", "allowed", "audit_log_id", "workspace_root", "cwd", "argv", "tier", "deny_reasons", "warnings"},
			"command_run":               {"ok", "executed", "exit_code", "read_only", "policy", "stdout", "stderr", "error"},
			"guard_check":               {"ok", "repo_root", "mode", "checked_files", "findings", "summary"},
			"trace_analysis":            {"ok", "kind", "input", "input_source", "trace_types", "finding_count", "findings", "warnings"},
			"worker_job":                {"ok", "id", "kind", "status", "created_at", "updated_at", "no_shell"},
			"command_audit":             {"ok", "kind", "audit_log_id", "log_path", "policy"},
			"verify_work":               {"ok", "kind", "repo", "git_status", "preflight", "guard", "command", "evidence", "evidence_matrix", "suggested_commands", "warnings"},
		},
		Warnings:     []string{},
		AdapterTools: mcpadapter.AdapterOwnedTools(),
		Verification: []string{"go test ./... -count=1", "go test ./cmd/harness -run Golden -count=1", "harness contract check --json"},
	}
	for _, want := range []string{"contract_schema", "worker_enqueue", "command_fake_run"} {
		if !containsString(toolNames, want) {
			contract.OK = false
			contract.Warnings = append(contract.Warnings, "missing_mcp_tool:"+want)
		}
	}
	b, _ := json.Marshal(struct {
		Name           string               `json:"name"`
		Version        int                  `json:"version"`
		CLICommands    []cliadapter.Command `json:"cli_commands"`
		MCPTools       []string             `json:"mcp_tools"`
		ResponseFields map[string][]string  `json:"response_fields"`
	}{contract.Name, contract.Version, contract.CLICommands, contract.MCPTools, contract.ResponseFields})
	sum := sha256.Sum256(b)
	contract.Hash = hex.EncodeToString(sum[:])
	return contract
}
