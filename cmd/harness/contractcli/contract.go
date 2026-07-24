package contractcli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/cmd/harness/selfworkflow"
	cliadapter "agent-harness/internal/adapter/cli"
	mcpadapter "agent-harness/internal/adapter/mcp"
)

var MCPTools = func() []map[string]any {
	return mcpcli.MCPTools()
}

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

func Run(args []string) error {
	if len(args) == 0 {
		contractUsage()
		return fmt.Errorf("missing contract subcommand")
	}
	switch args[0] {
	case "schema":
		return runContractSchema(args[1:])
	case "check":
		return runContractCheck(args[1:])
	case "conformance":
		return runConformance(args[1:])
	default:
		contractUsage()
		return fmt.Errorf("unknown contract subcommand %q", args[0])
	}
}

func contractUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
	  agent-harness contract schema [--json]
	  agent-harness contract check [--json]
	  agent-harness contract conformance baseline|live|replay|serve [flags]
`)
}

func runContractSchema(args []string) error {
	fs := flag.NewFlagSet("contract schema", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	contract := BuildCompatibilityContract()
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
	contract := BuildCompatibilityContract()
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

func BuildCompatibilityContract() CompatibilityContract {
	toolNames := []string{}
	for _, tool := range MCPTools() {
		if name, ok := tool["name"].(string); ok {
			toolNames = append(toolNames, name)
		}
	}
	sort.Strings(toolNames)
	contract := CompatibilityContract{
		OK:          true,
		Name:        "agent_harness_cli_mcp_compatibility",
		Version:     3,
		CLICommands: cliadapter.Commands(),
		MCPTools:    toolNames,
		ResponseFields: map[string][]string{
			"self_verification_summary":       selfworkflow.BuildSelfVerificationContract().RequiredFields,
			"harness_doctor":                  {"ok", "healthy", "kind", "repo_root", "state_dir", "lifecycle_state", "pipe_capacity_bytes", "checks", "issues"},
			"harness_status":                  {"ok", "kind", "version", "repo", "inspect", "doctor", "daemon", "state", "workers", "self_verify", "warnings"},
			"command_policy":                  {"ok", "allowed", "audit_log_id", "workspace_root", "cwd", "argv", "tier", "deny_reasons", "warnings"},
			"command_run":                     {"ok", "executed", "exit_code", "read_only", "policy", "stdout", "stderr", "error"},
			"guard_check":                     {"ok", "repo_root", "mode", "checked_files", "findings", "summary"},
			"trace_analysis":                  {"ok", "kind", "input", "input_source", "trace_types", "finding_count", "findings", "warnings"},
			"tool_conformance_report":         {"ok", "schema_version", "run_id", "profile", "case_count", "counts", "gate", "hosts", "warnings", "evidence"},
			"worker_job":                      {"ok", "id", "kind", "status", "created_at", "updated_at", "no_shell"},
			"issueops_record":                 {"ok", "schema_version", "id", "repo", "branch", "phase", "intent", "design_review", "domain_review", "issue_url", "plan_path", "worktree_path", "issue_links", "branch_prepare", "remote_artifact", "decisions", "plan_prep", "compatibility_review", "devils_advocate_review", "feedback", "regress_events", "delegation", "child_cycles", "execution", "routing_trace", "ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "ai_slop_clean_categories", "ai_slop_clean_verification", "phase_ledger", "created_at", "updated_at"},
			"issueops_execution":              {"ok", "id", "execution", "next_command"},
			"issueops_execution_prepare":      {"ok", "id", "preview", "requested_mode", "resolved_mode", "fallback_code", "workspace", "execution", "claim_token_path", "issue_body_sha256", "context_packet_path", "context_packet_sha256", "owner_prompt_path", "owner_prompt_sha256", "next_command"},
			"issueops_execution_replace":      {"ok", "id", "action", "execution", "inventory_fingerprint", "quiescence_fingerprint", "claim_token_path", "next_command"},
			"issueops_execution_reconcile":    {"ok", "id", "preview", "reconciled", "code", "execution", "pending"},
			"issueops_pr_readiness":           {"ok", "ready", "missing", "issue_url", "plan_path", "branch"},
			"issueops_cleanup_status":         {"ok", "ready", "id", "merged", "missing", "warnings", "choices", "worktree_path", "branch", "remote_artifact_url"},
			"issueops_cleanup_close_children": {"ok", "id", "merged", "confirmed", "dry_run", "closed_count", "children", "missing"},
			"issueops_remote_score":           {"ok", "provider", "threshold", "selected_related_issues", "rejected_related_issues", "selected_labels", "rejected_labels", "apply_instructions", "warnings"},
			"issueops_remote_render_template": {"kind", "template", "provider", "title", "body", "warnings", "missing_required_fields", "validation"},
			"issueops_remote_create_issue":    {"ok", "provider", "issue_url", "issue_number", "labels", "assignees", "preview"},
			"issueops_remote_create_child":    {"ok", "provider", "child_url", "child_number", "hierarchy_verified", "labels", "assignees", "preview"},
			"issueops_remote_create_pr":       {"ok", "provider", "url", "number", "target_branch", "labels", "assignees", "preview"},
			"issueops_benchmark_run":          {"ok", "id", "fixture_count", "average_score", "minimum_score", "critical_failure_count", "scores"},
			"issueops_benchmark_compare":      {"ok", "improved", "baseline_id", "candidate_id", "average_score_delta", "minimum_score_delta", "critical_failure_delta", "regressions"},
			"issueops_benchmark_gate":         {"ok", "keep_candidate", "candidate_id", "benchmark_compare", "edit_surface_violations", "target_dimension_regressions", "discard_reasons"},
			"command_audit":                   {"ok", "kind", "audit_log_id", "log_path", "policy"},
			"verify_work":                     {"ok", "kind", "repo", "git_status", "preflight", "guard", "command", "evidence", "evidence_matrix", "suggested_commands", "warnings"},
			"web_fetch":                       {"ok", "url", "final_url", "category", "stop_reason", "grid_exhausted", "attempted_routes", "untried_routes", "content", "metadata", "warnings", "retrieved_at", "duration_ms"},
		},
		Warnings:     []string{},
		AdapterTools: mcpadapter.AdapterOwnedTools(),
		Verification: []string{"go test ./... -count=1", "go test ./cmd/harness/contractgolden ./cmd/harness/harnessapp -run Golden -count=1", "agent-harness contract conformance baseline --json", "agent-harness contract check --json"},
	}
	for _, want := range []string{"contract_schema", "worker_enqueue", "command_fake_run"} {
		if !containsString(toolNames, want) {
			contract.OK = false
			contract.Warnings = append(contract.Warnings, "missing_mcp_tool:"+want)
		}
	}
	issueOpsTools := make([]string, 0, 1)
	for _, name := range toolNames {
		if strings.HasPrefix(name, "issueops_") {
			issueOpsTools = append(issueOpsTools, name)
		}
	}
	if len(issueOpsTools) != 1 || issueOpsTools[0] != "issueops_execution" {
		contract.OK = false
		contract.Warnings = append(contract.Warnings, "issueops_mcp_surface_mismatch:"+strings.Join(issueOpsTools, ","))
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

func printJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
